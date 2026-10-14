package message

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/folder"
)

func TestAPIMapGmailLabels(t *testing.T) {
	labels := []GmailLabel{
		{ID: "INBOX", Type: "system"}, {ID: "Label_1", Name: "Work/2026", Type: "user"},
		{ID: "Label_2", Name: "Ünïcode 件", Type: "user"}, {ID: "CHAT", Type: "system"},
	}
	tests := []struct {
		name     string
		in       []string
		folders  []string
		types    []folder.Type
		read     bool
		starred  bool
		draft    bool
		metadata []string
		unmapped []string
	}{
		{"inbox unread starred", []string{"INBOX", "UNREAD", "STARRED", "IMPORTANT", "CATEGORY_PROMOTIONS"},
			[]string{"INBOX"}, []folder.Type{folder.TypeInbox}, false, true, false,
			[]string{"CATEGORY_PROMOTIONS", "IMPORTANT"}, nil},
		{"multi label yields one row per folder", []string{"Label_2", "INBOX", "Label_1"},
			[]string{"INBOX", "Label_1", "Label_2"},
			[]folder.Type{folder.TypeInbox, folder.TypeFolder, folder.TypeFolder}, true, false, false, nil, nil},
		{"draft sets flag", []string{"DRAFT"}, []string{"DRAFT"}, []folder.Type{folder.TypeDrafts}, true, false, true, nil, nil},
		{"archived goes to all mail", []string{"CATEGORY_UPDATES"}, []string{""}, []folder.Type{folder.TypeAll}, true, false, false,
			[]string{"CATEGORY_UPDATES"}, nil},
		{"unknown and chat kept unmapped", []string{"SENT", "CHAT", "Label_99", "SENT"},
			[]string{"SENT"}, []folder.Type{folder.TypeSent}, true, false, false, nil, []string{"CHAT", "Label_99"}},
		{"trash and spam", []string{"TRASH", "SPAM"}, []string{"SPAM", "TRASH"},
			[]folder.Type{folder.TypeSpam, folder.TypeTrash}, true, false, false, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MapGmailLabels(tc.in, labels)
			var ids []string
			var types []folder.Type
			for _, f := range got.Folders {
				ids = append(ids, f.RemoteID)
				types = append(types, f.Type)
			}
			if !reflect.DeepEqual(ids, tc.folders) || !reflect.DeepEqual(types, tc.types) {
				t.Fatalf("folders = %v %v, want %v %v", ids, types, tc.folders, tc.types)
			}
			if got.IsRead != tc.read || got.IsStarred != tc.starred || got.IsDraft != tc.draft {
				t.Fatalf("flags read=%v starred=%v draft=%v", got.IsRead, got.IsStarred, got.IsDraft)
			}
			if !reflect.DeepEqual(got.Metadata, tc.metadata) || !reflect.DeepEqual(got.Unmapped, tc.unmapped) {
				t.Fatalf("metadata=%v unmapped=%v", got.Metadata, got.Unmapped)
			}
		})
	}
}

func TestAPIMapGraphFolders(t *testing.T) {
	m := MapGraphFolders("root", []GraphFolder{
		{ID: "f-inbox", Name: "Posteingang", ParentID: "root", WellKnown: "inbox"},
		{ID: "f-sent", Name: "Sent Items", ParentID: "root", WellKnown: "sentitems"},
		{ID: "f-del", Name: "Deleted Items", ParentID: "root", WellKnown: "deleteditems"},
		{ID: "f-sub", Name: "Projekte", ParentID: "f-inbox"},
		{ID: "f-deep", Name: "2026", ParentID: "f-sub"},
		{ID: "f-orphan", Name: "Lost", ParentID: "missing"},
		{ID: "f-loop-a", Name: "A", ParentID: "f-loop-b"},
		{ID: "f-loop-b", Name: "B", ParentID: "f-loop-a"},
	})
	want := map[string]struct {
		path string
		typ  folder.Type
	}{
		"f-inbox": {"Posteingang", folder.TypeInbox}, "f-sent": {"Sent Items", folder.TypeSent},
		"f-del": {"Deleted Items", folder.TypeTrash}, "f-sub": {"Posteingang/Projekte", folder.TypeFolder},
		"f-deep": {"Posteingang/Projekte/2026", folder.TypeFolder},
	}
	if len(m.Folders) != len(want) {
		t.Fatalf("mapped %d folders: %+v", len(m.Folders), m.Folders)
	}
	for id, w := range want {
		if got := m.Folders[id]; got.Path != w.path || got.Type != w.typ || got.RemoteID != id {
			t.Fatalf("%s = %+v, want %+v", id, got, w)
		}
	}
	if !reflect.DeepEqual(m.Unmapped, []string{"f-loop-a", "f-loop-b", "f-orphan"}) {
		t.Fatalf("unmapped = %v", m.Unmapped)
	}
	if got := m.MapGraphMessage("f-sub"); len(got.Folders) != 1 || got.Folders[0].RemoteID != "f-sub" {
		t.Fatalf("message mapping = %+v", got)
	}
	if got := m.MapGraphMessage("f-orphan"); len(got.Folders) != 0 || !reflect.DeepEqual(got.Unmapped, []string{"f-orphan"}) {
		t.Fatalf("unmapped message = %+v", got)
	}
}

