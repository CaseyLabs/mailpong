package backend

// Tests for the PendingQueue (Phase 2 Chunk 5).
//
// Coverage:
//   - Enqueue persists with the right shape + ordering.
//   - Drain success → row deleted + events.etag/provider_event_id updated.
//   - Drain transport failure → row stays + attempt bumped.
//   - Drain conflict → row deleted + calendar:write-conflict published.
//   - Drain skips rows past pendingMaxAttempts.
//   - DrainAll skips local sources.
//
// Drain interacts with provider.PushEvent / DeleteRemote. For Google,
// we already have the fakeAuth + rewriteTransport plumbing from
// provider_google_test.go; we reuse it here so the queue tests exercise
// the real googleProvider end-to-end against an httptest server.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
)

// --- recordingEventBus ----------------------------------------------------

// recordingEventBus implements coreapi.EventBus, capturing every Publish
// for assertions.
type recordingEventBus struct {
	mu       sync.Mutex
	captured []capturedEvent
}

type capturedEvent struct {
	Name    string
	Payload any
}

func (r *recordingEventBus) Publish(name string, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.captured = append(r.captured, capturedEvent{Name: name, Payload: payload})
	return nil
}

func (r *recordingEventBus) Subscribe(_ string, _ func(any)) (coreapi.Unsubscribe, error) {
	return func() {}, nil
}

func (r *recordingEventBus) events() []capturedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]capturedEvent, len(r.captured))
	copy(out, r.captured)
	return out
}

// --- Test helpers ---------------------------------------------------------

// newTestStore opens a Store on a temp dir. Cleaned up by t.TempDir.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// seedGoogleSource inserts a Google source + one calendar so Drain has
// targets. Returns (sourceID, calendarID).
func seedGoogleSource(t *testing.T, store *Store, calURL string) (string, string) {
	t.Helper()
	srcID := "src-g1"
	calID := "cal-g1"
	now := time.Now().Unix()
	err := store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{
			ID:        srcID,
			Type:      SourceTypeGoogle,
			Name:      "Test",
			AccountID: "acct-1",
			Enabled:   true,
			Writable:  true,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		return store.CreateCalendarTx(tx, Calendar{
			ID:          calID,
			SourceID:    srcID,
			URL:         calURL,
			DisplayName: "Personal",
			Visible:     true,
			CreatedAt:   now,
		})
	})
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return srcID, calID
}

// (no helpers needed — *sql.Tx is used directly.)

// --- Enqueue --------------------------------------------------------------

func TestPendingQueue_Enqueue(t *testing.T) {
	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: ""}, &recordingEventBus{})

	srcID, calID := seedGoogleSource(t, store, "primary")

	id, err := queue.Enqueue(PendingOp{
		SourceID:    srcID,
		CalendarID:  calID,
		Op:          PendingOpCreate,
		CalendarURL: "primary",
		UID:         "evt-uid@mailven-google",
		Summary:     "Test",
		DTStartUnix: 1700000000,
		DTEndUnix:   1700003600,
		ICSBlob:     "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id == "" {
		t.Errorf("Enqueue returned empty id")
	}

	// Verify row is in the table.
	row, err := queue.nextPending(srcID, nil)
	if err != nil {
		t.Fatalf("nextPending: %v", err)
	}
	if row == nil {
		t.Fatalf("expected pending row, got nil")
	}
	if row.Op != string(PendingOpCreate) {
		t.Errorf("row.Op = %q, want create", row.Op)
	}
	if row.Payload.UID != "evt-uid@mailven-google" {
		t.Errorf("row.Payload.UID = %q", row.Payload.UID)
	}
	if row.Attempt != 0 {
		t.Errorf("row.Attempt = %d, want 0", row.Attempt)
	}
}

// --- Drain: success path --------------------------------------------------

