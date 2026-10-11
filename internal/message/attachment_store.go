package message

import (
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/logging"
)

// AttachmentStore handles attachment metadata persistence
type AttachmentStore struct {
	db *database.DB
}

// NewAttachmentStore creates a new attachment store
func NewAttachmentStore(db *database.DB) *AttachmentStore {
	store := &AttachmentStore{db: db}
	store.ensureContentColumn()
	return store
}

// ensureContentColumn adds the content column if it doesn't exist
// This handles migration for existing databases
func (s *AttachmentStore) ensureContentColumn() {
	log := logging.WithComponent("attachment_store")

	// Check if content column exists
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('attachments') WHERE name = 'content'
	`).Scan(&count)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check for content column")
		return
	}

	if count == 0 {
		// Add the content column
		_, err := s.db.Exec(`ALTER TABLE attachments ADD COLUMN content BLOB`)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to add content column to attachments table")
			return
		}
		log.Info().Msg("Added content column to attachments table")
	}
}

// Create creates a new attachment record
// For inline attachments, also stores the content for offline access
func (s *AttachmentStore) Create(a *Attachment) error {
	query := `
		INSERT INTO attachments (id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path, content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	// Only store content for inline attachments to save space
	var content []byte
	if a.IsInline && len(a.Content) > 0 {
		content = a.Content
	}
	_, err := s.db.Exec(query, a.ID, a.MessageID, a.Filename, a.ContentType, a.Size, nullString(a.ContentID), boolToInt(a.IsInline), boolToInt(a.Embedded), nullString(a.LocalPath), content)
	if err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}
	return nil
}

// GetByMessage returns all attachments for a message
func (s *AttachmentStore) GetByMessage(messageID string) ([]*Attachment, error) {
	query := `
		SELECT id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path
		FROM attachments
		WHERE message_id = ?
		ORDER BY filename
	`
	rows, err := s.db.Query(query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*Attachment
	for rows.Next() {
		a := &Attachment{}
		var contentID, localPath sql.NullString
		var isInline, embedded int

		err := rows.Scan(&a.ID, &a.MessageID, &a.Filename, &a.ContentType, &a.Size, &contentID, &isInline, &embedded, &localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if contentID.Valid {
			a.ContentID = contentID.String
		}
		if localPath.Valid {
			a.LocalPath = localPath.String
		}
		a.IsInline = isInline == 1
		a.Embedded = embedded == 1

		attachments = append(attachments, a)
	}

	return attachments, nil
}

// Get returns a single attachment by ID
func (s *AttachmentStore) Get(id string) (*Attachment, error) {
	query := `
		SELECT id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path
		FROM attachments
		WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	a := &Attachment{}
	var contentID, localPath sql.NullString
	var isInline, embedded int

	err := row.Scan(&a.ID, &a.MessageID, &a.Filename, &a.ContentType, &a.Size, &contentID, &isInline, &embedded, &localPath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	if contentID.Valid {
		a.ContentID = contentID.String
	}
	if localPath.Valid {
		a.LocalPath = localPath.String
	}
	a.IsInline = isInline == 1
	a.Embedded = embedded == 1

	return a, nil
}

// UpdateLocalPath updates the local path for a downloaded attachment
func (s *AttachmentStore) UpdateLocalPath(id, localPath string) error {
	query := `UPDATE attachments SET local_path = ? WHERE id = ?`
	_, err := s.db.Exec(query, localPath, id)
	if err != nil {
		return fmt.Errorf("failed to update attachment path: %w", err)
	}
	return nil
}

// DeleteByMessage deletes all attachments for a message
func (s *AttachmentStore) DeleteByMessage(messageID string) error {
	_, err := s.db.Exec("DELETE FROM attachments WHERE message_id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to delete attachments: %w", err)
	}
	return nil
}

// GetInlineByMessage returns inline attachments with content as data URLs
// Returns a map of content_id -> data URL (e.g., "data:image/png;base64,...")
func (s *AttachmentStore) GetInlineByMessage(messageID string) (map[string]string, error) {
	query := `
		SELECT content_id, content_type, content
		FROM attachments
		WHERE message_id = ? AND is_inline = 1 AND content IS NOT NULL AND content_id IS NOT NULL
	`
	rows, err := s.db.Query(query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query inline attachments: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var contentID, contentType string
		var content []byte

		err := rows.Scan(&contentID, &contentType, &content)
		if err != nil {
			continue // Skip malformed rows
		}

		if len(content) > 0 && contentID != "" {
			// Build data URL
			dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(content))
			result[contentID] = dataURL
		}
	}

	return result, nil
}

// ReplaceForMessages replaces the attachment records of messageIDs with
// attachments in a single transaction. A re-fetched body re-extracts its
// attachments with new IDs, so inserting without deleting would list each
// attachment again.
func (s *AttachmentStore) ReplaceForMessages(messageIDs []string, attachments []*Attachment) error {
	if len(messageIDs) == 0 && len(attachments) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range messageIDs {
		if _, err := tx.Exec(`DELETE FROM attachments WHERE message_id = ?`, id); err != nil {
			return fmt.Errorf("failed to delete old attachments: %w", err)
		}
	}

	stmt, err := tx.Prepare(`
		INSERT INTO attachments (id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path, content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	log := logging.WithComponent("attachment_store")
	for _, a := range attachments {
		// Only store content for inline attachments to save space
		var content []byte
		if a.IsInline && len(a.Content) > 0 {
			content = a.Content
		}
		_, err := stmt.Exec(a.ID, a.MessageID, a.Filename, a.ContentType, a.Size, nullString(a.ContentID), boolToInt(a.IsInline), boolToInt(a.Embedded), nullString(a.LocalPath), content)
		if err != nil {
			log.Debug().Err(err).Str("filename", a.Filename).Msg("Failed to create attachment in batch")
			// Continue with other attachments
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteAttachmentsForFolder deletes all attachment records for messages in a folder.
// This is used during force re-sync to clear stale attachment data before re-extracting.
func (s *AttachmentStore) DeleteAttachmentsForFolder(folderID string) (int64, error) {
	log := logging.WithComponent("attachment_store")

	// Delete attachments where message_id belongs to the folder
	query := `
		DELETE FROM attachments
		WHERE message_id IN (
			SELECT id FROM messages WHERE folder_id = ?
		)
	`
	result, err := s.db.Exec(query, folderID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete attachments for folder: %w", err)
	}

	affected, _ := result.RowsAffected()
	log.Info().Str("folderID", folderID).Int64("deleted", affected).Msg("Deleted attachments for folder")
	return affected, nil
}

// boolToInt converts bool to int for SQLite storage
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