func apiPageFixture(t *testing.T) (*Store, string, string) {
	t.Helper()
	s, accountID, inbox := newBodyFailedTestStore(t)
	if _, err := s.db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('folder-2', ?, 'Work', 'Work', 'folder')`, accountID); err != nil {
		t.Fatal(err)
	}
	_ = inbox
	return s, accountID, "folder-2"
}

func apiUpsert(remote, thread string, folders ...string) APIUpsert {
	u := APIUpsert{Ref: APIRef{RemoteID: remote, RemoteThreadID: thread, Containers: folders, Version: "v1"}}
	for _, f := range folders {
		u.Rows = append(u.Rows, APIRow{Message: Message{FolderID: f, MessageID: "<" + remote + "@x>", Subject: "s " + remote, Date: time.Unix(1, 0).UTC()}})
	}
	return u
}

func countRows(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAPICommitPageSharedIdentityAndLabelChanges(t *testing.T) {
	s, acct, work := apiPageFixture(t)
	page := APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "mailbox", Cursor: "h1", CursorKind: "gmail-history",
		Upserts: []APIUpsert{apiUpsert("g1", "t1", "folder-1", work)}}
	if err := s.CommitAPIPage(page); err != nil {
		t.Fatal(err)
	}
	ref, rows, err := s.GetAPIRef(acct, APIProviderGmail, "g1")
	if err != nil || ref == nil || len(rows) != 2 || ref.RemoteThreadID != "t1" {
		t.Fatalf("ref=%+v rows=%v err=%v", ref, rows, err)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM api_message_refs`); n != 1 {
		t.Fatalf("%d identities, want 1", n)
	}
	// Label removal drops only the folder the message left; the surviving row keeps its ID.
	page2 := APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "mailbox", Cursor: "h2", CursorKind: "gmail-history",
		Upserts: []APIUpsert{apiUpsert("g1", "t1", "folder-1")}}
	if err := s.CommitAPIPage(page2); err != nil {
		t.Fatal(err)
	}
	_, rows2, _ := s.GetAPIRef(acct, APIProviderGmail, "g1")
	if len(rows2) != 1 || rows2["folder-1"] != rows["folder-1"] {
		t.Fatalf("rows after label removal = %v (was %v)", rows2, rows)
	}
	// Remote delete removes rows and ref.
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "mailbox", Removals: []string{"g1"}, Cursor: "h3"}); err != nil {
		t.Fatal(err)
	}
	if ref, _, _ := s.GetAPIRef(acct, APIProviderGmail, "g1"); ref != nil {
		t.Fatal("ref survived delete")
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM messages`); n != 0 {
		t.Fatalf("%d messages survived delete", n)
	}
}

func TestAPIUIDsAreLocalMonotonicSurrogates(t *testing.T) {
	s, acct, _ := apiPageFixture(t)
	commit := func(cursor string, ups ...APIUpsert) {
		t.Helper()
		cur, _ := s.LoadAPICursor(acct, "m")
		if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Generation: cur.Generation, Cursor: cursor, Upserts: ups}); err != nil {
			t.Fatal(err)
		}
	}
	commit("1", apiUpsert("a", "", "folder-1"), apiUpsert("b", "", "folder-1"))
	commit("2", apiUpsert("c", "", "folder-1"))
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Removals: []string{"c"}, Cursor: "3"}); err != nil {
		t.Fatal(err)
	}
	commit("4", apiUpsert("d", "", "folder-1"))
	var uids []uint32
	rows, err := s.db.Query(`SELECT uid FROM messages ORDER BY uid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var u uint32
		_ = rows.Scan(&u)
		uids = append(uids, u)
	}
	if !reflect.DeepEqual(uids, []uint32{1, 2, 4}) {
		t.Fatalf("uids = %v, want [1 2 4] (3 not reused)", uids)
	}
}

