package sync

import (
	"github.com/caseylabs/mailpong/internal/account"
	"github.com/caseylabs/mailpong/internal/folder"
)

// inboxSnapshot records an inbox's highest stored UID and UIDVALIDITY before
// a sync. The server assigns arriving mail UIDs above every existing one, so
// rows stored above highestUID are new mail, while older mail a sync backfills
// (a longer sync period, say) sits below it. The UIDNEXT seen by the last
// sync raises that floor, so backfill into an inbox with no stored rows
// below it isn't announced either.
type inboxSnapshot struct {
	highestUID  uint32
	uidValidity uint32
}

// snapshotInbox captures inbox state before a sync. It returns nil when the
// inbox has never synced (its first sync stores existing mail, not new mail)
// or the highest UID can't be read; newMail then reports nothing.
func (s *Scheduler) snapshotInbox(inbox *folder.Folder) *inboxSnapshot {
	if inbox.LastSync == nil {
		return nil
	}
	uid, err := s.engine.messageStore.GetHighestUID(inbox.ID)
	if err != nil {
		s.log.Warn().Err(err).Str("folder", inbox.ID).Msg("Failed to snapshot inbox before sync")
		return nil
	}
	if inbox.UIDNext > 0 {
		uid = max(uid, inbox.UIDNext-1)
	}
	return &inboxSnapshot{highestUID: uid, uidValidity: inbox.UIDValidity}
}

// newMail returns NewMailInfo for the rows stored above snap's highest UID, or
// nil when there are none. A UIDVALIDITY change re-stores the whole mailbox
// under new UIDs, so it reports nothing instead of announcing every message.
func (s *Scheduler) newMail(snap *inboxSnapshot, acc *account.Account, updated *folder.Folder) *NewMailInfo {
	if snap == nil || updated == nil {
		return nil
	}
	if updated.UIDValidity != snap.uidValidity {
		s.log.Info().Str("folder", updated.ID).Msg("UIDVALIDITY changed, skipping new-mail notification")
		return nil
	}
	fresh, err := s.engine.messageStore.GetIDsAboveUID(updated.ID, snap.highestUID)
	if err != nil {
		s.log.Warn().Err(err).Str("folder", updated.ID).Msg("Failed to list inbox after sync")
		return nil
	}
	if len(fresh) == 0 {
		return nil
	}
	return &NewMailInfo{
		AccountID:   acc.ID,
		AccountName: acc.Name,
		FolderID:    updated.ID,
		Count:       len(fresh),
		MessageIDs:  fresh,
	}
}
