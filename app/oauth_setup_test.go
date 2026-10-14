package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/oauth2"
)

func TestOAuthSetupDefaults(t *testing.T) {
	for _, tt := range []struct {
		provider, kind, host string
		invalid              bool
	}{
		{"microsoft", "personal", "smtp-mail.outlook.com", false},
		{"microsoft", "business", "smtp.office365.com", false},
		{"microsoft", "", "smtp.office365.com", false},
		{"google", "", "smtp.gmail.com", false},
		{"microsoft", "domain-guessed", "", true},
		{"google", "personal", "", true},
	} {
		t.Run(tt.provider+"/"+tt.kind, func(t *testing.T) {
			config, err := oauthAccountDefaults(tt.provider, tt.kind)
			if (err != nil) != tt.invalid {
				t.Fatal(err)
			}
			if tt.invalid {
				return
			}
			if config.SMTPHost != tt.host || config.SMTPPort != 587 || config.SMTPSecurity != account.SecurityStartTLS || config.IMAPPort != 993 || config.IMAPSecurity != account.SecurityTLS {
				t.Fatalf("wrong defaults: %+v", config)
			}
		})
	}
}

func TestLinkedMailboxReauthorizationSurvivesRestart(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "linked.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cred, err := credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	accounts := account.NewStore(db)
	parentConfig, _ := oauthAccountDefaults("microsoft", "business")
	parentConfig.Name, parentConfig.DisplayName, parentConfig.Email, parentConfig.Username = "Parent", "Parent", "parent@example.test", "parent@example.test"
	parent, err := accounts.Create(&parentConfig)
	if err != nil {
		t.Fatal(err)
	}
	childConfig := parentConfig
	childConfig.Name, childConfig.DisplayName, childConfig.Email, childConfig.Username = "Shared", "Shared", "shared@example.test", "shared@example.test"
	childConfig.SharedMailboxParentID = parent.ID
	child, err := accounts.Create(&childConfig)
	if err != nil {
		t.Fatal(err)
	}
	prior := &credentials.OAuthTokens{
		Provider: "microsoft", AccessToken: "expired", RefreshToken: "old-refresh",
		ExpiresAt: time.Now().Add(-time.Hour), MicrosoftAccountType: "business",
	}
	if err := cred.SetOAuthTokens(parent.ID, prior); err != nil {
		t.Fatal(err)
	}
	if err := cred.SetOAuthTokens(child.ID, prior); err != nil {
		t.Fatal(err)
	}
	a := &App{db: db, accountStore: accounts, credStore: cred}
	a.setPendingOAuth(&oauth2.TokenResponse{
		Provider: "microsoft", AccessToken: "reauthorized", RefreshToken: "rotated-refresh",
		Scope: "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send", ExpiresIn: 3600,
	}, parent.Email)
	if err := a.SavePendingOAuthTokens(child.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// A new database/store instance represents application restart. Tokens remain
	// keyed by each account's existing ID, while both linked slots get the grant.
	db, err = database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cred, err = credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{parent.ID, child.ID} {
		tokens, err := cred.GetOAuthTokens(id)
		if err != nil {
			t.Fatal(err)
		}
		if tokens.AccessToken != "reauthorized" || tokens.RefreshToken != "rotated-refresh" || tokens.MicrosoftAccountType != "business" {
			t.Errorf("account %s did not retain propagated reauthorization: %+v", id, tokens)
		}
		if tokens.RequireMailCapability("imap") != nil || tokens.RequireMailCapability("smtp") != nil {
			t.Errorf("account %s lost granted mail capabilities: %+v", id, tokens)
		}
	}
	gotParent, err := account.NewStore(db).Get(parent.ID)
	if err != nil || gotParent.ID != parent.ID {
		t.Fatalf("parent account identity changed after restart: %+v, %v", gotParent, err)
	}
	gotChild, err := account.NewStore(db).Get(child.ID)
	if err != nil || gotChild.ID != child.ID || gotChild.SharedMailboxParentID != parent.ID {
		t.Fatalf("shared mailbox link changed after restart: %+v, %v", gotChild, err)
	}
}

func TestOAuthSetupPreservesCustomAndExistingEndpoints(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cred, err := credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{db: db, accountStore: account.NewStore(db), credStore: cred}
	missingConfig, _ := oauthAccountDefaults("microsoft", "")
	missingConfig.Name, missingConfig.DisplayName, missingConfig.Email, missingConfig.Username = "Missing", "Missing", "missing@example.com", "missing@example.com"
	missing, err := a.accountStore.Create(&missingConfig)
	if err != nil {
		t.Fatal(err)
	}
	status, err := a.GetOAuthStatus(missing.ID)
	if err != nil || !status.NeedsReauth || status.ScopesKnown || status.MicrosoftAccountType != "" {
		t.Fatalf("missing token status: %+v %v", status, err)
	}
	config, _ := oauthAccountDefaults("microsoft", "personal")
	config.DisplayName = "Test User"
	config.Name, config.Email, config.Username = "Test", "user@custom.example", "user@custom.example"
	config.IMAPHost, config.SMTPHost = "imap.custom.example", "smtp.custom.example"
	response := &oauth2.TokenResponse{Provider: "microsoft", AccessToken: "new", RefreshToken: "refresh", Scope: "https://outlook.office.com/IMAP.AccessAsUser.All", ExpiresIn: 3600}
	a.setPendingOAuth(response, config.Email)
	acc, err := a.CompleteOAuthAccountSetupWithConfig("microsoft", config, "personal")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cred.DeleteAllCredentials(acc.ID) }()
	if acc.IMAPHost != config.IMAPHost || acc.SMTPHost != config.SMTPHost {
		t.Fatalf("custom endpoints overwritten: %+v", acc)
	}
	a.setPendingOAuth(&oauth2.TokenResponse{AccessToken: "reauthorized", Scope: "https://outlook.office.com/SMTP.Send", ExpiresIn: 3600}, config.Email)
	if err = a.SavePendingOAuthTokens(acc.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := a.accountStore.Get(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.IMAPHost != config.IMAPHost || restored.SMTPHost != config.SMTPHost {
		t.Fatal("reauthorization migrated existing endpoints")
	}
	tokens, err := cred.GetOAuthTokens(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.RefreshToken != "refresh" || tokens.MicrosoftAccountType != "personal" || tokens.RequireMailCapability("imap") == nil || tokens.RequireMailCapability("smtp") != nil {
		t.Fatalf("grant/type/refresh retention failed: %+v", tokens)
	}
}
