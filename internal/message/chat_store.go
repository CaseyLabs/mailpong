package message

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"
)

// Chat list sections.
const (
	ChatSectionAll      = "all"
	ChatSectionUnread   = "unread"
	ChatSectionPriority = "priority"
	ChatSectionLow      = "low"
	ChatSectionSnoozed  = "snoozed"
)

// Sender categories that override header-based bulk classification.
const (
	SenderCategoryPriority = "priority"
	SenderCategoryLow      = "low"
)

// SenderChatPrefix starts the chat key of a sender chat: every thread started
// by a combined sender maps to SenderChatPrefix + lowercased email. A thread
// key is a Message-ID or UUID, so it never has this prefix. Senders are
// combined per the chat_combine_senders setting (default on); sender_chat
// records a per-sender choice, and the account's own addresses never combine.
const SenderChatPrefix = "sender:"

// Chat is a conversation row in the chat list, with local triage state.
type Chat struct {
	Conversation
	ThreadKey     string     `json:"threadKey"`             // chat key: thread id without angle brackets, or SenderChatPrefix+email; key for pin/snooze
	SenderEmail   string     `json:"senderEmail,omitempty"` // set only for sender chats
	IsPinned      bool       `json:"isPinned"`
	SnoozedUntil  *time.Time `json:"snoozedUntil,omitempty"`
	IsLowPriority bool       `json:"isLowPriority"`
	LastFromMe    bool       `json:"lastFromMe"` // latest message is mine: awaiting their reply
	// Recipients names the other side of threads where every sender is me
	// (Sent folder rows).
	Recipients []Address `json:"recipients,omitempty"`

	threadKeys []string // thread keys grouped into this chat
}

// senderOfChatKey returns the sender email of a sender chat key, or "".
func senderOfChatKey(key string) string {
	email, ok := strings.CutPrefix(key, SenderChatPrefix)
	if !ok {
		return ""
	}
	return email
}

// ChatState is the local pin/snooze state of one thread. Nil fields are unset.
type ChatState struct {
	PinnedAt     *time.Time
	SnoozedUntil *time.Time
	SnoozedAt    *time.Time
}

func (st ChatState) empty() bool {
	return st.PinnedAt == nil && st.SnoozedUntil == nil
}

// DueSnooze is a snoozed thread whose snooze has ended.
type DueSnooze struct {
	AccountID  string
	ThreadKey  string
	WokeByMail bool // a new inbox message arrived during the snooze
	// The thread now belongs to a sender chat, so it has no chat row of its
	// own; its snooze ends without a wake-up.
	InSenderChat bool

	// Latest inbox message of the thread; empty when none remain.
	LatestMessageID string
	FolderID        string
	ThreadID        string
	Subject         string
	FromName        string
	FromEmail       string
}

// threadKeyExpr is the normalized thread key of a messages row, matching
// GetConversation's normalization.
func threadKeyExpr(prefix string) string {
	return fmt.Sprintf("REPLACE(REPLACE(COALESCE(%[1]sthread_id, %[1]sid), '<', ''), '>', '')", prefix)
}

// threadKeyMatch is an index-friendly condition selecting a thread's
// messages by key; it takes args threadKeyArgs(key).
func threadKeyMatch(prefix string) string {
	return fmt.Sprintf("(%[1]sthread_id IN (?, ?) OR (%[1]sthread_id IS NULL AND %[1]sid = ?))", prefix)
}

func threadKeyArgs(key string) []any {
	return []any{key, "<" + key + ">", key}
}

// threadKeysMatch is threadKeyMatch for many keys, passed as one JSON array;
// it takes args threadKeysArgs(keys) and avoids SQLite's variable limit.
func threadKeysMatch(prefix string) string {
	return fmt.Sprintf(`(%[1]sthread_id IN (SELECT value FROM json_each(?) UNION ALL SELECT '<' || value || '>' FROM json_each(?))
		OR (%[1]sthread_id IS NULL AND %[1]sid IN (SELECT value FROM json_each(?))))`, prefix)
}

func threadKeysArgs(keys []string) []any {
	j := keysJSON(keys)
	return []any{j, j, j}
}

// keysJSON encodes keys as a JSON array for json_each.
func keysJSON(keys []string) string {
	b, _ := json.Marshal(keys) // a []string always encodes
	return string(b)
}

// myAddresses selects (account_id, email) for each account's own addresses,
// lowercased: the account email and its identities.
const myAddresses = `SELECT id AS account_id, LOWER(email) AS email FROM accounts
	UNION SELECT account_id, LOWER(email) FROM identities`

// combineSendersDefault is the chat_combine_senders setting
// (settings.KeyChatCombineSenders) as 0 or 1; unset means on.
const combineSendersDefault = `COALESCE((SELECT value <> 'false' FROM settings WHERE key = 'chat_combine_senders'), 1)`

// senderCombinedExpr is true when the sender email in col is combined for the
// account: not empty, not one of the account's own addresses (joined as me),
// and combined in sender_chat (joined as sch) or, without a row there, by
// default. The query must include senderCombinedJoins for the same columns.
func senderCombinedExpr(col string) string {
	return fmt.Sprintf("(%[1]s <> '' AND me.email IS NULL AND COALESCE(sch.combined, %[2]s) = 1)", col, combineSendersDefault)
}

// senderCombinedJoins joins sender_chat as sch and myAddresses as me for the
// account and sender email columns, as senderCombinedExpr expects.
func senderCombinedJoins(accountCol, emailCol string) string {
	return fmt.Sprintf(`
		LEFT JOIN sender_chat sch ON sch.account_id = %[1]s AND sch.email = %[2]s
		LEFT JOIN (%[3]s) me ON me.account_id = %[1]s AND me.email = %[2]s`, accountCol, emailCol, myAddresses)
}

