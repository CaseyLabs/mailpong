package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/caseylabs/mailven/internal/cid"
)

// backfillEmbeddedAttachments marks the stored inline parts whose
// Content-ID their message's body_html embeds, then recomputes
// has_attachments for those messages from the parts left listed.
func backfillEmbeddedAttachments(tx *sql.Tx) error {
	// Collect the parts first, without bodies: body_html can be large, and a
	// join would read it once per inline part rather than once per message.
	type inlinePart struct{ id, messageID, contentID string }
	rows, err := tx.Query(`
		SELECT id, message_id, content_id FROM attachments
		WHERE is_inline = 1 AND content_id IS NOT NULL AND content_id != ''
		ORDER BY message_id
	`)
	if err != nil {
		return fmt.Errorf("query inline parts: %w", err)
	}
	var parts []inlinePart
	for rows.Next() {
		var p inlinePart
		if err := rows.Scan(&p.id, &p.messageID, &p.contentID); err != nil {
			rows.Close()
			return fmt.Errorf("scan inline part: %w", err)
		}
		parts = append(parts, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("read inline parts: %w", err)
	}

	bodyStmt, err := tx.Prepare(`SELECT body_html FROM messages WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare body read: %w", err)
	}
	defer bodyStmt.Close()
	var embeddedIDs []string
	var lastMessageID string
	var embedded map[string]struct{}
	for _, p := range parts {
		if p.messageID != lastMessageID {
			lastMessageID = p.messageID
			var bodyHTML sql.NullString
			// A part whose message is gone has nothing to embed it.
			err := bodyStmt.QueryRow(p.messageID).Scan(&bodyHTML)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("read body of %s: %w", p.messageID, err)
			}
			embedded = cid.Embedded(bodyHTML.String)
		}
		if _, ok := embedded[p.contentID]; ok {
			embeddedIDs = append(embeddedIDs, p.id)
		}
	}

	markStmt, err := tx.Prepare(`UPDATE attachments SET embedded = 1 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare mark: %w", err)
	}
	defer markStmt.Close()
	for _, id := range embeddedIDs {
		if _, err := markStmt.Exec(id); err != nil {
			return fmt.Errorf("mark embedded part: %w", err)
		}
	}
	if _, err := tx.Exec(`
		UPDATE messages SET has_attachments = EXISTS (
			SELECT 1 FROM attachments WHERE message_id = messages.id AND embedded = 0
		) WHERE id IN (
			SELECT message_id FROM attachments
			WHERE is_inline = 1 AND content_id IS NOT NULL AND content_id != ''
		)
	`); err != nil {
		return fmt.Errorf("recompute has_attachments: %w", err)
	}
	return nil
}