func TestAPICursorCrashBetweenPageAndCursor(t *testing.T) {
	s, acct, _ := apiPageFixture(t)
	first := APIPage{AccountID: acct, Provider: APIProviderGraph, Scope: "inbox", Cursor: "d1", CursorKind: "graph-delta",
		Upserts: []APIUpsert{apiUpsert("m1", "", "folder-1")}}
	if err := s.CommitAPIPage(first); err != nil {
		t.Fatal(err)
	}
	// A page whose second message fails (unknown folder) must store nothing and keep the cursor.
	bad := APIPage{AccountID: acct, Provider: APIProviderGraph, Scope: "inbox", Cursor: "d2", CursorKind: "graph-delta",
		Upserts: []APIUpsert{apiUpsert("m2", "", "folder-1"), apiUpsert("m3", "", "no-such-folder")}}
	if err := s.CommitAPIPage(bad); err == nil {
		t.Fatal("expected failure")
	}
	cur, _ := s.LoadAPICursor(acct, "inbox")
	if cur.Cursor != "d1" || countRows(t, s, `SELECT COUNT(*) FROM messages`) != 1 {
		t.Fatalf("cursor %q, messages not rolled back", cur.Cursor)
	}
	// Retrying the repaired page, even twice, neither skips nor duplicates.
	bad.Upserts[1] = apiUpsert("m3", "", "folder-1")
	for i := 0; i < 2; i++ {
		if err := s.CommitAPIPage(bad); err != nil {
			t.Fatal(err)
		}
	}
	cur, _ = s.LoadAPICursor(acct, "inbox")
	if cur.Cursor != "d2" || countRows(t, s, `SELECT COUNT(*) FROM messages`) != 3 || countRows(t, s, `SELECT COUNT(*) FROM api_message_refs`) != 3 {
		t.Fatalf("cursor %q, want d2 with 3 messages", cur.Cursor)
	}
}

func TestAPICursorInvalidateBumpsGeneration(t *testing.T) {
	s, acct, _ := apiPageFixture(t)
	page := APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "mailbox", Cursor: "h1", CursorKind: "gmail-history",
		Upserts: []APIUpsert{apiUpsert("g1", "", "folder-1")}}
	if err := s.CommitAPIPage(page); err != nil {
		t.Fatal(err)
	}
	if err := s.InvalidateAPICursor(acct, "mailbox"); err != nil {
		t.Fatal(err)
	}
	cur, _ := s.LoadAPICursor(acct, "mailbox")
	if cur.Cursor != "" || !cur.Invalid || cur.Generation != 1 {
		t.Fatalf("after invalidate: %+v", cur)
	}
	if countRows(t, s, `SELECT COUNT(*) FROM messages`) != 1 {
		t.Fatal("invalidate must not wipe local data")
	}
	// A page built before the bump is refused.
	page.Cursor = "stale"
	if err := s.CommitAPIPage(page); !errors.Is(err, ErrAPIStaleGeneration) {
		t.Fatalf("stale page err = %v", err)
	}
	// An unflagged page keeps the unreconciled marker; the reconcile page clears it.
	next := APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "mailbox", Generation: 1, Cursor: "p1", CursorKind: "gmail-history"}
	if err := s.CommitAPIPage(next); err != nil {
		t.Fatal(err)
	}
	if cur, _ = s.LoadAPICursor(acct, "mailbox"); !cur.Invalid || cur.Cursor != "p1" {
		t.Fatalf("mid-reconcile: %+v", cur)
	}
	next.ClearInvalid, next.Cursor = true, "h9"
	if err := s.CommitAPIPage(next); err != nil {
		t.Fatal(err)
	}
	if cur, _ = s.LoadAPICursor(acct, "mailbox"); cur.Invalid || cur.Cursor != "h9" || cur.Generation != 1 {
		t.Fatalf("after reconcile: %+v", cur)
	}
	ids, _ := s.ListAPIRemoteIDs(acct, APIProviderGmail)
	if !reflect.DeepEqual(ids, []string{"g1"}) {
		t.Fatalf("remote IDs = %v", ids)
	}
}

