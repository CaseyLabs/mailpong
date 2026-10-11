package smime

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"mime"
	"strings"

	"github.com/caseylabs/mailpong/internal/credentials"
	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

// Decryptor handles S/MIME message decryption
type Decryptor struct {
	store     *Store
	credStore *credentials.Store
	log       zerolog.Logger
}

// NewDecryptor creates a new S/MIME decryptor
func NewDecryptor(store *Store, credStore *credentials.Store, log zerolog.Logger) *Decryptor {
	return &Decryptor{
		store:     store,
		credStore: credStore,
		log:       log,
	}
}

// DecryptBytes decrypts raw PKCS#7 DER data using the account's S/MIME private key.
// Used for decrypting encrypted draft body data.
// recipientEmail narrows the key search to the matching identity; falls back to trying all keys.
func (d *Decryptor) DecryptBytes(accountID, recipientEmail string, encryptedData []byte) ([]byte, error) {
	p7, err := pkcs7.Parse(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS#7 data: %w", err)
	}

	// Try identity-specific certificate first
	if recipientEmail != "" {
		decrypted, err := d.tryDecryptWithEmail(p7, accountID, recipientEmail)
		if err == nil {
			return decrypted, nil
		}
		d.log.Debug().Err(err).Str("email", recipientEmail).Msg("Identity-specific decryption failed, trying all certs")
	}

	// Fall back to trying all certificates for this account
	certs, err := d.store.ListCertificates(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	for _, cert := range certs {
		_, certChainPEM, getErr := d.store.GetCertificate(cert.ID)
		if getErr != nil {
			continue
		}

		x509Cert, parseErr := parseCertificateFromPEM(certChainPEM)
		if parseErr != nil {
			continue
		}

		privateKeyPEM, keyErr := d.credStore.GetSMIMEPrivateKey(cert.ID)
		if keyErr != nil {
			continue
		}

		block, _ := pem.Decode(privateKeyPEM)
		if block == nil {
			continue
		}

		privateKey, keyParseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if keyParseErr != nil {
			continue
		}

		decrypted, decryptErr := p7.Decrypt(x509Cert, privateKey)
		if decryptErr != nil {
			continue
		}

		return decrypted, nil
	}

	return nil, fmt.Errorf("no matching private key found for decryption")
}

// tryDecryptWithEmail attempts decryption using the certificate matching the given email.
func (d *Decryptor) tryDecryptWithEmail(p7 *pkcs7.PKCS7, accountID, email string) ([]byte, error) {
	cert, certChainPEM, err := d.store.GetCertificateByEmail(accountID, email)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("no certificate for email %s", email)
	}

	x509Cert, err := parseCertificateFromPEM(certChainPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	privateKeyPEM, err := d.credStore.GetSMIMEPrivateKey(cert.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return p7.Decrypt(x509Cert, privateKey)
}

// DecryptMessage decrypts an S/MIME encrypted message.
// recipientEmail narrows the key search to the matching identity; falls back to trying all keys.
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

	// Check for S/MIME encrypted content
	isEncrypted := (strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime")) &&
		strings.EqualFold(params["smime-type"], "enveloped-data")

	if !isEncrypted {
		return nil, false, nil
	}

	body := raw[bodyStart:]

	// Try parsing as DER first; if that fails, base64-decode and retry
	p7, err := pkcs7.Parse(body)
	if err != nil {
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, body)
		decoded, decErr := base64.StdEncoding.DecodeString(string(cleaned))
		if decErr != nil {
			return nil, true, fmt.Errorf("failed to decode encrypted body: %w", err)
		}
		p7, err = pkcs7.Parse(decoded)
		if err != nil {
			return nil, true, fmt.Errorf("failed to parse PKCS#7 encrypted data: %w", err)
		}
	}

	// Try identity-specific certificate first
	if recipientEmail != "" {
		decrypted, decErr := d.tryDecryptWithEmail(p7, accountID, recipientEmail)
		if decErr == nil {
			d.log.Info().Str("email", recipientEmail).Msg("Successfully decrypted S/MIME message with identity cert")
			return decrypted, true, nil
		}
		d.log.Debug().Err(decErr).Str("email", recipientEmail).Msg("Identity-specific decryption failed, trying all certs")
	}

	// Fall back to trying all certificates for this account
	certs, err := d.store.ListCertificates(accountID)
	if err != nil {
		return nil, true, fmt.Errorf("failed to list certificates: %w", err)
	}

	for _, cert := range certs {
		_, certChainPEM, getErr := d.store.GetCertificate(cert.ID)
		if getErr != nil {
			continue
		}

		x509Cert, parseErr := parseCertificateFromPEM(certChainPEM)
		if parseErr != nil {
			continue
		}

		privateKeyPEM, keyErr := d.credStore.GetSMIMEPrivateKey(cert.ID)
		if keyErr != nil {
			continue
		}

		block, _ := pem.Decode(privateKeyPEM)
		if block == nil {
			continue
		}

		privateKey, keyParseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if keyParseErr != nil {
			continue
		}

		decrypted, decryptErr := p7.Decrypt(x509Cert, privateKey)
		if decryptErr != nil {
			continue
		}

		d.log.Info().Str("certID", cert.ID).Msg("Successfully decrypted S/MIME message")
		return decrypted, true, nil
	}

	return nil, true, fmt.Errorf("no matching private key found for decryption")
}
