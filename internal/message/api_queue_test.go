package message

import (
	"testing"
	"time"
)

func queueFixture(t *testing.T, provider string, folders ...string) (*Store, string, map[string]string) {
	t.Helper()
	s, acct, work := apiPageFixture(t)
	if len(folders) == 0 {
		folders = []string{"folder-1"}
	}
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: provider, Scope: "m", Cursor: "c", CursorKind: "k",
		Upserts: []APIUpsert{apiUpsert("r1", "t1", folders...)}}); err != nil {
		t.Fatal(err)
	}
	_, rows, err := s.GetAPIRef(acct, provider, "r1")
	if err != nil {
		t.Fatal(err)
	}
	rows["work"] = work
	return s, acct, rows
}

func flagOf(t *testing.T, s *Store, id string) bool {
	t.Helper()
	return countRows(t, s, `SELECT is_read FROM messages WHERE id=?`, id) == 1
}

func TestQueueFlagAtomicAndSiblings(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGmail, "folder-1", "folder-2")
	if err := s.EnqueueAPIFlag(acct, APIProviderGmail, APIActionRead, []string{rows["folder-1"]}, true); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{rows["folder-1"], rows["folder-2"]} {
		if !flagOf(t, s, id) {
			t.Fatalf("row %s not read", id)
		}
	}
	acts, _ := s.ListAPIActions(acct)
	if len(acts) != 1 || acts[0].RemoteID != "r1" || len(acts[0].MessageIDs) != 2 || acts[0].Payload.Prior {
		t.Fatalf("actions = %+v", acts)
	}
	// A message that is not an API row fails and leaves nothing behind.
	if err := s.EnqueueAPIFlag(acct, APIProviderGmail, APIActionRead, []string{rows["folder-1"], "nope"}, false); err == nil {
		t.Fatal("want error")
	}
	if !flagOf(t, s, rows["folder-1"]) {
		t.Fatal("failed enqueue changed the local row")
	}
	if acts, _ := s.ListAPIActions(acct); len(acts) != 1 {
		t.Fatalf("%d actions after failed enqueue", len(acts))
	}
}

func TestQueueFlagCoalesces(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGraph)
	id := rows["folder-1"]
	set := func(v bool) {
		t.Helper()
		if err := s.EnqueueAPIFlag(acct, APIProviderGraph, APIActionRead, []string{id}, v); err != nil {
			t.Fatal(err)
		}
	}
	set(true)
	set(false) // back to the original: nothing to send
	if acts, _ := s.ListAPIActions(acct); len(acts) != 0 {
		t.Fatalf("toggle back left %d actions", len(acts))
	}
	set(true)
	// A star change in between keeps its own row; a later read toggle then must
	// not merge across it.
	if err := s.EnqueueAPIFlag(acct, APIProviderGraph, APIActionStar, []string{id}, true); err != nil {
		t.Fatal(err)
	}
	set(false)
	acts, _ := s.ListAPIActions(acct)
	if len(acts) != 3 || acts[2].Payload.Value || !acts[2].Payload.Prior {
		t.Fatalf("actions = %+v", acts)
	}
}

