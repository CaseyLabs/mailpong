package snapshotrestore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/crypto"
	"github.com/caseylabs/mailven/internal/database"
)

type rig struct {
	t        *testing.T
	dir      string
	snapshot string
}

func (r *rig) live() string { return filepath.Join(r.dir, liveDB) }

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func (r *rig) open() *database.DB {
	r.t.Helper()
	db, err := database.Open(r.live())
	if err != nil {
		r.t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		r.t.Fatal(err)
	}
	return db
}

func addAccount(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	exec(t, db, `INSERT INTO accounts (id,name,email,imap_host,smtp_host,username) VALUES (?,?,?,'i','s',?)`, id, id, id+"@x.test", id+"@x.test")
}

// newRig builds a live database with accounts a and b (a has an active route
// and a pending API action), snapshots it, then adds account c and removes b,
// so the live database differs from the snapshot.
func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{t: t, dir: t.TempDir()}
	db := r.open()
	addAccount(t, db.DB, "a")
	addAccount(t, db.DB, "b")
	exec(t, db.DB, `INSERT INTO account_mail_api_routes (account_id,route,state,client_config_id,snapshot,updated_at) VALUES ('a','gmail-api','active','c','{}',1)`)
	exec(t, db.DB, `INSERT INTO api_pending_actions (account_id,provider,op,remote_id,status,created_at) VALUES ('a','gmail','read','r1','pending',1)`)
	path, err := apimigrate.NewStore(db).Snapshot(filepath.Join(r.dir, "migration-snapshots"), "a", json.RawMessage(`{}`), time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	r.snapshot = path
	addAccount(t, db.DB, "c")
	exec(t, db.DB, `DELETE FROM accounts WHERE id='b'`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *rig) stage(undoSource string) *Intent {
	r.t.Helper()
	live := r.open()
	defer live.Close()
	opt := Options{}
	path := r.snapshot
	if undoSource != "" {
		opt, path = Options{Undo: true}, undoSource
	}
	rep, err := Preflight(r.dir, path, live.DB, opt)
	if err != nil {
		r.t.Fatal(err)
	}
	in, err := Stage(r.dir, rep, "t1", 1)
	if err != nil {
		r.t.Fatal(err)
	}
	return in
}

func accountIDs(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id FROM accounts ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return strings.Join(ids, ",")
}

func TestPreflightReportsLossAndDifferences(t *testing.T) {
	r := newRig(t)
	live := r.open()
	defer live.Close()
	rep, err := Preflight(r.dir, r.snapshot, live.DB, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.AccountID != "a" || rep.CreatedAt != 1_700_000_000 || len(rep.Accounts) != 2 {
		t.Fatalf("report = %+v", rep)
	}
	if strings.Join(rep.AddedSince, ",") != "c@x.test" || strings.Join(rep.RemovedSince, ",") != "b@x.test" {
		t.Fatalf("added %v removed %v", rep.AddedSince, rep.RemovedSince)
	}
	if HasPending(r.dir) {
		t.Fatal("preflight must not stage anything")
	}
	if got := accountIDs(t, r.live()); got != "a,c" {
		t.Fatalf("live changed by preflight: %s", got)
	}
}

func TestPreflightRefusals(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(r *rig)
		want   string
	}{
		{"snapshot missing", func(r *rig) { _ = os.Remove(r.snapshot) }, "missing"},
		{"manifest missing", func(r *rig) { _ = os.Remove(strings.TrimSuffix(r.snapshot, ".db") + ".json") }, "manifest"},
		{"manifest version", func(r *rig) {
			p := strings.TrimSuffix(r.snapshot, ".db") + ".json"
			_ = os.WriteFile(p, []byte(`{"version":9,"accountId":"a"}`), 0o600)
		}, "unknown version"},
		{"corrupt", func(r *rig) { _ = os.WriteFile(r.snapshot, []byte("not a database at all, just text"), 0o600) }, "integrity"},
		{"schema newer", func(r *rig) {
			db, _ := sql.Open("sqlite", "file:"+r.snapshot)
			exec(t, db, `INSERT INTO migrations(version) VALUES (99999)`)
			_ = db.Close()
		}, "newer than this"},
		{"account absent", func(r *rig) {
			db, _ := sql.Open("sqlite", "file:"+r.snapshot)
			exec(t, db, `PRAGMA foreign_keys=OFF`)
			exec(t, db, `DELETE FROM accounts WHERE id='a'`)
			_ = db.Close()
		}, "migrated account"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			tc.break_(r)
			_, err := Preflight(r.dir, r.snapshot, nil, Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if HasPending(r.dir) {
				t.Fatal("nothing may be staged")
			}
		})
	}
}

func TestPreflightOlderSchemaMigratesOnCopy(t *testing.T) {
	r := newRig(t)
	db, _ := sql.Open("sqlite", "file:"+r.snapshot)
	exec(t, db, `DROP TABLE api_migrations`)
	exec(t, db, `DELETE FROM migrations WHERE version=61`)
	_ = db.Close()
	if _, err := Preflight(r.dir, r.snapshot, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	db, _ = sql.Open("sqlite", "file:"+r.snapshot+"?mode=ro")
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='api_migrations'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("preflight must migrate a copy only (n=%d err=%v)", n, err)
	}
}

func TestPreflightCredentialKey(t *testing.T) {
	enc := func(t *testing.T, dir string) string {
		e, err := crypto.NewEncryptor(dir)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := e.Encrypt("secret")
		return s
	}
	t.Run("matching key", func(t *testing.T) {
		r := newRig(t)
		setSecret(t, r, enc(t, r.dir))
		if _, err := Preflight(r.dir, r.snapshot, nil, Options{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		r := newRig(t)
		other := t.TempDir()
		setSecret(t, r, enc(t, other))
		_, err := Preflight(r.dir, r.snapshot, nil, Options{})
		if err == nil || !strings.Contains(err.Error(), "key file is missing") {
			t.Fatalf("err = %v", err)
		}
		if _, serr := os.Stat(filepath.Join(r.dir, keyFile)); serr == nil {
			t.Fatal("preflight must not create a key")
		}
	})
	t.Run("mismatched key", func(t *testing.T) {
		r := newRig(t)
		setSecret(t, r, enc(t, t.TempDir()))
		enc(t, r.dir)
		_, err := Preflight(r.dir, r.snapshot, nil, Options{})
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("err = %v", err)
		}
	})
}

func setSecret(t *testing.T, r *rig, v string) {
	t.Helper()
	// The snapshot hash is taken after this, so editing it is fine.
	db, _ := sql.Open("sqlite", "file:"+r.snapshot)
	exec(t, db, `UPDATE accounts SET encrypted_password=? WHERE id='a'`, v)
	_ = db.Close()
}

// finish opens the restored database and runs the post rules.
func (r *rig) finish(credOK func(string) bool) (*Result, *database.DB) {
	r.t.Helper()
	db := r.open()
	res, err := FinishPost(db, r.dir, credOK)
	if err != nil {
		r.t.Fatal(err)
	}
	return res, db
}

func TestRestoreReplacesDatabaseAndAppliesRules(t *testing.T) {
	r := newRig(t)
	if err := os.MkdirAll(filepath.Join(r.dir, "search", "c"), 0o700); err != nil {
		t.Fatal(err)
	}
	r.stage("")
	if res, err := Apply(r.dir, Hooks{}); err != nil || res != nil {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	res, db := r.finish(func(id string) bool { return id != "b" })
	defer db.Close()
	if got := accountIDs(t, r.live()); got != "a,b" {
		t.Fatalf("restored accounts = %s", got)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM account_mail_api_routes WHERE account_id='a'`).Scan(&state); err != nil || state != "staged" {
		t.Fatalf("route state = %q, %v", state, err)
	}
	var mig, status string
	if err := db.QueryRow(`SELECT state FROM api_migrations WHERE account_id='a'`).Scan(&mig); err != nil || mig != "rolled-back" {
		t.Fatalf("migration = %q, %v", mig, err)
	}
	if err := db.QueryRow(`SELECT status FROM api_pending_actions`).Scan(&status); err != nil || status != "unknown" {
		t.Fatalf("held action = %q, %v (nothing may stay pending)", status, err)
	}
	if res.Status != "restored" || res.HeldActions != 1 || len(res.NeedsReauth) != 1 || res.NeedsReauth[0] != "b@x.test" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.OrphanedAccounts) != 1 || res.OrphanedAccounts[0] != (AccountRef{ID: "c", Email: "c@x.test"}) {
		t.Fatalf("orphans = %v", res.OrphanedAccounts)
	}
	if got := accountIDs(t, res.BackupPath); got != "a,c" {
		t.Fatalf("backup holds %s, want the pre-restore state", got)
	}
	if _, err := os.Stat(filepath.Join(r.dir, "search", "c")); err == nil {
		t.Fatal("search index must be dropped")
	}
	if HasPending(r.dir) {
		t.Fatal("intent must be removed")
	}
	if stored, _ := ReadResult(r.dir); stored == nil || stored.ID != "t1" {
		t.Fatalf("stored result = %+v", stored)
	}
}

func TestRestoreCrashAtEveryBoundaryRecovers(t *testing.T) {
	points := []string{"backing-up:start", "backing-up:mid", "backing-up:end", "staging:start", "staging:mid",
		"staging:end", "swapping:start", "swapping:end"}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			r := newRig(t)
			r.stage("")
			boom := errors.New("boom")
			_, err := Apply(r.dir, Hooks{Fault: func(p string) error {
				if p == point {
					return boom
				}
				return nil
			}})
			if !errors.Is(err, boom) {
				t.Fatalf("expected the injected fault, got %v", err)
			}
			// The live database is whole: old content before the swap, new after.
			got := accountIDs(t, r.live())
			if got != "a,c" && got != "a,b" {
				t.Fatalf("live database is half replaced: %s", got)
			}
			if !strings.HasPrefix(point, "swapping") && got != "a,c" {
				t.Fatalf("live changed before the swap at %s: %s", point, got)
			}
			if res, err := Apply(r.dir, Hooks{}); err != nil || res != nil {
				t.Fatalf("resume = %+v, %v", res, err)
			}
			res, db := r.finish(nil)
			defer db.Close()
			if got := accountIDs(t, r.live()); got != "a,b" || res.Status != "restored" {
				t.Fatalf("after resume: %s %+v", got, res)
			}
			if accountIDs(t, res.BackupPath) != "a,c" {
				t.Fatal("backup lost the pre-restore state")
			}
		})
	}
}

func TestRestoreStaleWALIsNotReplayed(t *testing.T) {
	r := newRig(t)
	r.stage("")
	_, err := Apply(r.dir, Hooks{Fault: func(p string) error {
		if p == "staging:end" {
			return errors.New("stop")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("expected the stop")
	}
	// Plant a stale WAL and shm as a crashed writer would leave.
	for _, s := range []string{"-wal", "-shm"} {
		if err := os.WriteFile(r.live()+s, []byte(strings.Repeat("stale", 1000)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if res, err := Apply(r.dir, Hooks{}); err != nil || res != nil {
		t.Fatalf("resume = %+v, %v", res, err)
	}
	for _, s := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(r.live() + s); err == nil {
			t.Fatalf("%s survived the swap", s)
		}
	}
	_, db := r.finish(nil)
	defer db.Close()
	if err := integrityCheck(db.DB); err != nil {
		t.Fatal(err)
	}
	if got := accountIDs(t, r.live()); got != "a,b" {
		t.Fatalf("restored accounts = %s", got)
	}
}

func TestRestoreAbortsWhenDatabaseInUse(t *testing.T) {
	r := newRig(t)
	r.stage("")
	other := r.open() // a second process stand-in holding a read snapshot
	defer other.Close()
	tx, err := other.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(r.dir, Hooks{})
	if err != nil || res == nil || res.Status != "failed" || res.Phase != PhaseBackingUp {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	if !strings.Contains(res.Error, ErrExclusiveAccess.Error()) {
		t.Fatalf("error = %s", res.Error)
	}
	if HasPending(r.dir) {
		t.Fatal("intent must be cleared after an abort")
	}
	_ = tx.Rollback()
	_ = other.Close()
	if got := accountIDs(t, r.live()); got != "a,c" {
		t.Fatalf("live database changed: %s", got)
	}
}

func TestRestoreAbortOnHashMismatchKeepsBackup(t *testing.T) {
	r := newRig(t)
	in := r.stage("")
	if err := os.WriteFile(r.snapshot, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(r.dir, Hooks{})
	if err != nil || res == nil || res.Status != "failed" || res.Phase != PhaseStaging {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	if res.BackupPath != in.BackupPath {
		t.Fatalf("backup = %q, want it retained", res.BackupPath)
	}
	if _, err := os.Stat(filepath.Join(r.dir, restoringDB)); err == nil {
		t.Fatal("staged file must be removed on abort")
	}
	if got := accountIDs(t, r.live()); got != "a,c" {
		t.Fatalf("live database changed: %s", got)
	}
}

func TestRestoreAbortWhenBackupCannotBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	r := newRig(t)
	r.stage("")
	dir := filepath.Join(r.dir, backupDir)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	res, err := Apply(r.dir, Hooks{})
	if err != nil || res == nil || res.Status != "failed" || res.Phase != PhaseBackingUp {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	if got := accountIDs(t, r.live()); got != "a,c" {
		t.Fatalf("live database changed: %s", got)
	}
}

func TestRestoreRefusesWALWithoutDatabase(t *testing.T) {
	r := newRig(t)
	r.stage("")
	if err := os.Remove(r.live()); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(r.live() + "-wal")
	if err := os.WriteFile(r.live()+"-wal", []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(r.dir, Hooks{})
	if err != nil || res == nil || res.Status != "failed" || !strings.Contains(res.Error, "write-ahead log") {
		t.Fatalf("apply = %+v, %v", res, err)
	}
}

func TestRestoreCorruptIntentIsSetAside(t *testing.T) {
	r := newRig(t)
	if err := os.WriteFile(filepath.Join(r.dir, intentFile), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(r.dir, Hooks{})
	if err != nil || res == nil || res.Status != "intent-discarded" {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	if HasPending(r.dir) {
		t.Fatal("corrupt intent must be moved aside")
	}
	if got := accountIDs(t, r.live()); got != "a,c" {
		t.Fatalf("live database changed: %s", got)
	}
}

func TestUndoRestoreThroughPreRestoreBackup(t *testing.T) {
	r := newRig(t)
	r.stage("")
	if _, err := Apply(r.dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	res, db := r.finish(nil)
	_ = db.Close()
	if err := DismissResult(r.dir); err != nil {
		t.Fatal(err)
	}
	r.stage(res.BackupPath)
	if res2, err := Apply(r.dir, Hooks{}); err != nil || res2 != nil {
		t.Fatalf("undo apply = %+v, %v", res2, err)
	}
	res2, db2 := r.finish(nil)
	defer db2.Close()
	if got := accountIDs(t, r.live()); got != "a,c" || !res2.Undo {
		t.Fatalf("after undo: %s %+v", got, res2)
	}
}

func TestStageRefusesSecondIntent(t *testing.T) {
	r := newRig(t)
	r.stage("")
	live := r.open()
	defer live.Close()
	rep, _ := Preflight(r.dir, r.snapshot, live.DB, Options{})
	if _, err := Stage(r.dir, rep, "t2", 2); err == nil {
		t.Fatal("expected refusal")
	}
}

// A crash after the post-restore commit but before the intent is removed reruns
// the rules; the rerun must report the same outcome.
func TestFinishPostRerunAfterCrashIsStable(t *testing.T) {
	r := newRig(t)
	r.stage("")
	if _, err := Apply(r.dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	intent, err := os.ReadFile(filepath.Join(r.dir, intentFile))
	if err != nil {
		t.Fatal(err)
	}
	first, db := r.finish(func(id string) bool { return id != "b" })
	defer db.Close()
	if err := os.WriteFile(filepath.Join(r.dir, intentFile), intent, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := FinishPost(db, r.dir, func(id string) bool { return id != "b" })
	if err != nil || second == nil {
		t.Fatalf("rerun = %+v, %v", second, err)
	}
	first.FinishedAt, second.FinishedAt = 0, 0
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("rerun result differs:\n%+v\n%+v", first, second)
	}
	if HasPending(r.dir) {
		t.Fatal("intent must be removed after rerun")
	}
}
