package credentials

import (
	"errors"
	"path/filepath"
	"testing"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/caseylabs/mailpong/internal/database"
)

func TestFailedKeyringWriteDoesNotLeaveStaleValue(t *testing.T) {
	gokeyring.MockInit()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := NewStore(db.DB, tmp)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if !store.keyringEnabled {
		t.Fatal("mock keyring not enabled")
	}
	insertTestAccount(t, db, "acct")

	tests := []struct {
		name string
		set  func(value string) error
		get  func() (string, error)
	}{
		{"password", func(v string) error { return store.SetPassword("acct", v) }, func() (string, error) { return store.GetPassword("acct") }},
		{"smtp password", func(v string) error { return store.SetSMTPPassword("acct", v) }, func() (string, error) { return store.GetSMTPPassword("acct") }},
		{"access token", func(v string) error { return store.setOAuthAccessToken("acct", v) }, func() (string, error) { return store.getOAuthAccessToken("acct") }},
		{"refresh token", func(v string) error { return store.setOAuthRefreshToken("acct", v) }, func() (string, error) { return store.getOAuthRefreshToken("acct") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyringSet = gokeyring.Set
			if err := tc.set("old"); err != nil {
				t.Fatalf("first set: %v", err)
			}

			keyringSet = func(string, string, string) error { return errors.New("keyring locked") }
			t.Cleanup(func() { keyringSet = gokeyring.Set })
			if err := tc.set("new"); err != nil {
				t.Fatalf("fallback set: %v", err)
			}

			got, err := tc.get()
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got != "new" {
				t.Fatalf("got %q after a fallback write, want %q", got, "new")
			}
		})
	}
}
