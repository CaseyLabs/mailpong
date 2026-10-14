package message

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Operations of the API action queue.
const (
	APIActionRead = "read" // set is_read to Value
	APIActionStar = "star" // set is_starred to Value
	APIActionMove = "move" // move to a container (trash and archive included)
)

// Statuses of a queued action. Applied and rejected are terminal.
const (
	APIActionPending  = "pending"
	APIActionInFlight = "in-flight"
	// APIActionUnknown: the provider may or may not have applied it. It is
	// verified before any re-send, or surfaced for review; never replayed
	// blindly and never reverted.
	APIActionUnknown  = "unknown"
	APIActionApplied  = "applied"
	APIActionRejected = "rejected"
)

// APIActionPayload is the operation's argument and the state needed to undo
// the local change if the provider rejects it.
type APIActionPayload struct {
	// Value is the flag value to set (read, star); Prior the value before the
	// first queued change, restored on rejection.
	Value bool `json:"value,omitempty"`
	Prior bool `json:"prior,omitempty"`
	// Move: local folders, the remote container added (Dest) and removed
	// (Source, Gmail only), whether it is a move to the trash, and the ref's
	// containers before the move.
	FromFolderID    string   `json:"fromFolderId,omitempty"`
	ToFolderID      string   `json:"toFolderId,omitempty"`
	Dest            string   `json:"dest,omitempty"`
	Source          string   `json:"source,omitempty"`
	Trash           bool     `json:"trash,omitempty"`
	FromTrash       bool     `json:"fromTrash,omitempty"`
	PriorContainers []string `json:"priorContainers,omitempty"`
	// Merged records that the destination already held a row for the message,
	// so the source row was dropped; a revert must not move that row away.
	Merged bool `json:"merged,omitempty"`
}

// APIAction is one queued user action on one remote message.
type APIAction struct {
	ID            int64
	AccountID     string
	Provider      string
	Op            string
	MessageIDs    []string // local rows of the remote message at enqueue time
	RemoteID      string
	Payload       APIActionPayload
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	Error         string
	CreatedAt     time.Time
}

// ErrAPIActionNotAPI reports a message that has no API identity.
var ErrAPIActionNotAPI = errors.New("message is not an API message")

type apiRowInfo struct {
	messageID, folderID, remoteID string
}

func apiRowOf(tx *sql.Tx, accountID, provider, messageID string) (apiRowInfo, error) {
	r := apiRowInfo{messageID: messageID}
	err := tx.QueryRow(`SELECT m.folder_id, l.remote_id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.message_id=? AND l.account_id=? AND l.provider=?`, messageID, accountID, provider).Scan(&r.folderID, &r.remoteID)
	if errors.Is(err, sql.ErrNoRows) {
		return r, fmt.Errorf("%w: %s", ErrAPIActionNotAPI, messageID)
	}
	if err != nil {
		return r, fmt.Errorf("failed to read api row: %w", err)
	}
	return r, nil
}

