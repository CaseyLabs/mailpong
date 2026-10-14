package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/sync"
)

// errAPIPaused is returned for an API-routed account whose grant needs
// attention. There is no IMAP fallback: sync and queue wait for reauthorization.
var errAPIPaused = errors.New("API authorization needs attention; sync is paused until you reauthorize")

// apiRouteState serializes route transitions and API syncs per account.
type apiRouteState struct {
	transition gosync.Mutex // one activate/deactivate at a time
	mu         gosync.Mutex
	paused     map[string]bool
	locks      map[string]*gosync.Mutex
	cancels    map[string]context.CancelFunc

	// Test seams: a fake OAuth client registry and provider endpoints.
	provider func(string) (oauth2.ProviderConfig, error)
	http     *http.Client
	// migrationStep, if set, runs before each migration step (test seam).
	migrationStep func(step string) error
	gmailBase     string
	graphBase     string
}

func (s *apiRouteState) lock(accountID string) *gosync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks == nil {
		s.locks = map[string]*gosync.Mutex{}
	}
	if s.locks[accountID] == nil {
		s.locks[accountID] = &gosync.Mutex{}
	}
	return s.locks[accountID]
}

func (s *apiRouteState) setPaused(accountID string, paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused == nil {
		s.paused = map[string]bool{}
	}
	s.paused[accountID] = paused
}

func (s *apiRouteState) isPaused(accountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused[accountID]
}

// apiPaused reports whether an API-routed account's sync and queue are paused:
// a reauthorization event was raised, or the stored grant no longer passes the
// read/write/send gates. Local data is untouched either way.
func (a *App) apiPaused(accountID string) bool {
	if a.apiRoutes.isPaused(accountID) {
		return true
	}
	status, err := a.mailAPIOps().status(accountID)
	return err != nil || !status.Authorized
}

// syncAPIAccount is the scheduler hook: it syncs an account whose route is
// active through its API and reports handled=false for every other account.
func (a *App) syncAPIAccount(ctx context.Context, acc *account.Account) (bool, error) {
	route := a.apiRouted(acc.ID)
	if route == nil {
		return false, nil
	}
	return true, a.runAPISync(ctx, route, false)
}

// runAPISync runs one GmailSync/GraphSync pass. With wait false it skips when
// another pass for the account is running.
func (a *App) runAPISync(ctx context.Context, route *account.MailAPIRoute, wait bool) error {
	id := route.AccountID
	if route.Shared {
		return errors.New("shared mailbox sync is not supported yet")
	}
	if a.apiPaused(id) {
		return errAPIPaused
	}
	mu := a.apiRoutes.lock(id)
	if wait {
		mu.Lock()
	} else if !mu.TryLock() {
		return nil
	}
	defer mu.Unlock()
	if a.apiRouted(id) == nil {
		return nil // deactivated while waiting
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	a.apiRoutes.mu.Lock()
	if a.apiRoutes.cancels == nil {
		a.apiRoutes.cancels = map[string]context.CancelFunc{}
	}
	a.apiRoutes.cancels[id] = cancel
	a.apiRoutes.mu.Unlock()
	defer func() {
		a.apiRoutes.mu.Lock()
		delete(a.apiRoutes.cancels, id)
		a.apiRoutes.mu.Unlock()
	}()

	ops := a.mailAPIOps()
	tokens := &apiTokenSource{ops: ops, accountID: id}
	onFailure := func(kind oauth2.MailAPIFailure, err error) {
		if kind != oauth2.MailAPIFailureReauthorize && kind != oauth2.MailAPIFailurePartialConsent {
			return
		}
		a.apiRoutes.setPaused(id, true)
		// refreshFailed already raised the event for a rejected refresh grant.
		if status, serr := ops.status(id); serr == nil && status.Authorized {
			eventsEmit(a.ctx, "mail-api:reauth-required", map[string]any{
				"accountId": id, "route": route.Route, "failure": string(kind), "error": err.Error()})
		}
	}
	var err error
	switch route.Route {
	case account.MailRouteGmailAPI:
		_, err = (&sync.GmailSync{Engine: a.syncEngine, Reader: &gmailapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase}, AccountID: id,
			Tokens: func() (*credentials.OAuthTokens, error) { return ops.validToken(id) }, OnFailure: onFailure}).Run(ctx)
	case account.MailRouteGraphAPI:
		_, err = (&sync.GraphSync{Engine: a.syncEngine, Reader: &graphapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.graphBase}, AccountID: id,
			Tokens: func() (*credentials.OAuthTokens, error) { return ops.validToken(id) }, OnFailure: onFailure}).Run(ctx)
	default:
		err = fmt.Errorf("route %q is not an API route", route.Route)
	}
	a.updateAPICounts(id)
	return err
}

// updateAPICounts refreshes the visible folders' counts and last-sync time,
// which the API drivers do not maintain.
func (a *App) updateAPICounts(accountID string) {
	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return
	}
	counts := map[string]int{}
	for _, f := range folders {
		total, terr := a.messageStore.CountByFolder(f.ID)
		unread, uerr := a.messageStore.CountUnreadByFolder(f.ID)
		if terr != nil || uerr != nil {
			continue
		}
		if a.folderStore.UpdateCounts(f.ID, total, unread) == nil {
			counts[f.ID] = unread
		}
		_ = a.folderStore.TouchSync(f.ID)
	}
	if len(counts) > 0 {
		eventsEmit(a.ctx, "folders:countsChanged", counts)
	}
}

