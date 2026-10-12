package certificate

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store manages trusted certificates in the database and session memory.
// Trust is scoped to a host: a fingerprint accepted for one server never
// vouches for another.
type Store struct {
	db           *sql.DB
	mu           sync.RWMutex
	session      map[trustKey]bool // session-only trust
	failures     map[string]*Error
	failureOrder []string
}

type trustKey struct{ host, fingerprint string }

// NewStore creates a new certificate trust store
func NewStore(db *sql.DB) *Store {
	return &Store{
		db:       db,
		session:  make(map[trustKey]bool),
		failures: make(map[string]*Error),
	}
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

// IsTrusted reports whether the certificate fingerprint was accepted for
// host, permanently or for this session. A certificate accepted for an
// account's IMAP host also covers that account's SMTP host, since only IMAP
// connections can prompt for trust.
func (s *Store) IsTrusted(host, fingerprint string) bool {
	host = normalizeHost(host)
	if host == "" {
		return false
	}
	hosts := []string{host}
	rows, err := s.db.Query(
		"SELECT DISTINCT lower(imap_host) FROM accounts WHERE lower(smtp_host) = ? AND lower(imap_host) != ?",
		host, host,
	)
	if err == nil {
		for rows.Next() {
			var h string
			if rows.Scan(&h) == nil {
				hosts = append(hosts, h)
			}
		}
		rows.Close()
	}

	s.mu.RLock()
	for _, h := range hosts {
		if s.session[trustKey{h, fingerprint}] {
			s.mu.RUnlock()
			return true
		}
	}
	s.mu.RUnlock()

	query := "SELECT COUNT(*) FROM trusted_certificates WHERE fingerprint = ? AND host IN (?" +
		strings.Repeat(",?", len(hosts)-1) + ")"
	args := []interface{}{fingerprint}
	for _, h := range hosts {
		args = append(args, h)
	}
	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// AcceptPermanently stores a certificate in the database as trusted for host
func (s *Store) AcceptPermanently(host string, info *CertificateInfo) error {
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("host is required to trust a certificate")
	}
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO trusted_certificates (id, fingerprint, host, subject, issuer, not_before, not_after, accepted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, info.Fingerprint, host, info.Subject, info.Issuer, info.NotBefore, info.NotAfter, time.Now(),
	)
	return err
}

// AcceptSession trusts a certificate fingerprint for host in session memory only
func (s *Store) AcceptSession(host, fingerprint string) error {
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("host is required to trust a certificate")
	}
	s.mu.Lock()
	s.session[trustKey{host, fingerprint}] = true
	s.mu.Unlock()
	return nil
}

// GetByHosts returns permanently trusted certificates for the given hosts
func (s *Store) GetByHosts(hosts []string) ([]*CertificateInfo, error) {
	if len(hosts) == 0 {
		return nil, nil
	}

	// Build query with placeholders
	query := "SELECT fingerprint, host, subject, issuer, not_before, not_after FROM trusted_certificates WHERE host IN ("
	args := make([]interface{}, len(hosts))
	for i, h := range hosts {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = normalizeHost(h)
	}
	query += ") ORDER BY accepted_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*CertificateInfo
	for rows.Next() {
		var ci CertificateInfo
		var host string
		if err := rows.Scan(&ci.Fingerprint, &host, &ci.Subject, &ci.Issuer, &ci.NotBefore, &ci.NotAfter); err != nil {
			return nil, err
		}
		ci.Host = host
		certs = append(certs, &ci)
	}
	return certs, rows.Err()
}

// Remove deletes a trusted certificate from the database by fingerprint
func (s *Store) Remove(fingerprint string) error {
	_, err := s.db.Exec("DELETE FROM trusted_certificates WHERE fingerprint = ?", fingerprint)
	if err != nil {
		return err
	}

	// Also remove from session, for every host
	s.mu.Lock()
	for k := range s.session {
		if k.fingerprint == fingerprint {
			delete(s.session, k)
		}
	}
	s.mu.Unlock()

	return nil
}

// recordFailure keeps structured details from the failed request. The opaque
// token survives Wails' string-only error channel without parsing TLS text or
// trusting an error payload supplied by a remote server. Bound memory use.
func (s *Store) recordFailure(err *Error, host string) *Error {
	info := *err.Info
	info.Host = host
	failure := &Error{Info: &info, Reason: err.Reason, Host: host, token: uuid.NewString()}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[failure.token] = failure
	s.failureOrder = append(s.failureOrder, failure.token)
	if len(s.failureOrder) > 64 {
		delete(s.failures, s.failureOrder[0])
		s.failureOrder = s.failureOrder[1:]
	}
	return failure
}

// FailureDetails resolves only errors generated by this process, not arbitrary
// certificate-looking network error strings. It never makes a network request.
func (s *Store) FailureDetails(message string) *Error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for token, failure := range s.failures {
		if strings.Contains(message, "[certificate-check:"+token+"]") {
			return failure
		}
	}
	return nil
}

// GetAll lists permanently accepted pins, including DAV and discovery hosts.
func (s *Store) GetAll() ([]*CertificateInfo, error) {
	rows, err := s.db.Query(`SELECT DISTINCT host FROM trusted_certificates`)
	if err != nil {
		return nil, err
	}
	var hosts []string
	for rows.Next() {
		var host string
		if err = rows.Scan(&host); err != nil {
			rows.Close()
			return nil, err
		}
		hosts = append(hosts, host)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return s.GetByHosts(hosts)
}

// RemoveForHost revokes exactly one permanent/session pin.
func (s *Store) RemoveForHost(host, fingerprint string) error {
	host = normalizeHost(host)
	if _, err := s.db.Exec(`DELETE FROM trusted_certificates WHERE host=? AND fingerprint=?`, host, fingerprint); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.session, trustKey{host, fingerprint})
	s.mu.Unlock()
	return nil
}
