package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailpong/internal/account"
	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/draft"
	"github.com/caseylabs/mailpong/internal/folder"
	"github.com/caseylabs/mailpong/internal/smtp"
)

// TestSyncToIMAPFailureKeepsServerCopy checks that a failed sync leaves the
// recorded IMAP UID in place, so the existing server copy is still tracked.
func TestSyncToIMAPFailureKeepsServerCopy(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	accounts := account.NewStore(db)
	acc, err := accounts.Create(&account.AccountConfig{
		Name: "Work", DisplayName: "Pat", Email: "pat@example.com",
		IMAPHost: "imap.example.com", IMAPPort: 993, IMAPSecurity: account.SecurityTLS,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS,
		AuthType: account.AuthPassword, Username: "pat@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	folders := folder.NewStore(db)
	// An inbox only: the drafts folder lookup fails.
	inbox := &folder.Folder{AccountID: acc.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := folders.Create(inbox); err != nil {
		t.Fatal(err)
	}

	ops := &draftOps{accountStore: accounts, folderStore: folders, draftStore: draft.NewStore(db)}
	msg := smtp.ComposeMessage{From: smtp.Address{Address: "pat@example.com"}, Subject: "Hi"}
	d, err := ops.saveDraftToDB(acc.ID, nil, msg, &encryptResult{})
	if err != nil {
		t.Fatal(err)
	}
	if err := ops.draftStore.UpdateSyncStatus(d.ID, draft.SyncStatusSynced, 42, inbox.ID, ""); err != nil {
		t.Fatal(err)
	}

	var emitted uint32
	got := ops.syncToIMAP(context.Background(), d, msg, func(_ draft.SyncStatus, uid uint32, _ string) { emitted = uid })
	if got != nil {
		t.Fatalf("syncToIMAP returned a folder without a drafts folder")
	}
	stored, err := ops.draftStore.Get(d.ID)
	if err != nil || stored == nil {
		t.Fatalf("Get draft: %v", err)
	}
	if stored.SyncStatus != draft.SyncStatusFailed || stored.IMAPUID != 42 || stored.FolderID != inbox.ID || emitted != 42 {
		t.Errorf("after failure: status %q uid %d folder %q emitted %d, want failed 42 %q 42",
			stored.SyncStatus, stored.IMAPUID, stored.FolderID, emitted, inbox.ID)
	}
}