// chatKeyRows maps messages to chat keys. It selects (mid, thread_key,
// account_id, chat_key) for messages whose folder (f) matches cond. A
// thread's starter is its earliest message in scope; when the starter's
// sender is combined (senderCombinedExpr), the whole thread maps to the
// sender chat, otherwise chat_key is the thread key. cond may narrow by
// account or thread but must keep whole threads, or starters change.
func chatKeyRows(cond string) string {
	key := threadKeyExpr("m.")
	return `
		SELECT k.mid, k.thread_key, k.account_id,
			CASE WHEN ` + senderCombinedExpr("k.starter") + ` THEN '` + SenderChatPrefix + `' || k.starter ELSE k.thread_key END AS chat_key
		FROM (
			SELECT m.id AS mid, ` + key + ` AS thread_key, f.account_id,
				FIRST_VALUE(COALESCE(LOWER(m.from_email), '')) OVER (PARTITION BY ` + key + `, f.account_id ORDER BY m.date, m.id) AS starter
			FROM messages m
			INNER JOIN folders f ON m.folder_id = f.id
			WHERE ` + cond + `
		) k` + senderCombinedJoins("k.account_id", "k.starter")
}

// isSenderKey matches a sender chat key in col, case-sensitively (LIKE is
// not).
func isSenderKey(col string) string {
	return fmt.Sprintf("substr(%s, 1, %d) = '%s'", col, len(SenderChatPrefix), SenderChatPrefix)
}

// threadKeyRows is chatKeyRows without sender chats: every chat key is the
// thread key, so it skips the window pass over the whole scope.
func threadKeyRows(cond string) string {
	key := threadKeyExpr("m.")
	return `
		SELECT m.id AS mid, ` + key + ` AS thread_key, f.account_id, ` + key + ` AS chat_key
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id
		WHERE ` + cond
}

// hasSenderChats reports whether any sender may combine: the default is on,
// or some sender is combined explicitly.
func (s *Store) hasSenderChats() (bool, error) {
	var found bool
	err := s.db.QueryRow(`SELECT ` + combineSendersDefault + ` = 1 OR EXISTS (SELECT 1 FROM sender_chat WHERE combined = 1)`).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("failed to check sender chats: %w", err)
	}
	return found, nil
}

// isLowExpr classifies one message: a sender override wins, otherwise is_bulk.
const isLowExpr = `CASE sc.category WHEN 'low' THEN 1 WHEN 'priority' THEN 0 ELSE COALESCE(m.is_bulk, 0) END`

// snoozeActiveExpr is true while a thread's snooze runs: not yet due, and no
// message in scope arrived after it started. received_at is stored as UTC
// text, so its first 19 characters compare against snoozed_at as text.
const snoozeActiveExpr = `(cs.snoozed_until IS NOT NULL AND cs.snoozed_until > ?
	AND COALESCE(g.latest_received, '') <= strftime('%Y-%m-%d %H:%M:%S', COALESCE(cs.snoozed_at, 0), 'unixepoch'))`

// recipientsJSONExpr aggregates the To lists of a group's Sent messages for
// parseAggregatedToListJSON; it needs the folders join as f.
const recipientsJSONExpr = `json_group_array(json(CASE WHEN f.folder_type = 'sent' AND json_valid(m.to_list) THEN m.to_list ELSE '[]' END))`

// chatListColumns are the per-chat columns ListChats scans.
var chatListColumns = `
				MIN(COALESCE(m.thread_id, m.id)) AS conv_thread_id,
				ck.chat_key AS thread_key,
				COALESCE(MIN(m.subject), '') AS subject,
				MAX(m.snippet) AS snippet,
				COUNT(*) AS message_count,
				SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) AS unread_count,
				MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END) AS has_attachments,
				MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END) AS is_starred,
				MAX(m.date) AS latest_date,
				GROUP_CONCAT(m.id) AS message_ids,
				MAX(CASE WHEN m.smime_encrypted = 1 OR m.pgp_encrypted = 1 THEN 1 ELSE 0 END) AS is_encrypted,
				a.id AS account_id,
				a.name AS account_name,
				a.color AS account_color,
				MIN(f.id) AS folder_id,
				json_group_array(json_object('name', m.from_name, 'email', m.from_email, 'date', m.date, 'snippet', m.snippet)) AS participants_json,
				MIN(` + isLowExpr + `) AS is_low,
				MAX(REPLACE(SUBSTR(m.received_at, 1, 19), 'T', ' ')) AS latest_received,
				` + recipientsJSONExpr + ` AS recipients_json,
				CASE WHEN ` + isSenderKey("ck.chat_key") + ` THEN json_group_array(DISTINCT ck.thread_key) END AS thread_keys,
				CASE WHEN ` + isSenderKey("ck.chat_key") + ` THEN MAX(m.date || char(31) || COALESCE(m.subject, '')) END AS latest_subject
`

// chatCountColumns are the columns chatBaseQuery's GROUP BY and section
// filters need; counting skips the JSON aggregates.
var chatCountColumns = `
				ck.chat_key AS thread_key,
				a.id AS account_id,
				SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) AS unread_count,
				MIN(` + isLowExpr + `) AS is_low,
				MAX(REPLACE(SUBSTR(m.received_at, 1, 19), 'T', ' ')) AS latest_received
`

