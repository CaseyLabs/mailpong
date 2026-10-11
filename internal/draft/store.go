package draft

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/logging"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Store provides draft persistence operations
type Store struct {
	db  *database.DB
	log zerolog.Logger
}

// NewStore creates a new draft store
func NewStore(db *database.DB) *Store {
	return &Store{
		db:  db,
		log: logging.WithComponent("draft-store"),
	}
}

// Create creates a new draft
func (s *Store) Create(d *Draft) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	if d.SyncStatus == "" {
		d.SyncStatus = SyncStatusPending
	}

	query := `
		INSERT INTO drafts (
			id, account_id, to_list, cc_list, bcc_list, reply_to, subject,
			body_html, body_text, in_reply_to_id, reply_type, references_list,
			identity_id, sign_message, encrypted, encrypted_body,
			pgp_sign_message, pgp_encrypted, pgp_encrypted_body,
			attachments_data,
			sync_status, imap_uid, folder_id,
			last_sync_attempt, sync_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		d.ID, d.AccountID, d.ToList, d.CcList, d.BccList, nullString(d.ReplyTo), d.Subject,
		d.BodyHTML, d.BodyText, nullString(d.InReplyToID), nullString(d.ReplyType), nullString(d.ReferencesList),
		nullString(d.IdentityID), d.SignMessage, d.Encrypted, nullBytes(d.EncryptedBody),
		d.PGPSignMessage, d.PGPEncrypted, nullBytes(d.PGPEncryptedBody),
		nullBytes(d.AttachmentsData),
		d.SyncStatus, nullUint32(d.IMAPUID), nullString(d.FolderID),
		nullTime(d.LastSyncAttempt), nullString(d.SyncError), d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create draft: %w", err)
	}

	s.log.Debug().
		Str("id", d.ID).
		Str("subject", d.Subject).
		Msg("Created draft")

	return nil
}

// Update updates an existing draft
func (s *Store) Update(d *Draft) error {
	d.UpdatedAt = time.Now()

	query := `
		UPDATE drafts SET
			to_list = ?, cc_list = ?, bcc_list = ?, reply_to = ?, subject = ?,
			body_html = ?, body_text = ?, in_reply_to_id = ?, reply_type = ?,
			references_list = ?, identity_id = ?, sign_message = ?,
			encrypted = ?, encrypted_body = ?,
			pgp_sign_message = ?, pgp_encrypted = ?, pgp_encrypted_body = ?,
			attachments_data = ?,
			sync_status = ?, imap_uid = ?,
			folder_id = ?, last_sync_attempt = ?, sync_error = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query,
		d.ToList, d.CcList, d.BccList, nullString(d.ReplyTo), d.Subject,
		d.BodyHTML, d.BodyText, nullString(d.InReplyToID), nullString(d.ReplyType),
		nullString(d.ReferencesList), nullString(d.IdentityID), d.SignMessage,
		d.Encrypted, nullBytes(d.EncryptedBody),
		d.PGPSignMessage, d.PGPEncrypted, nullBytes(d.PGPEncryptedBody),
		nullBytes(d.AttachmentsData),
		d.SyncStatus, nullUint32(d.IMAPUID),
		nullString(d.FolderID), nullTime(d.LastSyncAttempt), nullString(d.SyncError), d.UpdatedAt,
		d.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update draft: %w", err)
	}

	s.log.Debug().
		Str("id", d.ID).
		Str("sync_status", string(d.SyncStatus)).
		Msg("Updated draft")

	return nil
}

