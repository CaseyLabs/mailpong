package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/apiactions"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
)

// gmailAllMailPath is the synthetic local folder of mail with no folder label.
const gmailAllMailPath = "[Gmail]/All Mail"

var gmailSystemLabels = []string{"INBOX", "SENT", "DRAFT", "TRASH", "SPAM"}

// apiActionState is the lazily built queue worker and its per-account timers.
type apiActionState struct {
	mu     sync.Mutex
	worker *apiactions.Worker
	timers map[string]*time.Timer
	dirty  map[string]bool
	busy   map[string]bool
}

// apiRouted returns the account's route when user actions go through its API,
// else nil (IMAP). Only an active route qualifies.
func (a *App) apiRouted(accountID string) *account.MailAPIRoute {
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	if err != nil || route == nil || route.State != account.MailRouteStateActive {
		return nil
	}
	return route
}

func apiProvider(route *account.MailAPIRoute) string {
	if route.Route == account.MailRouteGmailAPI {
		return message.APIProviderGmail
	}
	return message.APIProviderGraph
}

// apiActionWorker builds the worker on first use.
func (a *App) apiActionWorker() *apiactions.Worker {
	st := &a.apiActions
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.worker == nil {
		st.worker = &apiactions.Worker{
			Store:   a.messageStore,
			Remote:  a.apiRemote,
			Active:  func(id string) bool { return a.apiRouted(id) != nil && !a.apiPaused(id) },
			Emit:    func(event string, data any) { eventsEmit(a.ctx, event, data) },
			Changed: a.refreshAPIAccount,
		}
	}
	return st.worker
}

// refreshAPIAccount tells the UI to reload an account whose local state the
// worker changed (a reverted or removed message).
func (a *App) refreshAPIAccount(accountID string) {
	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return
	}
	for _, f := range folders {
		eventsEmit(a.ctx, "folder:synced", map[string]any{"accountId": accountID, "folderId": f.ID, "ok": true})
	}
}

// apiRemote builds the provider client for one account's queued action.
func (a *App) apiRemote(accountID, provider string) (apiactions.Remote, error) {
	route := a.apiRouted(accountID)
	if route == nil {
		return nil, errors.New("account is not routed through its API")
	}
	if route.Shared {
		return nil, errors.New("shared mailbox actions are not supported yet")
	}
	ops := a.mailAPIOps()
	grant, err := a.credStore.GetOAuthTokensForClientConfig(accountID, route.ClientConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API authorization: %w", err)
	}
	tokens := &apiTokenSource{ops: ops, accountID: accountID}
	switch provider {
	case message.APIProviderGmail:
		return &apiactions.Gmail{
			Writer: &gmailapi.Writer{Tokens: tokens, Grant: grant, HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase},
			Reader: &gmailapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase}}, nil
	case message.APIProviderGraph:
		return &apiactions.Graph{
			Writer: &graphapi.Writer{Tokens: tokens, Grant: grant, HTTP: a.apiRoutes.http, Base: a.apiRoutes.graphBase},
			Reader: &graphapi.Reader{Tokens: tokens, HTTP: a.apiRoutes.http, Base: a.apiRoutes.graphBase}}, nil
	}
	return nil, fmt.Errorf("unknown API provider %q", provider)
}

// startAPIActions recovers interrupted actions and drains every account with
// open ones. Call it once after the stores exist.
func (a *App) startAPIActions() {
	go func() {
		defer recoverPanic("app.apiactions", "start api actions")
		w := a.apiActionWorker()
		if err := w.Recover(); err != nil {
			log.Error().Err(err).Msg("Failed to recover api actions")
			return
		}
		ids, err := a.messageStore.OpenAPIActionAccounts()
		if err != nil {
			log.Error().Err(err).Msg("Failed to list api action accounts")
			return
		}
		for _, id := range ids {
			a.kickAPIActions(id)
		}
	}()
}