func TestPendingQueue_Drain_SuccessUpdatesEventAndDeletesRow(t *testing.T) {
	store := newTestStore(t)
	bus := &recordingEventBus{}

	// httptest server that returns success on POST /calendars/primary/events.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(googleEvent{
			ID:      "server-event-id",
			ICalUID: "evt-uid@mailven-google",
			ETag:    `"server-etag"`,
		})
	}))
	defer srv.Close()

	auth := fakeAuth{target: srv.URL}
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, auth, bus)

	srcID, calID := seedGoogleSource(t, store, "primary")

	// Seed an event row matching the queued UID so the success path's
	// updateEventTransportFields has something to update.
	now := time.Now().Unix()
	_ = store.WithTx(func(tx *sql.Tx) error {
		return store.UpsertEventTx(tx, Event{
			ID: "evt-row-1", CalendarID: calID, UID: "evt-uid@mailven-google",
			Summary: "Test", DTStartUnix: now, DTEndUnix: now + 3600,
			ICSBlob: "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		})
	})

	if _, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID,
		Op: PendingOpCreate, CalendarURL: "primary",
		UID: "evt-uid@mailven-google", ICSBlob: minimalGoogleICS(t, "evt-uid@mailven-google"),
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := queue.Drain(context.Background(), srcID); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1", hits.Load())
	}

	// Row should be gone.
	row, _ := queue.nextPending(srcID, nil)
	if row != nil {
		t.Errorf("expected queue empty after success, got row %+v", row)
	}

	// Event row should have the new ETag + ProviderEventID.
	ev, err := store.GetEvent("evt-row-1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if ev.ETag != `"server-etag"` {
		t.Errorf("event.ETag = %q, want \"server-etag\"", ev.ETag)
	}
	if ev.ProviderEventID != "server-event-id" {
		t.Errorf("event.ProviderEventID = %q, want server-event-id", ev.ProviderEventID)
	}
}

// minimalGoogleICS — small helper for queue tests to share with the
// Google-provider tests' pattern.
func minimalGoogleICS(t *testing.T, uid string) string {
	t.Helper()
	return minimalICSBlob(t, uid) // defined in provider_caldav_test.go
}

// --- Drain: transport failure → row stays --------------------------------

func TestPendingQueue_Drain_TransportFailureKeepsRowAndBumpsAttempt(t *testing.T) {
	store := newTestStore(t)
	bus := &recordingEventBus{}

	// httptest server that immediately closes the connection, triggering
	// a transport error on the client side.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("not a hijacker")
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	auth := fakeAuth{target: srv.URL}
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, auth, bus)

	srcID, calID := seedGoogleSource(t, store, "primary")
	if _, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID,
		Op: PendingOpCreate, CalendarURL: "primary",
		UID: "evt@mailven-google", ICSBlob: minimalGoogleICS(t, "evt@mailven-google"),
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := queue.Drain(context.Background(), srcID); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	// A transport failure ends the pass after one try and does not spend
	// the retry budget, so an offline sync can't strand the row.
	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1 attempt per drain", hits.Load())
	}
	var attempt int
	var lastError string
	if err := store.DB().QueryRow(
		`SELECT attempt, COALESCE(last_error, '') FROM pending_writes WHERE source_id = ?`,
		srcID,
	).Scan(&attempt, &lastError); err != nil {
		t.Fatalf("expected row to stay after transport failure, query err: %v", err)
	}
	if attempt != 0 {
		t.Errorf("attempt = %d, want 0 after transport failure", attempt)
	}
	if lastError == "" {
		t.Errorf("last_error should be populated after failure")
	}
}

// --- Drain: conflict path → row deleted, event published -----------------

func TestPendingQueue_Drain_ConflictDropsRowAndPublishesEvent(t *testing.T) {
	store := newTestStore(t)
	bus := &recordingEventBus{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer srv.Close()

	auth := fakeAuth{target: srv.URL}
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, auth, bus)

	srcID, calID := seedGoogleSource(t, store, "primary")
	if _, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID,
		Op:              PendingOpUpdate,
		Scope:           EditScopeAll,
		CalendarURL:     "primary",
		UID:             "evt@mailven-google",
		ProviderEventID: "existing-id",
		ETag:            `"stale"`,
		ICSBlob:         minimalGoogleICS(t, "evt@mailven-google"),
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := queue.Drain(context.Background(), srcID); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	row, _ := queue.nextPending(srcID, nil)
	if row != nil {
		t.Errorf("expected row dropped on conflict, still present")
	}

	captured := bus.events()
	found := false
	for _, e := range captured {
		if e.Name == "calendar:write-conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected calendar:write-conflict event, captured: %+v", captured)
	}
}

