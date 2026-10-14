package gmailapi

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/smtp"
)

const lookupFailedSentinelEmail = "[lookup-failed]"

// Send-as states of an identity.
const (
	StatusVerified      = "verified"
	StatusPending       = "pending"
	StatusUnknown       = "unknown"
	StatusNotConfigured = "not-configured"
)

// Store caches the latest send-as list per account.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store over the account_send_as table.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Cached is the cached send-as list of an account.
type Cached struct {
	Entries      []SendAs
	FetchedAt    time.Time // zero when no successful lookup has been recorded
	LookupFailed bool      // latest lookup failed; cached entries may be stale
}

// MarkLookupFailed records that the latest refresh failed while preserving
// the last successful entries and their fetched_at timestamps.
func (s *Store) MarkLookupFailed(accountID string, attemptedAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO account_send_as
			(account_id, email, verification_status, fetched_at)
		VALUES (?, ?, 'lookup-failed', ?)
	`, accountID, lookupFailedSentinelEmail, attemptedAt)
	if err != nil {
		return fmt.Errorf("record failed send-as lookup: %w", err)
	}
	return nil
}

// Replace swaps the cached list for the account.
func (s *Store) Replace(accountID string, entries []SendAs, fetchedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM account_send_as WHERE account_id = ?`, accountID); err != nil {
		return fmt.Errorf("clear send-as cache: %w", err)
	}
	for _, e := range entries {
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO account_send_as
				(account_id, email, display_name, reply_to, signature, verification_status, is_primary, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, accountID, strings.ToLower(e.Email), e.DisplayName, e.ReplyTo, e.Signature, e.VerificationStatus, e.IsPrimary, fetchedAt)
		if err != nil {
			return fmt.Errorf("cache send-as %s: %w", e.Email, err)
		}
	}
	// Preserve successful empty lookups: without a row, Get cannot distinguish
	// an empty list from a cache that has never been populated. The empty email
	// is a private metadata sentinel and is omitted from Cached.Entries.
	if len(entries) == 0 {
		if _, err := tx.Exec(`
			INSERT INTO account_send_as
				(account_id, email, fetched_at)
			VALUES (?, '', ?)
		`, accountID, fetchedAt); err != nil {
			return fmt.Errorf("cache empty send-as list: %w", err)
		}
	}
	return tx.Commit()
}

// Get returns the cached list. A successful empty list is represented by a
// metadata sentinel row so its fetch time survives process restarts.
func (s *Store) Get(accountID string) (Cached, error) {
	rows, err := s.db.Query(`
		SELECT email, display_name, reply_to, signature, verification_status, is_primary, fetched_at
		FROM account_send_as WHERE account_id = ? ORDER BY is_primary DESC, email
	`, accountID)
	if err != nil {
		return Cached{}, fmt.Errorf("list send-as cache: %w", err)
	}
	defer rows.Close()

	var c Cached
	for rows.Next() {
		var e SendAs
		var fetched time.Time
		if err := rows.Scan(&e.Email, &e.DisplayName, &e.ReplyTo, &e.Signature, &e.VerificationStatus, &e.IsPrimary, &fetched); err != nil {
			return Cached{}, fmt.Errorf("scan send-as cache: %w", err)
		}
		if e.Email == lookupFailedSentinelEmail {
			c.LookupFailed = true
			continue
		}
		if e.Email == "" {
			c.FetchedAt = fetched
			continue
		}
		c.Entries = append(c.Entries, e)
		c.FetchedAt = fetched
	}
	return c, rows.Err()
}

// StatusFor classifies an identity address against the cached list. Ambiguous
// verification statuses and never-fetched lists remain unknown.
func (c Cached) StatusFor(email string) string {
	if c.FetchedAt.IsZero() {
		return StatusUnknown
	}
	// Compare the same IDNA domains used at SMTP submission, while preserving
	// the original selected From. Unicode and ASCII domain spellings are equal.
	normalized, err := smtp.NormalizeEnvelopeAddress(email)
	if err != nil {
		return StatusUnknown
	}
	for _, e := range c.Entries {
		candidate, err := smtp.NormalizeEnvelopeAddress(e.Email)
		if err != nil || !strings.EqualFold(candidate, normalized) {
			continue
		}
		if e.Verified() {
			return StatusVerified
		}
		if strings.EqualFold(e.VerificationStatus, "pending") {
			return StatusPending
		}
		return StatusUnknown
	}
	return StatusNotConfigured
}