func TestAPIUpdateKeepsFetchedBody(t *testing.T) {
	s, acct, _ := apiPageFixture(t)
	u := apiUpsert("g1", "", "folder-1")
	u.Rows[0].Message.BodyText, u.Rows[0].Message.BodyFetched = "hello", true
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Upserts: []APIUpsert{u}}); err != nil {
		t.Fatal(err)
	}
	u2 := apiUpsert("g1", "", "folder-1")
	u2.Rows[0].Message.IsRead = true
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Upserts: []APIUpsert{u2}}); err != nil {
		t.Fatal(err)
	}
	_, rows, _ := s.GetAPIRef(acct, APIProviderGmail, "g1")
	m, err := s.Get(rows["folder-1"])
	if err != nil || m.BodyText != "hello" || !m.IsRead || !m.BodyFetched {
		t.Fatalf("message = %+v err=%v", m, err)
	}
}

func TestAPIRejectsDuplicateFolderRows(t *testing.T) {
	s, acct, _ := apiPageFixture(t)
	u := apiUpsert("g1", "", "folder-1", "folder-1")
	err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGmail, Scope: "m", Cursor: "x", Upserts: []APIUpsert{u}})
	if err == nil || countRows(t, s, `SELECT COUNT(*) FROM messages`) != 0 {
		t.Fatalf("duplicate folder accepted: %v", err)
	}
}

func TestAPIApplyMoveRekeysRefAndRow(t *testing.T) {
	s, acct, work := apiPageFixture(t)
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGraph, Scope: "f", Cursor: "c", CursorKind: "graph-delta",
		Upserts: []APIUpsert{apiUpsert("old", "t1", "folder-1")}}); err != nil {
		t.Fatal(err)
	}
	_, before, _ := s.GetAPIRef(acct, APIProviderGraph, "old")
	if err := s.ApplyAPIMove(APIMove{AccountID: acct, Provider: APIProviderGraph, OldRemoteID: "old", NewRemoteID: "new",
		Containers: []string{"remote-work"}, FromFolderID: "folder-1", ToFolderID: work}); err != nil {
		t.Fatal(err)
	}
	if ref, _, _ := s.GetAPIRef(acct, APIProviderGraph, "old"); ref != nil {
		t.Fatal("old ref survived")
	}
	ref, rows, err := s.GetAPIRef(acct, APIProviderGraph, "new")
	if err != nil || ref == nil || ref.RemoteThreadID != "t1" || len(ref.Containers) != 1 || ref.Containers[0] != "remote-work" {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if rows[work] != before["folder-1"] || len(rows) != 1 {
		t.Fatalf("rows = %v, want the same local row now in %s", rows, work)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM messages WHERE folder_id='folder-1'`); n != 0 {
		t.Fatalf("%d rows left in the source folder", n)
	}
}

func TestAPIApplyMoveKeepsIDAndRejectsBadMoves(t *testing.T) {
	s, acct, work := apiPageFixture(t)
	if err := s.CommitAPIPage(APIPage{AccountID: acct, Provider: APIProviderGraph, Scope: "f", Cursor: "c", CursorKind: "graph-delta",
		Upserts: []APIUpsert{apiUpsert("a", "t", "folder-1"), apiUpsert("b", "t", "folder-1")}}); err != nil {
		t.Fatal(err)
	}
	mv := APIMove{AccountID: acct, Provider: APIProviderGraph, OldRemoteID: "a", Containers: []string{"w"}, FromFolderID: "folder-1", ToFolderID: work}
	if err := s.ApplyAPIMove(mv); err != nil {
		t.Fatal(err)
	}
	if _, rows, _ := s.GetAPIRef(acct, APIProviderGraph, "a"); rows[work] == "" {
		t.Fatalf("rows = %v", rows)
	}
	// Source row is gone now.
	if err := s.ApplyAPIMove(mv); err == nil {
		t.Fatal("moving from a folder without a row succeeded")
	}
	// Re-keying onto an existing ref fails and changes nothing.
	clash := APIMove{AccountID: acct, Provider: APIProviderGraph, OldRemoteID: "b", NewRemoteID: "a", FromFolderID: "folder-1", ToFolderID: work}
	if err := s.ApplyAPIMove(clash); err == nil {
		t.Fatal("re-key onto an existing ref succeeded")
	}
	if _, rows, _ := s.GetAPIRef(acct, APIProviderGraph, "b"); rows["folder-1"] == "" || len(rows) != 1 {
		t.Fatalf("failed move changed rows: %v", rows)
	}
}