func refRowIDs(tx *sql.Tx, accountID, provider, remoteID string) ([]string, error) {
	rows, err := tx.Query(`SELECT message_id FROM api_message_rows WHERE account_id=? AND provider=? AND remote_id=?
		ORDER BY message_id`, accountID, provider, remoteID)
	if err != nil {
		return nil, fmt.Errorf("failed to read api rows: %w", err)
	}
	defer func() { _ = rows.Close() }()
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

func insertAPIAction(tx *sql.Tx, a *APIAction) error {
	ids, err := json.Marshal(nonNil(a.MessageIDs))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(a.Payload)
	if err != nil {
		return err
	}
	if a.Status == "" {
		a.Status = APIActionPending
	}
	res, err := tx.Exec(`INSERT INTO api_pending_actions(account_id,provider,op,message_ids,remote_id,payload,status,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, a.AccountID, a.Provider, a.Op, string(ids), a.RemoteID, string(payload), a.Status, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to queue api action: %w", err)
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// EnqueueAPIFlag sets a read or starred flag on the local rows of the messages
// and queues the remote change, in one transaction: a crash leaves either both
// or neither. Every folder row of the same remote message follows, since the
// flag belongs to the message. An opposite change to the same flag that is
// still pending is coalesced into it (and dropped when it restores the
// original value).
func (s *Store) EnqueueAPIFlag(accountID, provider, op string, messageIDs []string, value bool) error {
	column := ""
	switch op {
	case APIActionRead:
		column = "is_read"
	case APIActionStar:
		column = "is_starred"
	default:
		return fmt.Errorf("unsupported api flag operation %q", op)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api flag action: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	done := map[string]bool{}
	for _, id := range messageIDs {
		row, err := apiRowOf(tx, accountID, provider, id)
		if err != nil {
			return err
		}
		if done[row.remoteID] {
			continue
		}
		done[row.remoteID] = true
		siblings, err := refRowIDs(tx, accountID, provider, row.remoteID)
		if err != nil {
			return err
		}
		var current bool
		if err := tx.QueryRow(`SELECT `+column+` FROM messages WHERE id=?`, row.messageID).Scan(&current); err != nil {
			return fmt.Errorf("failed to read api flag: %w", err)
		}
		for _, sid := range siblings {
			if _, err := tx.Exec(`UPDATE messages SET `+column+`=? WHERE id=?`, value, sid); err != nil {
				return fmt.Errorf("failed to update api flag: %w", err)
			}
		}
		if err := queueFlag(tx, accountID, provider, op, row.remoteID, siblings, current, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// queueFlag coalesces with the latest pending action of the same op on the
// remote message, or inserts a new one.
func queueFlag(tx *sql.Tx, accountID, provider, op, remoteID string, siblings []string, current, value bool) error {
	var id int64
	var raw string
	err := tx.QueryRow(`SELECT id, payload FROM api_pending_actions WHERE account_id=? AND provider=? AND remote_id=?
		AND op=? AND status=? ORDER BY id DESC LIMIT 1`, accountID, provider, remoteID, op, APIActionPending).Scan(&id, &raw)
	if err == nil {
		// Only the newest pending row for the remote message may merge: a
		// later action of another kind must keep its order.
		var newest int64
		if err := tx.QueryRow(`SELECT MAX(id) FROM api_pending_actions WHERE account_id=? AND provider=? AND remote_id=?
			AND status IN (?,?,?)`, accountID, provider, remoteID, APIActionPending, APIActionInFlight, APIActionUnknown).Scan(&newest); err != nil {
			return fmt.Errorf("failed to read api queue: %w", err)
		}
		if newest == id {
			var p APIActionPayload
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				return fmt.Errorf("failed to decode queued api action: %w", err)
			}
			if p.Prior == value {
				_, err := tx.Exec(`DELETE FROM api_pending_actions WHERE id=?`, id)
				if err != nil {
					return fmt.Errorf("failed to drop coalesced api action: %w", err)
				}
				return nil
			}
			p.Value = value
			out, _ := json.Marshal(p)
			if _, err := tx.Exec(`UPDATE api_pending_actions SET payload=? WHERE id=?`, string(out), id); err != nil {
				return fmt.Errorf("failed to coalesce api action: %w", err)
			}
			return nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to read api queue: %w", err)
	}
	if current == value {
		return nil // already the local state and nothing in flight differs
	}
	return insertAPIAction(tx, &APIAction{AccountID: accountID, Provider: provider, Op: op, MessageIDs: siblings,
		RemoteID: remoteID, Payload: APIActionPayload{Value: value, Prior: current}})
}

// APIMoveRequest is a local-first move of one API message between folders.
type APIMoveRequest struct {
	AccountID, Provider string
	MessageID           string
	ToFolderID          string
	// Dest is the remote container the message joins (Gmail label ID, Graph
	// folder ID or well-known name). Source is the Gmail label it leaves ("" when
	// none). Containers is the ref's container set after the move.
	Dest, Source string
	Containers   []string
	Trash        bool
	FromTrash    bool
}

// EnqueueAPIMove moves the local row and queues the remote move in one
// transaction. If the message already has a row in the destination folder
// (a Gmail label it already carries), the source row is dropped instead.
func (s *Store) EnqueueAPIMove(req APIMoveRequest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api move action: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row, err := apiRowOf(tx, req.AccountID, req.Provider, req.MessageID)
	if err != nil {
		return err
	}
	if row.folderID == req.ToFolderID {
		return nil
	}
	var rawContainers string
	if err := tx.QueryRow(`SELECT containers FROM api_message_refs WHERE account_id=? AND provider=? AND remote_id=?`,
		req.AccountID, req.Provider, row.remoteID).Scan(&rawContainers); err != nil {
		return fmt.Errorf("failed to read api containers: %w", err)
	}
	var prior []string
	_ = json.Unmarshal([]byte(rawContainers), &prior)
	siblings, err := refRowIDs(tx, req.AccountID, req.Provider, row.remoteID)
	if err != nil {
		return err
	}

	var clash string
	merged := false
	err = tx.QueryRow(`SELECT m.id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.account_id=? AND l.provider=? AND l.remote_id=? AND m.folder_id=?`,
		req.AccountID, req.Provider, row.remoteID, req.ToFolderID).Scan(&clash)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		uid, err := allocateAPIUID(tx, req.ToFolderID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE messages SET folder_id=?, uid=? WHERE id=?`, req.ToFolderID, uid, req.MessageID); err != nil {
			return fmt.Errorf("failed to move api message row: %w", err)
		}
	case err != nil:
		return fmt.Errorf("failed to check api move destination: %w", err)
	default:
		if _, err := tx.Exec(`DELETE FROM messages WHERE id=?`, req.MessageID); err != nil {
			return fmt.Errorf("failed to drop duplicate api row: %w", err)
		}
		merged = true
	}
	containers, err := json.Marshal(nonNil(req.Containers))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE api_message_refs SET containers=? WHERE account_id=? AND provider=? AND remote_id=?`,
		string(containers), req.AccountID, req.Provider, row.remoteID); err != nil {
		return fmt.Errorf("failed to update api containers: %w", err)
	}
	if err := insertAPIAction(tx, &APIAction{AccountID: req.AccountID, Provider: req.Provider, Op: APIActionMove,
		MessageIDs: siblings, RemoteID: row.remoteID, Payload: APIActionPayload{
			FromFolderID: row.folderID, ToFolderID: req.ToFolderID, Dest: req.Dest, Source: req.Source,
			Trash: req.Trash, FromTrash: req.FromTrash, PriorContainers: prior, Merged: merged}}); err != nil {
		return err
	}
	return tx.Commit()
}

// APIRefContainers returns the stored remote containers of a ref.
func (s *Store) APIRefContainers(accountID, provider, remoteID string) ([]string, error) {
	var raw string
	err := s.db.QueryRow(`SELECT containers FROM api_message_refs WHERE account_id=? AND provider=? AND remote_id=?`,
		accountID, provider, remoteID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read api containers: %w", err)
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out, nil
}

// APIMessageRemote returns a local API row's remote ID and folder.
func (s *Store) APIMessageRemote(accountID, provider, messageID string) (remoteID, folderID string, err error) {
	err = s.db.QueryRow(`SELECT l.remote_id, m.folder_id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.message_id=? AND l.account_id=? AND l.provider=?`, messageID, accountID, provider).Scan(&remoteID, &folderID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("%w: %s", ErrAPIActionNotAPI, messageID)
	}
	return remoteID, folderID, err
}

const apiActionColumns = `id,account_id,provider,op,message_ids,remote_id,payload,status,attempts,next_attempt_at,error,created_at`

func scanAPIAction(sc interface{ Scan(...any) error }) (*APIAction, error) {
	var a APIAction
	var ids, payload string
	var next, created int64
	if err := sc.Scan(&a.ID, &a.AccountID, &a.Provider, &a.Op, &ids, &a.RemoteID, &payload, &a.Status,
		&a.Attempts, &next, &a.Error, &created); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(ids), &a.MessageIDs)
	if err := json.Unmarshal([]byte(payload), &a.Payload); err != nil {
		return nil, fmt.Errorf("failed to decode queued api action: %w", err)
	}
	a.NextAttemptAt, a.CreatedAt = time.Unix(next, 0), time.Unix(created, 0)
	return &a, nil
}

// NextAPIAction returns the oldest runnable action of the account: pending,
// due, and with no earlier open action on the same remote message. Rows in
// the unknown state are returned too when due, for verification. It returns
// nil when nothing is runnable now.
func (s *Store) NextAPIAction(accountID string, now time.Time) (*APIAction, error) {
	rows, err := s.db.Query(`SELECT `+apiActionColumns+` FROM api_pending_actions a
		WHERE a.account_id=? AND a.status IN (?,?) AND a.next_attempt_at<=?
		AND NOT EXISTS (SELECT 1 FROM api_pending_actions b WHERE b.account_id=a.account_id AND b.provider=a.provider
			AND b.remote_id=a.remote_id AND b.id<a.id AND b.status IN (?,?,?))
		ORDER BY a.id LIMIT 1`, accountID, APIActionPending, APIActionUnknown, now.Unix(),
		APIActionPending, APIActionInFlight, APIActionUnknown)
	if err != nil {
		return nil, fmt.Errorf("failed to read api queue: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return scanAPIAction(rows)
}

// SetAPIActionStatus records a state change. Applied and rejected also clear
// the retry schedule.
func (s *Store) SetAPIActionStatus(id int64, status, errText string) error {
	attempts := 0
	if status == APIActionInFlight {
		attempts = 1
	}
	if _, err := s.db.Exec(`UPDATE api_pending_actions SET status=?, error=?, attempts=attempts+?, next_attempt_at=0 WHERE id=?`,
		status, errText, attempts, id); err != nil {
		return fmt.Errorf("failed to update api action: %w", err)
	}
	return nil
}

// ScheduleAPIAction sets the status of an action and the time it is next due.
func (s *Store) ScheduleAPIAction(id int64, status string, next time.Time, errText string) error {
	if _, err := s.db.Exec(`UPDATE api_pending_actions SET status=?, error=?, next_attempt_at=? WHERE id=?`,
		status, errText, next.Unix(), id); err != nil {
		return fmt.Errorf("failed to schedule api action: %w", err)
	}
	return nil
}

// RecoverAPIActions marks every in-flight action of the account unknown, as
// after a restart: whether the request reached the provider cannot be known.
func (s *Store) RecoverAPIActions(accountID string) (int64, error) {
	res, err := s.db.Exec(`UPDATE api_pending_actions SET status=?, error=? WHERE account_id=? AND status=?`,
		APIActionUnknown, "interrupted; remote state not yet verified", accountID, APIActionInFlight)
	if err != nil {
		return 0, fmt.Errorf("failed to recover api actions: %w", err)
	}
	return res.RowsAffected()
}

// ListAPIActions returns the account's actions in the given statuses, oldest first.
func (s *Store) ListAPIActions(accountID string, statuses ...string) ([]*APIAction, error) {
	query := `SELECT ` + apiActionColumns + ` FROM api_pending_actions WHERE account_id=?`
	args := []any{accountID}
	if len(statuses) > 0 {
		query += ` AND status IN (?` + strings.Repeat(`,?`, len(statuses)-1) + `)`
		for _, st := range statuses {
			args = append(args, st)
		}
	}
	rows, err := s.db.Query(query+` ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list api actions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*APIAction
	for rows.Next() {
		a, err := scanAPIAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PruneAPIActions deletes terminal actions created before cutoff.
func (s *Store) PruneAPIActions(cutoff time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM api_pending_actions WHERE status IN (?,?) AND created_at<?`,
		APIActionApplied, APIActionRejected, cutoff.Unix()); err != nil {
		return fmt.Errorf("failed to prune api actions: %w", err)
	}
	return nil
}

// OpenAPIActionAccounts lists accounts that have actions awaiting work.
func (s *Store) OpenAPIActionAccounts() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT account_id FROM api_pending_actions WHERE status IN (?,?,?)`,
		APIActionPending, APIActionInFlight, APIActionUnknown)
	if err != nil {
		return nil, fmt.Errorf("failed to list api queue accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// hasOpenAPIAction reports whether the remote message has an unresolved action.
func hasOpenAPIAction(tx *sql.Tx, accountID, provider, remoteID string) (bool, error) {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM api_pending_actions WHERE account_id=? AND provider=? AND remote_id=?
		AND status IN (?,?,?)`, accountID, provider, remoteID, APIActionPending, APIActionInFlight, APIActionUnknown).Scan(&n); err != nil {
		return false, fmt.Errorf("failed to check api queue: %w", err)
	}
	return n > 0, nil
}

// RevertAPIFlag restores a flag the provider rejected, on every row of the
// remote message, to the value it had before the action.
func (s *Store) RevertAPIFlag(a *APIAction) error {
	column := "is_read"
	if a.Op == APIActionStar {
		column = "is_starred"
	}
	_, err := s.db.Exec(`UPDATE messages SET `+column+`=? WHERE id IN (SELECT message_id FROM api_message_rows
		WHERE account_id=? AND provider=? AND remote_id=?)`, a.Payload.Prior, a.AccountID, a.Provider, a.RemoteID)
	if err != nil {
		return fmt.Errorf("failed to revert api flag: %w", err)
	}
	return nil
}

// RevertAPIMove puts a rejected move back: the row returns to its source
// folder (a fresh surrogate UID) and the ref to its previous containers. A
// row the user has since moved again is left where it is, as is a merged
// destination row that predates the move (the next sync restores the source).
func (s *Store) RevertAPIMove(a *APIAction) error {
	p := a.Payload
	containers, err := json.Marshal(nonNil(p.PriorContainers))
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api move revert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var rowID string
	err = tx.QueryRow(`SELECT m.id FROM api_message_rows l JOIN messages m ON m.id=l.message_id
		WHERE l.account_id=? AND l.provider=? AND l.remote_id=? AND m.folder_id=?`,
		a.AccountID, a.Provider, a.RemoteID, p.ToFolderID).Scan(&rowID)
	if err == nil && p.Merged {
		err = sql.ErrNoRows
	}
	if err == nil {
		var occupied int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM api_message_rows l JOIN messages m ON m.id=l.message_id
			WHERE l.account_id=? AND l.provider=? AND l.remote_id=? AND m.folder_id=?`,
			a.AccountID, a.Provider, a.RemoteID, p.FromFolderID).Scan(&occupied); err != nil {
			return fmt.Errorf("failed to check api move source: %w", err)
		}
		if occupied == 0 {
			uid, err := allocateAPIUID(tx, p.FromFolderID)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE messages SET folder_id=?, uid=? WHERE id=?`, p.FromFolderID, uid, rowID); err != nil {
				return fmt.Errorf("failed to revert api message row: %w", err)
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to find api move row: %w", err)
	}
	if _, err := tx.Exec(`UPDATE api_message_refs SET containers=? WHERE account_id=? AND provider=? AND remote_id=?`,
		string(containers), a.AccountID, a.Provider, a.RemoteID); err != nil {
		return fmt.Errorf("failed to revert api containers: %w", err)
	}
	return tx.Commit()
}

// RemoveAPIMessage deletes a remote message's local rows and identity, for a
// message the provider reports gone.
func (s *Store) RemoveAPIMessage(accountID, provider, remoteID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin api removal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteAPIRef(tx, accountID, provider, remoteID); err != nil {
		return err
	}
	return tx.Commit()
}
