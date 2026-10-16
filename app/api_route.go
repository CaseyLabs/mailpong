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
	"github.com/caseylabs/mailven/internal/logging"
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
	// triggerIMAPSync replaces the scheduler request in disposable route tests.
	// Like Scheduler.TriggerSync, it reports no start/completion outcome.
	triggerIMAPSync func(accountID string)
	gmailBase       string
	graphBase       string
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
	onFailure := a.apiFailureHandler(id, route, ops)
	var err error
	switch route.Route {
	case account.MailRouteGmailAPI:
		acc, aerr := a.accountStore.Get(id)
		if aerr != nil {
			return aerr
		}
		_, err = a.newGmailSync(id, ops, tokens, onFailure, acc.SyncPeriodDays).Run(ctx)
	case account.MailRouteGraphAPI:
		_, err = (&sync.GraphSync{Engine: a.syncEngine, Reader: &graphapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.graphBase, Shared: sharedTarget(route)}, AccountID: id,
			Tokens: func() (*credentials.OAuthTokens, error) { return ops.validToken(id) }, OnFailure: onFailure}).Run(ctx)
	default:
		err = fmt.Errorf("route %q is not an API route", route.Route)
	}
	a.updateAPICounts(id)
	return err
}

// apiFailureHandler pauses the account's API route and raises the
// reauthorization event when the provider rejects the stored grant.
func (a *App) apiFailureHandler(id string, route *account.MailAPIRoute, ops *mailAPIOps) func(oauth2.MailAPIFailure, error) {
	return func(kind oauth2.MailAPIFailure, err error) {
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
}

// newGmailSync builds the Gmail driver for an account on its active route.
func (a *App) newGmailSync(id string, ops *mailAPIOps, tokens *apiTokenSource, onFailure func(oauth2.MailAPIFailure, error), syncPeriodDays int) *sync.GmailSync {
	return &sync.GmailSync{Engine: a.syncEngine, Reader: &gmailapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase}, AccountID: id,
		Tokens: func() (*credentials.OAuthTokens, error) { return ops.validToken(id) }, OnFailure: onFailure,
		SyncPeriodDays: syncPeriodDays}
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
// and, if it fails or the app quits during it, the account stays on IMAP
// (see recoverInterruptedAPIActivations).
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
	case route.Shared && !sharedMailboxLiveVerified:
		return errSharedMailboxGate
	}
	status, err := a.mailAPIOps().status(accountID)
	if err != nil {
		return err
	}
	if !status.Authorized {
		return fmt.Errorf("API authorization is not usable (%s); reauthorize first", status.Failure)
	}
	if route.Shared {
		if err := a.mailAPIOps().requireSharedCapabilities(accountID); err != nil {
			return err
		}
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
	// The mark outlives a quit mid-activation, so the next start can return
	// the account to IMAP. It is cleared once the outcome is settled.
	if err := a.accountStore.SetMailAPIActivating(accountID, true); err != nil {
		return err
	}
	// A mark left set only costs a needless return to IMAP at the next start,
	// so a failure to clear it is logged rather than failing the activation.
	settled := func() {
		if err := a.accountStore.SetMailAPIActivating(accountID, false); err != nil {
			log := logging.WithComponent("app")
			log.Error().Err(err).Str("account_id", accountID).
				Msg("Failed to clear the API activation mark; the next start returns the account to IMAP")
		}
	}
	// Going active first stops new IMAP syncs from starting; then check that
	// none is already running.
	if err := setState(account.MailRouteStateActive); err != nil {
		settled()
		return err
	}
	rs.setPaused(accountID, false)
	if a.imapSyncBusy(accountID) {
		_ = setState(account.MailRouteStateStaged)
		settled()
		return errors.New("an IMAP sync is in progress; try again when it finishes")
	}
	revert := func() {
		_ = setState(account.MailRouteStateStaged)
		// If the IMAP rows can't be restored, keep the mark so the next
		// start retries the return to IMAP.
		if err := a.stopAccountIdle(accountID, func() error {
			_, err := a.folderStore.SwapHidden(accountID, folder.RowsAPI, folder.RowsIMAP)
			return err
		}); err == nil {
			settled()
		}
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
	settled()
	a.refreshAPIAccount(accountID)
	a.kickAPIActions(accountID)
	return nil
}

// recoverInterruptedAPIActivations returns to IMAP every account whose API
// activation never settled because the app quit during it. It runs at startup
// before IDLE and the sync scheduler, so nothing resumes the half-done
// reconcile. Each step is safe to repeat if this is interrupted too: the IMAP
// rows are restored only while they are still hidden.
func (a *App) recoverInterruptedAPIActivations() {
	log := logging.WithComponent("app")
	ids, err := a.accountStore.ListActivatingMailAPIRoutes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to check for interrupted API activations")
		return
	}
	for _, id := range ids {
		if err := a.revertInterruptedAPIActivation(id); err != nil {
			log.Error().Err(err).Str("account_id", id).Msg("Failed to return an interrupted API activation to IMAP")
			continue
		}
		log.Warn().Str("account_id", id).Msg("API activation was interrupted; the account stays on IMAP")
	}
}

func (a *App) revertInterruptedAPIActivation(accountID string) error {
	hidden, err := a.folderStore.ListHidden(accountID, folder.RowsIMAP)
	if err != nil {
		return err
	}
	if len(hidden) > 0 {
		if _, err := a.folderStore.SwapHidden(accountID, folder.RowsAPI, folder.RowsIMAP); err != nil {
			return err
		}
	}
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return err
	}
	if route != nil && route.State != account.MailRouteStateStaged {
		route.State = account.MailRouteStateStaged
		if err := a.accountStore.SetMailAPIRoute(*route); err != nil {
			return err
		}
	}
	return a.accountStore.SetMailAPIActivating(accountID, false)
}

// DeactivateMailAPIRoute returns an active API route to IMAP. The API rows and
// queued actions are kept (hidden, never mixed in) until the API data is
// cleaned up; the route stays staged so it can be activated again.
func (a *App) DeactivateMailAPIRoute(accountID string) error {
	a.apiRoutes.transition.Lock()
	defer a.apiRoutes.transition.Unlock()
	_, err := a.deactivateAPIRoute(accountID)
	return err
}

// deactivateAPIRoute returns whether IMAP sync was requested, not completed.
func (a *App) deactivateAPIRoute(accountID string) (bool, error) {
	rs := &a.apiRoutes
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	switch {
	case err != nil:
		return false, err
	case route == nil || route.State != account.MailRouteStateActive:
		return false, errors.New("API route is not active")
	}
	next := *route
	next.State = account.MailRouteStateStaged
	if err := a.accountStore.SetMailAPIRoute(next); err != nil {
		return false, err
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
		return false, fmt.Errorf("failed to switch folders: %w", err)
	}
	a.startAccountIdle(accountID)
	a.refreshAPIAccount(accountID)
	if rs.triggerIMAPSync != nil {
		rs.triggerIMAPSync(accountID)
		return true, nil
	}
	if a.syncScheduler != nil {
		a.syncScheduler.TriggerSync(accountID)
		return true, nil
	}
	return false, nil
}
