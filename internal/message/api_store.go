package message

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrAPIStaleGeneration reports a page built against a cursor generation that
// has since been invalidated; the page is discarded, not applied.
var ErrAPIStaleGeneration = errors.New("api sync cursor generation changed")

// APIRef is the identity of one remote message. Every local row it maps to
// (one per folder) links to it.
type APIRef struct {
	AccountID      string
	Provider       string
	RemoteID       string
	RemoteThreadID string
	// Containers is every remote label/folder ID; Unmapped is the subset with
	// no local mapping, kept so nothing is dropped.
	Containers []string
	Unmapped   []string
	// Version is the opaque remote version (Graph changeKey, Gmail historyId).
	Version string
}

// APICursor is the stored position of one sync scope.
type APICursor struct {
	Cursor     string
	Kind       string
	Generation int
	// Invalid marks a cursor that needs a full reconcile before use.
	Invalid bool
}

// APIRow is one local row of a remote message. UID is assigned by the store.
type APIRow struct {
	Message Message
	// Extras are stored in the same transaction as the row, so a crash cannot
	// leave a body without its raw crypto parts or attachments.
	Extras *APIRowExtras
	// FlagsOnly refreshes only the read/starred/draft flags of an existing
	// row (a label change): no envelope or body is touched.
	FlagsOnly bool
}

// APIRowExtras are the parsed parts the Message struct does not persist.
// Attachment IDs and MessageIDs are assigned by the store.
type APIRowExtras struct {
	SMIMERawBody   []byte
	SMIMEEncrypted bool
	PGPRawBody     []byte
	PGPEncrypted   bool
	Attachments    []*Attachment
}

// APIUpsert adds or refreshes one remote message and its per-folder rows.
// Rows absent from the list are removed from folders the message left.
type APIUpsert struct {
	Ref  APIRef
	Rows []APIRow
}

// APIPage is one unit of sync work: its messages and the cursor reached once
// they are stored. They commit together or not at all.
type APIPage struct {
	AccountID  string
	Provider   string
	Scope      string
	Generation int
	Upserts    []APIUpsert
	// Removals are remote IDs deleted at the provider.
	Removals []string
	// Cursor and CursorKind are stored with the page.
	Cursor     string
	CursorKind string
	// ClearInvalid ends an unreconciled marker (the page completes a reconcile).
	ClearInvalid bool
}

// LoadAPICursor returns the scope's cursor; a scope never synced has the zero
// cursor at generation 0.
func (s *Store) LoadAPICursor(accountID, scope string) (APICursor, error) {
	var c APICursor
	var invalid int
	err := s.db.QueryRow(`SELECT cursor,cursor_kind,generation,invalid FROM api_sync_state WHERE account_id=? AND scope=?`,
		accountID, scope).Scan(&c.Cursor, &c.Kind, &c.Generation, &invalid)
	if errors.Is(err, sql.ErrNoRows) {
		return APICursor{}, nil
	}
	if err != nil {
		return APICursor{}, fmt.Errorf("failed to read api sync cursor: %w", err)
	}
	c.Invalid = invalid != 0
	return c, nil
}

// InvalidateAPICursor drops the scope's cursor, bumps its generation and marks
// it unreconciled. Pages built before the bump can no longer commit. Local
// messages are untouched.
func (s *Store) InvalidateAPICursor(accountID, scope string) error {
	_, err := s.db.Exec(`INSERT INTO api_sync_state(account_id,scope,cursor,cursor_kind,generation,invalid,updated_at)
		VALUES(?,?,'','',1,1,?) ON CONFLICT(account_id,scope) DO UPDATE SET
		cursor='',cursor_kind='',generation=generation+1,invalid=1,updated_at=excluded.updated_at`,
		accountID, scope, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to invalidate api sync cursor: %w", err)
	}
	return nil
}

