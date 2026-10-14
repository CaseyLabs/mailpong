package sync

import (
	"context"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
)

// APISyncFunc syncs an API-routed account. handled is false when the account
// is not routed through an API, in which case IMAP sync runs unchanged. When
// handled is true the IMAP engine is never used for the account, whatever err
// is: there is no fallback.
type APISyncFunc func(ctx context.Context, acc *account.Account) (handled bool, err error)

// SetAPISync installs the API sync hook used by scheduled and IDLE-style
// account syncs.
func (s *Scheduler) SetAPISync(fn APISyncFunc) {
	s.apiSync = fn
}

// IsSyncing reports whether a scheduled sync is running for the account.
func (s *Scheduler) IsSyncing(accountID string) bool {
	s.syncingMu.Lock()
	defer s.syncingMu.Unlock()
	return s.syncing[accountID]
}

// syncViaAPI runs the API hook, reporting new inbox mail the same way the IMAP
// path does (the API drivers allocate increasing surrogate UIDs).
func (s *Scheduler) syncViaAPI(ctx context.Context, acc *account.Account) (handled bool, info *NewMailInfo, err error) {
	if s.apiSync == nil {
		return false, nil, nil
	}
	var snap *inboxSnapshot
	if inbox, gerr := s.folderStore.GetByType(acc.ID, folder.TypeInbox); gerr == nil && inbox != nil {
		snap = s.snapshotInbox(inbox)
	}
	handled, err = s.apiSync(ctx, acc)
	if !handled || err != nil {
		return handled, nil, err
	}
	if updated, gerr := s.folderStore.GetByType(acc.ID, folder.TypeInbox); gerr == nil {
		info = s.newMail(snap, acc, updated)
	}
	return true, info, nil
}

// completeAPISync reports an API sync through the scheduler's usual callbacks.
func (s *Scheduler) completeAPISync(acc *account.Account, info *NewMailInfo, err error) {
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		return
	}
	folderID := ""
	if inbox, gerr := s.folderStore.GetByType(acc.ID, folder.TypeInbox); gerr == nil && inbox != nil {
		folderID = inbox.ID
	}
	if err == nil && info != nil && s.newMailCallback != nil {
		s.newMailCallback(*info)
	}
	if s.syncCompletedCallback != nil {
		s.syncCompletedCallback(acc.ID, folderID, err)
	}
}