func TestQueueMoveAndRevert(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGmail)
	id := rows["folder-1"]
	err := s.EnqueueAPIMove(APIMoveRequest{AccountID: acct, Provider: APIProviderGmail, MessageID: id,
		ToFolderID: rows["work"], Dest: "Label_1", Source: "INBOX", Containers: []string{"Label_1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, now, _ := s.GetAPIRef(acct, APIProviderGmail, "r1")
	if now[rows["work"]] != id || len(now) != 1 {
		t.Fatalf("rows = %v", now)
	}
	a, err := s.NextAPIAction(acct, time.Now())
	if err != nil || a == nil || a.Op != APIActionMove || a.Payload.FromFolderID != "folder-1" {
		t.Fatalf("next = %+v err=%v", a, err)
	}
	if err := s.RevertAPIMove(a); err != nil {
		t.Fatal(err)
	}
	_, back, _ := s.GetAPIRef(acct, APIProviderGmail, "r1")
	if back["folder-1"] != id || len(back) != 1 {
		t.Fatalf("after revert rows = %v", back)
	}
}

func TestQueueMoveIntoExistingLabelDropsSourceRow(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGmail, "folder-1", "folder-2")
	err := s.EnqueueAPIMove(APIMoveRequest{AccountID: acct, Provider: APIProviderGmail, MessageID: rows["folder-1"],
		ToFolderID: "folder-2", Dest: "Label_1", Source: "INBOX", Containers: []string{"Label_1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, now, _ := s.GetAPIRef(acct, APIProviderGmail, "r1")
	if len(now) != 1 || now["folder-2"] != rows["folder-2"] {
		t.Fatalf("rows = %v", now)
	}
	// A rejected merge must not move the label's own row back to the source.
	a, err := s.NextAPIAction(acct, time.Now())
	if err != nil || a == nil || !a.Payload.Merged {
		t.Fatalf("next = %+v err=%v", a, err)
	}
	if err := s.RevertAPIMove(a); err != nil {
		t.Fatal(err)
	}
	_, back, _ := s.GetAPIRef(acct, APIProviderGmail, "r1")
	if len(back) != 1 || back["folder-2"] != rows["folder-2"] {
		t.Fatalf("after revert rows = %v", back)
	}
}

func TestQueueOrderingRetryAndRecovery(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGraph)
	id := rows["folder-1"]
	_ = s.EnqueueAPIFlag(acct, APIProviderGraph, APIActionRead, []string{id}, true)
	_ = s.EnqueueAPIFlag(acct, APIProviderGraph, APIActionStar, []string{id}, true)
	now := time.Now()
	first, _ := s.NextAPIAction(acct, now)
	if first == nil || first.Op != APIActionRead {
		t.Fatalf("first = %+v", first)
	}
	if err := s.SetAPIActionStatus(first.ID, APIActionInFlight, ""); err != nil {
		t.Fatal(err)
	}
	// The later action on the same message waits for the open one.
	if a, _ := s.NextAPIAction(acct, now); a != nil {
		t.Fatalf("ran %+v behind an in-flight action", a)
	}
	if n, err := s.RecoverAPIActions(acct); err != nil || n != 1 {
		t.Fatalf("recovered %d err=%v", n, err)
	}
	got, _ := s.ListAPIActions(acct, APIActionUnknown)
	if len(got) != 1 || got[0].Attempts != 1 {
		t.Fatalf("unknown = %+v", got)
	}
	if err := s.SetAPIActionStatus(first.ID, APIActionApplied, ""); err != nil {
		t.Fatal(err)
	}
	next, _ := s.NextAPIAction(acct, now)
	if next == nil || next.Op != APIActionStar {
		t.Fatalf("next = %+v", next)
	}
	if err := s.ScheduleAPIAction(next.ID, APIActionPending, now.Add(time.Minute), "throttled"); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.NextAPIAction(acct, now); a != nil {
		t.Fatal("deferred action ran early")
	}
	if a, _ := s.NextAPIAction(acct, now.Add(2*time.Minute)); a == nil {
		t.Fatal("deferred action never became due")
	}
}

func TestQueueReconcileGuardAndRemoval(t *testing.T) {
	s, acct, rows := queueFixture(t, APIProviderGmail)
	id := rows["folder-1"]
	if err := s.EnqueueAPIFlag(acct, APIProviderGmail, APIActionRead, []string{id}, true); err != nil {
		t.Fatal(err)
	}
	stale := apiUpsert("r1", "t1", "folder-1")
	stale.Ref.Version = "v2"
	stale.Rows[0].Message.IsRead = false
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Cursor: "c2", CursorKind: "k",
		Upserts: []APIUpsert{stale}}); err != nil {
		t.Fatal(err)
	}
	if !flagOf(t, s, id) {
		t.Fatal("reconcile overwrote a pending local flag")
	}
	acts, _ := s.ListAPIActions(acct)
	if err := s.SetAPIActionStatus(acts[0].ID, APIActionApplied, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Cursor: "c3", CursorKind: "k",
		Upserts: []APIUpsert{stale}}); err != nil {
		t.Fatal(err)
	}
	if flagOf(t, s, id) {
		t.Fatal("reconcile did not resume once the action closed")
	}
	// Removals proceed even with an open action.
	_ = s.EnqueueAPIFlag(acct, APIProviderGmail, APIActionStar, []string{id}, true)
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Cursor: "c4", CursorKind: "k",
		Removals: []string{"r1"}}); err != nil {
		t.Fatal(err)
	}
	if ref, _, _ := s.GetAPIRef(acct, APIProviderGmail, "r1"); ref != nil {
		t.Fatal("removal blocked by an open action")
	}
}
