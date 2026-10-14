package apimigrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/database"
)

func openDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES ('a','n','a@x.test','i','s','a@x.test')`)
	return db
}

func mustExec(t *testing.T, db *database.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// fixture: IMAP inbox hidden (as after activation), API inbox visible.
func seed(t *testing.T, db *database.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO folders (id, account_id, name, path, folder_type) VALUES
		('fi','a','INBOX','`+"\x1fimap\x1f"+`INBOX','hidden:imap:inbox'),
		('fa','a','INBOX','INBOX','inbox')`)
	ins := func(id, folder, mid, date string, uid int, fetched int, body string) {
		mustExec(t, db, `INSERT INTO messages (id, account_id, folder_id, uid, message_id, date, body_text, body_fetched, is_read, is_answered, pgp_status)
			VALUES (?, 'a', ?, ?, ?, ?, ?, ?, 0, ?, ?)`, id, folder, uid, mid, date, body, fetched, map[bool]int{true: 1}[id == "i1"], map[bool]string{true: "valid"}[id == "i1"])
	}
	ins("i1", "fi", "<one@x>", "2026-01-01 10:00:00", 1, 1, "cached body one")
	ins("i2", "fi", "<two@x>", "2026-01-02 10:00:00", 2, 1, "cached body two")
	ins("i3", "fi", "<dup@x>", "2026-01-03 10:00:00", 3, 1, "dup a")
	ins("i4", "fi", "<dup@x>", "2026-01-03 10:00:00", 4, 1, "dup b")
	ins("a1", "fa", "<one@x>", "2026-01-01 10:00:00", 1, 0, "")
	ins("a3", "fa", "<dup@x>", "2026-01-03 10:00:00", 2, 0, "")
	ins("a5", "fa", "<new@x>", "2026-01-05 10:00:00", 3, 0, "")
	for _, id := range []string{"a1", "a3", "a5"} {
		mustExec(t, db, `INSERT OR IGNORE INTO api_message_refs(account_id,provider,remote_id) VALUES('a','gmail',?)`, "r-"+id)
		mustExec(t, db, `INSERT INTO api_message_rows(message_id,account_id,provider,remote_id) VALUES(?,'a','gmail',?)`, id, "r-"+id)
	}
	mustExec(t, db, `INSERT INTO attachments (id, message_id, filename, content_type, size) VALUES ('att','i1','f.txt','text/plain',3)`)
}

func TestCorrelateCarriesStateOnceAndKeepsIMAPRows(t *testing.T) {
	db := openDB(t)
	seed(t, db)
	s := NewStore(db)
	for run := 0; run < 2; run++ { // second run proves idempotence
		res, err := s.Correlate("a")
		if err != nil {
			t.Fatal(err)
		}
		if res.Correlated != 1 || res.Uncorrelated != 2 || res.Ambiguous != 1 {
			t.Fatalf("run %d result = %+v", run, res)
		}
	}
	var body, pgp string
	var fetched, answered int
	if err := db.QueryRow(`SELECT body_text, body_fetched, is_answered, pgp_status FROM messages WHERE id='a1'`).Scan(&body, &fetched, &answered, &pgp); err != nil {
		t.Fatal(err)
	}
	if body != "cached body one" || fetched != 1 || answered != 1 || pgp != "valid" {
		t.Fatalf("a1 = %q %d %d %q", body, fetched, answered, pgp)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE message_id='a1'`).Scan(&n)
	if n != 1 {
		t.Fatalf("attachments carried = %d, want exactly 1 after two runs", n)
	}
	_ = db.QueryRow(`SELECT body_fetched FROM messages WHERE id='a3'`).Scan(&fetched)
	if fetched != 0 {
		t.Fatal("ambiguous duplicate was correlated")
	}
	var imap, total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE folder_id='fi'`).Scan(&imap)
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&total)
	if imap != 4 || total != 7 {
		t.Fatalf("rows changed: imap=%d total=%d", imap, total)
	}
}

func TestSnapshotOwnerOnlyWithManifestAndCounts(t *testing.T) {
	db := openDB(t)
	seed(t, db)
	s := NewStore(db)
	dir := filepath.Join(t.TempDir(), "snaps")
	route := json.RawMessage(`{"route":"gmail-api"}`)
	path, err := s.Snapshot(dir, "a", route, time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dir, path} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode %v is not owner-only", p, st.Mode().Perm())
		}
	}
	data, err := os.ReadFile(path[:len(path)-3] + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.AccountID != "a" || m.Version != 1 || m.Counts.IMAPMessages != 4 || m.Counts.APIMessages != 3 || !bytes.Contains(bytes.Join(bytes.Fields(m.Route), nil), []byte(`"route":"gmail-api"`)) {
		t.Fatalf("manifest = %+v", m)
	}
	snap, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	var n int
	if err := snap.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil || n != 7 {
		t.Fatalf("snapshot messages = %d, %v", n, err)
	}
	// A second snapshot at the same instant replaces the first.
	if _, err := s.Snapshot(dir, "a", route, time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(dir, "../x", route, time.Now()); err == nil {
		t.Fatal("path-like account ID accepted")
	}
}

func TestCountSubmissionsAndRecordRoundTrip(t *testing.T) {
	db := openDB(t)
	mustExec(t, db, `INSERT INTO submission_attempts (id,message_id,account_id,draft_id,fingerprint,outcome,stage,created_at,updated_at)
		VALUES ('s1','m','a','d','f','unknown','data_body',1,1),('s2','m','a','d','f','accepted','done',1,1),('s3','m','a','d','f','not_submitted','queued',1,1)`)
	s := NewStore(db)
	c, err := s.Count("a")
	if err != nil || c.QueuedSubmissions != 2 {
		t.Fatalf("count = %+v, %v", c, err)
	}
	if r, err := s.Get("a"); err != nil || r != nil {
		t.Fatalf("empty get = %v, %v", r, err)
	}
	if err := s.Put(Record{AccountID: "a", State: StateActivated, Route: "gmail-api", Correlated: 3}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Get("a")
	if err != nil || r.State != StateActivated || r.Correlated != 3 || r.StartedAt == 0 {
		t.Fatalf("record = %+v, %v", r, err)
	}
}
