package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/gmailapi"
)

func newSendAsOps(t *testing.T, imapHost string, auth account.AuthType) (*composeOps, *account.Account) {
	t.Helper()
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
		Name: "Personal", DisplayName: "Sean", Email: "me@gmail.com",
		IMAPHost: imapHost, IMAPPort: 993, IMAPSecurity: account.SecurityTLS,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS,
		AuthType: auth, Username: "me@gmail.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return &composeOps{accountStore: accounts, sendAsStore: gmailapi.NewStore(db.DB)}, acc
}

func TestSendAsStatusUnsupportedAccounts(t *testing.T) {
	tests := []struct {
		name string
		host string
		auth account.AuthType
	}{
		{"password gmail", "imap.gmail.com", account.AuthPassword},
		{"oauth other host", "outlook.office365.com", account.AuthOAuth2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, acc := newSendAsOps(t, tt.host, tt.auth)
			st, err := ops.sendAsStatus(acc.ID)
			if err != nil || st.Supported || len(st.Identities) != 0 {
				t.Fatalf("status = %+v, %v", st, err)
			}
			if err := ops.refreshSendAs(t.Context(), acc.ID); err == nil {
				t.Error("refresh on a non-Gmail account should fail without calling the API")
			}
			if _, err := ops.importSendAs(acc.ID, []string{"x@y.com"}, false); err == nil {
				t.Error("import on a non-Gmail account should fail")
			}
		})
	}
}

func TestSendAsStatusAndImport(t *testing.T) {
	ops, acc := newSendAsOps(t, "imap.gmail.com", account.AuthOAuth2)
	stray, err := ops.accountStore.CreateIdentity(acc.ID, &account.IdentityConfig{Email: "stray@x.com", Name: "Stray"})
	if err != nil {
		t.Fatal(err)
	}

	// Never fetched: supported, every identity unknown, nothing to import.
	st, err := ops.sendAsStatus(acc.ID)
	if err != nil || !st.Supported || st.Identities[stray.ID] != gmailapi.StatusUnknown || len(st.Available) != 0 {
		t.Fatalf("before fetch: %+v, %v", st, err)
	}

	err = ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{
		{Email: "me@gmail.com", IsPrimary: true},
		{Email: "sean@caseylabs.com", DisplayName: "Sean C", ReplyTo: "reply@caseylabs.com", VerificationStatus: "accepted", Signature: "<p>sig</p>"},
		{Email: "wait@x.com", VerificationStatus: "pending"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	st, err = ops.sendAsStatus(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Identities[stray.ID] != gmailapi.StatusNotConfigured {
		t.Errorf("stray status = %q", st.Identities[stray.ID])
	}
	if len(st.Available) != 1 || st.Available[0].Email != "sean@caseylabs.com" {
		t.Fatalf("available = %+v", st.Available)
	}

	got, err := ops.importSendAs(acc.ID, []string{"Sean@caseylabs.com", "wait@x.com", "me@gmail.com"}, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("import = %+v, %v", got, err)
	}
	if got[0].Name != "Sean C" || got[0].ReplyTo != "reply@caseylabs.com" || got[0].SignatureHTML != "" {
		t.Errorf("imported identity = %+v", got[0])
	}
	if st, _ = ops.sendAsStatus(acc.ID); st.Identities[got[0].ID] != gmailapi.StatusVerified || len(st.Available) != 0 {
		t.Errorf("after import: %+v", st)
	}

	if again, _ := ops.importSendAs(acc.ID, []string{"sean@caseylabs.com"}, true); len(again) != 0 {
		t.Errorf("re-import created %d identities", len(again))
	}

	// The signature comes only on opt-in.
	if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: "b@x.com", Signature: "<p>sig</p>"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	signed, err := ops.importSendAs(acc.ID, []string{"b@x.com"}, true)
	if err != nil || len(signed) != 1 || signed[0].SignatureHTML != "<p>sig</p>" {
		t.Errorf("signature import = %+v, %v", signed, err)
	}
}
