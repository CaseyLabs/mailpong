package snapshotrestore

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/caseylabs/mailven/internal/database"
)

var phaseOrder = map[string]int{PhaseStaged: 0, PhaseBackingUp: 1, PhaseStaging: 2, PhaseSwapping: 3, PhasePost: 4}

// Apply runs a staged restore. Call it before the database is opened, with the
// single-instance lock held. It returns nil when there is no intent, or when
// the swap finished and FinishPost must run after migrations. A non-nil Result
// means the restore was aborted (live database untouched) or the intent was
// unusable and set aside; the result file is written. An error from a Hooks
// fault simulates a crash: the intent is left as it was.
func Apply(dataDir string, h Hooks) (*Result, error) {
	in, err := readIntent(dataDir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		aside := filepath.Join(dataDir, fmt.Sprintf("%s.corrupt-%d", intentFile, time.Now().Unix()))
		_ = os.Rename(filepath.Join(dataDir, intentFile), aside)
		res := Result{Status: "intent-discarded", Error: err.Error(),
			Notice: "A pending database restore could not be read and was set aside. Your database was not changed."}
		_ = writeResult(dataDir, res)
		return &res, nil
	}
	if phaseOrder[in.Phase] >= phaseOrder[PhasePost] {
		return nil, nil
	}
	for _, phase := range []string{PhaseBackingUp, PhaseStaging, PhaseSwapping} {
		if phaseOrder[in.Phase] > phaseOrder[phase] {
			continue
		}
		in.Phase = phase
		if err := saveIntent(dataDir, in); err != nil {
			return abort(dataDir, in, phase, err), nil
		}
		if err := h.at(phase + ":start"); err != nil {
			return nil, err
		}
		var stepErr error
		switch phase {
		case PhaseBackingUp:
			stepErr = backup(dataDir, in, h)
		case PhaseStaging:
			stepErr = stage(dataDir, in, h)
		case PhaseSwapping:
			stepErr = swap(dataDir)
		}
		if errors.Is(stepErr, errCrash) {
			return nil, stepErr
		}
		if stepErr != nil {
			return abort(dataDir, in, phase, stepErr), nil
		}
		if err := h.at(phase + ":end"); err != nil {
			return nil, err
		}
	}
	in.Phase = PhasePost
	if err := saveIntent(dataDir, in); err != nil {
		return abort(dataDir, in, PhaseSwapping, err), nil
	}
	return nil, nil
}

// errCrash marks a hook fault so Apply leaves everything as a crash would.
var errCrash = errors.New("simulated crash")

func (h Hooks) crash(point string) error {
	if err := h.at(point); err != nil {
		return fmt.Errorf("%w: %w", errCrash, err)
	}
	return nil
}

// abort ends a restore that failed before the swap completed. The live
// database is untouched: every earlier phase only reads it or writes other
// files, and the swap's final rename is atomic.
func abort(dataDir string, in *Intent, phase string, cause error) *Result {
	_ = os.Remove(filepath.Join(dataDir, restoringDB))
	_ = os.Remove(filepath.Join(dataDir, intentFile))
	res := Result{ID: in.ID, Status: "failed", Phase: phase, Error: cause.Error(), Undo: in.Undo,
		SnapshotCreatedAt: in.SnapshotCreatedAt,
		Notice:            "The database restore failed during " + phase + ". Your database was not changed."}
	if _, err := os.Stat(in.BackupPath); err == nil {
		res.BackupPath = in.BackupPath
	}
	_ = writeResult(dataDir, res)
	return &res
}

// openExclusive opens the live database and proves no other process holds it.
// WAL readers keep the shared-memory lock, so an exclusive lock plus a
// TRUNCATE checkpoint that is not busy shows nobody else has it open.
func openExclusive(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(0)&_pragma=locking_mode(EXCLUSIVE)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var busy, logFrames, ckpt int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &ckpt); err != nil || busy != 0 {
		_ = db.Close()
		return nil, ErrExclusiveAccess
	}
	if _, err := db.Exec(`SELECT COUNT(*) FROM sqlite_master`); err != nil {
		_ = db.Close()
		return nil, ErrExclusiveAccess
	}
	return db, nil
}

