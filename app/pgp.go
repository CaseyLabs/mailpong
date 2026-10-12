package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/pgp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PickPGPKeyFile opens a file picker for PGP key files (.asc, .gpg, .key, .pub)
func (a *App) PickPGPKeyFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select PGP Key File",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "PGP Key Files (*.asc, *.gpg, *.key, *.pub)",
				Pattern:     "*.asc;*.gpg;*.key;*.pub",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to open file dialog: %w", err)
	}
	return path, nil
}

// ImportPGPKeyFromPath imports a PGP keypair from a file path with optional passphrase
func (a *App) ImportPGPKeyFromPath(accountID, filePath, passphrase string) (*pgp.ImportResult, error) {
	if filePath == "" {
		return nil, fmt.Errorf("no file selected")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	armoredPrivate, armoredPublic, key, err := pgp.ImportKey(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to import key: %w", err)
	}

	key.AccountID = accountID

	// Validate that the key email matches the account or one of its aliases
	if err := a.validateKeyEmailForAccount(accountID, key.Email); err != nil {
		return nil, err
	}

	// Check if this is the first key for this account (make it default)
	existing, err := a.pgpStore.ListKeys(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing keys: %w", err)
	}
	if len(existing) == 0 {
		key.IsDefault = true
	}

	// Store the key metadata and public key
	if err := a.pgpStore.SaveKey(key, armoredPublic); err != nil {
		return nil, fmt.Errorf("failed to save key: %w", err)
	}

	// Store the private key securely
	hasPrivate := armoredPrivate != ""
	if hasPrivate {
		if err := a.credStore.SetPGPPrivateKey(key.ID, []byte(armoredPrivate)); err != nil {
			// Rollback: delete the key if private key storage fails
			if delErr := a.pgpStore.DeleteKey(key.ID); delErr != nil {
				log := logging.WithComponent("app")
				log.Warn().Err(delErr).Str("keyID", key.ID).Msg("Failed to roll back PGP key after private key storage failure")
			}
			return nil, fmt.Errorf("failed to store private key: %w", err)
		}
	}

	// If this is the first key, set it as default on the account
	if key.IsDefault {
		if err := a.pgpStore.SetDefaultKey(accountID, key.ID); err != nil {
			log := logging.WithComponent("app")
			log.Warn().Err(err).Str("keyID", key.ID).Msg("Failed to set default PGP key")
		}
	}

	return &pgp.ImportResult{
		Key:        key,
		HasPrivate: hasPrivate,
	}, nil
}

// ListPGPKeys returns all PGP keys for an account
func (a *App) ListPGPKeys(accountID string) ([]*pgp.Key, error) {
	keys, err := a.pgpStore.ListKeys(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	if keys == nil {
		return []*pgp.Key{}, nil
	}
	return keys, nil
}

// DeletePGPKey removes a PGP key and its private key
func (a *App) DeletePGPKey(keyID string) error {
	// Delete private key first
	if err := a.credStore.DeletePGPPrivateKey(keyID); err != nil {
		return fmt.Errorf("failed to delete private key: %w", err)
	}

	// Delete key
	if err := a.pgpStore.DeleteKey(keyID); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

// SetDefaultPGPKey sets the default signing key for an account
func (a *App) SetDefaultPGPKey(accountID, keyID string) error {
	return a.pgpStore.SetDefaultKey(accountID, keyID)
}

// GetPGPSignPolicy returns the PGP signing policy for an account
func (a *App) GetPGPSignPolicy(accountID string) (string, error) {
	return a.pgpStore.GetSignPolicy(accountID)
}

// SetPGPSignPolicy sets the PGP signing policy ('never', 'always')
func (a *App) SetPGPSignPolicy(accountID, policy string) error {
	switch policy {
	case "never", "always":
		return a.pgpStore.SetSignPolicy(accountID, policy)
	default:
		return fmt.Errorf("invalid sign policy: %s", policy)
	}
}

// GetPGPEncryptPolicy returns the PGP encryption policy for an account
func (a *App) GetPGPEncryptPolicy(accountID string) (string, error) {
	return a.pgpStore.GetEncryptPolicy(accountID)
}

// SetPGPEncryptPolicy sets the PGP encryption policy ('never', 'always')
func (a *App) SetPGPEncryptPolicy(accountID, policy string) error {
	switch policy {
	case "never", "always":
		return a.pgpStore.SetEncryptPolicy(accountID, policy)
	default:
		return fmt.Errorf("invalid encrypt policy: %s", policy)
	}
}

// ListPGPSenderKeys returns all cached PGP sender public keys
func (a *App) ListPGPSenderKeys() ([]*pgp.SenderKey, error) {
	keys, err := a.pgpStore.ListAllSenderKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to list sender keys: %w", err)
	}
	if keys == nil {
		return []*pgp.SenderKey{}, nil
	}
	return keys, nil
}

// DeletePGPSenderKey removes a cached sender public key
func (a *App) DeletePGPSenderKey(keyID string) error {
	return a.pgpStore.DeleteSenderKey(keyID)
}

// HasPGPKey returns whether an account has a default PGP key configured
func (a *App) HasPGPKey(accountID string) bool {
	return a.composeOps.hasPGPKey(accountID)
}

// GetPGPKeyForEmail returns the PGP key matching the given email.
// Returns nil if no matching key is found.
func (a *App) GetPGPKeyForEmail(accountID, email string) (*pgp.Key, error) {
	key, _, err := a.pgpStore.GetKeyByEmail(accountID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get key for email: %w", err)
	}
	return key, nil
}

// CheckRecipientPGPKeys checks which recipients have PGP public keys available
func (a *App) CheckRecipientPGPKeys(emails []string) (map[string]bool, error) {
	armoredKeys, err := a.pgpStore.GetSenderKeyArmoreds(emails)
	if err != nil {
		return nil, fmt.Errorf("failed to check recipient keys: %w", err)
	}

	result := make(map[string]bool)
	for _, email := range emails {
		_, hasKey := armoredKeys[email]
		result[email] = hasKey
	}
	return result, nil
}

// PickRecipientPGPKeyFile opens a file picker for PGP public key files
func (a *App) PickRecipientPGPKeyFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Recipient PGP Public Key",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "PGP Key Files (*.asc, *.gpg, *.key, *.pub)",
				Pattern:     "*.asc;*.gpg;*.key;*.pub",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to open file dialog: %w", err)
	}
	return path, nil
}

