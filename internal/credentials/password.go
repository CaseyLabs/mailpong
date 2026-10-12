package credentials

import (
	"fmt"
	gokeyring "github.com/zalando/go-keyring"
)

// PasswordBacking stores encrypted fallback values. The owner chooses its
// existing persistence location; the credential store has no domain schema.
type PasswordBacking interface {
	ReadEncryptedPassword() (string, error)
	WriteEncryptedPassword(value string) error
}

func (s *Store) SetBackedPassword(key, value string, backing PasswordBacking) error {
	if value == "" {
		return nil
	}
	if s.keyringEnabled {
		if err := s.setInKeyring(key, value); err == nil {
			return backing.WriteEncryptedPassword("")
		}
	}
	encrypted, err := s.encryptor.Encrypt(value)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	return backing.WriteEncryptedPassword(encrypted)
}
func (s *Store) GetBackedPassword(key string, backing PasswordBacking) (string, error) {
	if s.keyringEnabled {
		if value, err := gokeyring.Get(serviceName, key); err == nil {
			return value, nil
		}
	}
	encrypted, err := backing.ReadEncryptedPassword()
	if err != nil {
		return "", err
	}
	if encrypted == "" {
		return "", ErrCredentialNotFound
	}
	return s.encryptor.Decrypt(encrypted)
}
func (s *Store) DeleteBackedPassword(key string, backing PasswordBacking) error {
	if s.keyringEnabled {
		_ = gokeyring.Delete(serviceName, key)
	}
	return backing.WriteEncryptedPassword("")
}