// --- Drain: skips rows past pendingMaxAttempts ---------------------------

func TestPendingQueue_Drain_SkipsExhaustedRows(t *testing.T) {
	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: ""}, &recordingEventBus{})

	srcID, calID := seedGoogleSource(t, store, "primary")
	id, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID, Op: PendingOpCreate,
		CalendarURL: "primary", UID: "evt@mailven-google",
		ICSBlob: minimalGoogleICS(t, "evt@mailven-google"),
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Manually crank up the attempt counter past the limit.
	_, _ = store.DB().Exec(
		`UPDATE pending_writes SET attempt = ? WHERE id = ?`,
		pendingMaxAttempts, id,
	)

	// nextPending should return nil (row exhausted).
	row, err := queue.nextPending(srcID, nil)
	if err != nil {
		t.Fatalf("nextPending: %v", err)
	}
	if row != nil {
		t.Errorf("expected nextPending to skip exhausted row, got %+v", row)
	}
}

// --- DrainAll skips local sources ----------------------------------------

func TestPendingQueue_DrainAll_SkipsLocalSources(t *testing.T) {
	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: ""}, &recordingEventBus{})

	// Seed both a local and a google source.
	now := time.Now().Unix()
	_ = store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{
			ID:        "src-local",
			Type:      SourceTypeLocal,
			Name:      "Local",
			Enabled:   true,
			Writable:  true,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		return store.CreateSourceTx(tx, Source{
			ID:        "src-google",
			Type:      SourceTypeGoogle,
			Name:      "Google",
			AccountID: "acct-1",
			Enabled:   true,
			Writable:  true,
			CreatedAt: now,
		})
	})

	// DrainAll should silently no-op on the local source and not error
	// on the google source (no pending rows = empty drain).
	if err := queue.DrainAll(context.Background()); err != nil {
		t.Errorf("DrainAll: %v", err)
	}
}

// --- Drain: sequential writes to the same event --------------------------

// fakeGoogleETagServer emulates Google's optimistic concurrency: POST
// creates the event, PATCH requires If-Match to equal the current ETag
// (else 412). Every successful write bumps the ETag.
type fakeGoogleETagServer struct {
	mu      sync.Mutex
	version int
	methods []string
}

func (f *fakeGoogleETagServer) etag() string { return fmt.Sprintf(`"v%d"`, f.version) }

func (f *fakeGoogleETagServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.methods = append(f.methods, r.Method)
	if r.Method == http.MethodPatch && r.Header.Get("If-Match") != f.etag() {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	f.version++
	_ = json.NewEncoder(w).Encode(googleEvent{ID: "server-event-id", ICalUID: "evt@mailven-google", ETag: f.etag()})
}

func conflictCount(bus *recordingEventBus) int {
	n := 0
	for _, e := range bus.events() {
		if e.Name == "calendar:write-conflict" {
			n++
		}
	}
	return n
}

func TestPendingQueue_Drain_SequentialWritesAdvanceTransportState(t *testing.T) {
	const uid = "evt@mailven-google"
	tests := []struct {
		name          string
		startVersion  int // server ETag version before draining
		ops           []PendingOp
		wantMethods   []string
		wantConflicts int
	}{
		{
			name: "create then edit",
			ops: []PendingOp{
				{Op: PendingOpCreate},
				{Op: PendingOpUpdate, Scope: EditScopeAll},
			},
			wantMethods: []string{http.MethodPost, http.MethodPatch},
		},
		{
			name:         "two edits",
			startVersion: 1,
			ops: []PendingOp{
				{Op: PendingOpUpdate, Scope: EditScopeAll, ProviderEventID: "server-event-id", ETag: `"v1"`},
				{Op: PendingOpUpdate, Scope: EditScopeAll, ProviderEventID: "server-event-id", ETag: `"v1"`},
			},
			wantMethods: []string{http.MethodPatch, http.MethodPatch},
		},
		{
			name:         "remote change while offline still conflicts",
			startVersion: 2,
			ops: []PendingOp{
				{Op: PendingOpUpdate, Scope: EditScopeAll, ProviderEventID: "server-event-id", ETag: `"v1"`},
				{Op: PendingOpUpdate, Scope: EditScopeAll, ProviderEventID: "server-event-id", ETag: `"v1"`},
			},
			wantMethods:   []string{http.MethodPatch, http.MethodPatch},
			wantConflicts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			bus := &recordingEventBus{}
			fake := &fakeGoogleETagServer{version: tt.startVersion}
			srv := httptest.NewServer(fake)
			defer srv.Close()

			queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: srv.URL}, bus)
			srcID, calID := seedGoogleSource(t, store, "primary")
			for _, op := range tt.ops {
				op.SourceID, op.CalendarID, op.CalendarURL = srcID, calID, "primary"
				op.UID, op.ICSBlob = uid, minimalGoogleICS(t, uid)
				if _, err := queue.Enqueue(op); err != nil {
					t.Fatalf("Enqueue: %v", err)
				}
			}

			if err := queue.Drain(context.Background(), srcID); err != nil {
				t.Fatalf("Drain: %v", err)
			}
			if fmt.Sprint(fake.methods) != fmt.Sprint(tt.wantMethods) {
				t.Errorf("server methods = %v, want %v", fake.methods, tt.wantMethods)
			}
			if got := conflictCount(bus); got != tt.wantConflicts {
				t.Errorf("conflicts = %d, want %d", got, tt.wantConflicts)
			}
			if row, _ := queue.nextPending(srcID, nil); row != nil {
				t.Errorf("expected queue empty, got row %+v", row)
			}
		})
	}
}

