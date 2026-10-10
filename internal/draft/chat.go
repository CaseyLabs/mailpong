package draft

import (
	"database/sql"
	"fmt"
	"strings"
)

// ChatLink ties a chat thread to the draft its docked composer is editing.
// The typed text is the first TextLen bytes of the draft's text body.
type ChatLink struct {
	DraftID   string
	MessageID string // message being replied to
	ReplyAll  bool
	TextLen   int
}

// chatThreadKey normalizes a thread key like the message store does.
func chatThreadKey(key string) string {
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(key), "<"), ">")
}

// SetChatLink records the draft a thread's chat composer is editing.
func (s *Store) SetChatLink(accountID, threadKey string, link ChatLink) error {
	threadKey = chatThreadKey(threadKey)
	if threadKey == "" {
		return fmt.Errorf("empty thread key")
	}
	_, err := s.db.Exec(`
		INSERT INTO chat_drafts (account_id, thread_key, draft_id, message_id, reply_all, text_len)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, thread_key) DO UPDATE SET
			draft_id = excluded.draft_id,
			message_id = excluded.message_id,
			reply_all = excluded.reply_all,
			text_len = excluded.text_len`,
		accountID, threadKey, link.DraftID, link.MessageID, link.ReplyAll, link.TextLen)
	if err != nil {
		return fmt.Errorf("failed to set chat draft: %w", err)
	}
	return nil
}

// GetChatLink returns a thread's chat draft link, or nil when it has none.
func (s *Store) GetChatLink(accountID, threadKey string) (*ChatLink, error) {
	var link ChatLink
	err := s.db.QueryRow(`
		SELECT draft_id, message_id, reply_all, text_len FROM chat_drafts
		WHERE account_id = ? AND thread_key = ?`,
		accountID, chatThreadKey(threadKey),
	).Scan(&link.DraftID, &link.MessageID, &link.ReplyAll, &link.TextLen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chat draft: %w", err)
	}
	return &link, nil
}

// DeleteChatLink detaches a thread from its chat draft; the draft itself stays.
func (s *Store) DeleteChatLink(accountID, threadKey string) error {
	_, err := s.db.Exec(`DELETE FROM chat_drafts WHERE account_id = ? AND thread_key = ?`,
		accountID, chatThreadKey(threadKey))
	if err != nil {
		return fmt.Errorf("failed to delete chat draft: %w", err)
	}
	return nil
}

// MoveChatLink moves a chat draft link from one chat key to another, or drops
// it when to is empty; the draft itself stays. A link already on to wins.
func (s *Store) MoveChatLink(accountID, from, to string) error {
	if to != "" {
		_, err := s.db.Exec(`UPDATE OR IGNORE chat_drafts SET thread_key = ? WHERE account_id = ? AND thread_key = ?`,
			chatThreadKey(to), accountID, chatThreadKey(from))
		if err != nil {
			return fmt.Errorf("failed to move chat draft: %w", err)
		}
	}
	return s.DeleteChatLink(accountID, from)
}
