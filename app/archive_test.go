package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/caseylabs/mailpong/internal/account"
	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/folder"
	"github.com/caseylabs/mailpong/internal/message"
)

// newArchiveTestApp returns an App over a fresh database holding one account
// with the given folders, keyed by type, and one message per folder whose ID
// is the folder's type.
func newArchiveTestApp(t *testing.T, types ...folder.Type) (*App, map[folder.Type]*folder.Folder) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	a := &App{
		accountStore: account.NewStore(db),
		folderStore:  folder.NewStore(db),
		messageStore: message.NewStore(db),
	}
	acc, err := a.accountStore.Create(&account.AccountConfig{
		Name: "Me", DisplayName: "Me", Email: "me@example.com",
		IMAPHost: "imap.example.com", IMAPPort: 993, IMAPSecurity: account.SecurityTLS,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS,
		AuthType: account.AuthPassword, Username: "me@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	folders := map[folder.Type]*folder.Folder{}
	for i, typ := range types {
		f := &folder.Folder{AccountID: acc.ID, Name: string(typ), Path: string(typ), Type: typ}
		if err := a.folderStore.Create(f); err != nil {
			t.Fatal(err)
		}
		folders[typ] = f
		if err := a.messageStore.Create(&message.Message{
			ID: string(typ), AccountID: acc.ID, FolderID: f.ID, UID: uint32(i + 1),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return a, folders
}

func TestArchivableIDs(t *testing.T) {
	a, folders := newArchiveTestApp(t,
		folder.TypeInbox, folder.TypeSent, folder.TypeDrafts, folder.TypeArchive, folder.TypeAll)
	all := []string{"inbox", "sent", "drafts", "archive", "all", "unknown-id"}

	tests := []struct {
		name string
		dest *folder.Folder
		want []string
	}{
		{"archive folder skips its own messages", folders[folder.TypeArchive], []string{"inbox", "sent", "drafts", "all"}},
		{"all mail skips its own, sent and drafts", folders[folder.TypeAll], []string{"inbox", "archive"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.archivableIDs(all, tt.dest)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("archivableIDs = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestArchiveAllMailNoOp covers an account without an Archive folder:
// Archive falls back to All Mail, and a selection with nothing to move there
// succeeds without moving anything.
func TestArchiveAllMailNoOp(t *testing.T) {
	a, folders := newArchiveTestApp(t, folder.TypeSent, folder.TypeDrafts, folder.TypeAll)

	if err := a.Archive([]string{"all", "sent", "drafts"}); err != nil {
		t.Fatalf("Archive = %v, want nil", err)
	}
	for typ, f := range folders {
		m, err := a.messageStore.Get(string(typ))
		if err != nil || m == nil {
			t.Fatalf("Get(%s) = %v, %v", typ, m, err)
		}
		if m.FolderID != f.ID {
			t.Errorf("message %s moved to %s", typ, m.FolderID)
		}
	}
}