// chatBaseQuery builds the grouped chat query for a scope (a folder ID, or
// "" for every enabled account's inbox) and section, selecting columns from
// each chat group; callers add ordering and paging or wrap it in a count.
// senderChats selects chatKeyRows over threadKeyRows.
func chatBaseQuery(scope, section, columns string, now time.Time, senderChats bool) (string, []any, error) {
	folderCond := "f.folder_type = 'inbox'"
	var args []any
	if scope != "" {
		folderCond = "f.id = ?"
		args = append(args, scope)
	}

	var where string
	switch section {
	case ChatSectionAll, "":
		where = "NOT " + snoozeActiveExpr
	case ChatSectionUnread:
		where = "NOT " + snoozeActiveExpr + " AND g.unread_count > 0"
	case ChatSectionPriority:
		where = "NOT " + snoozeActiveExpr + " AND g.is_low = 0"
	case ChatSectionLow:
		where = "NOT " + snoozeActiveExpr + " AND g.is_low = 1"
	case ChatSectionSnoozed:
		where = snoozeActiveExpr
	default:
		return "", nil, fmt.Errorf("unknown chat section %q", section)
	}
	args = append(args, now.Unix())

	keyRows := threadKeyRows
	if senderChats {
		keyRows = chatKeyRows
	}

	// A chat is low priority only when every message in scope is, so one
	// human reply in a newsletter thread lifts it into Chats.
	query := `
		SELECT g.*, cs.pinned_at, cs.snoozed_until
		FROM (
			SELECT ` + columns + `
			FROM (` + keyRows(folderCond) + `) ck
			INNER JOIN messages m ON m.id = ck.mid
			INNER JOIN folders f ON m.folder_id = f.id
			INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
			LEFT JOIN sender_category sc ON sc.account_id = a.id AND sc.email = LOWER(m.from_email)
			GROUP BY ck.chat_key, a.id
		) g
		LEFT JOIN conversation_state cs ON cs.account_id = g.account_id AND cs.thread_key = g.thread_key
		WHERE ` + where
	return query, args, nil
}

// ListChats returns a page of chats for a scope (folder ID, or "" for the
// unified inbox) and section. Pinned chats sort first; the snoozed section
// sorts by wake time. Snoozed chats appear only in the snoozed section.
func (s *Store) ListChats(scope, section string, now time.Time, offset, limit int) ([]*Chat, error) {
	senderChats, err := s.hasSenderChats()
	if err != nil {
		return nil, err
	}
	query, args, err := chatBaseQuery(scope, section, chatListColumns, now, senderChats)
	if err != nil {
		return nil, err
	}
	order := " ORDER BY (cs.pinned_at IS NOT NULL) DESC, g.latest_date DESC"
	if section == ChatSectionSnoozed {
		order = " ORDER BY cs.snoozed_until ASC"
	}
	query += order + " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats: %w", err)
	}
	defer rows.Close()

	var chats []*Chat
	for rows.Next() {
		c := &Chat{}
		var snippet, latestDate, messageIDs, participantsJSON, latestReceived, recipientsJSON, threadKeys, latestSubject sql.NullString
		var pinnedAt, snoozedUntil sql.NullInt64
		if err := rows.Scan(
			&c.ThreadID, &c.ThreadKey, &c.Subject, &snippet, &c.MessageCount, &c.UnreadCount,
			&c.HasAttachments, &c.IsStarred, &latestDate, &messageIDs, &c.IsEncrypted,
			&c.AccountID, &c.AccountName, &c.AccountColor, &c.FolderID, &participantsJSON,
			&c.IsLowPriority, &latestReceived, &recipientsJSON, &threadKeys, &latestSubject, &pinnedAt, &snoozedUntil,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chat: %w", err)
		}
		c.Snippet = snippet.String
		c.SenderEmail = senderOfChatKey(c.ThreadKey)
		if latestSubject.Valid {
			// date, U+001F, subject: the newest message's subject. Dates
			// share one storage format, so text order is date order.
			_, c.Subject, _ = strings.Cut(latestSubject.String, "\x1f")
		}
		// Only sender chats aggregate their thread keys; a thread chat's
		// key is its one thread key.
		c.threadKeys = []string{c.ThreadKey}
		if threadKeys.Valid {
			if err := json.Unmarshal([]byte(threadKeys.String), &c.threadKeys); err != nil {
				return nil, fmt.Errorf("failed to parse chat thread keys: %w", err)
			}
		}
		if latestDate.String != "" {
			c.LatestDate = parseTimeString(latestDate.String)
		}
		if messageIDs.String != "" {
			c.MessageIDs = strings.Split(messageIDs.String, ",")
		}
		var latestSnippet string
		c.Participants, latestSnippet = parseParticipantsJSON(participantsJSON.String)
		if latestSnippet != "" {
			c.Snippet = latestSnippet
		}
		c.Recipients = parseAggregatedToListJSON(recipientsJSON.String)
		c.IsPinned = pinnedAt.Valid
		if snoozedUntil.Valid && section == ChatSectionSnoozed {
			t := time.Unix(snoozedUntil.Int64, 0).UTC()
			c.SnoozedUntil = &t
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chats: %w", err)
	}

	if err := s.fillLastFromMe(chats); err != nil {
		return nil, err
	}
	return chats, nil
}

// ChatSearchResult is a folder search result with the chat row's chat key
// and recipients.
type ChatSearchResult struct {
	ConversationSearchResult
	ThreadKey   string `json:"threadKey"`             // chat key, as Chat.ThreadKey
	SenderEmail string `json:"senderEmail,omitempty"` // set only for sender chats
	// Recipients names the other side of threads in a Sent folder, as
	// Chat.Recipients does.
	Recipients []Address `json:"recipients,omitempty"`
}

// SearchChats runs SearchConversations for a folder and merges the results
// as mergeChatResults does.
func (s *Store) SearchChats(folderID, query string, offset, limit int, filter string) ([]*ChatSearchResult, error) {
	found, _, err := s.SearchConversations(folderID, query, offset, limit, filter)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	// Folder search leaves the account unset; merging keys on it.
	var accountID string
	if err := s.db.QueryRow(`SELECT account_id FROM folders WHERE id = ?`, folderID).Scan(&accountID); err != nil {
		return nil, fmt.Errorf("failed to get folder account: %w", err)
	}
	for _, c := range found {
		c.AccountID = accountID
	}
	return s.mergeChatResults(found, query, "f.id = ?", folderID)
}

