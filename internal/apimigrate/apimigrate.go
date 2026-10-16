// Package apimigrate holds the database side of the opt-in move of an existing
// IMAP account onto its Gmail/Graph API route: the migration record, the
// pre-change snapshot and the correlation of API rows with the IMAP rows the
// account already has. Route activation itself stays in the app package.
package apimigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/database"
)

// Migration states. A record exists from the snapshot onward.
const (
	StateSnapshotted = "snapshotted" // snapshot written, account still on IMAP
	StateActivated   = "activated"   // route active and reconciled, not yet correlated
	StateCompleted   = "completed"
	StateFailed      = "failed" // returned to IMAP after an error
	StateRolledBack  = "rolled-back"
)

// Record is the persisted progress of one account's migration.
type Record struct {
	AccountID    string `json:"accountId"`
	State        string `json:"state"`
	Route        string `json:"route"`
	SnapshotPath string `json:"snapshotPath"`
	Correlated   int    `json:"correlated"`
	Uncorrelated int    `json:"uncorrelated"`
	Error        string `json:"error"`
	StartedAt    int64  `json:"startedAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// Counts are the account's row counts, reported before anything changes.
type Counts struct {
	Folders      int `json:"folders"`
	IMAPMessages int `json:"imapMessages"`
	APIMessages  int `json:"apiMessages"`
	Drafts       int `json:"drafts"`
	// QueuedSubmissions are attempts still queued or of unknown outcome.
	QueuedSubmissions int `json:"queuedSubmissions"`
}

// Manifest describes a snapshot; it is stored beside it.
type Manifest struct {
	Version   int             `json:"version"`
	AccountID string          `json:"accountId"`
	Route     json.RawMessage `json:"route"`
	Counts    Counts          `json:"counts"`
	CreatedAt int64           `json:"createdAt"`
	Database  string          `json:"database"`
}

const manifestVersion = 1

// Store reads and writes migration state.
type Store struct{ db *database.DB }

func NewStore(db *database.DB) *Store { return &Store{db: db} }

// Get returns the account's record, or nil.
func (s *Store) Get(accountID string) (*Record, error) {
	r := Record{AccountID: accountID}
	err := s.db.QueryRow(`SELECT state,route,snapshot_path,correlated,uncorrelated,error,started_at,updated_at
		FROM api_migrations WHERE account_id=?`, accountID).
		Scan(&r.State, &r.Route, &r.SnapshotPath, &r.Correlated, &r.Uncorrelated, &r.Error, &r.StartedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read api migration: %w", err)
	}
	return &r, nil
}

// Put inserts or replaces the account's record.
func (s *Store) Put(r Record) error {
	now := time.Now().Unix()
	if r.StartedAt == 0 {
		r.StartedAt = now
	}
	_, err := s.db.Exec(`INSERT INTO api_migrations(account_id,state,route,snapshot_path,correlated,uncorrelated,error,started_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(account_id) DO UPDATE SET state=excluded.state,route=excluded.route,
		snapshot_path=excluded.snapshot_path,correlated=excluded.correlated,uncorrelated=excluded.uncorrelated,
		error=excluded.error,started_at=excluded.started_at,updated_at=excluded.updated_at`,
		r.AccountID, r.State, r.Route, r.SnapshotPath, r.Correlated, r.Uncorrelated, r.Error, r.StartedAt, now)
	if err != nil {
		return fmt.Errorf("failed to store api migration: %w", err)
	}
	return nil
}

// Count returns the account's row counts. IMAP rows are the messages without
// an API link; they are counted whether or not their folders are hidden.
func (s *Store) Count(accountID string) (Counts, error) {
	var c Counts
	queries := []struct {
		dst *int
		sql string
	}{
		{&c.Folders, `SELECT COUNT(*) FROM folders WHERE account_id=?`},
		{&c.IMAPMessages, `SELECT COUNT(*) FROM messages m WHERE m.account_id=?
			AND NOT EXISTS (SELECT 1 FROM api_message_rows l WHERE l.message_id=m.id)`},
		{&c.APIMessages, `SELECT COUNT(*) FROM api_message_rows WHERE account_id=?`},
		{&c.Drafts, `SELECT COUNT(*) FROM drafts WHERE account_id=?`},
		{&c.QueuedSubmissions, `SELECT COUNT(*) FROM submission_attempts WHERE account_id=?
			AND (outcome='unknown' OR (outcome='not_submitted' AND stage='queued'))`},
	}
	for _, q := range queries {
		if err := s.db.QueryRow(q.sql, accountID).Scan(q.dst); err != nil {
			return Counts{}, fmt.Errorf("failed to count account rows: %w", err)
		}
	}
	return c, nil
}

// Snapshot writes a consistent copy of the whole database (VACUUM INTO) and a
// manifest into dir, both owner-only. route is the account's route row as JSON.
// No credential is copied anywhere the database does not already hold it.
func (s *Store) Snapshot(dir, accountID string, route json.RawMessage, now time.Time) (string, error) {
	if accountID == "" || strings.ContainsAny(accountID, `/\`) {
		return "", fmt.Errorf("invalid account ID for snapshot")
	}
	counts, err := s.Count(accountID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to secure snapshot directory: %w", err)
	}
	base := fmt.Sprintf("%s-%s", accountID, now.UTC().Format("20060102T150405Z"))
	dbPath := filepath.Join(dir, base+".db")
	// VACUUM INTO refuses an existing file; a leftover from an interrupted
	// attempt is incomplete, so it is replaced.
	if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to clear stale snapshot: %w", err)
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, dbPath); err != nil {
		_ = os.Remove(dbPath)
		return "", fmt.Errorf("failed to write database snapshot: %w", err)
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = os.Remove(dbPath)
		return "", fmt.Errorf("failed to secure snapshot: %w", err)
	}
	data, err := json.MarshalIndent(Manifest{Version: manifestVersion, AccountID: accountID, Route: route,
		Counts: counts, CreatedAt: now.Unix(), Database: filepath.Base(dbPath)}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, base+".json"), data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write snapshot manifest: %w", err)
	}
	return dbPath, nil
}

// CorrelateResult counts how API rows were matched.
type CorrelateResult struct {
	Correlated   int `json:"correlated"`
	Uncorrelated int `json:"uncorrelated"`
	// Ambiguous API rows matched more than one IMAP row, or shared their one
	// match with another API row, and were left alone.
	Ambiguous int `json:"ambiguous"`
	// NoMessageID API rows carry no Message-ID, so no identity exists to match
	// on; they are also counted in Uncorrelated.
	NoMessageID int `json:"noMessageId"`
}

type localRow struct {
	id         string
	folderType string
}

// Correlate carries local state from the account's IMAP rows onto the API rows
// that represent the same messages. A pair shares a Message-ID and a date
// (to the second); when several IMAP rows match, the ones in a folder of the
// same type win, and a remaining tie leaves the API row uncorrelated rather
// than guessing. An IMAP row claimed by several API rows (duplicate API
// messages) is likewise ambiguous: none of them receives its state. IMAP rows are never deleted or changed, so rollback keeps
// them. It is idempotent: it only fills values the API row lacks.
func (s *Store) Correlate(accountID string) (CorrelateResult, error) {
	var res CorrelateResult
	imap := map[string][]localRow{}
	rows, err := s.db.Query(`SELECT m.id, m.message_id, m.date, f.folder_type
		FROM messages m JOIN folders f ON f.id=m.folder_id
		WHERE m.account_id=? AND m.message_id IS NOT NULL AND m.message_id<>''
		AND f.folder_type LIKE 'hidden:imap:%'
		AND NOT EXISTS (SELECT 1 FROM api_message_rows l WHERE l.message_id=m.id)`, accountID)
	if err != nil {
		return res, fmt.Errorf("failed to read imap rows: %w", err)
	}
	for rows.Next() {
		var id, mid string
		var date sql.NullTime
		var ftype string
		if err := rows.Scan(&id, &mid, &date, &ftype); err != nil {
			_ = rows.Close()
			return res, err
		}
		k := key(mid, date.Time)
		imap[k] = append(imap[k], localRow{id, strings.TrimPrefix(ftype, "hidden:imap:")})
	}
	if err := rows.Close(); err != nil {
		return res, err
	}

	type apiRow struct {
		id, mid, ftype string
		date           time.Time
	}
	var api []apiRow
	rows, err = s.db.Query(`SELECT m.id, m.message_id, m.date, f.folder_type
		FROM messages m JOIN folders f ON f.id=m.folder_id JOIN api_message_rows l ON l.message_id=m.id
		WHERE m.account_id=? AND f.folder_type NOT LIKE 'hidden:%'`, accountID)
	if err != nil {
		return res, fmt.Errorf("failed to read api rows: %w", err)
	}
	for rows.Next() {
		var r apiRow
		var mid sql.NullString
		var date sql.NullTime
		if err := rows.Scan(&r.id, &mid, &date, &r.ftype); err != nil {
			_ = rows.Close()
			return res, err
		}
		r.mid, r.date = mid.String, date.Time
		api = append(api, r)
	}
	if err := rows.Close(); err != nil {
		return res, err
	}

	// The attachments.content column is added lazily by the message store.
	var hasContent bool
	if err := s.db.QueryRow(`SELECT COUNT(*)>0 FROM pragma_table_info('attachments') WHERE name='content'`).Scan(&hasContent); err != nil {
		return res, fmt.Errorf("failed to inspect attachments: %w", err)
	}
	// BEGIN IMMEDIATE takes the write lock up front: a deferred transaction
	// fails at once with SQLITE_BUSY(_SNAPSHOT) in WAL mode when another
	// connection (such as the API action drain) commits before its first write.
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return res, fmt.Errorf("failed to begin correlation: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return res, fmt.Errorf("failed to begin correlation: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()
	// First pass: pick each API row's single IMAP match, if any, and count how
	// many API rows claim each IMAP row.
	type pair struct{ api, imap string }
	var matches []pair
	claims := map[string]int{}
	for _, r := range api {
		if r.mid == "" {
			res.Uncorrelated++
			res.NoMessageID++
			continue
		}
		cands := imap[key(r.mid, r.date)]
		var same []localRow
		for _, c := range cands {
			if c.folderType == r.ftype {
				same = append(same, c)
			}
		}
		if len(same) == 0 {
			same = cands
		}
		switch {
		case len(same) == 0:
			res.Uncorrelated++
		case len(same) > 1:
			res.Uncorrelated++
			res.Ambiguous++
		default:
			matches = append(matches, pair{r.id, same[0].id})
			claims[same[0].id]++
		}
	}
	for _, m := range matches {
		if claims[m.imap] > 1 {
			res.Uncorrelated++
			res.Ambiguous++
			continue
		}
		if err := carryState(ctx, conn, m.imap, m.api, hasContent); err != nil {
			return res, err
		}
		res.Correlated++
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return res, fmt.Errorf("failed to commit correlation: %w", err)
	}
	committed = true
	return res, nil
}

// key identifies a message across routes: Message-ID without angle brackets
// (APIs and IMAP differ on them) plus the date to the second.
func key(messageID string, date time.Time) string {
	return fmt.Sprintf("%d|%s", date.Unix(), strings.Trim(strings.TrimSpace(messageID), "<>"))
}

// carryState copies what only the local database knows from an IMAP row onto
// its API counterpart. Provider-owned fields (read/starred/draft/labels,
// envelope) stay as the API sync set them.
func carryState(ctx context.Context, conn *sql.Conn, from, to string, hasContent bool) error {
	// Cached body and its crypto parts, only where the API row has none.
	if _, err := conn.ExecContext(ctx, `UPDATE messages SET
			body_text=(SELECT s.body_text FROM messages s WHERE s.id=?1),
			body_html=(SELECT s.body_html FROM messages s WHERE s.id=?1),
			body_fetched=1,
			smime_raw_body=(SELECT s.smime_raw_body FROM messages s WHERE s.id=?1),
			smime_encrypted=(SELECT s.smime_encrypted FROM messages s WHERE s.id=?1),
			pgp_raw_body=(SELECT s.pgp_raw_body FROM messages s WHERE s.id=?1),
			pgp_encrypted=(SELECT s.pgp_encrypted FROM messages s WHERE s.id=?1)
		WHERE id=?2 AND COALESCE(body_fetched,0)=0
		AND (SELECT COALESCE(s.body_fetched,0) FROM messages s WHERE s.id=?1)=1`, from, to); err != nil {
		return fmt.Errorf("failed to carry cached body: %w", err)
	}
	// Verification results and per-message local marks.
	if _, err := conn.ExecContext(ctx, `UPDATE messages SET
			smime_status=COALESCE(NULLIF(smime_status,''),(SELECT s.smime_status FROM messages s WHERE s.id=?1)),
			smime_signer_email=COALESCE(NULLIF(smime_signer_email,''),(SELECT s.smime_signer_email FROM messages s WHERE s.id=?1)),
			smime_signer_subject=COALESCE(NULLIF(smime_signer_subject,''),(SELECT s.smime_signer_subject FROM messages s WHERE s.id=?1)),
			pgp_status=COALESCE(NULLIF(pgp_status,''),(SELECT s.pgp_status FROM messages s WHERE s.id=?1)),
			pgp_signer_email=COALESCE(NULLIF(pgp_signer_email,''),(SELECT s.pgp_signer_email FROM messages s WHERE s.id=?1)),
			pgp_signer_key_id=COALESCE(NULLIF(pgp_signer_key_id,''),(SELECT s.pgp_signer_key_id FROM messages s WHERE s.id=?1)),
			read_receipt_to=COALESCE(NULLIF(read_receipt_to,''),(SELECT s.read_receipt_to FROM messages s WHERE s.id=?1)),
			read_receipt_handled=MAX(read_receipt_handled,(SELECT s.read_receipt_handled FROM messages s WHERE s.id=?1)),
			delivered_to=COALESCE(NULLIF(delivered_to,''),(SELECT s.delivered_to FROM messages s WHERE s.id=?1)),
			is_bulk=COALESCE(is_bulk,(SELECT s.is_bulk FROM messages s WHERE s.id=?1)),
			is_answered=MAX(COALESCE(is_answered,0),COALESCE((SELECT s.is_answered FROM messages s WHERE s.id=?1),0)),
			is_forwarded=MAX(COALESCE(is_forwarded,0),COALESCE((SELECT s.is_forwarded FROM messages s WHERE s.id=?1),0))
		WHERE id=?2`, from, to); err != nil {
		return fmt.Errorf("failed to carry local marks: %w", err)
	}
	// Attachments: copy the IMAP row's when the API row has none yet.
	cols, sel := "", ""
	if hasContent {
		cols, sel = ", content", ", content"
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO attachments (id, message_id, filename, content_type, size, content_id, is_inline, embedded, local_path`+cols+`)
		SELECT lower(hex(randomblob(16))), ?2, filename, content_type, size, content_id, is_inline, embedded, local_path`+sel+`
		FROM attachments WHERE message_id=?1
		AND NOT EXISTS (SELECT 1 FROM attachments WHERE message_id=?2)`, from, to); err != nil {
		return fmt.Errorf("failed to carry attachments: %w", err)
	}
	return nil
}
