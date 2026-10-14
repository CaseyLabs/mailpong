package apiactions

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/message"
)

type fakeRemote struct {
	applyErr  error
	res       Result
	verdict   Verdict
	verifyErr error
	applied   []string
	verified  int
}

func (f *fakeRemote) Apply(_ context.Context, a *message.APIAction) (Result, error) {
	f.applied = append(f.applied, a.Op+":"+a.RemoteID)
	return f.res, f.applyErr
}

func (f *fakeRemote) Verify(context.Context, *message.APIAction) (Verdict, error) {
	f.verified++
	return f.verdict, f.verifyErr
}

type env struct {
	s      *message.Store
	db     *database.DB
	w      *Worker
	remote *fakeRemote
	now    time.Time
	events []string
	rowID  string
}

func newEnv(t *testing.T, provider string) *env {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES ('a','T','t@x','i','s','t@x')`,
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('f1','a','INBOX','INBOX','inbox')`,
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('f2','a','Work','Work','folder')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s := message.NewStore(db)
	up := message.APIUpsert{Ref: message.APIRef{RemoteID: "r1", RemoteThreadID: "t", Containers: []string{"INBOX"}, Version: "v1"},
		Rows: []message.APIRow{{Message: message.Message{FolderID: "f1", MessageID: "<r1@x>", Subject: "s", Date: time.Unix(1, 0)}}}}
	if err := s.CommitAPIPage(message.APIPage{AccountID: "a", Provider: provider, Scope: "m", Cursor: "c", CursorKind: "k",
		Upserts: []message.APIUpsert{up}}); err != nil {
		t.Fatal(err)
	}
	_, rows, _ := s.GetAPIRef("a", provider, "r1")
	e := &env{s: s, db: db, remote: &fakeRemote{}, now: time.Unix(1000, 0), rowID: rows["f1"]}
	e.w = &Worker{Store: s, Now: func() time.Time { return e.now },
		Remote: func(string, string) (Remote, error) { return e.remote, nil },
		Emit:   func(ev string, _ any) { e.events = append(e.events, ev) }}
	return e
}

func (e *env) status(t *testing.T) string {
	t.Helper()
	acts, err := e.s.ListAPIActions("a")
	if err != nil || len(acts) == 0 {
		t.Fatalf("actions=%v err=%v", acts, err)
	}
	return acts[len(acts)-1].Status
}

func (e *env) read(t *testing.T) bool {
	t.Helper()
	m, err := e.s.Get(e.rowID)
	if err != nil {
		t.Fatal(err)
	}
	return m.IsRead
}

