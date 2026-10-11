package gmailapi

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
	Entries   []SendAs
	FetchedAt time.Time // zero when never fetched
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
	return tx.Commit()
}

// Get returns the cached list. An account with no rows reports a zero
// FetchedAt, even if the last fetch legitimately returned nothing.
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
		c.Entries = append(c.Entries, e)
		c.FetchedAt = fetched
	}
	return c, rows.Err()
}

// StatusFor classifies an identity address against the cached list: verified,
// pending, not-configured, or unknown when the list was never fetched.
func (c Cached) StatusFor(email string) string {
	if c.FetchedAt.IsZero() {
		return StatusUnknown
	}
	for _, e := range c.Entries {
		if !strings.EqualFold(e.Email, email) {
			continue
		}
		if e.Verified() {
			return StatusVerified
		}
		return StatusPending
	}
	return StatusNotConfigured
}