// syncRoutedAccount is the manual-sync path for an API-routed account. It
// reports through the usual sync events and never touches IMAP.
func (a *App) syncRoutedAccount(route *account.MailAPIRoute, folderID string) error {
	err := a.runAPISync(a.ctx, route, false)
	if err != nil {
		a.emitSyncError(route.AccountID, folderID, err.Error())
		return err
	}
	a.emitFolderSynced(route.AccountID, folderID, true)
	return nil
}

// imapSyncBusy reports whether an IMAP sync is running for the account.
func (a *App) imapSyncBusy(accountID string) bool {
	if a.syncScheduler != nil && a.syncScheduler.IsSyncing(accountID) {
		return true
	}
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	for key := range a.syncContexts {
		if strings.HasPrefix(key, accountID+":") {
			return true
		}
	}
	return false
}

// ActivateMailAPIRoute moves an account with a staged API route onto it. The
// grant must pass the read/write/send gates and no IMAP sync or in-flight queue
// action may exist. IMAP data, credentials, drafts and queued mail are kept;
// the IMAP folder rows are hidden, not deleted. One API reconcile runs first
// and, if it fails, the account stays on IMAP.
func (a *App) ActivateMailAPIRoute(accountID string) error {
	a.apiRoutes.transition.Lock()
	defer a.apiRoutes.transition.Unlock()
	return a.activateAPIRoute(accountID)
}

// activateAPIRoute is ActivateMailAPIRoute with the transition lock held.
func (a *App) activateAPIRoute(accountID string) error {
	rs := &a.apiRoutes
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	switch {
	case err != nil:
		return err
	case route == nil:
		return errors.New("account has no API route to activate")
	case route.State == account.MailRouteStateActive:
		return errors.New("API route is already active")
	case route.Shared:
		return errors.New("shared mailbox routes cannot be activated yet")
	}
	status, err := a.mailAPIOps().status(accountID)
	if err != nil {
		return err
	}
	if !status.Authorized {
		return fmt.Errorf("API authorization is not usable (%s); reauthorize first", status.Failure)
	}
	open, err := a.messageStore.ListAPIActions(accountID, message.APIActionInFlight, message.APIActionUnknown)
	if err != nil {
		return err
	}
	if len(open) > 0 {
		return errors.New("queued API actions are still in flight")
	}

	setState := func(state string) error {
		next := *route
		next.State = state
		return a.accountStore.SetMailAPIRoute(next)
	}
	// Going active first stops new IMAP syncs from starting; then check that
	// none is already running.
	if err := setState(account.MailRouteStateActive); err != nil {
		return err
	}
	rs.setPaused(accountID, false)
	if a.imapSyncBusy(accountID) {
		_ = setState(account.MailRouteStateStaged)
		return errors.New("an IMAP sync is in progress; try again when it finishes")
	}
	revert := func() {
		_ = setState(account.MailRouteStateStaged)
		_ = a.stopAccountIdle(accountID, func() error {
			_, err := a.folderStore.SwapHidden(accountID, folder.RowsAPI, folder.RowsIMAP)
			return err
		})
		a.startAccountIdle(accountID)
		a.refreshAPIAccount(accountID)
	}
	if err := a.stopAccountIdle(accountID, func() error {
		_, err := a.folderStore.SwapHidden(accountID, folder.RowsIMAP, folder.RowsAPI)
		return err
	}); err != nil {
		revert()
		return fmt.Errorf("failed to switch folders: %w", err)
	}
	active := *route
	active.State = account.MailRouteStateActive
	if err := a.runAPISync(a.ctx, &active, true); err != nil {
		revert()
		return fmt.Errorf("API reconcile failed; account stays on IMAP: %w", err)
	}
	a.refreshAPIAccount(accountID)
	a.kickAPIActions(accountID)
	return nil
}

// DeactivateMailAPIRoute returns an active API route to IMAP. The API rows and
// queued actions are kept (hidden, never mixed in) until the API data is
// cleaned up; the route stays staged so it can be activated again.
func (a *App) DeactivateMailAPIRoute(accountID string) error {
	a.apiRoutes.transition.Lock()
	defer a.apiRoutes.transition.Unlock()
	return a.deactivateAPIRoute(accountID)
}

func (a *App) deactivateAPIRoute(accountID string) error {
	rs := &a.apiRoutes
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	switch {
	case err != nil:
		return err
	case route == nil || route.State != account.MailRouteStateActive:
		return errors.New("API route is not active")
	}
	next := *route
	next.State = account.MailRouteStateStaged
	if err := a.accountStore.SetMailAPIRoute(next); err != nil {
		return err
	}
	rs.mu.Lock()
	cancel := rs.cancels[accountID]
	rs.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// Wait for a running pass to stop.
	mu := rs.lock(accountID)
	mu.Lock()
	//nolint:staticcheck // taking and releasing the lock is the wait
	mu.Unlock()
	if err := a.stopAccountIdle(accountID, func() error {
		_, err := a.folderStore.SwapHidden(accountID, folder.RowsAPI, folder.RowsIMAP)
		return err
	}); err != nil {
		// Roll the state back so rows and route stay consistent.
		route.State = account.MailRouteStateActive
		_ = a.accountStore.SetMailAPIRoute(*route)
		return fmt.Errorf("failed to switch folders: %w", err)
	}
	a.startAccountIdle(accountID)
	a.refreshAPIAccount(accountID)
	if a.syncScheduler != nil {
		a.syncScheduler.TriggerSync(accountID)
	}
	return nil
}