func (e *env) enqueue(t *testing.T) {
	t.Helper()
	if err := e.s.EnqueueAPIFlag("a", message.APIProviderGmail, message.APIActionRead, []string{e.rowID}, true); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerAppliesAndKeepsLocalState(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionApplied || len(e.remote.applied) != 1 || !e.read(t) {
		t.Fatalf("status=%s applied=%v read=%v", e.status(t), e.remote.applied, e.read(t))
	}
}

func TestWorkerRejectionRevertsAndReports(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	e.remote.applyErr = &apiwrite.Error{Outcome: apiwrite.Rejected, Status: 400, Err: errors.New("bad")}
	var changed string
	e.w.Changed = func(id string) { changed = id }
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionRejected || e.read(t) || changed != "a" || len(e.events) != 1 {
		t.Fatalf("status=%s read=%v changed=%q events=%v", e.status(t), e.read(t), changed, e.events)
	}
}

func TestWorkerGoneRemovesMessage(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	e.remote.applyErr = ErrGone
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if ref, _, _ := e.s.GetAPIRef("a", message.APIProviderGmail, "r1"); ref != nil {
		t.Fatal("gone message kept")
	}
	if e.status(t) != message.APIActionRejected {
		t.Fatalf("status = %s", e.status(t))
	}
}

func TestWorkerThrottleBacksOffThenRuns(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	e.remote.applyErr = &apiwrite.Error{Outcome: apiwrite.NotSent, Err: errors.New("no token")}
	next, err := e.w.Drain(context.Background(), "a")
	if err != nil || !next.Equal(e.now.Add(baseBackoff)) {
		t.Fatalf("next=%v err=%v", next, err)
	}
	if n := len(e.remote.applied); n != 1 {
		t.Fatalf("applied %d times", n)
	}
	_, _ = e.w.Drain(context.Background(), "a") // not due: no new request
	if n := len(e.remote.applied); n != 1 {
		t.Fatalf("re-sent before due: %d", n)
	}
	e.now = e.now.Add(time.Minute)
	e.remote.applyErr = nil
	if _, err := e.w.Drain(context.Background(), "a"); err != nil || e.status(t) != message.APIActionApplied {
		t.Fatalf("status=%s err=%v", e.status(t), err)
	}
}

func TestWorkerUnknownVerifiesBeforeResend(t *testing.T) {
	cases := []struct {
		name      string
		verdict   Verdict
		wantSent  int
		wantState string
	}{
		{"remote already applied: not re-sent", Matches, 1, message.APIActionApplied},
		{"remote still old: re-sent once", Differs, 2, message.APIActionApplied},
		{"unverifiable: held and surfaced", Unverifiable, 1, message.APIActionUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, message.APIProviderGmail)
			e.enqueue(t)
			e.remote.applyErr = &apiwrite.Error{Outcome: apiwrite.Unknown, Status: 503, Err: errors.New("503")}
			if _, err := e.w.Drain(context.Background(), "a"); err != nil {
				t.Fatal(err)
			}
			if e.status(t) != message.APIActionUnknown || len(e.remote.applied) != 1 {
				t.Fatalf("after 5xx: status=%s sent=%d", e.status(t), len(e.remote.applied))
			}
			e.now = e.now.Add(time.Minute)
			e.remote.applyErr = nil
			e.remote.verdict = tc.verdict
			if _, err := e.w.Drain(context.Background(), "a"); err != nil {
				t.Fatal(err)
			}
			if e.status(t) != tc.wantState || len(e.remote.applied) != tc.wantSent || e.remote.verified != 1 || !e.read(t) {
				t.Fatalf("status=%s sent=%d verified=%d read=%v", e.status(t), len(e.remote.applied), e.remote.verified, e.read(t))
			}
		})
	}
}

func TestWorkerUnknownStaysUnknownWithoutClient(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	e.remote.applyErr = &apiwrite.Error{Outcome: apiwrite.Unknown, Status: 503, Err: errors.New("503")}
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	remote := e.w.Remote
	e.w.Remote = func(string, string) (Remote, error) { return nil, errors.New("no token") }
	e.now = e.now.Add(time.Minute)
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionUnknown {
		t.Fatalf("status = %s, want unknown kept for verification", e.status(t))
	}
	e.w.Remote = remote
	e.remote.applyErr = nil
	e.remote.verdict = Matches
	e.now = e.now.Add(time.Hour)
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionApplied || len(e.remote.applied) != 1 || e.remote.verified != 1 {
		t.Fatalf("status=%s sent=%d verified=%d", e.status(t), len(e.remote.applied), e.remote.verified)
	}
}

func TestWorkerRestartTurnsInFlightIntoUnknown(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	a, _ := e.s.NextAPIAction("a", e.now)
	if err := e.s.SetAPIActionStatus(a.ID, message.APIActionInFlight, ""); err != nil { // crash mid-request
		t.Fatal(err)
	}
	if err := e.w.Recover(); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionUnknown {
		t.Fatalf("status = %s", e.status(t))
	}
	e.remote.verdict = Matches
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if e.status(t) != message.APIActionApplied || len(e.remote.applied) != 0 {
		t.Fatalf("status=%s sent=%v", e.status(t), e.remote.applied)
	}
}

func TestWorkerDeactivatedAndCancelled(t *testing.T) {
	e := newEnv(t, message.APIProviderGmail)
	e.enqueue(t)
	e.w.Active = func(string) bool { return false }
	if _, err := e.w.Drain(context.Background(), "a"); err != nil || len(e.remote.applied) != 0 || e.status(t) != message.APIActionPending {
		t.Fatalf("deactivated: sent=%v status=%s err=%v", e.remote.applied, e.status(t), err)
	}
	e.w.Active = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.w.Drain(ctx, "a"); !errors.Is(err, context.Canceled) || len(e.remote.applied) != 0 {
		t.Fatalf("cancelled: sent=%v err=%v", e.remote.applied, err)
	}
	if e.status(t) != message.APIActionPending {
		t.Fatalf("status = %s", e.status(t))
	}
}

