package account

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/database"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func validAccountConfig() *AccountConfig {
	return &AccountConfig{
		Name:         "Test Account",
		DisplayName:  "Test User",
		Email:        "test@example.com",
		IMAPHost:     "imap.example.com",
		IMAPPort:     993,
		IMAPSecurity: SecurityTLS,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPSecurity: SecurityStartTLS,
		AuthType:     AuthPassword,
		Username:     "test@example.com",
	}
}

func TestAccountConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(c *AccountConfig)
		wantErr   error
		checkFunc func(t *testing.T, c *AccountConfig)
	}{
		{
			name:   "valid config",
			modify: func(c *AccountConfig) {},
		},
		{
			name:    "missing name",
			modify:  func(c *AccountConfig) { c.Name = "" },
			wantErr: ErrNameRequired,
		},
		{
			name:    "missing email",
			modify:  func(c *AccountConfig) { c.Email = "" },
			wantErr: ErrEmailRequired,
		},
		{
			name:    "missing display name",
			modify:  func(c *AccountConfig) { c.DisplayName = "" },
			wantErr: ErrDisplayNameRequired,
		},
		{
			name:    "missing IMAP host",
			modify:  func(c *AccountConfig) { c.IMAPHost = "" },
			wantErr: ErrIMAPHostRequired,
		},
		{
			name:    "missing SMTP host",
			modify:  func(c *AccountConfig) { c.SMTPHost = "" },
			wantErr: ErrSMTPHostRequired,
		},
		{
			name:    "missing username",
			modify:  func(c *AccountConfig) { c.Username = "" },
			wantErr: ErrUsernameRequired,
		},
		{
			name: "defaults applied for ports",
			modify: func(c *AccountConfig) {
				c.IMAPPort = 0
				c.SMTPPort = 0
			},
			checkFunc: func(t *testing.T, c *AccountConfig) {
				if c.IMAPPort != 993 {
					t.Errorf("IMAPPort = %d, want 993", c.IMAPPort)
				}
				if c.SMTPPort != 587 {
					t.Errorf("SMTPPort = %d, want 587", c.SMTPPort)
				}
			},
		},
		{
			name: "defaults applied for sync settings",
			modify: func(c *AccountConfig) {
				c.SyncPeriodDays = -1
				c.SyncInterval = -1
			},
			checkFunc: func(t *testing.T, c *AccountConfig) {
				if c.SyncPeriodDays != 30 {
					t.Errorf("SyncPeriodDays = %d, want 30", c.SyncPeriodDays)
				}
				if c.SyncInterval != 30 {
					t.Errorf("SyncInterval = %d, want 30", c.SyncInterval)
				}
			},
		},
		{
			name: "defaults applied for security and auth",
			modify: func(c *AccountConfig) {
				c.IMAPSecurity = ""
				c.SMTPSecurity = ""
				c.AuthType = ""
			},
			checkFunc: func(t *testing.T, c *AccountConfig) {
				if c.IMAPSecurity != SecurityTLS {
					t.Errorf("IMAPSecurity = %q, want %q", c.IMAPSecurity, SecurityTLS)
				}
				if c.SMTPSecurity != SecurityStartTLS {
					t.Errorf("SMTPSecurity = %q, want %q", c.SMTPSecurity, SecurityStartTLS)
				}
				if c.AuthType != AuthPassword {
					t.Errorf("AuthType = %q, want %q", c.AuthType, AuthPassword)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAccountConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func TestIdentityConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *IdentityConfig
		wantErr error
	}{
		{
			name:   "valid config",
			config: &IdentityConfig{Email: "test@example.com", Name: "Test User"},
		},
		{
			name:    "missing email",
			config:  &IdentityConfig{Email: "", Name: "Test User"},
			wantErr: ErrEmailRequired,
		},
		{
			name:    "missing name",
			config:  &IdentityConfig{Email: "test@example.com", Name: ""},
			wantErr: ErrDisplayNameRequired,
		},
		{
			name:   "valid reply-to",
			config: &IdentityConfig{Email: "test@example.com", Name: "Test User", ReplyTo: "reply@example.com"},
		},
		{
			name:    "invalid reply-to",
			config:  &IdentityConfig{Email: "test@example.com", Name: "Test User", ReplyTo: "not an address"},
			wantErr: ErrInvalidReplyTo,
		},
		{
			name:    "reply-to with display name",
			config:  &IdentityConfig{Email: "test@example.com", Name: "Test User", ReplyTo: "Boss <boss@example.com>"},
			wantErr: ErrInvalidReplyTo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestGetFolderMapping(t *testing.T) {
	acct := &Account{
		SentFolderPath:    "Sent",
		DraftsFolderPath:  "Drafts",
		TrashFolderPath:   "Trash",
		SpamFolderPath:    "Spam",
		ArchiveFolderPath: "Archive",
		AllMailFolderPath: "All Mail",
		StarredFolderPath: "Starred",
	}

	tests := []struct {
		folderType string
		want       string
	}{
		{"sent", "Sent"},
		{"drafts", "Drafts"},
		{"trash", "Trash"},
		{"spam", "Spam"},
		{"archive", "Archive"},
		{"all", "All Mail"},
		{"starred", "Starred"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.folderType, func(t *testing.T) {
			got := acct.GetFolderMapping(tt.folderType)
			if got != tt.want {
				t.Errorf("GetFolderMapping(%q) = %q, want %q", tt.folderType, got, tt.want)
			}
		})
	}
}

func TestStoreCreate(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	cfg := validAccountConfig()
	acct, err := store.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if acct == nil {
		t.Fatal("Create() returned nil account")
	}
	if acct.ID == "" {
		t.Error("Create() account ID is empty")
	}
	if acct.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", acct.Name, cfg.Name)
	}
	if acct.Email != cfg.Email {
		t.Errorf("Email = %q, want %q", acct.Email, cfg.Email)
	}
	if acct.Color == "" {
		t.Error("Create() auto-assigned color is empty")
	}
	if !acct.Enabled {
		t.Error("Create() account should be enabled by default")
	}
}

func TestStoreGet(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	cfg := validAccountConfig()
	created, err := store.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.Name != created.Name {
		t.Errorf("Name = %q, want %q", got.Name, created.Name)
	}
	if got.Email != created.Email {
		t.Errorf("Email = %q, want %q", got.Email, created.Email)
	}
	if got.IMAPHost != created.IMAPHost {
		t.Errorf("IMAPHost = %q, want %q", got.IMAPHost, created.IMAPHost)
	}
	if got.SMTPHost != created.SMTPHost {
		t.Errorf("SMTPHost = %q, want %q", got.SMTPHost, created.SMTPHost)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestStoreList(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	cfg1 := validAccountConfig()
	cfg2 := validAccountConfig()
	cfg2.Email = "test2@example.com"

	if _, err := store.Create(cfg1); err != nil {
		t.Fatalf("Create(1) error = %v", err)
	}
	if _, err := store.Create(cfg2); err != nil {
		t.Fatalf("Create(2) error = %v", err)
	}

	accounts, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("List() returned %d accounts, want 2", len(accounts))
	}
}

func TestStoreUpdate(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	cfg := validAccountConfig()
	created, err := store.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cfg.Name = "Updated Account"
	updated, err := store.Update(created.ID, cfg)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Updated Account" {
		t.Errorf("Name = %q, want %q", updated.Name, "Updated Account")
	}

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() after Update() error = %v", err)
	}
	if got.Name != "Updated Account" {
		t.Errorf("persisted Name = %q, want %q", got.Name, "Updated Account")
	}
}

func TestStoreDelete(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	cfg := validAccountConfig()
	created, err := store.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get(created.ID)
	if err == nil {
		t.Fatal("Get() after Delete() expected error, got nil")
	}
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("Get() after Delete() error = %v, want ErrAccountNotFound", err)
	}
}

func TestIdentityReplyToRoundTrip(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	acct, err := store.Create(validAccountConfig())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ident, err := store.CreateIdentity(acct.ID, &IdentityConfig{Email: "alias@example.com", Name: "Alias", ReplyTo: "reply@example.com"})
	if err != nil {
		t.Fatalf("CreateIdentity() error = %v", err)
	}
	got, err := store.GetIdentity(ident.ID)
	if err != nil || got.ReplyTo != "reply@example.com" {
		t.Fatalf("GetIdentity() = %+v, %v; want ReplyTo reply@example.com", got, err)
	}

	if _, err := store.UpdateIdentity(ident.ID, &IdentityConfig{Email: "alias@example.com", Name: "Alias"}); err != nil {
		t.Fatalf("UpdateIdentity() error = %v", err)
	}
	list, err := store.GetIdentities(acct.ID)
	if err != nil {
		t.Fatalf("GetIdentities() error = %v", err)
	}
	for _, i := range list {
		if i.ReplyTo != "" {
			t.Errorf("identity %s ReplyTo = %q, want empty", i.Email, i.ReplyTo)
		}
	}
}

func TestStoreSyncError(t *testing.T) {
	tests := []struct {
		name      string
		clearFrom string
		want      []SyncError
	}{
		{"other folder keeps it", "f2", []SyncError{{FolderID: "f1", Error: "boom"}}},
		{"failed folder clears it", "f1", nil},
		{"any folder clears it", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(openTestDB(t))
			acct, err := store.Create(validAccountConfig())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SetSyncError(acct.ID, "f0", "old"); err != nil {
				t.Fatal(err)
			}
			if err := store.SetSyncError(acct.ID, "f1", "boom"); err != nil {
				t.Fatal(err)
			}
			if err := store.ClearSyncError(acct.ID, tt.clearFrom); err != nil {
				t.Fatal(err)
			}
			got, err := store.ListSyncErrors()
			if err != nil {
				t.Fatal(err)
			}
			for i := range tt.want {
				tt.want[i].AccountID = acct.ID
			}
			if len(got) != len(tt.want) || (len(got) == 1 && got[0] != tt.want[0]) {
				t.Errorf("ListSyncErrors() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMailAPIRouteStore(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	acc, err := store.Create(validAccountConfig())
	if err != nil {
		t.Fatal(err)
	}
	if r, err := store.GetMailAPIRoute(acc.ID); err != nil || r != nil {
		t.Fatalf("new account must be protocol: %v %v", r, err)
	}
	want := MailAPIRoute{AccountID: acc.ID, Route: MailRouteGraphAPI, ClientConfigID: "microsoft-mail-api", Shared: true,
		Snapshot: MailAPISnapshot{Provider: "microsoft", TokenURL: "https://login.example.test/token", ClientID: "client", Scopes: []string{"a", "b"}, Resource: "https://graph.microsoft.com"}}
	if err := store.SetMailAPIRoute(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetMailAPIRoute(acc.ID)
	if err != nil || got == nil || got.State != MailRouteStateStaged || !got.Shared || got.Snapshot.ClientID != "client" || len(got.Snapshot.Scopes) != 2 {
		t.Fatalf("route not round-tripped: %+v %v", got, err)
	}
	want.Route, want.Shared = MailRouteGmailAPI, false
	if err := store.SetMailAPIRoute(want); err != nil {
		t.Fatal(err)
	}
	if got, _ = store.GetMailAPIRoute(acc.ID); got.Route != MailRouteGmailAPI || got.Shared {
		t.Fatalf("route not replaced: %+v", got)
	}
	for _, bad := range []MailAPIRoute{{AccountID: acc.ID, Route: "imap", ClientConfigID: "x"}, {Route: MailRouteGmailAPI, ClientConfigID: "x"}, {AccountID: acc.ID, Route: MailRouteGmailAPI}} {
		if store.SetMailAPIRoute(bad) == nil {
			t.Fatalf("accepted invalid route %+v", bad)
		}
	}
	if err := store.ClearMailAPIRoute(acc.ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := store.GetMailAPIRoute(acc.ID); r != nil {
		t.Fatal("route not cleared")
	}
	if err := store.SetMailAPIRoute(want); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(acc.ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := store.GetMailAPIRoute(acc.ID); r != nil {
		t.Fatal("route survived account deletion")
	}
}

func TestMailAPIRouteLinkedFields(t *testing.T) {
	store := NewStore(openTestDB(t))
	create := func(email string) *Account {
		cfg := validAccountConfig()
		cfg.Email, cfg.Username = email, email
		acc, err := store.Create(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return acc
	}
	parent, child := create("parent@example.com"), create("shared@example.com")
	base := MailAPIRoute{AccountID: child.ID, Route: MailRouteGraphAPI, ClientConfigID: "microsoft-mail-api", Shared: true}

	linked := base
	linked.MailboxAddress, linked.GrantAccountID = "  Shared@Example.COM ", parent.ID
	if err := store.SetMailAPIRoute(linked); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetMailAPIRoute(child.ID)
	if err != nil || got.MailboxAddress != "shared@example.com" || got.GrantAccountID != parent.ID || !got.Linked() || got.GrantOwnerID() != parent.ID {
		t.Fatalf("linked route %+v: %v", got, err)
	}
	ids, err := store.ListMailAPIRoutesGrantedBy(parent.ID)
	if err != nil || len(ids) != 1 || ids[0] != child.ID {
		t.Fatalf("dependents %v: %v", ids, err)
	}
	if ids, _ = store.ListMailAPIRoutesGrantedBy(child.ID); len(ids) != 0 {
		t.Fatalf("child owns no grant: %v", ids)
	}
	own := base
	own.Shared = false
	if r := (&own); r.Linked() || r.GrantOwnerID() != child.ID {
		t.Fatalf("own route must resolve to itself: %+v", r)
	}

	for name, bad := range map[string]MailAPIRoute{
		"target without owner":  {AccountID: child.ID, Route: MailRouteGraphAPI, ClientConfigID: "x", Shared: true, MailboxAddress: "s@example.com"},
		"owner without target":  {AccountID: child.ID, Route: MailRouteGraphAPI, ClientConfigID: "x", Shared: true, GrantAccountID: parent.ID},
		"self owner":            {AccountID: child.ID, Route: MailRouteGraphAPI, ClientConfigID: "x", Shared: true, MailboxAddress: "s@example.com", GrantAccountID: child.ID},
		"target without shared": {AccountID: child.ID, Route: MailRouteGraphAPI, ClientConfigID: "x", MailboxAddress: "s@example.com", GrantAccountID: parent.ID},
	} {
		if store.SetMailAPIRoute(bad) == nil {
			t.Errorf("%s: route accepted", name)
		}
	}
}