func TestPendingQueue_Drain_CalDAVCreateThenEditUpdatesResource(t *testing.T) {
	const uid = "evt@mailven"
	var mu sync.Mutex
	var reqs []recordedRequest
	srv := newFakeCalDAVServer(t, func(req recordedRequest, w http.ResponseWriter) {
		mu.Lock()
		defer mu.Unlock()
		reqs = append(reqs, req)
		w.Header().Set("ETag", fmt.Sprintf(`"v%d"`, len(reqs)))
		w.WriteHeader(http.StatusCreated)
	})
	defer srv.Close()

	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{}, &recordingEventBus{})
	now := time.Now().Unix()
	if err := store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{
			ID: "src-c1", Type: SourceTypeCalDAV, Name: "DAV", URL: srv.URL,
			Username: "user", Enabled: true, Writable: true, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := store.CreateCalendarTx(tx, Calendar{
			ID: "cal-c1", SourceID: "src-c1", URL: "/cal/", DisplayName: "Personal", Visible: true, CreatedAt: now,
		}); err != nil {
			return err
		}
		return store.UpsertEventTx(tx, Event{
			ID: "evt-row-1", CalendarID: "cal-c1", UID: uid,
			DTStartUnix: now, DTEndUnix: now + 3600, ICSBlob: minimalICSBlob(t, uid),
		})
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, op := range []PendingOpKind{PendingOpCreate, PendingOpUpdate} {
		if _, err := queue.Enqueue(PendingOp{
			SourceID: "src-c1", CalendarID: "cal-c1", EventID: "evt-row-1", Op: op,
			CalendarURL: "/cal/", UID: uid, ICSBlob: minimalICSBlob(t, uid),
		}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}
	if err := queue.Drain(context.Background(), "src-c1"); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if reqs[0].ifNoneMatch != "*" {
		t.Errorf("create If-None-Match = %q, want *", reqs[0].ifNoneMatch)
	}
	if reqs[1].ifNoneMatch != "" || reqs[1].ifMatch != `"v1"` || reqs[1].path != reqs[0].path {
		t.Errorf("edit = %+v, want If-Match \"v1\" on %s", reqs[1], reqs[0].path)
	}
	ev, err := store.GetEvent("evt-row-1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if ev.ETag != `"v2"` || ev.Href != "/cal/"+uid+".ics" {
		t.Errorf("event etag/href = %q/%q, want \"v2\"//cal/%s.ics", ev.ETag, ev.Href, uid)
	}
}

// --- Drain: invitation-delivery preference survives the queue -------------

func TestPendingQueue_Drain_PreservesSendUpdates(t *testing.T) {
	store := newTestStore(t)

	var gotSendUpdates string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSendUpdates = r.URL.Query().Get("sendUpdates")
		_ = json.NewEncoder(w).Encode(googleEvent{
			ID: "server-event-id", ICalUID: "evt@mailven-google", ETag: `"v1"`,
		})
	}))
	defer srv.Close()

	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: srv.URL}, &recordingEventBus{})
	srcID, calID := seedGoogleSource(t, store, "primary")
	if _, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID,
		Op: PendingOpCreate, CalendarURL: "primary",
		UID: "evt@mailven-google", ICSBlob: minimalGoogleICS(t, "evt@mailven-google"),
		SendUpdates: "externalOnly",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := queue.Drain(context.Background(), srcID); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if gotSendUpdates != "externalOnly" {
		t.Errorf("sendUpdates query = %q, want externalOnly", gotSendUpdates)
	}
}

