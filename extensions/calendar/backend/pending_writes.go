package backend

// Pending-write queue — Phase 2 Chunk 5.
//
// Persists user-initiated writes that failed at the transport layer (DNS,
// connection refused, timeout — anything errors.Is(err, ErrTransport))
// so they can be retried when connectivity returns. The user's local DB
// commit succeeds immediately, the queued write drains later.
//
// HTTP errors (412 conflict, 5xx) are NOT queued — they're real server
// responses that surface immediately to the user via event_crud.go's
// existing error path.
//
// Drain runs:
//   - inside Syncer.syncSourceInner, after each successful per-source
//     sync (catch-up for stuck rows).
//   - implicitly via the system:network-online + system:wake handlers
//     since those call SyncAllSources → SyncSource → syncSourceInner →
//     Drain.
//
// Order: rows for a given source replay in insertion order. Each Drain
// makes at most one attempt per row. A transport failure ends the pass for
// that source (connectivity is down, and later rows must not overtake it)
// without spending the row's retry budget; the next drain retries it.
// Only non-transport failures count toward pendingMaxAttempts, and
// ResetExhausted (run on system:network-online / system:wake) gives
// exhausted rows a fresh budget so they are never stranded permanently.
//
// Sequential offline writes to the same event (create then edit, or
// several edits) are stored separately and replayed in order. Each was
// enqueued against the same pre-offline transport state, so after a row
// succeeds, later rows still holding that state are advanced to the
// server's new ETag / ProviderEventID / Href. A remote change made while
// offline still conflicts on the first row, and on the later rows since
// they are not advanced.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	coreapi "github.com/caseylabs/mailpong/internal/core/api/v1"
)

// PendingOpKind enumerates queueable operations.
type PendingOpKind string

const (
	PendingOpCreate PendingOpKind = "create"
	PendingOpUpdate PendingOpKind = "update"
	PendingOpDelete PendingOpKind = "delete"
)

// pendingMaxAttempts is the per-row budget for non-transport failures.
// After this many, Drain skips the row until ResetExhausted restores it.
const pendingMaxAttempts = 3

// PendingOp is the input shape to Enqueue. The caller supplies the
// full event-shaped state needed to replay the push (or delete) without
// re-reading the events row at drain time — important because
// soft-committed deletes have no events row to read.
type PendingOp struct {
	SourceID   string
	CalendarID string
	EventID    string // local events.id; empty when no row (rare)
	Op         PendingOpKind
	Scope      EditScope // applies to update/delete; empty for create

	// CalendarURL holds the URL/ID the provider uses to address the
	// calendar (CalDAV path, Google calendarId, Microsoft Graph id).
	// Copied from Calendar.URL at enqueue time so Drain doesn't need a
	// fresh DB lookup.
	CalendarURL string

	// Event fields needed to construct the replay request. For deletes,
	// only ETag + Href + ProviderEventID are load-bearing; the rest are
	// stored for traceability.
	UID             string
	ETag            string
	Href            string
	ProviderEventID string
	Summary         string
	Description     string
	Location        string
	DTStartUnix     int64
	DTEndUnix       int64
	IsAllDay        bool
	TZName          string
	RRuleText       string
	ICSBlob         string

	// SendUpdates is the user's invitation-delivery choice for this write
	// (see Event.SendUpdates); replayed so queued writes notify attendees
	// the same way an immediate write would.
	SendUpdates string
}

// pendingRow mirrors a row in the pending_writes table for read-back.
type pendingRow struct {
	Seq             int64 // SQLite rowid; insertion order
	ID              string
	SourceID        string
	CalendarID      string
	EventID         string
	Op              string
	Scope           string
	Payload         pendingPayload
	Attempt         int
	LastAttemptUnix int64
	LastError       string
	CreatedUnix     int64
}

