package smime

import (
	"crypto/sha256"
	"database/sql"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Store manages S/MIME certificates in the database
type Store struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewStore creates a new S/MIME certificate store
func NewStore(db *sql.DB, log zerolog.Logger) *Store {
	return &Store{
		db:  db,
		log: log,
	}
}

// SaveCertificate stores a user's S/MIME certificate
func (s *Store) SaveCertificate(cert *Certificate, certChainPEM string) error {
	if cert.ID == "" {
		cert.ID = uuid.New().String()
	}

	_, err := s.db.Exec(`
		INSERT INTO smime_certificates (id, account_id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, cert_chain_pem, is_default, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			account_id = excluded.account_id,
			is_default = excluded.is_default`,
		cert.ID, cert.AccountID, cert.Email, cert.Subject, cert.Issuer,
		cert.SerialNumber, cert.Fingerprint, cert.NotBefore, cert.NotAfter,
		certChainPEM, cert.IsDefault, time.Now(),
	)
	return err
}

// GetCertificate retrieves a certificate by ID, returning the cert and its PEM chain
func (s *Store) GetCertificate(id string) (*Certificate, string, error) {
	cert := &Certificate{}
	var certChainPEM string

	err := s.db.QueryRow(`
		SELECT id, account_id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, cert_chain_pem, is_default, created_at
		FROM smime_certificates WHERE id = ?`, id,
	).Scan(
		&cert.ID, &cert.AccountID, &cert.Email, &cert.Subject, &cert.Issuer,
		&cert.SerialNumber, &cert.Fingerprint, &cert.NotBefore, &cert.NotAfter,
		&certChainPEM, &cert.IsDefault, &cert.CreatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	cert.IsExpired = time.Now().After(cert.NotAfter)
	cert.IsSelfSigned = cert.Subject == cert.Issuer
	return cert, certChainPEM, nil
}

// ListCertificates returns all S/MIME certificates for an account
func (s *Store) ListCertificates(accountID string) ([]*Certificate, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, is_default, created_at
		FROM smime_certificates WHERE account_id = ?
		ORDER BY is_default DESC, created_at DESC`, accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*Certificate
	for rows.Next() {
		cert := &Certificate{}
		if err := rows.Scan(
			&cert.ID, &cert.AccountID, &cert.Email, &cert.Subject, &cert.Issuer,
			&cert.SerialNumber, &cert.Fingerprint, &cert.NotBefore, &cert.NotAfter,
			&cert.IsDefault, &cert.CreatedAt,
		); err != nil {
			return nil, err
		}
		cert.IsExpired = time.Now().After(cert.NotAfter)
		cert.IsSelfSigned = cert.Subject == cert.Issuer
		certs = append(certs, cert)
	}
	return certs, rows.Err()
}

// DeleteCertificate removes an S/MIME certificate by ID
func (s *Store) DeleteCertificate(id string) error {
	_, err := s.db.Exec("DELETE FROM smime_certificates WHERE id = ?", id)
	return err
}

// SetDefaultCertificate sets a certificate as the default for its account
func (s *Store) SetDefaultCertificate(accountID, certID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing defaults for this account
	if _, err := tx.Exec(
		"UPDATE smime_certificates SET is_default = 0 WHERE account_id = ?", accountID,
	); err != nil {
		return err
	}

	// Set the new default
	if _, err := tx.Exec(
		"UPDATE smime_certificates SET is_default = 1 WHERE id = ? AND account_id = ?", certID, accountID,
	); err != nil {
		return err
	}

	// Update account's default cert ID
	if _, err := tx.Exec(
		"UPDATE accounts SET smime_default_cert_id = ? WHERE id = ?", certID, accountID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// GetCertificateByEmail returns the certificate matching a specific email for an account.
// Returns nil (not an error) if no matching certificate exists.
func (s *Store) GetCertificateByEmail(accountID, email string) (*Certificate, string, error) {
	cert := &Certificate{}
	var certChainPEM string

	err := s.db.QueryRow(`
		SELECT id, account_id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, cert_chain_pem, is_default, created_at
		FROM smime_certificates
		WHERE account_id = ? AND LOWER(email) = LOWER(?)`, accountID, email,
	).Scan(
		&cert.ID, &cert.AccountID, &cert.Email, &cert.Subject, &cert.Issuer,
		&cert.SerialNumber, &cert.Fingerprint, &cert.NotBefore, &cert.NotAfter,
		&certChainPEM, &cert.IsDefault, &cert.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	cert.IsExpired = time.Now().After(cert.NotAfter)
	cert.IsSelfSigned = cert.Subject == cert.Issuer
	return cert, certChainPEM, nil
}

// GetDefaultCertificate returns the default signing certificate for an account
func (s *Store) GetDefaultCertificate(accountID string) (*Certificate, string, error) {
	cert := &Certificate{}
	var certChainPEM string

	err := s.db.QueryRow(`
		SELECT id, account_id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, cert_chain_pem, is_default, created_at
		FROM smime_certificates
		WHERE account_id = ? AND is_default = 1`, accountID,
	).Scan(
		&cert.ID, &cert.AccountID, &cert.Email, &cert.Subject, &cert.Issuer,
		&cert.SerialNumber, &cert.Fingerprint, &cert.NotBefore, &cert.NotAfter,
		&certChainPEM, &cert.IsDefault, &cert.CreatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	cert.IsExpired = time.Now().After(cert.NotAfter)
	cert.IsSelfSigned = cert.Subject == cert.Issuer
	return cert, certChainPEM, nil
}

// CacheSenderCert stores or updates a sender's public certificate. trusted
// marks certificates that chain to a trusted root or that the user imported;
// only those are used for encryption. Negative trusted values record a known
// revoked chain until a later successful online verification clears them.
func (s *Store) CacheSenderCert(email, certPEM string, trusted bool) error {
	// Parse the PEM to extract metadata
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return fmt.Errorf("failed to decode PEM certificate")
	}

	fingerprint := fmt.Sprintf("%x", sha256.Sum256(block.Bytes))

	// Parse the certificate for metadata
	cert, err := parseCertificateFromPEM(certPEM)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = s.db.Exec(`
		INSERT INTO smime_sender_certs (id, email, subject, issuer, serial_number,
			fingerprint, not_before, not_after, cert_pem, collected_at, last_seen_at, trusted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			last_seen_at = excluded.last_seen_at,
			trusted = CASE WHEN trusted < 0 THEN trusted ELSE MAX(trusted, excluded.trusted) END`,
		id, email, cert.Subject.String(), cert.Issuer.String(),
		cert.SerialNumber.String(), fingerprint,
		cert.NotBefore, cert.NotAfter, certPEM, now, now, trusted,
	)
	return err
}

// GetSenderCerts returns cached public certificates for an email address
func (s *Store) GetSenderCerts(email string) ([]*SenderCert, error) {
	rows, err := s.db.Query(`
		SELECT id, email, subject, issuer, serial_number, fingerprint,
			not_before, not_after, collected_at, last_seen_at
		FROM smime_sender_certs WHERE email = ?
		ORDER BY last_seen_at DESC`, email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*SenderCert
	for rows.Next() {
		cert := &SenderCert{}
		if err := rows.Scan(
			&cert.ID, &cert.Email, &cert.Subject, &cert.Issuer,
			&cert.SerialNumber, &cert.Fingerprint,
			&cert.NotBefore, &cert.NotAfter,
			&cert.CollectedAt, &cert.LastSeenAt,
		); err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, rows.Err()
}

// ListAllSenderCerts returns all cached sender certificates
func (s *Store) ListAllSenderCerts() ([]*SenderCert, error) {
	rows, err := s.db.Query(`
		SELECT id, email, subject, issuer, serial_number, fingerprint,
			not_before, not_after, collected_at, last_seen_at
		FROM smime_sender_certs
		ORDER BY last_seen_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*SenderCert
	for rows.Next() {
		cert := &SenderCert{}
		if err := rows.Scan(
			&cert.ID, &cert.Email, &cert.Subject, &cert.Issuer,
			&cert.SerialNumber, &cert.Fingerprint,
			&cert.NotBefore, &cert.NotAfter,
			&cert.CollectedAt, &cert.LastSeenAt,
		); err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, rows.Err()
}

// DeleteSenderCert removes a cached sender certificate
func (s *Store) DeleteSenderCert(id string) error {
	_, err := s.db.Exec("DELETE FROM smime_sender_certs WHERE id = ?", id)
	return err
}

// GetSenderCertPEM returns the PEM-encoded certificate for a sender cert
func (s *Store) GetSenderCertPEM(id string) (string, error) {
	var certPEM string
	err := s.db.QueryRow("SELECT cert_pem FROM smime_sender_certs WHERE id = ?", id).Scan(&certPEM)
	return certPEM, err
}

// UpdateMessageSMIMEStatus stores the S/MIME verification result for a message
func (s *Store) UpdateMessageSMIMEStatus(messageID string, result *SignatureResult) error {
	_, err := s.db.Exec(`
		UPDATE messages SET smime_status = ?, smime_signer_email = ?, smime_signer_subject = ?
		WHERE id = ?`,
		string(result.Status), result.SignerEmail, result.SignerName, messageID,
	)
	return err
}

// GetSignPolicy returns the S/MIME signing policy for an account
func (s *Store) GetSignPolicy(accountID string) (string, error) {
	var policy string
	err := s.db.QueryRow(
		"SELECT smime_sign_policy FROM accounts WHERE id = ?", accountID,
	).Scan(&policy)
	return policy, err
}

// SetSignPolicy updates the S/MIME signing policy for an account
func (s *Store) SetSignPolicy(accountID, policy string) error {
	_, err := s.db.Exec(
		"UPDATE accounts SET smime_sign_policy = ? WHERE id = ?", policy, accountID,
	)
	return err
}

// GetEncryptPolicy returns the S/MIME encryption policy for an account
func (s *Store) GetEncryptPolicy(accountID string) (string, error) {
	var policy string
	err := s.db.QueryRow(
		"SELECT smime_encrypt_policy FROM accounts WHERE id = ?", accountID,
	).Scan(&policy)
	return policy, err
}

// SetEncryptPolicy updates the S/MIME encryption policy for an account
func (s *Store) SetEncryptPolicy(accountID, policy string) error {
	_, err := s.db.Exec(
		"UPDATE accounts SET smime_encrypt_policy = ? WHERE id = ?", policy, accountID,
	)
	return err
}

// GetSenderCertPEMs returns PEM-encoded certificates for multiple email addresses (batch lookup for encryption).
// Returns a map of email -> certPEM for emails that have a valid (non-expired)
// trusted certificate; untrusted collected certs are never used for encryption.
func (s *Store) GetSenderCertPEMs(emails []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(emails) == 0 {
		return result, nil
	}

	now := time.Now()
	for _, email := range emails {
		var certPEM string
		var notAfter time.Time
		err := s.db.QueryRow(`
			SELECT cert_pem, not_after FROM smime_sender_certs
			WHERE email = ? AND trusted = 1 ORDER BY last_seen_at DESC LIMIT 1`, email,
		).Scan(&certPEM, &notAfter)
		if err != nil {
			continue
		}
		if now.After(notAfter) {
			continue // Skip expired certs
		}
		result[email] = certPEM
	}

	return result, nil
}

// ImportSenderCertFromFile imports a recipient certificate from PEM/DER file content
func (s *Store) ImportSenderCertFromFile(email string, certData []byte) error {
	// Try PEM first
	block, _ := pem.Decode(certData)
	var derBytes []byte
	if block != nil {
		derBytes = block.Bytes
	} else {
		// Assume DER
		derBytes = certData
	}

	cert, err := parseCertificateFromDER(derBytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Encode to PEM for storage
	certPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}))

	// An explicit import is the user's acceptance of the certificate.
	return s.CacheSenderCert(email, certPEM, true)
}

// SenderCertRevoked records/checks previously verified revocation without a
// network request. Negative trust is excluded by GetSenderCertPEMs, which only
// selects trusted=1. Existing Boolean trust rows remain compatible.
func (s *Store) SenderCertRevoked(fingerprint string) (bool, error) {
	var trusted int
	err := s.db.QueryRow(`SELECT trusted FROM smime_sender_certs WHERE fingerprint=?`, fingerprint).Scan(&trusted)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return trusted < 0, err
}
func (s *Store) SetSenderCertRevoked(fingerprint string, revoked bool) error {
	_, err := s.db.Exec(`UPDATE smime_sender_certs SET trusted=CASE WHEN ? THEN -1 WHEN trusted<0 THEN 0 ELSE trusted END WHERE fingerprint=?`, revoked, fingerprint)
	return err
}