// SearchChatsUnifiedInbox runs SearchConversationsUnifiedInbox and merges the
// results as mergeChatResults does, never across accounts.
func (s *Store) SearchChatsUnifiedInbox(query string, offset, limit int, filter string) ([]*ChatSearchResult, error) {
	found, _, err := s.SearchConversationsUnifiedInbox(query, offset, limit, filter)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return s.mergeChatResults(found, query, "f.folder_type = 'inbox'")
}

// chatRef names a thread or chat key within an account: sender chats are per
// account.
type chatRef struct{ account, key string }

// mergeChatResults merges search results (with AccountID set) in the folder
// scope cond (taking args) that belong to the same sender chat into one
// result, at the first, newest one's position, and in a Sent folder adds each
// thread's recipients so rows name them instead of me. A merged page can hold
// fewer results than were found.
func (s *Store) mergeChatResults(found []*ConversationSearchResult, query, cond string, args ...any) ([]*ChatSearchResult, error) {
	refs := make([]chatRef, len(found))
	keys := make([]string, len(found))
	unbracket := strings.NewReplacer("<", "", ">", "")
	for i, c := range found {
		keys[i] = unbracket.Replace(c.ThreadID)
		refs[i] = chatRef{c.AccountID, keys[i]}
	}
	chatKeys, err := s.threadChatKeys(cond, keys, args...)
	if err != nil {
		return nil, err
	}

	var results []*ChatSearchResult
	byChat := map[chatRef]*ChatSearchResult{}
	senderFirst := map[chatRef]*ChatSearchResult{} // first thread -> sender chat
	byThread := make([]*ChatSearchResult, len(found))
	for i, c := range found {
		chatKey, ok := chatKeys[refs[i]]
		if !ok {
			chatKey = keys[i]
		}
		chat := chatRef{c.AccountID, chatKey}
		if r := byChat[chat]; r != nil {
			r.MessageCount += c.MessageCount
			r.UnreadCount += c.UnreadCount
			r.HasAttachments = r.HasAttachments || c.HasAttachments
			r.IsStarred = r.IsStarred || c.IsStarred
			r.IsEncrypted = r.IsEncrypted || c.IsEncrypted
			r.MessageIDs = append(r.MessageIDs, c.MessageIDs...)
			r.Participants = mergeParticipants(r.Participants, c.Participants)
			byThread[i] = r
			continue
		}
		r := &ChatSearchResult{ConversationSearchResult: *c, ThreadKey: chatKey, SenderEmail: senderOfChatKey(chatKey)}
		byChat[chat] = r
		byThread[i] = r
		results = append(results, r)
		if r.SenderEmail != "" {
			senderFirst[refs[i]] = r
		}
	}
	if err := s.fillLatestSubjects(cond, args, query, senderFirst); err != nil {
		return nil, err
	}
	if found[0].FolderType != "sent" {
		return results, nil
	}
	recipients, err := s.threadRecipients(cond, keys, args...)
	if err != nil {
		return nil, err
	}
	seen := map[*ChatSearchResult]map[string]bool{}
	for i, r := range byThread {
		if seen[r] == nil {
			seen[r] = map[string]bool{}
		}
		for _, a := range recipients[refs[i]] {
			email := strings.ToLower(a.Email)
			if seen[r][email] {
				continue
			}
			seen[r][email] = true
			r.Recipients = append(r.Recipients, a)
		}
	}
	return results, nil
}

