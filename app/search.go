package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/sync"
)

// ============================================================================
// Search API - Exposed to frontend via Wails bindings
// ============================================================================

// SearchConversations searches for conversations in a folder using full-text search
// Returns matching conversations with highlighted text
func (a *App) SearchConversations(accountID, folderID, query string, offset, limit int, filter string) ([]*message.ConversationSearchResult, error) {
	results, _, err := a.messageStore.SearchConversations(folderID, query, offset, limit, filter)
	return results, err
}

// GetSearchCount returns the total count of search results in a folder
func (a *App) GetSearchCount(accountID, folderID, query, filter string) (int, error) {
	_, count, err := a.messageStore.SearchConversations(folderID, query, 0, 0, filter)
	return count, err
}

// SearchUnifiedInbox searches across all inbox folders for all accounts
func (a *App) SearchUnifiedInbox(query string, offset, limit int, filter string) ([]*message.ConversationSearchResult, error) {
	results, _, err := a.messageStore.SearchConversationsUnifiedInbox(query, offset, limit, filter)
	return results, err
}

// GetSearchCountUnifiedInbox returns the total count of search results across all inboxes
func (a *App) GetSearchCountUnifiedInbox(query, filter string) (int, error) {
	_, count, err := a.messageStore.SearchConversationsUnifiedInbox(query, 0, 0, filter)
	return count, err
}

// GetFTSIndexStatus returns the indexing status for a specific folder
func (a *App) GetFTSIndexStatus(folderID string) (*message.FTSIndexStatus, error) {
	return a.ftsIndexer.GetIndexStatus(folderID)
}

// GetFTSIndexStatusAll returns the indexing status for all folders
func (a *App) GetFTSIndexStatusAll() (map[string]*message.FTSIndexStatus, error) {
	return a.ftsIndexer.GetAllIndexStatuses()
}

// IsFTSIndexComplete checks if a folder is fully indexed
func (a *App) IsFTSIndexComplete(folderID string) bool {
	return a.ftsIndexer.IsIndexComplete(folderID)
}

// IsFTSIndexing returns true if any folder is currently being indexed
func (a *App) IsFTSIndexing() bool {
	return a.ftsIndexer.IsAnyIndexing()
}

// RebuildFTSIndex forces a rebuild of the FTS index for a folder
func (a *App) RebuildFTSIndex(folderID string) error {
	return a.ftsIndexer.RebuildIndex(a.ctx, folderID)
}

// GmailServerSearchResult reports a server-side Gmail search.
type GmailServerSearchResult struct {
	Found int `json:"found"` // matches the server reported (capped by limit)
	Added int `json:"added"` // matches newly stored locally
}

// SearchGmailServer searches the whole Gmail mailbox, beyond the sync window,
// and stores the matches not yet local so the normal search lists them. It
// serializes with the account's sync.
func (a *App) SearchGmailServer(accountID, query string, limit int) (*GmailServerSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("enter a search first")
	}
	var res GmailServerSearchResult
	err := a.withGmailSync(accountID, "Server-side search for this account", func(ctx context.Context, g *sync.GmailSync) (err error) {
		res.Found, res.Added, err = g.SearchServer(ctx, query, limit)
		return err
	}, func() int { return res.Added })
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// OlderMailResult reports one Load older step (Gmail API or IMAP).
type OlderMailResult struct {
	Found int  `json:"found"` // messages the server listed for this step
	Added int  `json:"added"` // messages newly stored locally
	Done  bool `json:"done"`  // nothing older remains
}

// LoadOlderGmail stores the next page of Gmail mail older than what is already
// loaded, starting at the sync window's edge, without widening the sync period.
func (a *App) LoadOlderGmail(accountID string) (*OlderMailResult, error) {
	var res OlderMailResult
	err := a.withGmailSync(accountID, "Loading older mail for this account", func(ctx context.Context, g *sync.GmailSync) (err error) {
		res.Found, res.Added, res.Done, err = g.LoadOlder(ctx, olderPageSize)
		return err
	}, func() int { return res.Added })
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// olderPageSize is how many messages one Load older step lists.
const olderPageSize = 100

// withGmailSync runs fn with a Gmail reader for an account on the Gmail API
// route, serialized with the account's sync; unsupported names what the route
// check refuses. added reports how many messages fn stored, to refresh counts.
func (a *App) withGmailSync(accountID, unsupported string, fn func(context.Context, *sync.GmailSync) error, added func() int) error {
	route := a.apiRouted(accountID)
	if route == nil || route.Route != account.MailRouteGmailAPI {
		return apiUnsupported(unsupported)
	}
	if a.apiPaused(accountID) {
		return errAPIPaused
	}
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return err
	}
	mu := a.apiRoutes.lock(accountID)
	mu.Lock()
	defer mu.Unlock()
	if a.apiRouted(accountID) == nil {
		return errAPIPaused
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	ops := a.mailAPIOps()
	tokens := &apiTokenSource{ops: ops, accountID: accountID}
	if err := fn(ctx, a.newGmailSync(accountID, ops, tokens, a.apiFailureHandler(accountID, route, ops), acc.SyncPeriodDays)); err != nil {
		return err
	}
	if added() > 0 {
		a.updateAPICounts(accountID)
	}
	return nil
}

// IMAPServerSearchResult reports a server-side IMAP search.
type IMAPServerSearchResult struct {
	Found int `json:"found"` // matches the server reported across the account's folders
	Added int `json:"added"` // matches newly stored locally (capped by limit)
}

// SearchIMAPServer searches the account's folders on the IMAP server and stores
// the matches not yet local, pinned so the sync window keeps them, so the
// normal search lists them.
func (a *App) SearchIMAPServer(accountID, query string, limit int) (*IMAPServerSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("enter a search first")
	}
	if a.apiRouted(accountID) != nil {
		return nil, apiUnsupported("Server-side IMAP search")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	found, added, err := a.syncEngine.SearchServer(ctx, accountID, query, limit)
	if err != nil {
		return nil, err
	}
	return &IMAPServerSearchResult{Found: found, Added: added}, nil
}

// LoadOlderIMAP stores the next page of mail from before the sync window in
// each of the account's folders, without widening the sync period.
func (a *App) LoadOlderIMAP(accountID string) (*OlderMailResult, error) {
	if a.apiRouted(accountID) != nil {
		return nil, apiUnsupported("Loading older mail over IMAP")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	var res OlderMailResult
	var err error
	res.Found, res.Added, res.Done, err = a.syncEngine.LoadOlder(ctx, accountID, a.syncPeriodDays(accountID), olderPageSize)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