// CommitAPIPage stores the page and advances its cursor in one transaction. A
// failure leaves both unchanged, so a retry neither skips nor repeats the page.
func (s *Store) CommitAPIPage(p APIPage) error {
	if p.AccountID == "" || p.Provider == "" || p.Scope == "" {
		return fmt.Errorf("api page needs an account, provider and scope")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api page: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var generation int
	var invalid int
	err = tx.QueryRow(`SELECT generation,invalid FROM api_sync_state WHERE account_id=? AND scope=?`,
		p.AccountID, p.Scope).Scan(&generation, &invalid)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to read api sync state: %w", err)
	}
	if generation != p.Generation {
		return ErrAPIStaleGeneration
	}
	for _, u := range p.Upserts {
		u.Ref.AccountID, u.Ref.Provider = p.AccountID, p.Provider
		if err := s.applyAPIUpsert(tx, u); err != nil {
			return err
		}
	}
	for _, id := range p.Removals {
		if err := deleteAPIRef(tx, p.AccountID, p.Provider, id); err != nil {
			return err
		}
	}
	if invalid != 0 && !p.ClearInvalid {
		invalid = 1
	} else {
		invalid = 0
	}
	if _, err := tx.Exec(`INSERT INTO api_sync_state(account_id,scope,cursor,cursor_kind,generation,invalid,updated_at)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(account_id,scope) DO UPDATE SET cursor=excluded.cursor,
		cursor_kind=excluded.cursor_kind,invalid=excluded.invalid,updated_at=excluded.updated_at`,
		p.AccountID, p.Scope, p.Cursor, p.CursorKind, generation, invalid, time.Now().Unix()); err != nil {
		return fmt.Errorf("failed to advance api sync cursor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit api page: %w", err)
	}
	return nil
}