// ImportRecipientPGPKey imports a recipient's PGP public key from a file
func (a *App) ImportRecipientPGPKey(email, filePath string) error {
	if filePath == "" {
		return fmt.Errorf("no file selected")
	}
	if email == "" {
		return fmt.Errorf("email address required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	return a.pgpStore.ImportSenderKeyFromFile(email, data)
}

// LookupWKD performs a Web Key Directory lookup for the given email address
// and caches the result if a key is found
func (a *App) LookupWKD(email string) (string, error) {
	armored, err := pgp.LookupWKD(email)
	if err != nil {
		return "", fmt.Errorf("WKD lookup failed: %w", err)
	}
	if armored == "" {
		return "", nil
	}

	// Cache the discovered key
	if err := a.pgpStore.CacheSenderKey(email, armored, "wkd"); err != nil {
		// Don't fail — we still return the key
		return armored, nil
	}

	return armored, nil
}

// LookupHKP performs an HKP key server lookup for the given email address
// and caches the result if a key is found
func (a *App) LookupHKP(email string) (string, error) {
	armored, err := pgp.LookupHKP(email, a.getHKPServers())
	if err != nil {
		return "", fmt.Errorf("HKP lookup failed: %w", err)
	}
	if armored == "" {
		return "", nil
	}

	// Cache the discovered key
	if err := a.pgpStore.CacheSenderKey(email, armored, "hkp"); err != nil {
		return armored, nil
	}

	return armored, nil
}

// LookupPGPKey performs a unified WKD+HKP lookup for the given email address
// and caches the result if a key is found. WKD is tried first, then HKP.
func (a *App) LookupPGPKey(email string) (string, error) {
	result, err := pgp.LookupKey(email, a.getHKPServers())
	if err != nil {
		return "", fmt.Errorf("PGP key lookup failed: %w", err)
	}
	if result == nil {
		return "", nil
	}

	// Cache the discovered key with the source that found it
	if err := a.pgpStore.CacheSenderKey(email, result.Armored, result.Source); err != nil {
		return result.Armored, nil
	}

	return result.Armored, nil
}

// getHKPServers reads configured key servers from the database table.
// Falls back to DefaultHKPServers if the table is empty.
func (a *App) getHKPServers() []string {
	servers, err := a.pgpStore.ListKeyServers()
	if err != nil || len(servers) == 0 {
		return pgp.DefaultHKPServers
	}

	urls := make([]string, len(servers))
	for i, s := range servers {
		urls[i] = s.URL
	}
	return urls
}

// GetPGPKeyServers returns all configured PGP key servers
func (a *App) GetPGPKeyServers() ([]pgp.KeyServer, error) {
	servers, err := a.pgpStore.ListKeyServers()
	if err != nil {
		return nil, fmt.Errorf("failed to list key servers: %w", err)
	}
	if servers == nil {
		return []pgp.KeyServer{}, nil
	}
	return servers, nil
}

// AddPGPKeyServer adds a new key server URL
func (a *App) AddPGPKeyServer(url string) error {
	return a.pgpStore.AddKeyServer(url)
}

// RemovePGPKeyServer removes a key server by ID
func (a *App) RemovePGPKeyServer(id int) error {
	return a.pgpStore.RemoveKeyServer(id)
}

// validateKeyEmailForAccount checks that the given email matches the account email
// or one of its identity aliases. Returns an error if no match is found.
func (a *App) validateKeyEmailForAccount(accountID, keyEmail string) error {
	if keyEmail == "" {
		return nil // No email to validate
	}

	keyEmailLower := strings.ToLower(strings.TrimSpace(keyEmail))

	// Check account email
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	if acc != nil && strings.ToLower(strings.TrimSpace(acc.Email)) == keyEmailLower {
		return nil
	}

	// Check identity emails
	identities, err := a.accountStore.GetIdentities(accountID)
	if err != nil {
		return fmt.Errorf("failed to get identities: %w", err)
	}
	for _, id := range identities {
		if strings.ToLower(strings.TrimSpace(id.Email)) == keyEmailLower {
			return nil
		}
	}

	return fmt.Errorf("key email %q does not match this account or any of its aliases", keyEmail)
}