// kickAPIActions drains the account's queue in the background and re-arms a
// timer for the next retry. A kick during a drain makes it run once more.
func (a *App) kickAPIActions(accountID string) {
	st := &a.apiActions
	st.mu.Lock()
	if st.busy == nil {
		st.busy, st.dirty, st.timers = map[string]bool{}, map[string]bool{}, map[string]*time.Timer{}
	}
	if st.busy[accountID] {
		st.dirty[accountID] = true
		st.mu.Unlock()
		return
	}
	st.busy[accountID] = true
	if t := st.timers[accountID]; t != nil {
		t.Stop()
		delete(st.timers, accountID)
	}
	st.mu.Unlock()

	go func() {
		defer recoverPanic("app.apiactions", "drain api actions")
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		var next time.Time
		for {
			var err error
			next, err = a.apiActionWorker().Drain(ctx, accountID)
			if err != nil && ctx.Err() == nil {
				log.Error().Err(err).Str("account", accountID).Msg("api action drain failed")
				next = time.Now().Add(time.Minute)
			}
			st.mu.Lock()
			if st.dirty[accountID] && ctx.Err() == nil {
				delete(st.dirty, accountID)
				st.mu.Unlock()
				continue
			}
			delete(st.busy, accountID)
			if !next.IsZero() && ctx.Err() == nil {
				st.timers[accountID] = time.AfterFunc(max(time.Until(next), time.Second), func() { a.kickAPIActions(accountID) })
			}
			st.mu.Unlock()
			return
		}
	}()
}

// apiUnsupported is the explicit failure of an operation the API route does
// not offer, so nothing is silently done over IMAP or dropped.
func apiUnsupported(op string) error {
	return fmt.Errorf("%s is not available for accounts using the mail API", op)
}

// apiAccountOf reports the first message's account when it is API routed.
func (a *App) apiAccountOf(messageIDs []string) *account.MailAPIRoute {
	if len(messageIDs) == 0 {
		return nil
	}
	msgs, err := a.messageStore.GetByIDs(messageIDs[:1])
	if err != nil || len(msgs) == 0 {
		return nil
	}
	return a.apiRouted(msgs[0].AccountID)
}

// apiEnqueueFlags records a read or star change for API messages and starts
// the drain. The local rows are updated in the same transaction.
func (a *App) apiEnqueueFlags(op string, value bool, byAccount map[string][]string) error {
	for accountID, ids := range byAccount {
		route := a.apiRouted(accountID)
		if route == nil {
			return errors.New("account is no longer routed through its API")
		}
		if err := a.messageStore.EnqueueAPIFlag(accountID, apiProvider(route), op, ids, value); err != nil {
			return fmt.Errorf("failed to queue flag change: %w", err)
		}
		a.kickAPIActions(accountID)
	}
	return nil
}

// splitAPIMessages separates messages of API-routed accounts (grouped by
// account) from the rest.
func (a *App) splitAPIMessages(messages []*message.Message) (api map[string][]string, other []*message.Message) {
	routed := map[string]bool{}
	for _, m := range messages {
		ok, seen := routed[m.AccountID]
		if !seen {
			ok = a.apiRouted(m.AccountID) != nil
			routed[m.AccountID] = ok
		}
		if !ok {
			other = append(other, m)
			continue
		}
		if api == nil {
			api = map[string][]string{}
		}
		api[m.AccountID] = append(api[m.AccountID], m.ID)
	}
	return api, other
}

// apiMove moves messages of one API account to destFolder locally and queues
// the remote moves. Messages already in the destination are skipped.
func (a *App) apiMove(route *account.MailAPIRoute, messages []*message.Message, destFolder *folder.Folder) error {
	accountID := route.AccountID
	provider := apiProvider(route)
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	resolve, err := a.apiResolver(ctx, route)
	if err != nil {
		return err
	}
	queued := false
	defer func() {
		// Moves queued before a mid-batch failure still need the worker.
		if queued {
			a.kickAPIActions(accountID)
		}
	}()
	for _, m := range messages {
		if m.FolderID == destFolder.ID {
			continue
		}
		src, err := a.folderStore.Get(m.FolderID)
		if err != nil || src == nil {
			return fmt.Errorf("source folder not found: %s", m.FolderID)
		}
		remoteID, _, err := a.messageStore.APIMessageRemote(accountID, provider, m.ID)
		if err != nil {
			return err
		}
		prior, err := a.messageStore.APIRefContainers(accountID, provider, remoteID)
		if err != nil {
			return err
		}
		req, err := resolve(src, destFolder, prior)
		if err != nil {
			return err
		}
		req.AccountID, req.Provider, req.MessageID, req.ToFolderID = accountID, provider, m.ID, destFolder.ID
		if err := a.messageStore.EnqueueAPIMove(req); err != nil {
			return fmt.Errorf("failed to queue move: %w", err)
		}
		queued = true
	}
	return nil
}

type apiMoveResolver func(src, dest *folder.Folder, prior []string) (message.APIMoveRequest, error)

