package app

import (
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/smtp"
)

func TestDraftPersistsSenderIdentity(t *testing.T) {
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
	alias, err := accounts.CreateIdentity(acc.ID, &account.IdentityConfig{Email: "sales@example.com", Name: "Sales"})
	if err != nil {
		t.Fatal(err)
	}

	ops := &draftOps{accountStore: accounts, draftStore: draft.NewStore(db)}
	msg := smtp.ComposeMessage{From: smtp.Address{Name: "Sales", Address: "Sales@Example.com"}, Subject: "Hi"}

	d, err := ops.saveDraftToDB(acc.ID, nil, msg, &encryptResult{})
	if err != nil {
		t.Fatal(err)
	}
	if d.IdentityID != alias.ID {
		t.Fatalf("created draft IdentityID = %q, want alias %q", d.IdentityID, alias.ID)
	}

	stored, err := ops.draftStore.Get(d.ID)
	if err != nil || stored == nil {
		t.Fatalf("Get draft: %v", err)
	}
	if got := ops.toComposeMessage(stored).From; got.Address != "sales@example.com" || got.Name != "Sales" {
		t.Errorf("restored From = %+v, want Sales <sales@example.com>", got)
	}

	// Switching back to the default sender on update is persisted too.
	msg.From = smtp.Address{Address: "pat@example.com"}
	if _, err := ops.saveDraftToDB(acc.ID, stored, msg, &encryptResult{}); err != nil {
		t.Fatal(err)
	}
	stored, _ = ops.draftStore.Get(d.ID)
	if got := ops.toComposeMessage(stored).From.Address; got != "pat@example.com" {
		t.Errorf("restored From after update = %q, want pat@example.com", got)
	}
}