// pendingPayload is what we JSON-marshal into pending_writes.payload_json.
// It captures everything Drain needs to replay the push (or delete) so
// the events row doesn't have to exist at drain time.
type pendingPayload struct {
	CalendarURL     string `json:"calendarUrl"`
	UID             string `json:"uid"`
	ETag            string `json:"etag,omitempty"`
	Href            string `json:"href,omitempty"`
	ProviderEventID string `json:"providerEventId,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Description     string `json:"description,omitempty"`
	Location        string `json:"location,omitempty"`
	DTStartUnix     int64  `json:"dtstartUnix,omitempty"`
	DTEndUnix       int64  `json:"dtendUnix,omitempty"`
	IsAllDay        bool   `json:"isAllDay,omitempty"`
	TZName          string `json:"tzName,omitempty"`
	RRuleText       string `json:"rruleText,omitempty"`
	ICSBlob         string `json:"icsBlob,omitempty"`
	SendUpdates     string `json:"sendUpdates,omitempty"`
}

// PendingQueue owns the pending_writes table + the drain loop. Shared
// between API (Enqueue on transport failure) and Syncer (Drain after
// sync). Goroutine-safe — methods serialize via the underlying SQLite
// connection.
type PendingQueue struct {
	store   *Store
	secrets coreapi.Secrets
	auth    coreapi.Auth
	events  coreapi.EventBus
}

// NewPendingQueue constructs a queue. events may be nil (skips the
// conflict-event publish on 412); the other deps are required.
func NewPendingQueue(store *Store, secrets coreapi.Secrets, auth coreapi.Auth, events coreapi.EventBus) *PendingQueue {
	return &PendingQueue{
		store:   store,
		secrets: secrets,
		auth:    auth,
		events:  events,
	}
}

// Enqueue persists a new pending row. Returns the row ID.
func (q *PendingQueue) Enqueue(op PendingOp) (string, error) {
	if op.SourceID == "" || op.CalendarID == "" {
		return "", errors.New("pending queue: SourceID and CalendarID required")
	}
	payload, err := json.Marshal(pendingPayload{
		CalendarURL:     op.CalendarURL,
		UID:             op.UID,
		ETag:            op.ETag,
		Href:            op.Href,
		ProviderEventID: op.ProviderEventID,
		Summary:         op.Summary,
		Description:     op.Description,
		Location:        op.Location,
		DTStartUnix:     op.DTStartUnix,
		DTEndUnix:       op.DTEndUnix,
		IsAllDay:        op.IsAllDay,
		TZName:          op.TZName,
		RRuleText:       op.RRuleText,
		ICSBlob:         op.ICSBlob,
		SendUpdates:     op.SendUpdates,
	})
	if err != nil {
		return "", fmt.Errorf("marshal pending payload: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().Unix()
	_, err = q.store.DB().Exec(`
		INSERT INTO pending_writes (
			id, source_id, calendar_id, event_id, op, scope,
			payload_json, attempt, created_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		id, op.SourceID, op.CalendarID, nullIfEmpty(op.EventID),
		string(op.Op), nullIfEmpty(string(op.Scope)),
		string(payload), now,
	)
	if err != nil {
		return "", fmt.Errorf("insert pending_writes: %w", err)
	}
	// Notify the frontend so the composer's success path can surface a
	// "Saved — will sync when online" toast instead of a regular success.
	if q.events != nil {
		_ = q.events.Publish("calendar:write-queued", map[string]any{
			"sourceId": op.SourceID,
			"op":       string(op.Op),
		})
	}
	return id, nil
}

// Drain replays all pending rows for the given sourceID in
// created_unix ASC order. Stops on ctx cancellation. Per-row outcomes:
//   - success → row deleted, events row updated with new ETag +
//     ProviderEventID where applicable.
//   - ErrTransport → row stays with last_error populated, attempt
//     counter unchanged, and the pass ends for this source.
//   - ErrConflict (412) → row deleted, calendar:write-conflict event
//     published, user surfaces a "please re-edit" toast via frontend.
//   - other errors → row stays with last_error populated + attempt
//     bumped. Rows past pendingMaxAttempts are skipped until
//     ResetExhausted.
//
// Each row is attempted at most once per call. Returns the first hard
// error encountered (ctx canceled, store failure). Per-row conflict /
// hard failures don't bubble up — the pass continues to the next row so
// one stuck write doesn't block others.
func (q *PendingQueue) Drain(ctx context.Context, sourceID string) error {
	if sourceID == "" {
		return errors.New("pending queue: sourceID required")
	}
	var after *pendingRow
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := q.nextPending(sourceID, after)
		if err != nil {
			return err
		}
		if row == nil {
			return nil // nothing left for this pass
		}
		if errors.Is(q.processRow(ctx, *row), ErrTransport) {
			return nil // offline; retry on the next drain
		}
		after = row
	}
}

// ResetExhausted restores the retry budget of rows that used it up, so
// the next drain retries them. Called when connectivity returns.
func (q *PendingQueue) ResetExhausted() error {
	_, err := q.store.DB().Exec(
		`UPDATE pending_writes SET attempt = 0 WHERE attempt >= ?`, pendingMaxAttempts)
	return err
}

