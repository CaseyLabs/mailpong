package carddav

import (
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	"path/filepath"
	"testing"
)

func TestPasswordStorePreservesLegacyStorage(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	source, err := NewStore(db.DB).CreateSource(&SourceConfig{Name: "Test", Type: SourceTypeCardDAV})
	if err != nil {
		t.Fatal(err)
	}
	creds, err := credentials.NewStore(db.DB, dir)
	if err != nil {
		t.Fatal(err)
	}
	passwords := NewPasswordStore(db.DB, creds)
	if err = passwords.SetPassword(source.ID, "secret"); err != nil {
		t.Fatal(err)
	}
	got, err := passwords.GetPassword(source.ID)
	if err != nil || got != "secret" {
		t.Fatalf("password=%q,err=%v", got, err)
	}
	// A new adapter uses the same legacy keyring key and fallback column.
	got, err = NewPasswordStore(db.DB, creds).GetPassword(source.ID)
	if err != nil || got != "secret" {
		t.Fatalf("reopen=%q,err=%v", got, err)
	}
	if err = passwords.DeletePassword(source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = passwords.GetPassword(source.ID); err != credentials.ErrCredentialNotFound {
		t.Fatalf("deleted password error=%v", err)
	}
}
