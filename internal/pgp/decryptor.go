package pgp

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/caseylabs/mailpong/internal/credentials"
	"github.com/rs/zerolog"
)

// Decryptor handles PGP/MIME message decryption
type Decryptor struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewDecryptor creates a new PGP decryptor
func NewDecryptor(store *Store, credStore *credentials.Store, log zerolog.Logger) *Decryptor {
	return &Decryptor{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// DecryptBytes decrypts raw PGP-encrypted data using the account's PGP private key.
// Used for decrypting encrypted draft body data.
// recipientEmail narrows the key search to the matching identity; falls back to all keys.
func (d *Decryptor) DecryptBytes(accountID, recipientEmail string, encryptedData []byte) ([]byte, error) {
	// Try identity-specific key first, fall back to all account keys
	keyring, err := d.buildKeyringForEmail(accountID, recipientEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to build keyring: %w", err)
	}

	// Try to read as armored first, then binary
	var reader io.Reader
	block, err := armor.Decode(bytes.NewReader(encryptedData))
	if err == nil {
		reader = block.Body
	} else {
		reader = bytes.NewReader(encryptedData)
	}

	md, err := openpgp.ReadMessage(reader, keyring, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	decrypted, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return decrypted, nil
}

// DecryptMessage decrypts a PGP/MIME encrypted message (RFC 3156).
// recipientEmail narrows the key search to the matching identity; falls back to all keys.
// Returns the decrypted bytes (may be multipart/signed if sign-then-encrypt),
// a boolean indicating whether the message was encrypted, and any error.
func (d *Decryptor) DecryptMessage(accountID, recipientEmail string, raw []byte) ([]byte, bool, error) {
	// Parse the message to find Content-Type
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return nil, false, fmt.Errorf("cannot find header/body boundary")
	}

	headers := raw[:headerEnd]
	ct := extractHeaderValue(headers, "Content-Type")
	if ct == "" {
		return nil, false, nil
	}

	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, false, nil
	}

	// Check for PGP/MIME encrypted content
	if !strings.EqualFold(mediaType, "multipart/encrypted") {
		return nil, false, nil
	}
	protocol := params["protocol"]
	if !strings.EqualFold(protocol, "application/pgp-encrypted") {
		return nil, false, nil
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, true, fmt.Errorf("missing boundary parameter")
	}

	body := raw[bodyStart:]

	// Parse the multipart/encrypted structure
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	// Part 1: PGP/MIME version identification (skip it)
	if p, err := reader.NextPart(); err == nil {
		_, _ = io.Copy(io.Discard, p)
	}

	// Part 2: The encrypted data
	encPart, err := reader.NextPart()
	if err != nil {
		return nil, true, fmt.Errorf("failed to read encrypted part: %w", err)
	}
	encData, err := io.ReadAll(encPart)
	if err != nil {
		return nil, true, fmt.Errorf("failed to read encrypted data: %w", err)
	}

	// Try identity-specific key first, fall back to all account keys
	keyring, err := d.buildKeyringForEmail(accountID, recipientEmail)
	if err != nil {
		return nil, true, fmt.Errorf("failed to build keyring: %w", err)
	}

	// Try to read as armored first, then binary
	var encReader io.Reader
	block, armorErr := armor.Decode(bytes.NewReader(encData))
	if armorErr == nil {
		encReader = block.Body
	} else {
		encReader = bytes.NewReader(encData)
	}

	md, err := openpgp.ReadMessage(encReader, keyring, nil, nil)
	if err != nil {
		return nil, true, fmt.Errorf("failed to decrypt message: %w", err)
	}

	decrypted, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, true, fmt.Errorf("failed to read decrypted message: %w", err)
	}

	d.log.Info().Str("accountID", accountID).Msg("Successfully decrypted PGP message")
	return decrypted, true, nil
}

// buildKeyringForEmail builds a keyring preferring the key matching recipientEmail, falling back to all keys.
func (d *Decryptor) buildKeyringForEmail(accountID, recipientEmail string) (openpgp.EntityList, error) {
	if recipientEmail != "" {
		key, _, err := d.store.GetKeyByEmail(accountID, recipientEmail)
		if err == nil && key != nil {
			armoredPrivate, keyErr := d.credStore.GetPGPPrivateKey(key.ID)
			if keyErr == nil {
				entities, parseErr := ParseArmoredKey(string(armoredPrivate))
				if parseErr == nil && len(entities) > 0 {
					return entities, nil
				}
			}
		}
		d.log.Debug().Str("email", recipientEmail).Msg("No identity-specific PGP key found, falling back to all keys")
	}
	return d.buildPrivateKeyring(accountID)
}

// buildPrivateKeyring creates an openpgp.EntityList from all private keys for an account
func (d *Decryptor) buildPrivateKeyring(accountID string) (openpgp.EntityList, error) {
	keys, err := d.store.ListKeys(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	var keyring openpgp.EntityList
	for _, key := range keys {
		armoredPrivate, keyErr := d.credStore.GetPGPPrivateKey(key.ID)
		if keyErr != nil {
			d.log.Debug().Err(keyErr).Str("keyID", key.ID).Msg("Failed to get PGP private key")
			continue
		}

		entities, parseErr := ParseArmoredKey(string(armoredPrivate))
		if parseErr != nil {
			d.log.Debug().Err(parseErr).Str("keyID", key.ID).Msg("Failed to parse PGP private key")
			continue
		}

		keyring = append(keyring, entities...)
	}

	if len(keyring) == 0 {
		return nil, fmt.Errorf("no private keys found for account")
	}

	return keyring, nil
}