// Get returns a draft by ID
func (s *Store) Get(id string) (*Draft, error) {
	query := `
		SELECT id, account_id, to_list, cc_list, bcc_list, reply_to, subject,
			body_html, body_text, in_reply_to_id, reply_type, references_list,
			identity_id, sign_message, encrypted, encrypted_body,
			pgp_sign_message, pgp_encrypted, pgp_encrypted_body,
			attachments_data,
			sync_status, imap_uid, folder_id,
			last_sync_attempt, sync_error, created_at, updated_at
		FROM drafts
		WHERE id = ?
	`

	d := &Draft{}
	var replyTo, inReplyToID, replyType, referencesList, identityID, folderID, syncError sql.NullString
	var imapUID sql.NullInt64
	var lastSyncAttempt sql.NullTime
	var encryptedBody, pgpEncryptedBody, attachmentsData []byte

	err := s.db.QueryRow(query, id).Scan(
		&d.ID, &d.AccountID, &d.ToList, &d.CcList, &d.BccList, &replyTo, &d.Subject,
		&d.BodyHTML, &d.BodyText, &inReplyToID, &replyType, &referencesList,
		&identityID, &d.SignMessage, &d.Encrypted, &encryptedBody,
		&d.PGPSignMessage, &d.PGPEncrypted, &pgpEncryptedBody,
		&attachmentsData,
		&d.SyncStatus, &imapUID, &folderID,
		&lastSyncAttempt, &syncError, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get draft: %w", err)
	}

	d.ReplyTo = replyTo.String
	d.InReplyToID = inReplyToID.String
	d.ReplyType = replyType.String
	d.ReferencesList = referencesList.String
	d.IdentityID = identityID.String
	d.EncryptedBody = encryptedBody
	d.PGPEncryptedBody = pgpEncryptedBody
	d.AttachmentsData = attachmentsData
	d.FolderID = folderID.String
	d.SyncError = syncError.String
	if imapUID.Valid {
		d.IMAPUID = uint32(imapUID.Int64)
	}
	if lastSyncAttempt.Valid {
		d.LastSyncAttempt = &lastSyncAttempt.Time
	}

	return d, nil
}

// GetByIMAPUID returns a draft by its IMAP UID and folder ID
func (s *Store) GetByIMAPUID(folderID string, imapUID uint32) (*Draft, error) {
	query := `
		SELECT id, account_id, to_list, cc_list, bcc_list, reply_to, subject,
			body_html, body_text, in_reply_to_id, reply_type, references_list,
			identity_id, sign_message, encrypted, encrypted_body,
			pgp_sign_message, pgp_encrypted, pgp_encrypted_body,
			attachments_data,
			sync_status, imap_uid, folder_id,
			last_sync_attempt, sync_error, created_at, updated_at
		FROM drafts
		WHERE folder_id = ? AND imap_uid = ?
	`

	d := &Draft{}
	var replyTo, inReplyToID, replyType, referencesList, identityID, folderIDVal, syncError sql.NullString
	var imapUIDVal sql.NullInt64
	var lastSyncAttempt sql.NullTime
	var encryptedBody, pgpEncryptedBody, attachmentsData []byte

	err := s.db.QueryRow(query, folderID, imapUID).Scan(
		&d.ID, &d.AccountID, &d.ToList, &d.CcList, &d.BccList, &replyTo, &d.Subject,
		&d.BodyHTML, &d.BodyText, &inReplyToID, &replyType, &referencesList,
		&identityID, &d.SignMessage, &d.Encrypted, &encryptedBody,
		&d.PGPSignMessage, &d.PGPEncrypted, &pgpEncryptedBody,
		&attachmentsData,
		&d.SyncStatus, &imapUIDVal, &folderIDVal,
		&lastSyncAttempt, &syncError, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get draft by IMAP UID: %w", err)
	}

	d.ReplyTo = replyTo.String
	d.InReplyToID = inReplyToID.String
	d.ReplyType = replyType.String
	d.ReferencesList = referencesList.String
	d.IdentityID = identityID.String
	d.EncryptedBody = encryptedBody
	d.PGPEncryptedBody = pgpEncryptedBody
	d.AttachmentsData = attachmentsData
	d.FolderID = folderIDVal.String
	d.SyncError = syncError.String
	if imapUIDVal.Valid {
		d.IMAPUID = uint32(imapUIDVal.Int64)
	}
	if lastSyncAttempt.Valid {
		d.LastSyncAttempt = &lastSyncAttempt.Time
	}

	return d, nil
}

// Delete deletes a draft by ID
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM drafts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete draft: %w", err)
	}

	s.log.Debug().Str("id", id).Msg("Deleted draft")
	return nil
}

