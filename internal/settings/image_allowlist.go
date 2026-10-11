// Package settings provides global application settings storage
package settings

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/logging"
	"github.com/rs/zerolog"
)

// Allowlist type constants
const (
	AllowlistTypeDomain = "domain"
	AllowlistTypeSender = "sender"
)

// AllowlistEntry represents an entry in the image allowlist
type AllowlistEntry struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	CreatedAt string `json:"createdAt"`
}

// ImageAllowlistStore provides image allowlist persistence operations
type ImageAllowlistStore struct {
	db  *database.DB
	log zerolog.Logger
}

// NewImageAllowlistStore creates a new image allowlist store
func NewImageAllowlistStore(db *database.DB) *ImageAllowlistStore {
	return &ImageAllowlistStore{
		db:  db,
		log: logging.WithComponent("image-allowlist-store"),
	}
}

// Add adds an entry to the allowlist
func (s *ImageAllowlistStore) Add(entryType, value string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}
	if entryType != AllowlistTypeDomain && entryType != AllowlistTypeSender {
		return fmt.Errorf("invalid type: %s (must be 'domain' or 'sender')", entryType)
	}

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO image_allowlist (type, value) VALUES (?, ?)
	`, entryType, value)
	if err != nil {
		return fmt.Errorf("failed to add allowlist entry: %w", err)
	}

	s.log.Debug().Str("type", entryType).Str("value", value).Msg("Added image allowlist entry")
	return nil
}

// Remove removes an entry from the allowlist by ID
func (s *ImageAllowlistStore) Remove(id int64) error {
	_, err := s.db.Exec(`DELETE FROM image_allowlist WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to remove allowlist entry: %w", err)
	}

	s.log.Debug().Int64("id", id).Msg("Removed image allowlist entry")
	return nil
}

// IsAllowed checks if an email address or its domain is in the allowlist
// Returns true if the sender's email or domain is allowed
func (s *ImageAllowlistStore) IsAllowed(email string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false, nil
	}

	// Extract domain from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false, nil
	}
	domain := parts[1]

	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM image_allowlist
		WHERE (type = 'sender' AND value = ?)
		   OR (type = 'domain' AND value = ?)
	`, email, domain).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check allowlist: %w", err)
	}

	return count > 0, nil
}

// List returns all allowlist entries
func (s *ImageAllowlistStore) List() ([]*AllowlistEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, type, value, created_at
		FROM image_allowlist
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list allowlist entries: %w", err)
	}
	defer rows.Close()

	entries := make([]*AllowlistEntry, 0)
	for rows.Next() {
		var entry AllowlistEntry
		var createdAt sql.NullString
		if err := rows.Scan(&entry.ID, &entry.Type, &entry.Value, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan allowlist entry: %w", err)
		}
		if createdAt.Valid {
			entry.CreatedAt = createdAt.String
		}
		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating allowlist entries: %w", err)
	}

	return entries, nil
}
