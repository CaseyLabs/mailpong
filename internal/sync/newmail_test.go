package sync

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/caseylabs/mailpong/internal/account"
	"github.com/caseylabs/mailpong/internal/folder"
	"github.com/caseylabs/mailpong/internal/message"
	"github.com/rs/zerolog"
)

// TestNewMail covers which rows a sync reports as new mail: only rows stored
// above the pre-sync highest UID or last UIDNEXT, and nothing on a first
// sync or after a UIDVALIDITY change.
func TestNewMail(t *testing.T) {
	store := newTestMessageStore(t)
	s := &Scheduler{engine: &Engine{messageStore: store}, log: zerolog.Nop()}
	acc := &account.Account{ID: "acct-1", Name: "Test"}
	add := func(id string, uid uint32) {
		t.Helper()
		now := time.Now()
		if err := store.Create(&message.Message{ID: id, AccountID: "acct-1", FolderID: "inbox-1", UID: uid, Date: now, ReceivedAt: now}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	add("old-1", 1)
	add("old-2", 2)

	add("old-5", 5)

	synced := time.Now()
	inbox := &folder.Folder{ID: "inbox-1", UIDValidity: 7, LastSync: &synced}
	snap := s.snapshotInbox(inbox)
	if got := s.newMail(snap, acc, inbox); got != nil {
		t.Errorf("no new rows: newMail = %+v, want nil", got)
	}
	if got := s.snapshotInbox(&folder.Folder{ID: "inbox-1", UIDValidity: 7}); got != nil {
		t.Errorf("never-synced inbox: snapshotInbox = %+v, want nil", got)
	}

	// The last sync saw UIDNEXT 10, so UID 9 predates this sync.
	floorSnap := s.snapshotInbox(&folder.Folder{ID: "inbox-1", UIDValidity: 7, UIDNext: 10, LastSync: &synced})

	// Backfilled older mail lands below the snapshot's highest UID; a
	// UID-less placeholder row is not new mail either.
	add("new-1", 9)
	add("new-2", 10)
	add("backfill", 3)
	add("placeholder", 0)
	tests := []struct {
		name    string
		snap    *inboxSnapshot
		updated *folder.Folder
		want    []string
	}{
		{"new rows reported", snap, inbox, []string{"new-1", "new-2"}},
		{"uidvalidity reset", snap, &folder.Folder{ID: "inbox-1", UIDValidity: 8}, nil},
		{"uidvalidity first seen", &inboxSnapshot{}, &folder.Folder{ID: "inbox-1", UIDValidity: 8}, nil},
		{"uidnext above stored rows", floorSnap, inbox, []string{"new-2"}},
		{"no snapshot", nil, inbox, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.newMail(tt.snap, acc, tt.updated)
			if tt.want == nil {
				if got != nil {
					t.Errorf("newMail = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("newMail = nil, want %v", tt.want)
			}
			ids := append([]string(nil), got.MessageIDs...)
			sort.Strings(ids)
			if !reflect.DeepEqual(ids, tt.want) || got.Count != len(tt.want) || got.FolderID != "inbox-1" || got.AccountName != "Test" {
				t.Errorf("newMail = %+v, want IDs %v", got, tt.want)
			}
		})
	}
}