// ListByAccount returns all drafts for an account
func (s *Store) ListByAccount(accountID string) ([]*Draft, error) {
	query := `
		SELECT id, account_id, to_list, cc_list, bcc_list, reply_to, subject,
			body_html, body_text, in_reply_to_id, reply_type, references_list,
			identity_id, sign_message, encrypted, encrypted_body,
			pgp_sign_message, pgp_encrypted, pgp_encrypted_body,
			attachments_data,
			sync_status, imap_uid, folder_id,
			last_sync_attempt, sync_error, created_at, updated_at
		FROM drafts
		WHERE account_id = ?
		ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list drafts: %w", err)
	}
	defer rows.Close()

	return s.scanDrafts(rows)
}

// ListPendingSync returns all drafts that need to be synced to IMAP
func (s *Store) ListPendingSync(accountID string) ([]*Draft, error) {
	query := `
		SELECT id, account_id, to_list, cc_list, bcc_list, reply_to, subject,
			body_html, body_text, in_reply_to_id, reply_type, references_list,
			identity_id, sign_message, encrypted, encrypted_body,
			pgp_sign_message, pgp_encrypted, pgp_encrypted_body,
			attachments_data,
			sync_status, imap_uid, folder_id,
			last_sync_attempt, sync_error, created_at, updated_at
		FROM drafts
		WHERE account_id = ? AND sync_status IN ('pending', 'failed')
		ORDER BY updated_at ASC
	`

	rows, err := s.db.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending drafts: %w", err)
	}
	defer rows.Close()

	return s.scanDrafts(rows)
}

// UpdateSyncStatus updates the sync status of a draft
func (s *Store) UpdateSyncStatus(id string, status SyncStatus, imapUID uint32, folderID string, syncError string) error {
	now := time.Now()
	query := `
		UPDATE drafts SET
			sync_status = ?,
			imap_uid = ?,
			folder_id = ?,
			last_sync_attempt = ?,
			sync_error = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query, status, nullUint32(imapUID), nullString(folderID), now, nullString(syncError), now, id)
	if err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	s.log.Debug().
		Str("id", id).
		Str("status", string(status)).
		Uint32("imap_uid", imapUID).
		Msg("Updated draft sync status")

	return nil
}

// DeleteByAccount deletes all drafts for an account
func (s *Store) DeleteByAccount(accountID string) error {
	_, err := s.db.Exec("DELETE FROM drafts WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete drafts: %w", err)
	}
	return nil
}

// scanDrafts scans multiple drafts from rows
func (s *Store) scanDrafts(rows *sql.Rows) ([]*Draft, error) {
	var drafts []*Draft

	for rows.Next() {
		d := &Draft{}
		var replyTo, inReplyToID, replyType, referencesList, identityID, folderID, syncError sql.NullString
		var imapUID sql.NullInt64
		var lastSyncAttempt sql.NullTime
		var encryptedBody, pgpEncryptedBody, attachmentsData []byte

		err := rows.Scan(
			&d.ID, &d.AccountID, &d.ToList, &d.CcList, &d.BccList, &replyTo, &d.Subject,
			&d.BodyHTML, &d.BodyText, &inReplyToID, &replyType, &referencesList,
			&identityID, &d.SignMessage, &d.Encrypted, &encryptedBody,
			&d.PGPSignMessage, &d.PGPEncrypted, &pgpEncryptedBody,
			&attachmentsData,
			&d.SyncStatus, &imapUID, &folderID,
			&lastSyncAttempt, &syncError, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan draft: %w", err)
		}

		d.ReplyTo = replyTo.String
		d.InReplyToID = inReplyToID.String
		d.ReplyType = replyType.String
		d.ReferencesList = referencesList.String
		d.IdentityID = identityID.String
		d.EncryptedBody = encryptedBody
		d.PGPEncryptedBody = pgpEncryptedBody
		d.AttachmentsData = attachmentsData
		d.FolderID = folderID.String
		d.SyncError = syncError.String
		if imapUID.Valid {
			d.IMAPUID = uint32(imapUID.Int64)
		}
		if lastSyncAttempt.Valid {
			d.LastSyncAttempt = &lastSyncAttempt.Time
		}

		drafts = append(drafts, d)
	}

	return drafts, nil
}

// Helper functions for nullable fields
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullBytes(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}

func nullUint32(v uint32) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
