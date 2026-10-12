package pgp

import (
	"bytes"
	"crypto/rand"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/rs/zerolog"
)

// Signer handles PGP/MIME signing (RFC 3156)
type Signer struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewSigner creates a new PGP signer
func NewSigner(store *Store, credStore *credentials.Store, log zerolog.Logger) *Signer {
	return &Signer{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// SignMessage wraps a raw RFC 822 message in a PGP/MIME multipart/signed structure.
// fromEmail selects the key matching the sender identity; falls back to the account default.
func (s *Signer) SignMessage(accountID, fromEmail string, rawMsg []byte) ([]byte, error) {
	// Try identity-specific key first, fall back to account default
	key, _, err := s.store.GetKeyByEmail(accountID, fromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to look up PGP key for %s: %w", fromEmail, err)
	}
	if key == nil {
		key, _, err = s.store.GetDefaultKey(accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("no default PGP key for account: %w", err)
	}

	// Get the private key
	armoredPrivate, err := s.credStore.GetPGPPrivateKey(key.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PGP private key: %w", err)
	}

	// Parse the private key
	entities, err := ParseArmoredKey(string(armoredPrivate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PGP private key: %w", err)
	}

	entity := entities[0]

	// Split the raw message into headers and body
	headerEnd := bytes.Index(rawMsg, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(rawMsg, []byte("\n\n"))
		if headerEnd == -1 {
			return nil, fmt.Errorf("failed to find header/body boundary")
		}
	}

	originalHeaders := rawMsg[:headerEnd]
	var bodyStart int
	if bytes.HasPrefix(rawMsg[headerEnd:], []byte("\r\n\r\n")) {
		bodyStart = headerEnd + 4
	} else {
		bodyStart = headerEnd + 2
	}
	originalBody := rawMsg[bodyStart:]

	// Extract the original Content-Type from headers
	originalContentType := extractHeader(originalHeaders, "Content-Type")
	if originalContentType == "" {
		originalContentType = "text/plain; charset=utf-8"
	}

	// Extract Content-Transfer-Encoding if present
	originalCTE := extractHeader(originalHeaders, "Content-Transfer-Encoding")

	// Build the inner body part (original content with its Content-Type)
	var innerPart bytes.Buffer
	innerPart.WriteString("Content-Type: " + originalContentType + "\r\n")
	if originalCTE != "" {
		innerPart.WriteString("Content-Transfer-Encoding: " + originalCTE + "\r\n")
	}
	innerPart.WriteString("\r\n")
	innerPart.Write(originalBody)

	innerPartBytes := innerPart.Bytes()

	// Create the detached PGP signature
	var sigBuf bytes.Buffer
	armorWriter, err := armor.Encode(&sigBuf, "PGP SIGNATURE", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create armor writer: %w", err)
	}

	if err := openpgp.DetachSignText(armorWriter, entity, bytes.NewReader(innerPartBytes), nil); err != nil {
		return nil, fmt.Errorf("failed to create detached signature: %w", err)
	}
	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close armor writer: %w", err)
	}

	// Build the multipart/signed message
	boundary, err := generateBoundary()
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer

	// Write non-content headers from the original message
	writeFilteredHeaders(&result, originalHeaders)

	// Write the multipart/signed Content-Type header (RFC 3156)
	result.WriteString("Content-Type: multipart/signed;\r\n")
	result.WriteString("\tprotocol=\"application/pgp-signature\";\r\n")
	result.WriteString("\tmicalg=pgp-sha256;\r\n")
	result.WriteString(fmt.Sprintf("\tboundary=\"%s\"\r\n", boundary))
	result.WriteString("\r\n")

	// First part: the exact bytes that were signed
	result.WriteString("--" + boundary + "\r\n")
	result.Write(innerPartBytes)
	result.WriteString("\r\n")

	// Second part: the detached PGP signature
	result.WriteString("--" + boundary + "\r\n")
	result.WriteString("Content-Type: application/pgp-signature; name=\"signature.asc\"\r\n")
	result.WriteString("Content-Disposition: attachment; filename=\"signature.asc\"\r\n")
	result.WriteString("Content-Description: OpenPGP digital signature\r\n")
	result.WriteString("\r\n")
	result.Write(sigBuf.Bytes())
	result.WriteString("\r\n")

	// Closing boundary
	result.WriteString("--" + boundary + "--\r\n")

	return result.Bytes(), nil
}

// generateBoundary creates a random MIME boundary string
func generateBoundary() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate boundary: %w", err)
	}
	return fmt.Sprintf("----=_pgp_%x", buf), nil
}
