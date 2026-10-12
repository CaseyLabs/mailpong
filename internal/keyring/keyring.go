// Package keyring provides secure credential storage using the OS keychain
package keyring

import (
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "mailven"

// Keyring provides secure credential storage
type Keyring struct{}

// New creates a new Keyring instance
func New() *Keyring {
	return &Keyring{}
}

// SetPassword stores a password for an account
func (k *Keyring) SetPassword(accountID, password string) error {
	err := gokeyring.Set(serviceName, accountID, password)
	if err != nil {
		return fmt.Errorf("failed to store password: %w", err)
	}
	return nil
}

// GetPassword retrieves a password for an account
func (k *Keyring) GetPassword(accountID string) (string, error) {
	password, err := gokeyring.Get(serviceName, accountID)
	if err == gokeyring.ErrNotFound {
		return "", ErrCredentialNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to retrieve password: %w", err)
	}
	return password, nil
}

// DeletePassword removes a password for an account
func (k *Keyring) DeletePassword(accountID string) error {
	err := gokeyring.Delete(serviceName, accountID)
	if err == gokeyring.ErrNotFound {
		return nil // Already deleted, not an error
	}
	if err != nil {
		return fmt.Errorf("failed to delete password: %w", err)
	}
	return nil
}

// SetOAuthTokens stores OAuth2 tokens for an account
func (k *Keyring) SetOAuthTokens(accountID, accessToken, refreshToken string) error {
	// Store access token
	if err := gokeyring.Set(serviceName, accountID+":access_token", accessToken); err != nil {
		return fmt.Errorf("failed to store access token: %w", err)
	}
	// Store refresh token
	if err := gokeyring.Set(serviceName, accountID+":refresh_token", refreshToken); err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}
	return nil
}

// GetOAuthTokens retrieves OAuth2 tokens for an account
func (k *Keyring) GetOAuthTokens(accountID string) (accessToken, refreshToken string, err error) {
	accessToken, err = gokeyring.Get(serviceName, accountID+":access_token")
	if err == gokeyring.ErrNotFound {
		return "", "", ErrCredentialNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve access token: %w", err)
	}

	refreshToken, err = gokeyring.Get(serviceName, accountID+":refresh_token")
	if err == gokeyring.ErrNotFound {
		return "", "", ErrCredentialNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// DeleteOAuthTokens removes OAuth2 tokens for an account
func (k *Keyring) DeleteOAuthTokens(accountID string) error {
	_ = gokeyring.Delete(serviceName, accountID+":access_token")
	_ = gokeyring.Delete(serviceName, accountID+":refresh_token")
	return nil
}

// DeleteAllCredentials removes all credentials for an account
func (k *Keyring) DeleteAllCredentials(accountID string) error {
	_ = k.DeletePassword(accountID)
	_ = k.DeleteOAuthTokens(accountID)
	return nil
}