// apiResolver maps local folders to the provider's containers.
func (a *App) apiResolver(ctx context.Context, route *account.MailAPIRoute) (apiMoveResolver, error) {
	if route.Route == account.MailRouteGmailAPI {
		return a.gmailResolver(ctx, route.AccountID), nil
	}
	cursor, err := a.messageStore.LoadAPICursor(route.AccountID, "folders")
	if err != nil {
		return nil, err
	}
	var byID map[string]string
	if err := json.Unmarshal([]byte(cursor.Cursor), &byID); err != nil || len(byID) == 0 {
		return nil, errors.New("folders have not been synced through the API yet")
	}
	idOf := func(f *folder.Folder) (string, error) {
		for id, path := range byID {
			if path == f.Path {
				return id, nil
			}
		}
		return "", fmt.Errorf("folder %q is not a mailbox folder", f.Name)
	}
	return func(src, dest *folder.Folder, _ []string) (message.APIMoveRequest, error) {
		to, err := idOf(dest)
		if err != nil {
			return message.APIMoveRequest{}, err
		}
		from, err := idOf(src)
		if err != nil {
			return message.APIMoveRequest{}, err
		}
		return message.APIMoveRequest{Dest: to, Source: from, Containers: []string{to}}, nil
	}, nil
}

func (a *App) gmailResolver(ctx context.Context, accountID string) apiMoveResolver {
	var labels []gmailapi.Label
	labelID := func(f *folder.Folder) (string, error) {
		switch {
		case f.Path == gmailAllMailPath:
			return "", nil
		case slices.Contains(gmailSystemLabels, f.Path):
			return f.Path, nil
		}
		if labels == nil {
			var err error
			reader := &gmailapi.Reader{Tokens: &apiTokenSource{ops: a.mailAPIOps(), accountID: accountID},
				HTTP: a.apiRoutes.http, Base: a.apiRoutes.gmailBase}
			if labels, err = reader.Labels(ctx); err != nil {
				return "", fmt.Errorf("failed to list Gmail labels: %w", err)
			}
		}
		for _, l := range labels {
			if l.Type == "user" && l.Name == f.Path {
				return l.ID, nil
			}
		}
		return "", fmt.Errorf("folder %q is not a Gmail label", f.Name)
	}
	return func(src, dest *folder.Folder, prior []string) (message.APIMoveRequest, error) {
		from, err := labelID(src)
		if err != nil {
			return message.APIMoveRequest{}, err
		}
		to, err := labelID(dest)
		if err != nil {
			return message.APIMoveRequest{}, err
		}
		req := message.APIMoveRequest{Dest: to, Source: from,
			Trash: dest.Type == folder.TypeTrash, FromTrash: src.Type == folder.TypeTrash}
		containers := slices.DeleteFunc(slices.Clone(prior), func(c string) bool { return c == from })
		switch {
		case req.Trash:
			containers = []string{"TRASH"}
		case to != "" && !slices.Contains(containers, to):
			containers = append(containers, to)
		}
		slices.Sort(containers)
		req.Containers = containers
		return req, nil
	}
}

// anyAPIRouted reports whether any message belongs to an API-routed account.
func (a *App) anyAPIRouted(messageIDs []string) bool {
	msgs, err := a.messageStore.GetByIDs(messageIDs)
	if err != nil {
		return false
	}
	_, other := a.splitAPIMessages(msgs)
	return len(other) != len(msgs)
}

// apiMoveLocal is the API branch of moveToFolder: it queues the move with its
// local effect, then refreshes the UI and folder counts. API moves are not
// undoable.
func (a *App) apiMoveLocal(route *account.MailAPIRoute, messages []*message.Message, destFolder *folder.Folder) error {
	if err := a.apiMove(route, messages, destFolder); err != nil {
		return err
	}
	ids := make([]string, 0, len(messages))
	folders := map[string]bool{destFolder.ID: true}
	for _, m := range messages {
		ids = append(ids, m.ID)
		folders[m.FolderID] = true
	}
	eventsEmit(a.ctx, "messages:moved", map[string]interface{}{"messageIds": ids, "destFolderId": destFolder.ID})
	go func() {
		defer recoverPanic("app.actions", "update folder counts after api move")
		counts := map[string]int{}
		for id := range folders {
			unread, err := a.messageStore.CountUnreadByFolder(id)
			if err != nil {
				continue
			}
			f, err := a.folderStore.Get(id)
			if err != nil || f == nil {
				continue
			}
			total, err := a.messageStore.CountByFolder(id)
			if err != nil {
				continue
			}
			if err := a.folderStore.UpdateCounts(id, total, unread); err == nil {
				counts[id] = unread
			}
		}
		if len(counts) > 0 {
			eventsEmit(a.ctx, "folders:countsChanged", counts)
		}
	}()
	return nil
}
