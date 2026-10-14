package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Destination is one intended mailbox copy, not a global exactly-one promise.
type Destination struct {
	MailboxRoute string `json:"mailboxRoute"`
	AccountID    string `json:"accountId"`
	FolderPath   string `json:"folderPath"`
	Mode         string `json:"mode"` // provider, append, unresolved
}
type SentPolicy struct {
	Route        string        `json:"route"`
	Destinations []Destination `json:"destinations"`
}
type Copy struct {
	MailboxRoute string `json:"mailboxRoute"`
	AttemptID    string `json:"attemptId"`
	MessageID    string `json:"messageId"`
	AccountID    string `json:"accountId"`
	FolderPath   string `json:"folderPath"`
	Mode         string `json:"mode"`
	State        string `json:"state"`
	Evidence     string `json:"evidence"`
}

func (s *Store) SentPolicy(accountID, route string) (*SentPolicy, error) {
	p := &SentPolicy{Route: route, Destinations: []Destination{}}
	var recorded, data string
	err := s.db.QueryRow(`SELECT route,destinations FROM sent_policies WHERE account_id=?`, accountID).Scan(&recorded, &data)
	if err != nil {
		if err == sql.ErrNoRows {
			return p, nil
		}
		return nil, err
	}
	// A policy for a different route is evidence only of the old configuration.
	if recorded != route {
		return p, nil
	}
	err = json.Unmarshal([]byte(data), &p.Destinations)
	return p, err
}
func (s *Store) SetSentPolicy(accountID string, p SentPolicy) error {
	seen := map[string]bool{}
	for _, d := range p.Destinations {
		if d.Mode != "provider" && d.Mode != "append" && d.Mode != "unresolved" {
			return fmt.Errorf("invalid Sent policy mode")
		}
		if d.AccountID == "" || d.FolderPath == "" {
			return fmt.Errorf("Sent destination requires mailbox and folder")
		}
		key := d.AccountID + "\x00" + d.FolderPath
		if seen[key] {
			return fmt.Errorf("duplicate Sent destination")
		}
		seen[key] = true
	}
	b, err := json.Marshal(p.Destinations)
	if err != nil {
		return err
	}
	return s.write(`INSERT INTO sent_policies(account_id,route,destinations) VALUES(?,?,?) ON CONFLICT(account_id) DO UPDATE SET route=excluded.route,destinations=excluded.destinations`, accountID, p.Route, string(b))
}

// PlanCopies snapshots the reviewed route policy before submission. Empty policy
// leaves an explicit unresolved copy; it never guesses from the receive host.
func (s *Store) PlanCopies(a *Attempt, p SentPolicy) error {
	dests := p.Destinations
	if len(dests) == 0 {
		dests = []Destination{{AccountID: a.AccountID, Mode: "unresolved"}}
	}
	for _, d := range dests {
		if err := s.write(`INSERT OR IGNORE INTO submission_copies(attempt_id,account_id,folder_path,mode,state,evidence,mailbox_route) VALUES(?,?,?,?,?,?,?)`, a.ID, d.AccountID, d.FolderPath, d.Mode, "pending", p.Route, d.MailboxRoute); err != nil {
			return err
		}
	}
	return nil
}

// ClaimAppend is persisted before network I/O. A crash or lost APPEND reply
// leaves unknown evidence, never permission to blindly repeat APPEND.
func (s *Store) ClaimAppend(c Copy) (bool, error) {
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(context.Background(), "PRAGMA synchronous=FULL"); err != nil {
		return false, err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "PRAGMA synchronous=NORMAL") }()
	result, err := conn.ExecContext(context.Background(), `UPDATE submission_copies SET state='unknown' WHERE attempt_id=? AND account_id=? AND folder_path=? AND mode='append' AND state='pending' AND EXISTS(SELECT 1 FROM submission_attempts WHERE id=? AND outcome='accepted')`, c.AttemptID, c.AccountID, c.FolderPath, c.AttemptID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (s *Store) SetCopyState(c Copy, state, evidence string) error {
	return s.write(`UPDATE submission_copies SET state=?,evidence=? WHERE attempt_id=? AND account_id=? AND folder_path=?`, state, evidence, c.AttemptID, c.AccountID, c.FolderPath)
}

// ReconcileCopies only observes already-synced messages, by stable Message-ID
// and exact destination. Missing local evidence may mean delayed discovery.
// It does not send, append, or infer delivery from a Sent copy. It covers every
// accepted attempt for the account; per-send callers use ReconcileAttempt.
func (s *Store) ReconcileCopies(accountID string) ([]Copy, error) {
	return s.reconcile(accountID, "")
}

// ReconcileAttempt is ReconcileCopies scoped to one attempt, so the cost of a
// send does not grow with the account's submission journal.
func (s *Store) ReconcileAttempt(accountID, attemptID string) ([]Copy, error) {
	return s.reconcile(accountID, attemptID)
}

// reconcile observes copies for the account's accepted attempts, or only for
// attemptID when it is non-empty.
func (s *Store) reconcile(accountID, attemptID string) ([]Copy, error) {
	if err := s.write(`UPDATE submission_copies SET state='observed', evidence='Message-ID found in intended destination' WHERE state<>'observed' AND (?='' OR attempt_id=?) AND EXISTS(SELECT 1 FROM submission_attempts a JOIN messages m ON TRIM(m.message_id,'<>')=TRIM(a.message_id,'<>') JOIN folders f ON f.id=m.folder_id WHERE a.id=submission_copies.attempt_id AND a.account_id=? AND a.outcome='accepted' AND m.account_id=submission_copies.account_id AND f.account_id=submission_copies.account_id AND f.path=submission_copies.folder_path)`, attemptID, attemptID, accountID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT c.attempt_id,a.message_id,c.account_id,c.folder_path,c.mode,c.state,c.evidence,c.mailbox_route FROM submission_copies c JOIN submission_attempts a ON a.id=c.attempt_id WHERE a.account_id=? AND a.outcome='accepted' AND (?='' OR c.attempt_id=?) ORDER BY a.created_at DESC`, accountID, attemptID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Copy{}
	for rows.Next() {
		var c Copy
		if err := rows.Scan(&c.AttemptID, &c.MessageID, &c.AccountID, &c.FolderPath, &c.Mode, &c.State, &c.Evidence, &c.MailboxRoute); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
