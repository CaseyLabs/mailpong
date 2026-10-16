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

// IMAPSearchFolder performs a server-side IMAP SEARCH query on a specific folder.
// Returns results with local message data where available, envelope data for non-local messages.
// When limit > 0, only the newest `limit` results are returned but totalCount reflects all matches.
func (a *App) IMAPSearchFolder(accountID, folderID, query string, limit int) (*sync.IMAPSearchResponse, error) {
	if a.apiRouted(accountID) != nil {
		return nil, apiUnsupported("Server-side search")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return a.syncEngine.IMAPSearch(ctx, accountID, folderID, query, limit)
}

// FetchServerMessage fetches a full message by UID from the IMAP server, saves it locally,
// and returns it. Used when interacting with non-local server search results.
func (a *App) FetchServerMessage(accountID, folderID string, uid int) (*message.Message, error) {
	if a.apiRouted(accountID) != nil {
		return nil, apiUnsupported("Fetching a server message")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	return a.syncEngine.FetchServerMessage(ctx, accountID, folderID, uint32(uid))
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
	route := a.apiRouted(accountID)
	if route == nil || route.Route != account.MailRouteGmailAPI {
		return nil, apiUnsupported("Server-side search for this account")
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("enter a search first")
	}
	if a.apiPaused(accountID) {
		return nil, errAPIPaused
	}
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	mu := a.apiRoutes.lock(accountID)
	mu.Lock()
	defer mu.Unlock()
	if a.apiRouted(accountID) == nil {
		return nil, errAPIPaused
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	ops := a.mailAPIOps()
	tokens := &apiTokenSource{ops: ops, accountID: accountID}
	found, added, err := a.newGmailSync(accountID, ops, tokens, a.apiFailureHandler(accountID, route, ops), acc.SyncPeriodDays).
		SearchServer(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if added > 0 {
		a.updateAPICounts(accountID)
	}
	return &GmailServerSearchResult{Found: found, Added: added}, nil
}
