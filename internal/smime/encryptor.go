package smime

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/caseylabs/mailpong/internal/credentials"
	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

// Encryptor handles S/MIME message encryption
type Encryptor struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewEncryptor creates a new S/MIME encryptor
func NewEncryptor(store *Store, credStore *credentials.Store, log zerolog.Logger) *Encryptor {
	return &Encryptor{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// EncryptBytes encrypts raw data using the sender's own S/MIME certificate (encrypt-to-self).
// Used for encrypting draft body data at rest.
// fromEmail selects the certificate matching the sender identity; falls back to the account default.
func (enc *Encryptor) EncryptBytes(accountID, fromEmail string, data []byte) ([]byte, error) {
	// Get sender's own certificate (identity-specific, then default)
	_, certPEM, err := enc.store.GetCertificateByEmail(accountID, fromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to look up certificate for %s: %w", fromEmail, err)
	}
	if certPEM == "" {
		_, certPEM, err = enc.store.GetDefaultCertificate(accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("no default certificate for account: %w", err)
	}

	senderCert, err := parseCertificateFromPEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender certificate: %w", err)
	}

	// Encrypt using PKCS#7 with AES-256-CBC
	pkcs7.ContentEncryptionAlgorithm = pkcs7.EncryptionAlgorithmAES256CBC
	encrypted, err := pkcs7.Encrypt(data, []*x509.Certificate{senderCert})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	return encrypted, nil
}

// EncryptMessageToSelf encrypts an RFC 822 message using only the sender's own certificate.
// Used for encrypting draft messages before syncing to IMAP.
// fromEmail selects the certificate matching the sender identity; falls back to the account default.
func (enc *Encryptor) EncryptMessageToSelf(accountID, fromEmail string, rawMsg []byte) ([]byte, error) {
	// Get sender's own certificate (identity-specific, then default)
	_, certPEM, err := enc.store.GetCertificateByEmail(accountID, fromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to look up certificate for %s: %w", fromEmail, err)
	}
	if certPEM == "" {
		_, certPEM, err = enc.store.GetDefaultCertificate(accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("no default certificate for account: %w", err)
	}

	senderCert, err := parseCertificateFromPEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender certificate: %w", err)
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

	// Encrypt using PKCS#7 with AES-256-CBC (to self only)
	pkcs7.ContentEncryptionAlgorithm = pkcs7.EncryptionAlgorithmAES256CBC
	encrypted, err := pkcs7.Encrypt(innerContent.Bytes(), []*x509.Certificate{senderCert})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	// Build the encrypted RFC 822 message
	var result bytes.Buffer

	// Write non-content headers from the original message
	writeFilteredHeaders(&result, originalHeaders)

	// Write the S/MIME encrypted Content-Type
	result.WriteString("Content-Type: application/pkcs7-mime;\r\n")
	result.WriteString("\tsmime-type=enveloped-data;\r\n")
	result.WriteString("\tname=\"smime.p7m\"\r\n")
	result.WriteString("Content-Transfer-Encoding: base64\r\n")
	result.WriteString("Content-Disposition: attachment; filename=\"smime.p7m\"\r\n")
	result.WriteString("\r\n")

	// Write base64-encoded encrypted data with 76-char line wrapping (RFC 2045)
	b64 := base64.StdEncoding.EncodeToString(encrypted)
	for i := 0; i < len(b64); i += 76 {
		end := i + 76
		if end > len(b64) {
			end = len(b64)
		}
		result.WriteString(b64[i:end] + "\r\n")
	}

	return result.Bytes(), nil
}

// EncryptMessage encrypts an RFC 822 message for the given recipients using S/MIME.
// The sender's own certificate is included so they can decrypt their own sent mail.
// fromEmail selects the certificate matching the sender identity for encrypt-to-self.
// Returns the encrypted RFC 822 message.
func (enc *Encryptor) EncryptMessage(accountID, fromEmail string, recipientEmails []string, rawMsg []byte) ([]byte, error) {
	// Collect recipient certificates
	certPEMs, err := enc.store.GetSenderCertPEMs(recipientEmails)
	if err != nil {
		return nil, fmt.Errorf("failed to look up recipient certificates: %w", err)
	}

	// Check that all recipients have certs
	var missing []string
	for _, email := range recipientEmails {
		if _, ok := certPEMs[email]; !ok {
			missing = append(missing, email)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no certificate for: %v", missing)
	}

	// Parse all recipient x509 certs
	var recipientCerts []*x509.Certificate
	for _, pemData := range certPEMs {
		cert, parseErr := parseCertificateFromPEM(pemData)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse recipient certificate: %w", parseErr)
		}
		recipientCerts = append(recipientCerts, cert)
	}

	// Include sender's own certificate so they can decrypt their own sent mail
	// Try identity-specific cert first, fall back to account default
	senderCert, senderPEM, _ := enc.store.GetCertificateByEmail(accountID, fromEmail)
	if senderCert == nil {
		senderCert, senderPEM, _ = enc.store.GetDefaultCertificate(accountID)
	}
	if senderCert != nil {
		ownCert, parseErr := parseCertificateFromPEM(senderPEM)
		if parseErr == nil {
			recipientCerts = append(recipientCerts, ownCert)
		}
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

	// Encrypt using PKCS#7 with AES-256-CBC
	pkcs7.ContentEncryptionAlgorithm = pkcs7.EncryptionAlgorithmAES256CBC
	encrypted, err := pkcs7.Encrypt(innerContent.Bytes(), recipientCerts)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	// Build the encrypted RFC 822 message
	var result bytes.Buffer

	// Write non-content headers from the original message
	writeFilteredHeaders(&result, originalHeaders)

	// Write the S/MIME encrypted Content-Type
	result.WriteString("Content-Type: application/pkcs7-mime;\r\n")
	result.WriteString("\tsmime-type=enveloped-data;\r\n")
	result.WriteString("\tname=\"smime.p7m\"\r\n")
	result.WriteString("Content-Transfer-Encoding: base64\r\n")
	result.WriteString("Content-Disposition: attachment; filename=\"smime.p7m\"\r\n")
	result.WriteString("\r\n")

	// Write base64-encoded encrypted data with 76-char line wrapping (RFC 2045)
	b64 := base64.StdEncoding.EncodeToString(encrypted)
	for i := 0; i < len(b64); i += 76 {
		end := i + 76
		if end > len(b64) {
			end = len(b64)
		}
		result.WriteString(b64[i:end] + "\r\n")
	}

	return result.Bytes(), nil
}