func (s *Store) applyAPIUpsert(tx *sql.Tx, u APIUpsert) error {
	r := u.Ref
	if r.RemoteID == "" {
		return fmt.Errorf("api message needs a remote ID")
	}
	// A message with an unresolved local action keeps its local state: the
	// provider's view predates the action, and the queue reconciles afterwards.
	open, err := hasOpenAPIAction(tx, r.AccountID, r.Provider, r.RemoteID)
	if err != nil {
		return err
	}
	if open {
		return nil
	}
	containers, err := json.Marshal(nonNil(r.Containers))
	if err != nil {
		return err
	}
	unmapped, err := json.Marshal(nonNil(r.Unmapped))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO api_message_refs(account_id,provider,remote_id,remote_thread_id,containers,unmapped,version)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(account_id,provider,remote_id) DO UPDATE SET
		remote_thread_id=excluded.remote_thread_id,containers=excluded.containers,
		unmapped=excluded.unmapped,version=excluded.version`,
		r.AccountID, r.Provider, r.RemoteID, r.RemoteThreadID, string(containers), string(unmapped), r.Version); err != nil {
		return fmt.Errorf("failed to store api message ref: %w", err)
	}

	existing := map[string]string{} // folder ID -> local message ID
	rows, err := tx.Query(`SELECT m.folder_id, m.id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.account_id=? AND l.provider=? AND l.remote_id=?`, r.AccountID, r.Provider, r.RemoteID)
	if err != nil {
		return fmt.Errorf("failed to read api message rows: %w", err)
	}
	for rows.Next() {
		var folderID, id string
		if err := rows.Scan(&folderID, &id); err != nil {
			_ = rows.Close()
			return err
		}
		existing[folderID] = id
	}
	if err := rows.Close(); err != nil {
		return err
	}

	want := map[string]bool{}
	for _, row := range u.Rows {
		m := row.Message
		m.AccountID = r.AccountID
		if want[m.FolderID] {
			return fmt.Errorf("api message %q lists folder %q twice", r.RemoteID, m.FolderID)
		}
		want[m.FolderID] = true
		if row.FlagsOnly {
			id, ok := existing[m.FolderID]
			if !ok {
				return fmt.Errorf("api message %q has no row in folder %q to refresh", r.RemoteID, m.FolderID)
			}
			if _, err := tx.Exec(`UPDATE messages SET is_read=?, is_starred=?, is_draft=? WHERE id=?`,
				m.IsRead, m.IsStarred, m.IsDraft, id); err != nil {
				return fmt.Errorf("failed to refresh api message flags: %w", err)
			}
			continue
		}
		if id, ok := existing[m.FolderID]; ok {
			if err := updateAPIRow(tx, id, &m); err != nil {
				return err
			}
			if err := storeAPIExtras(tx, id, row.Extras); err != nil {
				return err
			}
			continue
		}
		uid, err := allocateAPIUID(tx, m.FolderID)
		if err != nil {
			return err
		}
		m.ID, m.UID = uuid.New().String(), uid
		if err := s.createWith(tx, &m); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO api_message_rows(message_id,account_id,provider,remote_id) VALUES(?,?,?,?)`,
			m.ID, r.AccountID, r.Provider, r.RemoteID); err != nil {
			return fmt.Errorf("failed to link api message row: %w", err)
		}
		if err := storeAPIExtras(tx, m.ID, row.Extras); err != nil {
			return err
		}
	}
	for folderID, id := range existing {
		if want[folderID] {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE id=?`, id); err != nil {
			return fmt.Errorf("failed to remove api message row: %w", err)
		}
	}
	return nil
}

// updateAPIRow refreshes envelope and flags; stored bodies are replaced only
// when the new row carries one, so a fetched body is never blanked.
func updateAPIRow(tx *sql.Tx, id string, m *Message) error {
	_, err := tx.Exec(`UPDATE messages SET message_id=?, in_reply_to=?, references_list=?, thread_id=?,
		subject=?, from_name=?, from_email=?, to_list=?, cc_list=?, bcc_list=?, reply_to=?, date=?, snippet=?,
		is_read=?, is_starred=?, is_answered=?, is_forwarded=?, is_draft=?, is_deleted=?, size=?, has_attachments=?,
		body_text=CASE WHEN ? THEN ? ELSE body_text END, body_html=CASE WHEN ? THEN ? ELSE body_html END,
		body_fetched=CASE WHEN ? THEN 1 ELSE body_fetched END WHERE id=?`,
		nullString(m.MessageID), nullString(m.InReplyTo), nullString(m.References), nullString(m.ThreadID),
		m.Subject, m.FromName, m.FromEmail, nullString(m.ToList), nullString(m.CcList), nullString(m.BccList),
		nullString(m.ReplyTo), m.Date, nullString(m.Snippet),
		m.IsRead, m.IsStarred, m.IsAnswered, m.IsForwarded, m.IsDraft, m.IsDeleted, m.Size, m.HasAttachments,
		m.BodyFetched, nullString(m.BodyText), m.BodyFetched, nullString(m.BodyHTML), m.BodyFetched, id)
	if err != nil {
		return fmt.Errorf("failed to update api message row: %w", err)
	}
	return nil
}

// storeAPIExtras writes the raw crypto parts and replaces the attachments of
// one local row. Inline content is kept, like AttachmentStore.ReplaceForMessages.
func storeAPIExtras(tx *sql.Tx, id string, x *APIRowExtras) error {
	if x == nil {
		return nil
	}
	var smimeRaw, pgpRaw interface{}
	if len(x.SMIMERawBody) > 0 {
		smimeRaw = x.SMIMERawBody
	}
	if len(x.PGPRawBody) > 0 {
		pgpRaw = x.PGPRawBody
	}
	if _, err := tx.Exec(`UPDATE messages SET smime_raw_body=?, smime_encrypted=?, pgp_raw_body=?, pgp_encrypted=? WHERE id=?`,
		smimeRaw, x.SMIMEEncrypted, pgpRaw, x.PGPEncrypted, id); err != nil {
		return fmt.Errorf("failed to store api crypto parts: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM attachments WHERE message_id=?`, id); err != nil {
		return fmt.Errorf("failed to clear api attachments: %w", err)
	}
	for _, a := range x.Attachments {
		var content []byte
		if a.IsInline && len(a.Content) > 0 {
			content = a.Content
		}
		if _, err := tx.Exec(`INSERT INTO attachments (id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path, content)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.New().String(), id, a.Filename, a.ContentType, a.Size,
			nullString(a.ContentID), boolToInt(a.IsInline), boolToInt(a.Embedded), nullString(a.LocalPath), content); err != nil {
			return fmt.Errorf("failed to store api attachment: %w", err)
		}
	}
	return nil
}

// allocateAPIUID hands out the folder's next surrogate UID from folders.uid_next,
// which only ever increases, so a UID is never reused after a delete. It is
// local; no provider sees it.
func allocateAPIUID(tx *sql.Tx, folderID string) (uint32, error) {
	var next, highest sql.NullInt64
	if err := tx.QueryRow(`SELECT uid_next, (SELECT MAX(uid) FROM messages WHERE folder_id=?) FROM folders WHERE id=?`,
		folderID, folderID).Scan(&next, &highest); err != nil {
		return 0, fmt.Errorf("failed to allocate api uid: %w", err)
	}
	uid := next.Int64
	if highest.Valid && highest.Int64+1 > uid {
		uid = highest.Int64 + 1
	}
	if uid < 1 {
		uid = 1
	}
	if _, err := tx.Exec(`UPDATE folders SET uid_next=? WHERE id=?`, uid+1, folderID); err != nil {
		return 0, fmt.Errorf("failed to advance api uid: %w", err)
	}
	return uint32(uid), nil
}

func deleteAPIRef(tx *sql.Tx, accountID, provider, remoteID string) error {
	if _, err := tx.Exec(`DELETE FROM messages WHERE id IN (SELECT message_id FROM api_message_rows
		WHERE account_id=? AND provider=? AND remote_id=?)`, accountID, provider, remoteID); err != nil {
		return fmt.Errorf("failed to remove api message rows: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM api_message_refs WHERE account_id=? AND provider=? AND remote_id=?`,
		accountID, provider, remoteID); err != nil {
		return fmt.Errorf("failed to remove api message ref: %w", err)
	}
	return nil
}

// GetAPIRef returns a remote message's identity and its local message IDs by
// folder, or nil when unknown.
func (s *Store) GetAPIRef(accountID, provider, remoteID string) (*APIRef, map[string]string, error) {
	r := APIRef{AccountID: accountID, Provider: provider, RemoteID: remoteID}
	var containers, unmapped string
	err := s.db.QueryRow(`SELECT remote_thread_id,containers,unmapped,version FROM api_message_refs
		WHERE account_id=? AND provider=? AND remote_id=?`, accountID, provider, remoteID).
		Scan(&r.RemoteThreadID, &containers, &unmapped, &r.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read api message ref: %w", err)
	}
	if err := json.Unmarshal([]byte(containers), &r.Containers); err != nil {
		return nil, nil, fmt.Errorf("bad api container set: %w", err)
	}
	if err := json.Unmarshal([]byte(unmapped), &r.Unmapped); err != nil {
		return nil, nil, fmt.Errorf("bad api unmapped set: %w", err)
	}
	rows, err := s.db.Query(`SELECT m.folder_id, m.id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.account_id=? AND l.provider=? AND l.remote_id=?`, accountID, provider, remoteID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read api message rows: %w", err)
	}
	defer rows.Close()
	byFolder := map[string]string{}
	for rows.Next() {
		var folderID, id string
		if err := rows.Scan(&folderID, &id); err != nil {
			return nil, nil, err
		}
		byFolder[folderID] = id
	}
	return &r, byFolder, rows.Err()
}

// ListAPIRemoteIDs returns every remote ID the account holds locally, for
// diffing against the provider during a reconcile.
func (s *Store) ListAPIRemoteIDs(accountID, provider string) ([]string, error) {
	rows, err := s.db.Query(`SELECT remote_id FROM api_message_refs WHERE account_id=? AND provider=? ORDER BY remote_id`,
		accountID, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list api remote IDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListUnpinnedAPIRemoteIDs is ListAPIRemoteIDs without pinned refs: messages a
// server search pulled in from outside the sync window, which a windowed
// reconcile must keep.
func (s *Store) ListUnpinnedAPIRemoteIDs(accountID, provider string) ([]string, error) {
	rows, err := s.db.Query(`SELECT remote_id FROM api_message_refs WHERE account_id=? AND provider=? AND pinned=0 ORDER BY remote_id`,
		accountID, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list unpinned api remote IDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListPinnedAPIRemoteIDs returns the pinned refs: messages a server search or
// "Load older" pulled in, which a windowed reconcile does not list.
func (s *Store) ListPinnedAPIRemoteIDs(accountID, provider string) ([]string, error) {
	rows, err := s.db.Query(`SELECT remote_id FROM api_message_refs WHERE account_id=? AND provider=? AND pinned=1 ORDER BY remote_id`,
		accountID, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list pinned api remote IDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OldestAPIReceived returns the earliest receive time (Unix seconds) among the
// stored rows of the given remote IDs; ok is false when none is stored.
func (s *Store) OldestAPIReceived(accountID, provider string, remoteIDs []string) (oldest int64, ok bool, err error) {
	for chunk := range slices.Chunk(remoteIDs, 200) {
		args := []any{accountID, provider}
		for _, id := range chunk {
			args = append(args, id)
		}
		var n sql.NullInt64
		if err = s.db.QueryRow(`SELECT MIN(CAST(strftime('%s', REPLACE(SUBSTR(m.received_at, 1, 19), 'T', ' ')) AS INTEGER))
			FROM api_message_rows l JOIN messages m ON m.id=l.message_id
			WHERE l.account_id=? AND l.provider=? AND l.remote_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")+`)`,
			args...).Scan(&n); err != nil {
			return 0, false, fmt.Errorf("failed to read oldest api message: %w", err)
		}
		if n.Valid && (!ok || n.Int64 < oldest) {
			oldest, ok = n.Int64, true
		}
	}
	return oldest, ok, nil
}

// PinAPIRefs marks refs as kept across a windowed reconcile. Unknown IDs are
// ignored.
func (s *Store) PinAPIRefs(accountID, provider string, remoteIDs []string) error {
	for _, id := range remoteIDs {
		if _, err := s.db.Exec(`UPDATE api_message_refs SET pinned=1 WHERE account_id=? AND provider=? AND remote_id=?`,
			accountID, provider, id); err != nil {
			return fmt.Errorf("failed to pin api ref: %w", err)
		}
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ListAPIRemoteIDsInContainer returns the remote IDs whose stored container
// set (label or folder IDs) includes container.
func (s *Store) ListAPIRemoteIDsInContainer(accountID, provider, container string) ([]string, error) {
	rows, err := s.db.Query(`SELECT remote_id FROM api_message_refs r WHERE account_id=? AND provider=?
		AND EXISTS (SELECT 1 FROM json_each(r.containers) WHERE value=?) ORDER BY remote_id`,
		accountID, provider, container)
	if err != nil {
		return nil, fmt.Errorf("failed to list api remote IDs in container: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountFolderMessages returns how many messages a local folder holds.
func (s *Store) CountFolderMessages(folderID string) (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE folder_id=?`, folderID).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count folder messages: %w", err)
	}
	return n, nil
}

// APIMove records a completed provider move: the message's remote ID and
// container set after the move, and its local row moving between folders.
type APIMove struct {
	AccountID, Provider string
	OldRemoteID         string
	NewRemoteID         string // "" or equal to OldRemoteID keeps the ID
	Containers          []string
	FromFolderID        string
	ToFolderID          string
}

// ApplyAPIMove commits a provider move with the local row in one transaction,
// so the ref, its row links and the row's folder never disagree. The row takes
// a fresh surrogate UID in the destination folder. A changed remote ID re-keys
// the ref (its row links follow). It fails if the ref has no row in
// FromFolderID, or if NewRemoteID already names another ref.
func (s *Store) ApplyAPIMove(mv APIMove) error {
	if mv.NewRemoteID == "" {
		mv.NewRemoteID = mv.OldRemoteID
	}
	containers, err := json.Marshal(nonNil(mv.Containers))
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api move: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var rowID string
	err = tx.QueryRow(`SELECT m.id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.account_id=? AND l.provider=? AND l.remote_id=? AND m.folder_id=?`,
		mv.AccountID, mv.Provider, mv.OldRemoteID, mv.FromFolderID).Scan(&rowID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("api message %q has no row in folder %q", mv.OldRemoteID, mv.FromFolderID)
	}
	if err != nil {
		return fmt.Errorf("failed to find api move row: %w", err)
	}
	if mv.FromFolderID != mv.ToFolderID {
		var clash int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM api_message_rows l JOIN messages m ON m.id=l.message_id
			WHERE l.account_id=? AND l.provider=? AND l.remote_id=? AND m.folder_id=?`,
			mv.AccountID, mv.Provider, mv.OldRemoteID, mv.ToFolderID).Scan(&clash); err != nil {
			return fmt.Errorf("failed to check api move destination: %w", err)
		}
		if clash > 0 {
			return fmt.Errorf("api message %q already has a row in folder %q", mv.OldRemoteID, mv.ToFolderID)
		}
		uid, err := allocateAPIUID(tx, mv.ToFolderID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE messages SET folder_id=?, uid=? WHERE id=?`, mv.ToFolderID, uid, rowID); err != nil {
			return fmt.Errorf("failed to move api message row: %w", err)
		}
	}
	if mv.NewRemoteID == mv.OldRemoteID {
		if _, err := tx.Exec(`UPDATE api_message_refs SET containers=? WHERE account_id=? AND provider=? AND remote_id=?`,
			string(containers), mv.AccountID, mv.Provider, mv.OldRemoteID); err != nil {
			return fmt.Errorf("failed to update api containers: %w", err)
		}
		return tx.Commit()
	}
	// Re-key: the primary key cannot be updated under the foreign key, so
	// copy the ref under the new ID, repoint its rows, then drop the old ref.
	res, err := tx.Exec(`INSERT INTO api_message_refs(account_id,provider,remote_id,remote_thread_id,containers,unmapped,version)
		SELECT account_id,provider,?,remote_thread_id,?,unmapped,version FROM api_message_refs
		WHERE account_id=? AND provider=? AND remote_id=?
		ON CONFLICT DO NOTHING`, mv.NewRemoteID, string(containers), mv.AccountID, mv.Provider, mv.OldRemoteID)
	if err != nil {
		return fmt.Errorf("failed to re-key api ref: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("api message %q cannot be re-keyed to %q", mv.OldRemoteID, mv.NewRemoteID)
	}
	if _, err := tx.Exec(`UPDATE api_message_rows SET remote_id=? WHERE account_id=? AND provider=? AND remote_id=?`,
		mv.NewRemoteID, mv.AccountID, mv.Provider, mv.OldRemoteID); err != nil {
		return fmt.Errorf("failed to repoint api rows: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM api_message_refs WHERE account_id=? AND provider=? AND remote_id=?`,
		mv.AccountID, mv.Provider, mv.OldRemoteID); err != nil {
		return fmt.Errorf("failed to drop old api ref: %w", err)
	}
	// Queued actions follow the message to its new identity.
	if _, err := tx.Exec(`UPDATE api_pending_actions SET remote_id=? WHERE account_id=? AND provider=? AND remote_id=?`,
		mv.NewRemoteID, mv.AccountID, mv.Provider, mv.OldRemoteID); err != nil {
		return fmt.Errorf("failed to re-key queued api actions: %w", err)
	}
	return tx.Commit()
}

// StoreAPIBody saves a body fetched on demand for one existing API row, with
// its raw crypto parts and attachments, in one transaction. The envelope and
// flags are left alone.
func (s *Store) StoreAPIBody(id string, m *Message, x *APIRowExtras) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api body update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Exec(`UPDATE messages SET body_text=?, body_html=?, body_fetched=1, has_attachments=?, snippet=COALESCE(?, snippet)
		WHERE id=? AND id IN (SELECT message_id FROM api_message_rows)`,
		nullString(m.BodyText), nullString(m.BodyHTML), m.HasAttachments, nullString(m.Snippet), id)
	if err != nil {
		return fmt.Errorf("failed to store api body: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrAPIActionNotAPI, id)
	}
	if err := storeAPIExtras(tx, id, x); err != nil {
		return err
	}
	return tx.Commit()
}