// --- Drain: hard failures spend one attempt per pass; reset recovers ------

func TestPendingQueue_Drain_HardFailureOneAttemptPerPassAndReset(t *testing.T) {
	store := newTestStore(t)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: srv.URL}, &recordingEventBus{})
	srcID, calID := seedGoogleSource(t, store, "primary")
	id, err := queue.Enqueue(PendingOp{
		SourceID: srcID, CalendarID: calID,
		Op: PendingOpCreate, CalendarURL: "primary",
		UID: "evt@mailven-google", ICSBlob: minimalGoogleICS(t, "evt@mailven-google"),
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	attempt := func() int {
		t.Helper()
		var n int
		if err := store.DB().QueryRow(`SELECT attempt FROM pending_writes WHERE id = ?`, id).Scan(&n); err != nil {
			t.Fatalf("query attempt: %v", err)
		}
		return n
	}

	if err := queue.Drain(context.Background(), srcID); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if hits.Load() != 1 || attempt() != 1 {
		t.Fatalf("after one drain: hits = %d, attempt = %d, want 1 and 1", hits.Load(), attempt())
	}

	for i := 1; i < pendingMaxAttempts; i++ {
		_ = queue.Drain(context.Background(), srcID)
	}
	if got := attempt(); got != pendingMaxAttempts {
		t.Fatalf("attempt = %d, want %d", got, pendingMaxAttempts)
	}
	if row, _ := queue.nextPending(srcID, nil); row != nil {
		t.Fatalf("exhausted row should be skipped")
	}

	if err := queue.ResetExhausted(); err != nil {
		t.Fatalf("ResetExhausted: %v", err)
	}
	if row, _ := queue.nextPending(srcID, nil); row == nil || row.ID != id {
		t.Fatalf("reset row should be eligible again, got %+v", row)
	}
}

// --- Stuck writes: list, retry, discard ----------------------------------

func TestPendingQueue_StuckWrites(t *testing.T) {
	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: ""}, &recordingEventBus{})
	srcID, calID := seedGoogleSource(t, store, "primary")

	enqueue := func(uid, summary string) string {
		t.Helper()
		id, err := queue.Enqueue(PendingOp{
			SourceID: srcID, CalendarID: calID, Op: PendingOpUpdate,
			CalendarURL: "primary", UID: uid, Summary: summary, DTStartUnix: 1700000000,
		})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		return id
	}
	stuckID := enqueue("stuck@x", "Stuck")
	freshID := enqueue("fresh@x", "Fresh")
	if err := queue.recordFailure(stuckID, pendingMaxAttempts, "HTTP 400"); err != nil {
		t.Fatalf("recordFailure: %v", err)
	}
	if err := queue.recordFailure(freshID, 1, "HTTP 500"); err != nil {
		t.Fatalf("recordFailure: %v", err)
	}

	list, err := queue.ListStuck()
	if err != nil {
		t.Fatalf("ListStuck: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListStuck = %+v, want only the exhausted row", list)
	}
	got := list[0]
	if got.ID != stuckID || got.SourceID != srcID || got.SourceName != "Test" || got.Op != "update" ||
		got.Summary != "Stuck" || got.DTStartUnix != 1700000000 || got.LastError != "HTTP 400" || got.LastAttemptUnix == 0 {
		t.Errorf("ListStuck[0] = %+v", got)
	}

	if src, err := queue.Retry(stuckID); err != nil || src != srcID {
		t.Fatalf("Retry = %q, %v; want %q", src, err, srcID)
	}
	if st, err := queue.RetryStatus(stuckID); err != nil || st != RetryPending {
		t.Errorf("RetryStatus before drain = %q, %v; want pending", st, err)
	}
	if row, err := queue.nextPending(srcID, nil); err != nil || row == nil || row.ID != stuckID {
		t.Fatalf("after Retry nextPending = %+v, %v; want the retried row first", row, err)
	}
	if list, _ := queue.ListStuck(); len(list) != 0 {
		t.Errorf("after Retry ListStuck = %+v, want empty", list)
	}
	// One more failure re-exhausts the retried row.
	if err := queue.recordFailure(stuckID, 1, "HTTP 400"); err != nil {
		t.Fatalf("recordFailure: %v", err)
	}
	if list, _ := queue.ListStuck(); len(list) != 1 || list[0].ID != stuckID {
		t.Errorf("after failed retry ListStuck = %+v, want the row back", list)
	}
	if st, _ := queue.RetryStatus(stuckID); st != RetryFailed {
		t.Errorf("RetryStatus after failure = %q, want failed", st)
	}

	if src, err := queue.Discard(freshID); err != nil || src != srcID {
		t.Fatalf("Discard = %q, %v; want %q", src, err, srcID)
	}
	var n int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM pending_writes WHERE id = ?`, freshID).Scan(&n); err != nil || n != 0 {
		t.Errorf("discarded row count = %d, %v; want 0", n, err)
	}
	if st, _ := queue.RetryStatus(freshID); st != RetrySynced {
		t.Errorf("RetryStatus of removed row = %q, want synced", st)
	}

	if _, err := queue.Retry("missing"); err == nil {
		t.Error("Retry(missing) = nil error, want not found")
	}
	if _, err := queue.Discard("missing"); err == nil {
		t.Error("Discard(missing) = nil error, want not found")
	}
}

// Discarding a stuck create removes its never-synced local event and any
// later queued rows for the same event; other events are untouched.
func TestPendingQueue_DiscardCreateDropsLocalEvent(t *testing.T) {
	store := newTestStore(t)
	queue := NewPendingQueue(store, fakeSecrets{password: "x"}, fakeAuth{target: ""}, &recordingEventBus{})
	srcID, calID := seedGoogleSource(t, store, "primary")

	now := time.Now().Unix()
	for _, ev := range []Event{
		{ID: "evt-new", CalendarID: calID, UID: "new@x", Summary: "New", DTStartUnix: now, DTEndUnix: now + 3600},
		{ID: "evt-other", CalendarID: calID, UID: "other@x", Summary: "Other", DTStartUnix: now, DTEndUnix: now + 3600},
	} {
		if err := store.WithTx(func(tx *sql.Tx) error { return store.UpsertEventTx(tx, ev) }); err != nil {
			t.Fatalf("UpsertEventTx: %v", err)
		}
	}
	enqueue := func(op PendingOpKind, uid string) string {
		t.Helper()
		id, err := queue.Enqueue(PendingOp{SourceID: srcID, CalendarID: calID, Op: op, CalendarURL: "primary", UID: uid})
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		return id
	}
	createID := enqueue(PendingOpCreate, "new@x")
	enqueue(PendingOpUpdate, "new@x")
	otherID := enqueue(PendingOpUpdate, "other@x")

	if _, err := queue.Discard(createID); err != nil {
		t.Fatalf("Discard: %v", err)
	}

	var rows, events int
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM pending_writes`).Scan(&rows)
	if rows != 1 {
		t.Errorf("pending rows = %d, want 1 (only %s)", rows, otherID)
	}
	if st, _ := queue.RetryStatus(otherID); st != RetryPending {
		t.Errorf("other row status = %q, want pending", st)
	}
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE uid = 'new@x'`).Scan(&events)
	if events != 0 {
		t.Errorf("discarded create's local event still present")
	}
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE uid = 'other@x'`).Scan(&events)
	if events != 1 {
		t.Errorf("unrelated event removed")
	}
}
