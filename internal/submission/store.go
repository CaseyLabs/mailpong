// Package submission journals SMTP attempts. It deliberately has no replay worker.
package submission

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/google/uuid"
)

type Attempt struct {
	ID          string `json:"id"`
	MessageID   string `json:"messageId"`
	AccountID   string `json:"accountId"`
	DraftID     string `json:"draftId"`
	Fingerprint string `json:"-"`
	Outcome     string `json:"outcome"`
	Stage       string `json:"stage"`
	Recipients  string `json:"recipients"`
	Error       string `json:"error"`
	Warnings    string `json:"warnings"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type Store struct{ db *database.DB }

func NewStore(db *database.DB) *Store { return &Store{db: db} }

func Fingerprint(msg smtp.ComposeMessage) string {
	msg.MessageID, msg.ReviewAttemptID, msg.SubmissionDraftID, msg.SenderReviewToken = "", "", "", ""
	b, _ := json.Marshal(msg)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// write uses FULL synchronous on a dedicated connection: the pre-DATA unknown
// checkpoint must survive a process crash and a power loss, even in WAL mode.
func (s *Store) write(query string, args ...any) error {
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(context.Background(), "PRAGMA synchronous=FULL"); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "PRAGMA synchronous=NORMAL") }()
	_, err = conn.ExecContext(context.Background(), query, args...)
	return err
}

func (s *Store) Update(a *Attempt) error {
	a.UpdatedAt = time.Now().UnixMilli()
	return s.write(`UPDATE submission_attempts SET outcome=?,stage=?,recipients=?,error=?,warnings=?,updated_at=? WHERE id=?`, a.Outcome, a.Stage, a.Recipients, a.Error, a.Warnings, a.UpdatedAt, a.ID)
}

const columns = `id,message_id,account_id,draft_id,fingerprint,outcome,stage,recipients,error,warnings,created_at,updated_at`

func scan(row interface{ Scan(...any) error }) (*Attempt, error) {
	a := &Attempt{}
	err := row.Scan(&a.ID, &a.MessageID, &a.AccountID, &a.DraftID, &a.Fingerprint, &a.Outcome, &a.Stage, &a.Recipients, &a.Error, &a.Warnings, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// Review includes all unresolved/accepted attempts for the same content OR
// draft. Editing a draft cannot silently erase an ambiguous previous send.
func (s *Store) Review(accountID, draftID string, msg smtp.ComposeMessage) (*Attempt, error) {
	return scan(s.db.QueryRow(`SELECT `+columns+` FROM submission_attempts WHERE ((account_id=? AND fingerprint=?) OR (draft_id<>'' AND draft_id=?)) AND outcome IN ('unknown','accepted') ORDER BY created_at DESC,rowid DESC LIMIT 1`, accountID, Fingerprint(msg), draftID))
}

func (s *Store) List(accountID string) ([]*Attempt, error) {
	rows, err := s.db.Query(`SELECT `+columns+` FROM submission_attempts WHERE account_id=? ORDER BY created_at DESC,rowid DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Attempt{}
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func ReviewError(a *Attempt) error {
	status := "the server may already have accepted this message"
	if a.Outcome == "accepted" {
		status = "the server accepted this message"
	}
	retained := "Draft retained."
	if a.DraftID == "" {
		retained = "The original message is retained."
	}
	return fmt.Errorf("Submission %s (%s): %s. Check Sent and confirm with recipients before a new submission; sending again can create duplicates. %s Provider acceptance does not prove delivery.", a.ID, a.Outcome, status, retained)
}

func (s *Store) Get(id string) (*Attempt, error) {
	return scan(s.db.QueryRow(`SELECT `+columns+` FROM submission_attempts WHERE id=?`, id))
}

// RecoverQueued keeps content in existing drafts. Interrupted pre-DATA work is
// definitely unsent; unknown checkpoints and accepted attempts are untouched.
// Only the main application's single-instance startup calls this method.
func (s *Store) RecoverQueued() error {
	return s.write(`UPDATE submission_attempts SET stage='recovered',updated_at=? WHERE outcome='not_submitted' AND stage='queued'`, time.Now().UnixMilli())
}

// Begin serializes review and creation against other windows/processes.
func (s *Store) Begin(accountID, draftID string, msg smtp.ComposeMessage) (*Attempt, error) {
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(context.Background(), "PRAGMA synchronous=FULL"); err != nil {
		return nil, err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "PRAGMA synchronous=NORMAL") }()
	// BEGIN IMMEDIATE avoids deferred-transaction read/write races in WAL mode.
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		return nil, err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()
	fingerprint := Fingerprint(msg)
	var active int
	err = conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM submission_attempts WHERE ((account_id=? AND fingerprint=?) OR (draft_id<>'' AND draft_id=?)) AND outcome='not_submitted' AND stage='queued'`, accountID, fingerprint, draftID).Scan(&active)
	if err != nil {
		return nil, err
	}
	if active != 0 {
		return nil, fmt.Errorf("this message already has a pending submission; wait for its outcome or undo it")
	}
	previous, err := scan(conn.QueryRowContext(context.Background(), `SELECT `+columns+` FROM submission_attempts WHERE ((account_id=? AND fingerprint=?) OR (draft_id<>'' AND draft_id=?)) AND outcome IN ('unknown','accepted') ORDER BY created_at DESC,rowid DESC LIMIT 1`, accountID, fingerprint, draftID))
	if err != nil {
		return nil, err
	}
	if previous != nil && previous.ID != msg.ReviewAttemptID {
		return nil, ReviewError(previous)
	}
	// Safe temporary rejection retries are user-invoked and rate limited. There
	// is no automatic retry, route fallback, or replay, even after restart.
	var updated int64
	var rejections int
	err = conn.QueryRowContext(context.Background(), `SELECT COALESCE(MAX(updated_at),0), COUNT(*) FROM submission_attempts WHERE account_id=? AND fingerprint=? AND outcome='rejected_temporary'`, accountID, fingerprint).Scan(&updated, &rejections)
	if err != nil {
		return nil, err
	}
	delay := min(5*time.Second*time.Duration(1<<min(max(rejections-1, 0), 6)), 5*time.Minute)
	if updated > time.Now().Add(-delay).UnixMilli() {
		return nil, fmt.Errorf("server temporarily rejected this message; wait at least %d seconds before trying again", int(delay.Seconds()))
	}
	now := time.Now().UnixMilli()
	a := &Attempt{ID: uuid.NewString(), MessageID: smtp.NewMessageID(msg.From.Address), AccountID: accountID, DraftID: draftID, Fingerprint: fingerprint, Outcome: "not_submitted", Stage: "queued", Recipients: "[]", CreatedAt: now, UpdatedAt: now}
	_, err = conn.ExecContext(context.Background(), `INSERT INTO submission_attempts (id,message_id,account_id,draft_id,fingerprint,outcome,stage,recipients,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, a.ID, a.MessageID, a.AccountID, a.DraftID, a.Fingerprint, a.Outcome, a.Stage, a.Recipients, now, now)
	if err != nil {
		return nil, err
	}
	_, err = conn.ExecContext(context.Background(), "COMMIT")
	return a, err
}
