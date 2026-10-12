package pgp

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/rs/zerolog"
)

// Encryptor handles PGP/MIME message encryption
type Encryptor struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewEncryptor creates a new PGP encryptor
func NewEncryptor(store *Store, credStore *credentials.Store, log zerolog.Logger) *Encryptor {
	return &Encryptor{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// EncryptBytes encrypts raw data using the sender's own PGP public key (encrypt-to-self).
// Used for encrypting draft body data at rest.
// fromEmail selects the key matching the sender identity; falls back to the account default.
func (enc *Encryptor) EncryptBytes(accountID, fromEmail string, data []byte) ([]byte, error) {
	// Get sender's own key (identity-specific, then default)
	_, pubArmored, err := enc.store.GetKeyByEmail(accountID, fromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to look up PGP key for %s: %w", fromEmail, err)
	}
	if pubArmored == "" {
		_, pubArmored, err = enc.store.GetDefaultKey(accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("no default PGP key for account: %w", err)
	}

	entities, err := ParseArmoredKey(pubArmored)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender public key: %w", err)
	}

	// Encrypt
	var encrypted bytes.Buffer
	w, err := openpgp.Encrypt(&encrypted, entities, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write encrypted data: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encryption writer: %w", err)
	}

	return encrypted.Bytes(), nil
}

// EncryptMessage encrypts an RFC 822 message for the given recipients using PGP/MIME (RFC 3156).
// The sender's own key is included so they can decrypt their own sent mail.
// fromEmail selects the key matching the sender identity for encrypt-to-self.
func (enc *Encryptor) EncryptMessage(accountID, fromEmail string, recipientEmails []string, rawMsg []byte) ([]byte, error) {
	var recipientEntities openpgp.EntityList

	// Collect recipient public keys (if any)
	if len(recipientEmails) > 0 {
		recipientArmoreds, err := enc.store.GetSenderKeyArmoreds(recipientEmails)
		if err != nil {
			return nil, fmt.Errorf("failed to look up recipient keys: %w", err)
		}

		// Check that all recipients have keys
		var missing []string
		for _, email := range recipientEmails {
			if _, ok := recipientArmoreds[email]; !ok {
				missing = append(missing, email)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("no PGP key for: %v", missing)
		}

		// Parse all recipient entities
		for _, armored := range recipientArmoreds {
			entities, parseErr := ParseArmoredKey(armored)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse recipient key: %w", parseErr)
			}
			recipientEntities = append(recipientEntities, entities...)
		}
	}

	// Include sender's own key so they can decrypt their own sent mail
	// Try identity-specific key first, fall back to account default
	_, senderArmored, _ := enc.store.GetKeyByEmail(accountID, fromEmail)
	if senderArmored == "" {
		_, senderArmored, _ = enc.store.GetDefaultKey(accountID)
	}
	if senderArmored != "" {
		senderEntities, parseErr := ParseArmoredKey(senderArmored)
		if parseErr == nil {
			recipientEntities = append(recipientEntities, senderEntities...)
		}
	}

	if len(recipientEntities) == 0 {
		return nil, fmt.Errorf("no PGP keys available for encryption")
	}

	// Split the raw message into headers and body
	headerEnd := bytes.Index(rawMsg, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(rawMsg, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return nil, fmt.Errorf("failed to find header/body boundary")
	}

	originalHeaders := rawMsg[:headerEnd]
	messageBody := rawMsg[bodyStart:]

	// Build the inner content to encrypt (Content-Type + body)
	originalContentType := extractHeader(originalHeaders, "Content-Type")
	if originalContentType == "" {
		originalContentType = "text/plain; charset=utf-8"
	}
	originalCTE := extractHeader(originalHeaders, "Content-Transfer-Encoding")

	var innerContent bytes.Buffer
	innerContent.WriteString("Content-Type: " + originalContentType + "\r\n")
	if originalCTE != "" {
		innerContent.WriteString("Content-Transfer-Encoding: " + originalCTE + "\r\n")
	}
	innerContent.WriteString("\r\n")
	innerContent.Write(messageBody)

	// Encrypt the inner content
	var encryptedBuf bytes.Buffer
	armorWriter, err := armor.Encode(&encryptedBuf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create armor writer: %w", err)
	}

	w, err := openpgp.Encrypt(armorWriter, recipientEntities, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption writer: %w", err)
	}
	if _, err := io.Copy(w, bytes.NewReader(innerContent.Bytes())); err != nil {
		return nil, fmt.Errorf("failed to write encrypted content: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encryption writer: %w", err)
	}
	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close armor writer: %w", err)
	}

	// Build the PGP/MIME encrypted message (RFC 3156)
	boundary, err := generateEncryptedBoundary()
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer

	// Write non-content headers from the original message
	writeFilteredHeaders(&result, originalHeaders)

	// Write the multipart/encrypted Content-Type header
	result.WriteString("Content-Type: multipart/encrypted;\r\n")
	result.WriteString("\tprotocol=\"application/pgp-encrypted\";\r\n")
	result.WriteString(fmt.Sprintf("\tboundary=\"%s\"\r\n", boundary))
	result.WriteString("\r\n")

	// Part 1: PGP/MIME version identification
	result.WriteString("--" + boundary + "\r\n")
	result.WriteString("Content-Type: application/pgp-encrypted\r\n")
	result.WriteString("Content-Description: PGP/MIME version identification\r\n")
	result.WriteString("\r\n")
	result.WriteString("Version: 1\r\n")
	result.WriteString("\r\n")

	// Part 2: Encrypted data
	result.WriteString("--" + boundary + "\r\n")
	result.WriteString("Content-Type: application/octet-stream; name=\"encrypted.asc\"\r\n")
	result.WriteString("Content-Disposition: inline; filename=\"encrypted.asc\"\r\n")
	result.WriteString("Content-Description: OpenPGP encrypted message\r\n")
	result.WriteString("\r\n")
	result.Write(encryptedBuf.Bytes())
	result.WriteString("\r\n")

	// Closing boundary
	result.WriteString("--" + boundary + "--\r\n")

	return result.Bytes(), nil
}

// EncryptMessageToSelf encrypts an RFC 822 message using only the sender's own key.
// Used for encrypting draft messages before syncing to IMAP.
// fromEmail selects the key matching the sender identity; falls back to the account default.
func (enc *Encryptor) EncryptMessageToSelf(accountID, fromEmail string, rawMsg []byte) ([]byte, error) {
	return enc.EncryptMessage(accountID, fromEmail, nil, rawMsg)
}

// generateEncryptedBoundary creates a random MIME boundary for encrypted messages
func generateEncryptedBoundary() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate boundary: %w", err)
	}
	return fmt.Sprintf("----=_pgpenc_%x", buf), nil
}
