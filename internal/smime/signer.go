package smime

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/caseylabs/mailpong/internal/credentials"
	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

// Signer handles CMS SignedData creation for S/MIME
type Signer struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewSigner creates a new S/MIME signer
func NewSigner(store *Store, credStore *credentials.Store, log zerolog.Logger) *Signer {
	return &Signer{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// SignMessage wraps a raw RFC 822 message in a multipart/signed structure
// with a CMS detached signature (clear signing).
// fromEmail selects the certificate matching the sender identity; falls back to the account default.
func (s *Signer) SignMessage(accountID, fromEmail string, rawMsg []byte) ([]byte, error) {
	// Try identity-specific certificate first, fall back to account default
	cert, certChainPEM, err := s.store.GetCertificateByEmail(accountID, fromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to look up certificate for %s: %w", fromEmail, err)
	}
	if cert == nil {
		cert, certChainPEM, err = s.store.GetDefaultCertificate(accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("no default certificate for account: %w", err)
	}

	// Parse the certificate chain
	certs, err := ParseCertChainFromPEM(certChainPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate chain: %w", err)
	}

	// Get the private key
	privateKeyPEM, err := s.credStore.GetSMIMEPrivateKey(cert.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	// Parse the private key
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not implement crypto.Signer")
	}

	// Split the raw message into headers and body
	headerEnd := bytes.Index(rawMsg, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(rawMsg, []byte("\n\n"))
		if headerEnd == -1 {
			return nil, fmt.Errorf("failed to find header/body boundary")
		}
	}

	originalHeaders := rawMsg[:headerEnd]
	// Body starts after the blank line separator
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

	// Create the CMS detached signature
	signedData, err := pkcs7.NewSignedData(innerPartBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed data: %w", err)
	}

	// Use SHA-256 instead of the default SHA-1 (rejected by modern NSS/GnuTLS clients)
	signedData.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)

	// Add the signer with the certificate chain
	if err := signedData.AddSigner(certs[0], signer, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("failed to add signer: %w", err)
	}

	// Add intermediate certificates
	for _, intermediateCert := range certs[1:] {
		signedData.AddCertificate(intermediateCert)
	}

	// Detach the content (for clear signing)
	signedData.Detach()

	// Finish and get the DER-encoded signature
	derSignature, err := signedData.Finish()
	if err != nil {
		return nil, fmt.Errorf("failed to finish signing: %w", err)
	}

	// Build the multipart/signed message manually.
	// We MUST NOT use multipart.Writer for the first part because Go's
	// textproto.MIMEHeader iterates map keys in unspecified order, which
	// would produce different header ordering than innerPartBytes. The raw
	// bytes of the first part must exactly match what was signed.
	boundary, err := generateBoundary()
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer

	// Write non-content headers from the original message
	writeFilteredHeaders(&result, originalHeaders)

	// Write the multipart/signed Content-Type header
	result.WriteString("Content-Type: multipart/signed;\r\n")
	result.WriteString("\tprotocol=\"application/pkcs7-signature\";\r\n")
	result.WriteString("\tmicalg=sha-256;\r\n")
	result.WriteString(fmt.Sprintf("\tboundary=\"%s\"\r\n", boundary))
	result.WriteString("\r\n")

	// First part: the exact bytes that were signed (headers + blank line + body)
	result.WriteString("--" + boundary + "\r\n")
	result.Write(innerPartBytes)
	result.WriteString("\r\n")

	// Second part: the detached CMS signature
	result.WriteString("--" + boundary + "\r\n")
	result.WriteString("Content-Type: application/pkcs7-signature; name=\"smime.p7s\"\r\n")
	result.WriteString("Content-Transfer-Encoding: base64\r\n")
	result.WriteString("Content-Disposition: attachment; filename=\"smime.p7s\"\r\n")
	result.WriteString("\r\n")

	// Write base64-encoded signature with 76-char line wrapping (RFC 2045)
	b64 := base64.StdEncoding.EncodeToString(derSignature)
	for i := 0; i < len(b64); i += 76 {
		end := i + 76
		if end > len(b64) {
			end = len(b64)
		}
		result.WriteString(b64[i:end] + "\r\n")
	}

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
	return fmt.Sprintf("----=_smime_%x", buf), nil
}

// extractHeader extracts a header value from raw headers
func extractHeader(headers []byte, name string) string {
	lines := strings.Split(string(headers), "\n")
	lowerName := strings.ToLower(name)

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		headerName := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		if headerName != lowerName {
			continue
		}

		value := strings.TrimSpace(line[colonIdx+1:])

		// Handle multi-line headers (continuation lines start with whitespace)
		for j := i + 1; j < len(lines); j++ {
			nextLine := strings.TrimRight(lines[j], "\r")
			if len(nextLine) == 0 {
				break
			}
			if nextLine[0] == ' ' || nextLine[0] == '\t' {
				value += " " + strings.TrimSpace(nextLine)
			} else {
				break
			}
		}

		return value
	}
	return ""
}

// writeFilteredHeaders writes headers from the original message, excluding
// Content-Type and Content-Transfer-Encoding (which will be replaced)
func writeFilteredHeaders(buf *bytes.Buffer, headers []byte) {
	lines := strings.Split(string(headers), "\n")
	skipHeaders := map[string]bool{
		"content-type":              true,
		"content-transfer-encoding": true,
		"mime-version":              true,
	}

	skipContinuation := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}

		// Check if this is a continuation line
		if line[0] == ' ' || line[0] == '\t' {
			if skipContinuation {
				continue
			}
			buf.WriteString(line + "\r\n")
			continue
		}

		// Check if this header should be skipped
		colonIdx := strings.Index(line, ":")
		if colonIdx != -1 {
			headerName := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
			if skipHeaders[headerName] {
				skipContinuation = true
				continue
			}
		}

		skipContinuation = false
		buf.WriteString(line + "\r\n")
	}

	// Always add MIME-Version for signed messages
	buf.WriteString("MIME-Version: 1.0\r\n")
}