// StuckWrite is a queued write that used up its retry budget. Listed in
// the calendar UI so the user can retry or discard it.
type StuckWrite struct {
	ID              string `json:"id"`
	SourceID        string `json:"sourceId"`
	SourceName      string `json:"sourceName"`
	Op              string `json:"op"`
	Summary         string `json:"summary"`
	DTStartUnix     int64  `json:"dtstartUnix"`
	LastError       string `json:"lastError"`
	LastAttemptUnix int64  `json:"lastAttemptUnix"`
}

// ListStuck returns rows past pendingMaxAttempts, oldest first.
func (q *PendingQueue) ListStuck() ([]StuckWrite, error) {
	rows, err := q.store.DB().Query(`
		SELECT p.id, p.source_id, COALESCE(s.name, ''), p.op, p.payload_json,
		       COALESCE(p.last_error, ''), COALESCE(p.last_attempt_unix, 0)
		FROM pending_writes p
		LEFT JOIN calendar_sources s ON s.id = p.source_id
		WHERE p.attempt >= ?
		ORDER BY p.created_unix ASC, p.rowid ASC`, pendingMaxAttempts)
	if err != nil {
		return nil, fmt.Errorf("query stuck pending_writes: %w", err)
	}
	defer rows.Close()
	out := []StuckWrite{}
	for rows.Next() {
		var w StuckWrite
		var payloadJSON string
		if err := rows.Scan(&w.ID, &w.SourceID, &w.SourceName, &w.Op, &payloadJSON,
			&w.LastError, &w.LastAttemptUnix); err != nil {
			return nil, fmt.Errorf("scan stuck pending_writes: %w", err)
		}
		var p pendingPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err == nil {
			w.Summary, w.DTStartUnix = p.Summary, p.DTStartUnix
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Retry grants one row a single extra attempt so the next drain replays
// it; another failure puts it straight back in ListStuck. Returns the
// row's source ID.
func (q *PendingQueue) Retry(id string) (string, error) {
	sourceID, err := q.sourceOf(id)
	if err != nil {
		return "", err
	}
	if _, err := q.store.DB().Exec(`UPDATE pending_writes SET attempt = ? WHERE id = ?`, pendingMaxAttempts-1, id); err != nil {
		return "", fmt.Errorf("reset pending_writes %s: %w", id, err)
	}
	return sourceID, nil
}

// Discard drops one row without replaying it. For other ops the local copy
// keeps the unsynced change until the source is resynced. A discarded create
// never reached the server, and not every provider prunes local-only events
// on sync, so its local event goes too, along with later queued rows for
// that event, which would otherwise fail against a missing remote event.
// Returns the row's source ID.
func (q *PendingQueue) Discard(id string) (string, error) {
	var sourceID, calendarID, op, payloadJSON string
	err := q.store.DB().QueryRow(
		`SELECT source_id, calendar_id, op, payload_json FROM pending_writes WHERE id = ?`, id,
	).Scan(&sourceID, &calendarID, &op, &payloadJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("pending write %s not found", id)
	}
	if err != nil {
		return "", fmt.Errorf("load pending_writes %s: %w", id, err)
	}
	var p pendingPayload
	if PendingOpKind(op) != PendingOpCreate || json.Unmarshal([]byte(payloadJSON), &p) != nil || p.UID == "" {
		if err := q.deleteRow(id); err != nil {
			return "", fmt.Errorf("delete pending_writes %s: %w", id, err)
		}
		return sourceID, nil
	}

	tx, err := q.store.DB().Begin()
	if err != nil {
		return "", fmt.Errorf("begin discard: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		DELETE FROM pending_writes
		WHERE id = ? OR (calendar_id = ? AND json_extract(payload_json, '$.uid') = ?)`,
		id, calendarID, p.UID); err != nil {
		return "", fmt.Errorf("delete pending_writes for %s: %w", p.UID, err)
	}
	if err := q.store.DeleteEventByUIDTx(tx, calendarID, p.UID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit discard: %w", err)
	}
	return sourceID, nil
}

// Retry outcomes reported by RetryStatus.
const (
	RetrySynced  = "synced"  // row replayed and removed
	RetryFailed  = "failed"  // row replayed and is stuck again
	RetryPending = "pending" // not replayed yet; the next drain will
)

// RetryStatus reports what happened to a row after Retry plus a drain.
func (q *PendingQueue) RetryStatus(id string) (string, error) {
	var attempt int
	err := q.store.DB().QueryRow(`SELECT attempt FROM pending_writes WHERE id = ?`, id).Scan(&attempt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return RetrySynced, nil
	case err != nil:
		return "", fmt.Errorf("load pending_writes %s: %w", id, err)
	case attempt >= pendingMaxAttempts:
		return RetryFailed, nil
	default:
		return RetryPending, nil
	}
}

func (q *PendingQueue) sourceOf(id string) (string, error) {
	var sourceID string
	err := q.store.DB().QueryRow(`SELECT source_id FROM pending_writes WHERE id = ?`, id).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("pending write %s not found", id)
	}
	if err != nil {
		return "", fmt.Errorf("load pending_writes %s: %w", id, err)
	}
	return sourceID, nil
}

// DrainAll iterates all sources known to the store and drains each in
// turn. Used by callers that want a single entry point on the
// system:network-online path.
func (q *PendingQueue) DrainAll(ctx context.Context) error {
	sources, err := q.store.ListSources()
	if err != nil {
		return fmt.Errorf("list sources for drain: %w", err)
	}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Skip local sources — they can't have pending writes.
		if src.Type == SourceTypeLocal {
			continue
		}
		_ = q.Drain(ctx, src.ID)
	}
	return nil
}

// nextPending returns the oldest pending row for sourceID with attempt <
// pendingMaxAttempts that is ordered after `after` (nil = from the start),
// or nil when none. Skips exhausted rows so they don't permanently block
// the queue.
func (q *PendingQueue) nextPending(sourceID string, after *pendingRow) (*pendingRow, error) {
	var afterCreated, afterSeq int64 = -1, -1
	if after != nil {
		afterCreated, afterSeq = after.CreatedUnix, after.Seq
	}
	rowRes := q.store.DB().QueryRow(`
		SELECT rowid, id, source_id, calendar_id, COALESCE(event_id, ''), op,
		       COALESCE(scope, ''), payload_json, attempt,
		       COALESCE(last_attempt_unix, 0), COALESCE(last_error, ''),
		       created_unix
		FROM pending_writes
		WHERE source_id = ? AND attempt < ?
		  AND (created_unix > ? OR (created_unix = ? AND rowid > ?))
		ORDER BY created_unix ASC, rowid ASC
		LIMIT 1`, sourceID, pendingMaxAttempts, afterCreated, afterCreated, afterSeq)

	var row pendingRow
	var payloadJSON string
	err := rowRes.Scan(
		&row.Seq, &row.ID, &row.SourceID, &row.CalendarID, &row.EventID, &row.Op,
		&row.Scope, &payloadJSON, &row.Attempt,
		&row.LastAttemptUnix, &row.LastError,
		&row.CreatedUnix,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan pending_writes: %w", err)
	}
	if err := json.Unmarshal([]byte(payloadJSON), &row.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal pending payload: %w", err)
	}
	return &row, nil
}

// processRow dispatches one pending row to the provider, handles the
// result (delete row / bump attempt / publish conflict), and returns the
// provider error so Drain can stop on transport failures. The row's state
// in the table reflects what happened.
func (q *PendingQueue) processRow(ctx context.Context, row pendingRow) error {
	src, err := q.store.GetSource(row.SourceID)
	if err != nil || src == nil {
		// Source deleted (or load failed); drop the orphaned row.
		_ = q.deleteRow(row.ID)
		return nil
	}

	provider := ProviderForSource(*src, ProviderDeps{
		Store:   q.store,
		Secrets: q.secrets,
		Events:  q.events,
		Auth:    q.auth,
	})

	cal := Calendar{ID: row.CalendarID, URL: row.Payload.CalendarURL}
	ev := Event{
		ID:              row.EventID,
		CalendarID:      row.CalendarID,
		UID:             row.Payload.UID,
		ETag:            row.Payload.ETag,
		Href:            row.Payload.Href,
		ProviderEventID: row.Payload.ProviderEventID,
		Summary:         row.Payload.Summary,
		Description:     row.Payload.Description,
		Location:        row.Payload.Location,
		DTStartUnix:     row.Payload.DTStartUnix,
		DTEndUnix:       row.Payload.DTEndUnix,
		IsAllDay:        row.Payload.IsAllDay,
		TZName:          row.Payload.TZName,
		RRuleText:       row.Payload.RRuleText,
		ICSBlob:         row.Payload.ICSBlob,
		SendUpdates:     row.Payload.SendUpdates,
	}

	pushCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var perr error
	switch PendingOpKind(row.Op) {
	case PendingOpCreate, PendingOpUpdate:
		result, presErr := provider.PushEvent(pushCtx, *src, cal, ev)
		perr = presErr
		if perr == nil {
			next := ev
			next.ETag = result.ETag
			if result.ProviderEventID != "" {
				next.ProviderEventID = result.ProviderEventID
			}
			// CalDAV creates synthesize the href; record it so later
			// writes update the resource instead of re-creating it.
			if next.Href == "" && src.Type == SourceTypeCalDAV {
				next.Href = joinHref(cal.URL, ev.UID+".ics")
			}
			_ = q.updateEventTransportFields(row.CalendarID, next)
			_ = q.advanceDependentRows(row, next)
		}
	case PendingOpDelete:
		perr = provider.DeleteRemote(pushCtx, *src, cal, ev)
	default:
		// Unknown op — drop the row.
		_ = q.deleteRow(row.ID)
		return nil
	}

	q.recordOutcome(row, perr)
	return perr
}

// recordOutcome translates the per-row push result into table state +
// emits a conflict event when applicable.
func (q *PendingQueue) recordOutcome(row pendingRow, perr error) {
	if perr == nil {
		_ = q.deleteRow(row.ID)
		return
	}
	if errors.Is(perr, ErrConflict) {
		// Drop the row + signal the frontend so the user re-edits.
		_ = q.deleteRow(row.ID)
		if q.events != nil {
			_ = q.events.Publish("calendar:write-conflict", map[string]any{
				"sourceId":   row.SourceID,
				"calendarId": row.CalendarID,
				"eventId":    row.EventID,
				"op":         row.Op,
			})
		}
		return
	}
	// Transport errors are expected while offline and retried on the next
	// drain without spending the budget. Other errors bump attempt; Drain
	// skips rows past pendingMaxAttempts until ResetExhausted.
	bump := 1
	if errors.Is(perr, ErrTransport) {
		bump = 0
	}
	_ = q.recordFailure(row.ID, bump, perr.Error())
}

func (q *PendingQueue) deleteRow(id string) error {
	_, err := q.store.DB().Exec(`DELETE FROM pending_writes WHERE id = ?`, id)
	return err
}

func (q *PendingQueue) recordFailure(id string, bump int, lastError string) error {
	_, err := q.store.DB().Exec(`
		UPDATE pending_writes
		SET attempt = attempt + ?,
		    last_attempt_unix = ?,
		    last_error = ?
		WHERE id = ?`, bump, time.Now().Unix(), lastError, id)
	return err
}

func (q *PendingQueue) updateEventTransportFields(calendarID string, ev Event) error {
	_, err := q.store.DB().Exec(`
		UPDATE events
		SET etag = ?,
		    provider_event_id = CASE WHEN ? = '' THEN provider_event_id ELSE ? END,
		    href = CASE WHEN ? = '' THEN href ELSE ? END
		WHERE calendar_id = ? AND uid = ?`,
		ev.ETag, ev.ProviderEventID, ev.ProviderEventID, ev.Href, ev.Href, calendarID, ev.UID)
	return err
}

// advanceDependentRows rewrites the transport state of later queued rows
// for the same event after row succeeded. Only rows still carrying the
// state row was replayed with are advanced, so rows enqueued against a
// different server state keep their own optimistic-concurrency check.
func (q *PendingQueue) advanceDependentRows(row pendingRow, next Event) error {
	rows, err := q.store.DB().Query(`
		SELECT id, payload_json FROM pending_writes
		WHERE source_id = ? AND calendar_id = ?
		  AND (created_unix > ? OR (created_unix = ? AND rowid > ?))`,
		row.SourceID, row.CalendarID, row.CreatedUnix, row.CreatedUnix, row.Seq)
	if err != nil {
		return fmt.Errorf("query dependent pending_writes: %w", err)
	}
	updates := map[string]string{}
	for rows.Next() {
		var id, payloadJSON string
		if err := rows.Scan(&id, &payloadJSON); err != nil {
			rows.Close()
			return fmt.Errorf("scan dependent pending_writes: %w", err)
		}
		var p pendingPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			continue
		}
		prev := row.Payload
		if p.UID != prev.UID || p.ETag != prev.ETag ||
			p.ProviderEventID != prev.ProviderEventID || p.Href != prev.Href {
			continue
		}
		p.ETag, p.ProviderEventID, p.Href = next.ETag, next.ProviderEventID, next.Href
		b, err := json.Marshal(p)
		if err != nil {
			continue
		}
		updates[id] = string(b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dependent pending_writes: %w", err)
	}
	for id, payloadJSON := range updates {
		if _, err := q.store.DB().Exec(`UPDATE pending_writes SET payload_json = ? WHERE id = ?`, payloadJSON, id); err != nil {
			return fmt.Errorf("advance pending_writes %s: %w", id, err)
		}
	}
	return nil
}
