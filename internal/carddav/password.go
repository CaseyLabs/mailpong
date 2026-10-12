package carddav

import (
	"database/sql"
	"github.com/caseylabs/mailven/internal/credentials"
)

// PasswordStore keeps legacy host-owned source passwords in place while using
// generic keyring/encryption primitives. It never opens an extension store.
type PasswordStore struct {
	db          *sql.DB
	credentials *credentials.Store
}

func NewPasswordStore(db *sql.DB, creds *credentials.Store) *PasswordStore {
	return &PasswordStore{db: db, credentials: creds}
}
func (s *PasswordStore) GetPassword(id string) (string, error) {
	return s.credentials.GetBackedPassword("carddav:"+id, passwordBacking{s.db, id})
}
func (s *PasswordStore) SetPassword(id, value string) error {
	return s.credentials.SetBackedPassword("carddav:"+id, value, passwordBacking{s.db, id})
}
func (s *PasswordStore) DeletePassword(id string) error {
	return s.credentials.DeleteBackedPassword("carddav:"+id, passwordBacking{s.db, id})
}

type passwordBacking struct {
	db *sql.DB
	id string
}

func (b passwordBacking) ReadEncryptedPassword() (string, error) {
	var encrypted sql.NullString
	err := b.db.QueryRow(`SELECT encrypted_password FROM contact_sources WHERE id=?`, b.id).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", credentials.ErrCredentialNotFound
	}
	return encrypted.String, err
}
func (b passwordBacking) WriteEncryptedPassword(value string) error {
	_, err := b.db.Exec(`UPDATE contact_sources SET encrypted_password=NULLIF(?,'') WHERE id=?`, value, b.id)
	return err
}