// fillLatestSubjects gives each sender chat result the subject of its newest
// message in the folder scope cond (taking args), as ListChats does. Results
// are newest first, so that message is in the result's first thread; first
// maps that thread to its result.
func (s *Store) fillLatestSubjects(cond string, args []any, query string, first map[chatRef]*ChatSearchResult) error {
	if len(first) == 0 {
		return nil
	}
	keys := make([]string, 0, len(first))
	for ref := range first {
		keys = append(keys, ref.key)
	}
	rows, err := s.db.Query(`
		SELECT f.account_id, `+threadKeyExpr("m.")+` AS thread_key, MAX(m.date || char(31) || COALESCE(m.subject, ''))
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id
		WHERE `+cond+` AND `+threadKeysMatch("m.")+`
		GROUP BY f.account_id, thread_key`, append(slices.Clone(args), threadKeysArgs(keys)...)...)
	if err != nil {
		return fmt.Errorf("failed to query latest subjects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref chatRef
		var latest string
		if err := rows.Scan(&ref.account, &ref.key, &latest); err != nil {
			return fmt.Errorf("failed to scan latest subject: %w", err)
		}
		if r := first[ref]; r != nil {
			_, r.Subject, _ = strings.Cut(latest, "\x1f")
			r.HighlightedSubject = highlightMatches(r.Subject, query)
		}
	}
	return rows.Err()
}

// mergeParticipants appends the addresses in more not already in list,
// comparing emails case-insensitively.
func mergeParticipants(list, more []Address) []Address {
	for _, a := range more {
		if !slices.ContainsFunc(list, func(b Address) bool { return strings.EqualFold(a.Email, b.Email) }) {
			list = append(list, a)
		}
	}
	return list
}

// threadChatKeys returns the sender chat key of each given thread, by account
// and thread key, that maps to one within the folder scope cond (taking
// args); other threads are absent.
func (s *Store) threadChatKeys(cond string, keys []string, args ...any) (map[chatRef]string, error) {
	chatKeys := map[chatRef]string{}
	if len(keys) == 0 {
		return chatKeys, nil
	}
	senderChats, err := s.hasSenderChats()
	if err != nil || !senderChats {
		return chatKeys, err
	}
	return chatKeys, s.queryChatKeys(chatKeys, cond, keys, args...)
}

// inboxChatKeys is threadChatKeys over the inbox for threads by account,
// checking for sender chats once.
func (s *Store) inboxChatKeys(byAccount map[string][]string) (map[chatRef]string, error) {
	chatKeys := map[chatRef]string{}
	if len(byAccount) == 0 {
		return chatKeys, nil
	}
	senderChats, err := s.hasSenderChats()
	if err != nil || !senderChats {
		return chatKeys, err
	}
	for accountID, keys := range byAccount {
		if err := s.queryChatKeys(chatKeys, "f.folder_type = 'inbox' AND f.account_id = ?", keys, accountID); err != nil {
			return nil, err
		}
	}
	return chatKeys, nil
}

// queryChatKeys adds to chatKeys the sender chat key of each given thread
// that maps to one within the folder scope cond (taking args).
func (s *Store) queryChatKeys(chatKeys map[chatRef]string, cond string, keys []string, args ...any) error {
	if len(keys) == 0 {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT DISTINCT ck.account_id, ck.thread_key, ck.chat_key
		FROM (`+chatKeyRows(cond+" AND "+threadKeysMatch("m."))+`) ck
		WHERE ck.chat_key <> ck.thread_key`, append(slices.Clone(args), threadKeysArgs(keys)...)...)
	if err != nil {
		return fmt.Errorf("failed to query chat keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref chatRef
		var chatKey string
		if err := rows.Scan(&ref.account, &ref.key, &chatKey); err != nil {
			return fmt.Errorf("failed to scan chat key: %w", err)
		}
		chatKeys[ref] = chatKey
	}
	return rows.Err()
}

// threadRecipients returns the Sent recipients of threads in the folder scope
// cond (taking args), by account and thread key, aggregated as in the chat
// list.
func (s *Store) threadRecipients(cond string, keys []string, args ...any) (map[chatRef][]Address, error) {
	query := `
		SELECT f.account_id, ` + threadKeyExpr("m.") + ` AS thread_key, ` + recipientsJSONExpr + `
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id
		WHERE ` + cond + ` AND ` + threadKeysMatch("m.") + `
		GROUP BY f.account_id, thread_key`
	rows, err := s.db.Query(query, append(slices.Clone(args), threadKeysArgs(keys)...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query thread recipients: %w", err)
	}
	defer rows.Close()

	recipients := make(map[chatRef][]Address, len(keys))
	for rows.Next() {
		var ref chatRef
		var recipientsJSON sql.NullString
		if err := rows.Scan(&ref.account, &ref.key, &recipientsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan thread recipients: %w", err)
		}
		recipients[ref] = parseAggregatedToListJSON(recipientsJSON.String)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate thread recipients: %w", err)
	}
	return recipients, nil
}

// CountChats returns the number of chats ListChats would return unpaged.
func (s *Store) CountChats(scope, section string, now time.Time) (int, error) {
	senderChats, err := s.hasSenderChats()
	if err != nil {
		return 0, err
	}
	query, args, err := chatBaseQuery(scope, section, chatCountColumns, now, senderChats)
	if err != nil {
		return 0, err
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM ("+query+")", args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count chats: %w", err)
	}
	return n, nil
}

// fillLastFromMe marks chats whose latest message is mine: either a Sent copy
// at least as new as the latest listed message, or a listed message from one
// of the account's identities.
func (s *Store) fillLastFromMe(chats []*Chat) error {
	if len(chats) == 0 {
		return nil
	}

	mine := map[string]map[string]bool{} // account -> lowercased identity emails
	rows, err := s.db.Query(myAddresses)
	if err != nil {
		return fmt.Errorf("failed to load identities: %w", err)
	}
	for rows.Next() {
		var accountID, email string
		if err := rows.Scan(&accountID, &email); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan identity: %w", err)
		}
		if mine[accountID] == nil {
			mine[accountID] = map[string]bool{}
		}
		mine[accountID][email] = true
	}
	rows.Close()

	// Latest Sent date per chat on this page, matching each account's thread
	// keys as one JSON array.
	type threadRef struct{ accountID, key string }
	chatOf := map[threadRef]*Chat{}
	keysOf := map[string][]string{}
	for _, c := range chats {
		for _, k := range c.threadKeys {
			chatOf[threadRef{c.AccountID, k}] = c
			keysOf[c.AccountID] = append(keysOf[c.AccountID], k)
		}
	}
	conds := make([]string, 0, len(keysOf))
	args := make([]any, 0, len(keysOf)*4)
	for accountID, keys := range keysOf {
		conds = append(conds, "(m.account_id = ? AND "+threadKeysMatch("m.")+")")
		args = append(args, accountID)
		args = append(args, threadKeysArgs(keys)...)
	}
	rows, err = s.db.Query(`
		SELECT m.account_id, `+threadKeyExpr("m.")+` AS k, m.date
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'sent'
		WHERE `+strings.Join(conds, " OR "), args...)
	if err != nil {
		return fmt.Errorf("failed to query sent replies: %w", err)
	}
	latestSent := map[*Chat]time.Time{}
	for rows.Next() {
		var accountID, key string
		var date sql.NullString
		if err := rows.Scan(&accountID, &key, &date); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan sent reply: %w", err)
		}
		c := chatOf[threadRef{accountID, key}]
		if t := parseTimeString(date.String); c != nil && t.After(latestSent[c]) {
			latestSent[c] = t
		}
	}
	rows.Close()

	for _, c := range chats {
		if sent, ok := latestSent[c]; ok && !sent.Before(c.LatestDate) {
			c.LastFromMe = true
			continue
		}
		if len(c.Participants) > 0 && mine[c.AccountID][strings.ToLower(c.Participants[0].Email)] {
			c.LastFromMe = true
		}
	}
	return nil
}

// GetChatState returns a thread's pin/snooze state (zero when unset).
func (s *Store) GetChatState(accountID, threadKey string) (ChatState, error) {
	var st ChatState
	var pinnedAt, snoozedUntil, snoozedAt sql.NullInt64
	err := s.db.QueryRow(`
		SELECT pinned_at, snoozed_until, snoozed_at FROM conversation_state
		WHERE account_id = ? AND thread_key = ?`,
		accountID, normalizeMessageID(threadKey),
	).Scan(&pinnedAt, &snoozedUntil, &snoozedAt)
	if err == sql.ErrNoRows {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("failed to get chat state: %w", err)
	}
	st.PinnedAt = unixPtr(pinnedAt)
	st.SnoozedUntil = unixPtr(snoozedUntil)
	st.SnoozedAt = unixPtr(snoozedAt)
	return st, nil
}

// SetChatState replaces a thread's pin/snooze state; an empty state deletes the row.
func (s *Store) SetChatState(accountID, threadKey string, st ChatState) error {
	threadKey = normalizeMessageID(threadKey)
	if threadKey == "" {
		return fmt.Errorf("empty thread key")
	}
	if st.empty() {
		if _, err := s.db.Exec(`DELETE FROM conversation_state WHERE account_id = ? AND thread_key = ?`, accountID, threadKey); err != nil {
			return fmt.Errorf("failed to clear chat state: %w", err)
		}
		return nil
	}
	switch {
	case st.SnoozedUntil == nil:
		st.SnoozedAt = nil
	case st.SnoozedAt == nil:
		// Without a start, any inbox mail would count as newer and end the snooze.
		now := time.Now()
		st.SnoozedAt = &now
	}
	_, err := s.db.Exec(`
		INSERT INTO conversation_state (account_id, thread_key, pinned_at, snoozed_until, snoozed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_id, thread_key) DO UPDATE SET
			pinned_at = excluded.pinned_at,
			snoozed_until = excluded.snoozed_until,
			snoozed_at = excluded.snoozed_at`,
		accountID, threadKey, unixArg(st.PinnedAt), unixArg(st.SnoozedUntil), unixArg(st.SnoozedAt))
	if err != nil {
		return fmt.Errorf("failed to set chat state: %w", err)
	}
	return nil
}

// NextSnoozeWake returns the earliest snooze end, or nil when nothing is snoozed.
func (s *Store) NextSnoozeWake() (*time.Time, error) {
	var until sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(snoozed_until) FROM conversation_state`).Scan(&until); err != nil {
		return nil, fmt.Errorf("failed to get next snooze: %w", err)
	}
	return unixPtr(until), nil
}

// ListDueSnoozes returns snoozed threads whose snooze ended at or before now,
// with the latest inbox message of each.
func (s *Store) ListDueSnoozes(now time.Time) ([]DueSnooze, error) {
	type due struct {
		accountID, threadKey string
		snoozedAt            int64
	}
	rows, err := s.db.Query(`
		SELECT account_id, thread_key, COALESCE(snoozed_at, 0) FROM conversation_state
		WHERE snoozed_until IS NOT NULL AND snoozed_until <= ?
		ORDER BY account_id, thread_key`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("failed to query due snoozes: %w", err)
	}
	var dues []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.accountID, &d.threadKey, &d.snoozedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan due snooze: %w", err)
		}
		dues = append(dues, d)
	}
	rows.Close()

	// Find thread snoozes now inside a sender chat, one query per account.
	threadKeys := map[string][]string{}
	for _, d := range dues {
		if senderOfChatKey(d.threadKey) == "" {
			threadKeys[d.accountID] = append(threadKeys[d.accountID], d.threadKey)
		}
	}
	inSenderChat, err := s.inboxChatKeys(threadKeys)
	if err != nil {
		return nil, err
	}
	result := make([]DueSnooze, 0, len(dues))
	for _, d := range dues {
		ds := DueSnooze{AccountID: d.accountID, ThreadKey: d.threadKey}
		if inSenderChat[chatRef{d.accountID, d.threadKey}] != "" {
			ds.InSenderChat = true
			result = append(result, ds)
			continue
		}
		var threadID, fromName, fromEmail sql.NullString
		var woke bool
		// A sender chat's latest message is found across its mapped threads.
		from := `messages m INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'inbox'`
		cond := "m.account_id = ? AND " + threadKeyMatch("m.")
		args := append([]any{d.snoozedAt, d.accountID}, threadKeyArgs(d.threadKey)...)
		if senderOfChatKey(d.threadKey) != "" {
			from = `(` + chatKeyRows("f.folder_type = 'inbox' AND f.account_id = ?") + `) ck INNER JOIN messages m ON m.id = ck.mid`
			cond = "ck.chat_key = ?"
			args = []any{d.snoozedAt, d.accountID, d.threadKey}
		}
		err := s.db.QueryRow(`
			SELECT m.id, m.folder_id, COALESCE(m.thread_id, m.id), COALESCE(m.subject, ''), m.from_name, m.from_email,
				REPLACE(SUBSTR(m.received_at, 1, 19), 'T', ' ') > strftime('%Y-%m-%d %H:%M:%S', ?, 'unixepoch')
			FROM `+from+`
			WHERE `+cond+`
			ORDER BY m.received_at DESC, m.date DESC
			LIMIT 1`, args...,
		).Scan(&ds.LatestMessageID, &ds.FolderID, &threadID, &ds.Subject, &fromName, &fromEmail, &woke)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get snoozed thread message: %w", err)
		}
		ds.ThreadID, ds.FromName, ds.FromEmail, ds.WokeByMail = threadID.String, fromName.String, fromEmail.String, woke
		result = append(result, ds)
	}
	return result, nil
}