func TestWorkerMoveRekeysQueuedActions(t *testing.T) {
	e := newEnv(t, message.APIProviderGraph)
	err := e.s.EnqueueAPIMove(message.APIMoveRequest{AccountID: "a", Provider: message.APIProviderGraph, MessageID: e.rowID,
		ToFolderID: "f2", Dest: "work-id", Source: "inbox-id", Containers: []string{"work-id"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.EnqueueAPIFlag("a", message.APIProviderGraph, message.APIActionRead, []string{e.rowID}, true); err != nil {
		t.Fatal(err)
	}
	e.remote.res = Result{NewRemoteID: "r2", Containers: []string{"work-id"}}
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	// The move ran first; the held flag action followed under the new ID.
	if len(e.remote.applied) != 2 || e.remote.applied[0] != "move:r1" || e.remote.applied[1] != "read:r2" {
		t.Fatalf("applied = %v", e.remote.applied)
	}
	if ref, rows, _ := e.s.GetAPIRef("a", message.APIProviderGraph, "r2"); ref == nil || rows["f2"] != e.rowID {
		t.Fatalf("ref=%v rows=%v", ref, rows)
	}
}

func TestWorkerMoveRekeyFailureRetriesWithoutDeleting(t *testing.T) {
	e := newEnv(t, message.APIProviderGraph)
	err := e.s.EnqueueAPIMove(message.APIMoveRequest{AccountID: "a", Provider: message.APIProviderGraph, MessageID: e.rowID,
		ToFolderID: "f2", Dest: "work-id", Source: "inbox-id", Containers: []string{"work-id"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.EnqueueAPIFlag("a", message.APIProviderGraph, message.APIActionRead, []string{e.rowID}, true); err != nil {
		t.Fatal(err)
	}
	e.remote.res = Result{NewRemoteID: "r2", Containers: []string{"work-id"}}
	// Fault: the new ID is already taken locally, so the re-key fails.
	if _, err := e.db.Exec(`INSERT INTO api_message_refs(account_id,provider,remote_id,containers) VALUES('a','graph','r2','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	// The move was sent once; the flag action stays held and nothing was deleted.
	if len(e.remote.applied) != 1 || e.remote.applied[0] != "move:r1" {
		t.Fatalf("applied = %v", e.remote.applied)
	}
	acts, _ := e.s.ListAPIActions("a", message.APIActionPending)
	if len(acts) != 2 || acts[0].Payload.MovedTo != "r2" {
		t.Fatalf("pending = %+v", acts)
	}
	if _, err := e.s.Get(e.rowID); err != nil {
		t.Fatalf("message lost: %v", err)
	}
	// Clear the fault; after backoff only the re-key is retried.
	if _, err := e.db.Exec(`DELETE FROM api_message_refs WHERE remote_id='r2'`); err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(time.Hour)
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if len(e.remote.applied) != 2 || e.remote.applied[1] != "read:r2" {
		t.Fatalf("applied = %v", e.remote.applied)
	}
	if ref, rows, _ := e.s.GetAPIRef("a", message.APIProviderGraph, "r2"); ref == nil || rows["f2"] != e.rowID {
		t.Fatalf("ref=%v rows=%v", ref, rows)
	}
}

func TestWorkerMoveRekeyRetryAcceptsUnrecordedRekey(t *testing.T) {
	e := newEnv(t, message.APIProviderGraph)
	err := e.s.EnqueueAPIMove(message.APIMoveRequest{AccountID: "a", Provider: message.APIProviderGraph, MessageID: e.rowID,
		ToFolderID: "f2", Dest: "work-id", Source: "inbox-id", Containers: []string{"work-id"}})
	if err != nil {
		t.Fatal(err)
	}
	acts, _ := e.s.ListAPIActions("a")
	a := acts[0]
	// A retry re-keyed the message but could not record the action as applied.
	if err := e.s.ApplyAPIMove(message.APIMove{AccountID: "a", Provider: message.APIProviderGraph, OldRemoteID: "r1",
		NewRemoteID: "r2", Containers: []string{"work-id"}, FromFolderID: "f2", ToFolderID: "f2"}); err != nil {
		t.Fatal(err)
	}
	a.Payload.MovedTo = "r2"
	if err := e.s.SetAPIActionPayload(a.ID, a.Payload); err != nil {
		t.Fatal(err)
	}
	if _, err := e.w.Drain(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if len(e.remote.applied) != 0 || e.status(t) != message.APIActionApplied {
		t.Fatalf("applied=%v status=%s", e.remote.applied, e.status(t))
	}
}