func backup(dataDir string, in *Intent, h Hooks) error {
	live := filepath.Join(dataDir, liveDB)
	if _, err := os.Stat(live); errors.Is(err, os.ErrNotExist) {
		if _, werr := os.Stat(live + "-wal"); werr == nil {
			return errors.New("a write-ahead log exists without its database; refusing to continue")
		}
		in.BackupPath = ""
		return nil
	}
	if _, err := os.Stat(in.BackupPath); err == nil {
		return nil // an earlier attempt completed the backup; never overwrite
	}
	db, err := openExclusive(live)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := os.MkdirAll(filepath.Dir(in.BackupPath), 0o700); err != nil {
		return err
	}
	partial := in.BackupPath + ".partial"
	_ = os.Remove(partial)
	if _, err := db.Exec(`VACUUM INTO ?`, partial); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("cannot write the pre-restore backup: %w", err)
	}
	if err := h.crash("backing-up:mid"); err != nil {
		return err
	}
	if err := os.Chmod(partial, 0o600); err != nil {
		return err
	}
	if err := syncFile(partial); err != nil {
		return err
	}
	if err := os.Rename(partial, in.BackupPath); err != nil {
		return err
	}
	return syncDir(filepath.Dir(in.BackupPath))
}

func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func stage(dataDir string, in *Intent, h Hooks) error {
	if sum, err := fileSHA256(in.SnapshotPath); err != nil || sum != in.SnapshotSHA256 {
		return errors.New("the snapshot changed or disappeared since it was checked")
	}
	restoring := filepath.Join(dataDir, restoringDB)
	for _, p := range []string{restoring, restoring + "-wal", restoring + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	src, err := sql.Open("sqlite", "file:"+in.SnapshotPath+"?mode=ro&_pragma=busy_timeout(0)")
	if err != nil {
		return err
	}
	defer src.Close()
	if _, err := src.Exec(`VACUUM INTO ?`, restoring); err != nil {
		_ = os.Remove(restoring)
		return fmt.Errorf("cannot copy the snapshot (disk space or permissions): %w", err)
	}
	if err := h.crash("staging:mid"); err != nil {
		return err
	}
	if err := os.Chmod(restoring, 0o600); err != nil {
		return err
	}
	chk, err := sql.Open("sqlite", "file:"+restoring+"?mode=ro")
	if err != nil {
		return err
	}
	defer chk.Close()
	if err := integrityCheck(chk); err != nil {
		_ = os.Remove(restoring)
		return fmt.Errorf("integrity check failed: %w", err)
	}
	return syncFile(restoring)
}

// swap removes the live WAL and shared memory, then renames the staged
// database over the live one. The removal precedes the rename so stale frames
// can never replay onto the restored file. With the staged file already gone
// the rename happened in an earlier run.
func swap(dataDir string) error {
	live := filepath.Join(dataDir, liveDB)
	restoring := filepath.Join(dataDir, restoringDB)
	if _, err := os.Stat(restoring); errors.Is(err, os.ErrNotExist) {
		if _, lerr := os.Stat(live); lerr != nil {
			return errors.New("neither the staged nor the live database exists")
		}
		return removeWAL(live)
	}
	if err := removeWAL(live); err != nil {
		return err
	}
	if err := os.Rename(restoring, live); err != nil {
		return err
	}
	return syncDir(dataDir)
}

func removeWAL(live string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(live + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// heldNote marks API actions held by a restore; they are never replayed.
const heldNote = "held by database restore; review before any re-send"

// Pending returns the intent awaiting post-restore rules, or nil.
func Pending(dataDir string) *Intent {
	in, err := readIntent(dataDir)
	if err != nil || in.Phase != PhasePost {
		return nil
	}
	return in
}

// FinishPost applies the post-restore rules to the opened, migrated database,
// writes the result and removes the intent. credOK reports whether an
// account's credentials are present and usable (nil means all are).
func FinishPost(db *database.DB, dataDir string, credOK func(accountID string) bool) (*Result, error) {
	in := Pending(dataDir)
	if in == nil {
		return nil, nil
	}
	res := Result{ID: in.ID, Status: "restored", Phase: PhasePost, Undo: in.Undo, BackupPath: in.BackupPath,
		SnapshotCreatedAt: in.SnapshotCreatedAt, NeedsReauth: []string{}, OrphanedAccounts: []AccountRef{}}
	if _, err := os.Stat(in.BackupPath); err != nil {
		res.BackupPath = ""
	}
	now := time.Now().Unix()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if in.AccountID != "" && !in.Undo {
		note := "database restored from a snapshot (" + in.ID + ")"
		if _, err := tx.Exec(`UPDATE account_mail_api_routes SET state='staged', updated_at=? WHERE account_id=?`, now, in.AccountID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO api_migrations(account_id,state,route,snapshot_path,error,started_at,updated_at)
			VALUES(?, 'rolled-back', COALESCE((SELECT route FROM account_mail_api_routes WHERE account_id=?),''), ?, ?, ?, ?)
			ON CONFLICT(account_id) DO UPDATE SET state='rolled-back', error=excluded.error, updated_at=excluded.updated_at`,
			in.AccountID, in.AccountID, in.SnapshotPath, note, now, now); err != nil {
			return nil, err
		}
	}
	// Held actions are counted by their marker, so a rerun after a crash
	// between commit and intent removal reports the same number.
	if _, err := tx.Exec(`UPDATE api_pending_actions SET status='unknown', error=?
		WHERE status IN ('pending','in-flight')`, heldNote); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM api_pending_actions WHERE status='unknown' AND error=?`, heldNote).Scan(&res.HeldActions); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM submission_attempts
		WHERE outcome='unknown' OR (outcome='not_submitted' AND stage='queued')`).Scan(&res.QueuedSubmissions); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM fts_index_status`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if credOK != nil {
		emails, err := accountEmails(db.DB)
		if err != nil {
			return nil, err
		}
		for id, email := range emails {
			if !credOK(id) {
				res.NeedsReauth = append(res.NeedsReauth, email)
			}
		}
		sort.Strings(res.NeedsReauth)
	}
	res.OrphanedAccounts = orphanedAccounts(db.DB, res.BackupPath)
	rebuildSearch(dataDir)
	res.Notice = "Database restored. Server-side changes were not undone and will re-sync."
	if err := writeResult(dataDir, res); err != nil {
		return nil, err
	}
	if err := os.Remove(filepath.Join(dataDir, intentFile)); err != nil {
		return nil, err
	}
	return &res, nil
}

// orphanedAccounts lists accounts that exist only in the pre-restore backup.
func orphanedAccounts(db *sql.DB, backupPath string) []AccountRef {
	out := []AccountRef{}
	if backupPath == "" {
		return out
	}
	bk, err := sql.Open("sqlite", "file:"+backupPath+"?mode=ro")
	if err != nil {
		return out
	}
	defer bk.Close()
	then, err := accountEmails(bk)
	if err != nil {
		return out
	}
	now, err := accountEmails(db)
	if err != nil {
		return out
	}
	for id, email := range then {
		if _, ok := now[id]; !ok {
			out = append(out, AccountRef{ID: id, Email: email})
		}
	}
	return out
}

// rebuildSearch drops per-account search indexes; they are derived data and
// rebuild from the restored messages.
func rebuildSearch(dataDir string) {
	dir := filepath.Join(dataDir, "search")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