// CleanupChatState deletes pin/snooze rows whose thread has no messages
// left. Sender chat rows are handled by StaleChatKeys: a combined sender's
// state is kept while its chat has no inbox mail, since it shows again when
// mail arrives.
func (s *Store) CleanupChatState() (int64, error) {
	res, err := s.db.Exec(`
		DELETE FROM conversation_state
		WHERE NOT ` + isSenderKey("thread_key") + `
			AND NOT EXISTS (
				SELECT 1 FROM messages m
				WHERE m.account_id = conversation_state.account_id
					AND (m.thread_id IN (conversation_state.thread_key, '<' || conversation_state.thread_key || '>')
						OR (m.thread_id IS NULL AND m.id = conversation_state.thread_key))
			)`)
	if err != nil {
		return 0, fmt.Errorf("failed to clean up chat state: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ChatKeyMove is a key of local chat state (pin/snooze or chat draft link)
// that no longer names a chat in the inbox list. From is a thread that now
// belongs to sender chat To, or a sender chat whose sender no longer
// combines, with To empty.
type ChatKeyMove struct {
	AccountID string
	From, To  string
}

// chatStateKeys selects (account_id, thread_key) of every pin/snooze row and
// chat draft link.
const chatStateKeys = `SELECT account_id, thread_key FROM conversation_state
	UNION SELECT account_id, thread_key FROM chat_drafts`

// StaleChatKeys lists the chat state keys that no longer match the inbox
// chat list, after the combine default or the inbox threads changed.
func (s *Store) StaleChatKeys() ([]ChatKeyMove, error) {
	starter := fmt.Sprintf("substr(k.thread_key, %d)", len(SenderChatPrefix)+1)
	rows, err := s.db.Query(`
		SELECT k.account_id, k.thread_key, ` + isSenderKey("k.thread_key") + `,
			` + isSenderKey("k.thread_key") + ` AND NOT ` + senderCombinedExpr(starter) + `
		FROM (` + chatStateKeys + `) k` + senderCombinedJoins("k.account_id", starter) + `
		ORDER BY k.account_id, k.thread_key`)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat state keys: %w", err)
	}
	var moves []ChatKeyMove
	threads := map[string][]string{}
	for rows.Next() {
		var accountID, key string
		var isSender, split bool
		if err := rows.Scan(&accountID, &key, &isSender, &split); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan chat state key: %w", err)
		}
		switch {
		case split:
			moves = append(moves, ChatKeyMove{AccountID: accountID, From: key})
		case !isSender:
			threads[accountID] = append(threads[accountID], key)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chat state keys: %w", err)
	}

	chatKeys, err := s.inboxChatKeys(threads)
	if err != nil {
		return nil, err
	}
	for _, accountID := range slices.Sorted(maps.Keys(threads)) {
		for _, key := range threads[accountID] {
			if to := chatKeys[chatRef{accountID, key}]; to != "" {
				moves = append(moves, ChatKeyMove{AccountID: accountID, From: key, To: to})
			}
		}
	}
	return moves, nil
}

// MoveChatState moves m.From's pin/snooze state into m.To, or drops it when
// To is empty. A pin or snooze already on To wins over From's.
func (s *Store) MoveChatState(m ChatKeyMove) error {
	from, err := s.GetChatState(m.AccountID, m.From)
	if err != nil || from.empty() {
		return err
	}
	if m.To != "" {
		to, err := s.GetChatState(m.AccountID, m.To)
		if err != nil {
			return err
		}
		if to.PinnedAt == nil {
			to.PinnedAt = from.PinnedAt
		}
		if to.SnoozedUntil == nil {
			to.SnoozedUntil, to.SnoozedAt = from.SnoozedUntil, from.SnoozedAt
		}
		if err := s.SetChatState(m.AccountID, m.To, to); err != nil {
			return err
		}
	}
	return s.SetChatState(m.AccountID, m.From, ChatState{})
}

// NormalizeSenderEmail returns the form sender emails are stored and keyed
// in: trimmed and lowercased.
func NormalizeSenderEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SetSenderChat records whether the threads a sender starts combine into one
// sender chat for an account or show separately, overriding the default.
func (s *Store) SetSenderChat(accountID, email string, combined bool) error {
	email = NormalizeSenderEmail(email)
	if email == "" {
		return fmt.Errorf("empty sender email")
	}
	_, err := s.db.Exec(`
		INSERT INTO sender_chat (account_id, email, combined) VALUES (?, ?, ?)
		ON CONFLICT(account_id, email) DO UPDATE SET combined = excluded.combined`,
		accountID, email, combined)
	if err != nil {
		return fmt.Errorf("failed to set sender chat: %w", err)
	}
	return nil
}

// SenderChatChoice returns the per-sender combine choice recorded for an
// account, or nil when the sender follows the default.
func (s *Store) SenderChatChoice(accountID, email string) (*bool, error) {
	var combined bool
	err := s.db.QueryRow(`SELECT combined FROM sender_chat WHERE account_id = ? AND email = ?`,
		accountID, NormalizeSenderEmail(email)).Scan(&combined)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sender chat choice: %w", err)
	}
	return &combined, nil
}

// ClearSenderChat removes a sender's combine choice, so the default applies.
func (s *Store) ClearSenderChat(accountID, email string) error {
	if _, err := s.db.Exec(`DELETE FROM sender_chat WHERE account_id = ? AND email = ?`,
		accountID, NormalizeSenderEmail(email)); err != nil {
		return fmt.Errorf("failed to clear sender chat: %w", err)
	}
	return nil
}

// IsSenderChat reports whether a sender's threads are combined for an
// account, as chatKeyRows decides.
func (s *Store) IsSenderChat(accountID, email string) (bool, error) {
	var combined bool
	err := s.db.QueryRow(`
		SELECT `+senderCombinedExpr("k.starter")+`
		FROM (SELECT ? AS account_id, ? AS starter) k`+senderCombinedJoins("k.account_id", "k.starter"),
		accountID, NormalizeSenderEmail(email)).Scan(&combined)
	if err != nil {
		return false, fmt.Errorf("failed to get sender chat: %w", err)
	}
	return combined, nil
}

// SenderChatKey returns the chat key of a sender's combined chat.
func SenderChatKey(email string) string {
	return SenderChatPrefix + NormalizeSenderEmail(email)
}

// SetSenderCategory overrides bulk classification for a sender; an empty
// category removes the override.
func (s *Store) SetSenderCategory(accountID, email, category string) error {
	email = NormalizeSenderEmail(email)
	if email == "" {
		return fmt.Errorf("empty sender email")
	}
	if category != "" && category != SenderCategoryPriority && category != SenderCategoryLow {
		return fmt.Errorf("unknown sender category %q", category)
	}
	if category == "" {
		_, err := s.db.Exec(`DELETE FROM sender_category WHERE account_id = ? AND email = ?`, accountID, email)
		if err != nil {
			return fmt.Errorf("failed to clear sender category: %w", err)
		}
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO sender_category (account_id, email, category) VALUES (?, ?, ?)
		ON CONFLICT(account_id, email) DO UPDATE SET category = excluded.category`,
		accountID, email, category)
	if err != nil {
		return fmt.Errorf("failed to set sender category: %w", err)
	}
	return nil
}

// UnclassifiedMessage is a message whose bulk flag is not yet known.
type UnclassifiedMessage struct {
	ID        string
	UID       uint32
	FromEmail string
}

// ListUnclassifiedBulk returns up to limit messages in a folder with NULL
// is_bulk and a UID above afterUID, in UID order.
func (s *Store) ListUnclassifiedBulk(folderID string, afterUID uint32, limit int) ([]UnclassifiedMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, uid, from_email FROM messages
		WHERE folder_id = ? AND is_bulk IS NULL AND uid > ?
		ORDER BY uid LIMIT ?`, folderID, afterUID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list unclassified messages: %w", err)
	}
	defer rows.Close()
	var out []UnclassifiedMessage
	for rows.Next() {
		var m UnclassifiedMessage
		var from sql.NullString
		if err := rows.Scan(&m.ID, &m.UID, &from); err != nil {
			return nil, fmt.Errorf("failed to scan unclassified message: %w", err)
		}
		m.FromEmail = from.String
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetBulkFlags stores bulk classifications by message ID.
func (s *Store) SetBulkFlags(flags map[string]bool) error {
	if len(flags) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin bulk flag update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`UPDATE messages SET is_bulk = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare bulk flag update: %w", err)
	}
	defer stmt.Close()
	for id, bulk := range flags {
		if _, err := stmt.Exec(bulk, id); err != nil {
			return fmt.Errorf("failed to set bulk flag: %w", err)
		}
	}
	return tx.Commit()
}

func unixPtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(v.Int64, 0).UTC()
	return &t
}

func unixArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

// NewMail is a recently arrived message, as new-mail notifications see it.
type NewMail struct {
	UID       uint32
	ThreadID  string
	ChatKey   string // chat the message shows in: sender chat key or thread key
	Subject   string
	FromName  string
	FromEmail string
	IsLow     bool // sender override, else is_bulk

	accountID string
}

// newMailChunk keeps ListNewMail's IN list well under SQLite's variable limit.
const newMailChunk = 500

// ListNewMail returns the messages with the given IDs, highest UID first,
// each classified like chat rows (isLowExpr). Unknown IDs are skipped.
func (s *Store) ListNewMail(ids []string) ([]NewMail, error) {
	var out []NewMail
	for start := 0; start < len(ids); start += newMailChunk {
		chunk := ids[start:min(start+newMailChunk, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.db.Query(`
			SELECT m.uid, COALESCE(m.thread_id, m.id), `+threadKeyExpr("m.")+`, f.account_id,
				COALESCE(m.subject, ''), COALESCE(m.from_name, ''), COALESCE(m.from_email, ''),
				`+isLowExpr+`
			FROM messages m
			INNER JOIN folders f ON f.id = m.folder_id
			LEFT JOIN sender_category sc ON sc.account_id = f.account_id AND sc.email = LOWER(m.from_email)
			WHERE m.id IN (?`+strings.Repeat(", ?", len(chunk)-1)+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list new mail: %w", err)
		}
		for rows.Next() {
			var nm NewMail
			if err := rows.Scan(&nm.UID, &nm.ThreadID, &nm.ChatKey, &nm.accountID, &nm.Subject, &nm.FromName, &nm.FromEmail, &nm.IsLow); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan new mail: %w", err)
			}
			out = append(out, nm)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to list new mail: %w", err)
		}
	}
	if err := s.fillNewMailChatKeys(out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	return out, nil
}

// fillNewMailChatKeys replaces the thread key of new mail in a sender chat
// with the sender chat key, mapping threads as the inbox chat list does.
func (s *Store) fillNewMailChatKeys(mail []NewMail) error {
	byAccount := map[string][]string{}
	for _, nm := range mail {
		byAccount[nm.accountID] = append(byAccount[nm.accountID], nm.ChatKey)
	}
	chatKeys, err := s.inboxChatKeys(byAccount)
	if err != nil {
		return err
	}
	for i := range mail {
		if chatKey, ok := chatKeys[chatRef{mail[i].accountID, mail[i].ChatKey}]; ok {
			mail[i].ChatKey = chatKey
		}
	}
	return nil
}
