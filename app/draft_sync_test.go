package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/smtp"
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

func TestDraftOriginalFolderAfterMigration(t *testing.T) {
	r := newMigrationRig(t)
	folders := r.app.folderStore
	old := &folder.Folder{AccountID: r.acc.ID, Name: "Drafts", Path: "Original/Drafts", Type: folder.TypeDrafts}
	if err := folders.Create(old); err != nil {
		t.Fatal(err)
	}
	if _, err := folders.SwapHidden(r.acc.ID, folder.RowsIMAP, folder.RowsAPI); err != nil {
		t.Fatal(err)
	}
	api := &folder.Folder{AccountID: r.acc.ID, Name: "Drafts", Path: "DRAFT", Type: folder.TypeDrafts}
	if err := folders.Create(api); err != nil {
		t.Fatal(err)
	}
	route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
	route.State = account.MailRouteStateActive
	if err := r.app.accountStore.SetMailAPIRoute(*route); err != nil {
		t.Fatal(err)
	}
	ops := &draftOps{accountStore: r.app.accountStore, folderStore: folders, draftStore: draft.NewStore(r.db)}
	d := &draft.Draft{AccountID: r.acc.ID, FolderID: old.ID, IMAPUID: 42, SyncStatus: draft.SyncStatusPending}
	got, err := ops.originalDraftFolder(d)
	if err != nil || got == nil || got.ID != old.ID || got.Path != "Original/Drafts" {
		t.Fatalf("original folder = %+v, %v", got, err)
	}
	d.FolderID = api.ID
	if got, _ := ops.originalDraftFolder(d); got != nil {
		t.Fatal("API folder accepted as IMAP linkage")
	}
	d.FolderID = old.ID
	if err := ops.draftStore.Create(d); err != nil {
		t.Fatal(err)
	}
	// No IMAP pool: reaching it would panic instead of retaining a local draft.
	ops.syncToIMAP(t.Context(), d, smtp.ComposeMessage{}, func(draft.SyncStatus, uint32, string) {})
	saved, err := ops.draftStore.Get(d.ID)
	if err != nil || saved.IMAPUID != 42 || saved.FolderID != old.ID || saved.SyncError == "" {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
}
