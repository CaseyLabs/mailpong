// Package snapshotrestore replaces the whole local database with a migration
// snapshot (docs/API_SNAPSHOT_RESTORE_DESIGN.md). The swap runs offline at the
// next startup, before the database is opened, driven by a pending-restore
// intent that records each phase so a crash resumes or aborts safely.
package snapshotrestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	intentFile  = "pending-restore.json"
	resultFile  = "restore-result.json"
	backupDir   = "restore-backups"
	liveDB      = "mailven.db"
	intentVer   = 1
	restoringDB = liveDB + ".restoring"
)

// Phases of an intent. Each is written before it starts and is idempotent.
const (
	PhaseStaged    = "staged"
	PhaseBackingUp = "backing-up"
	PhaseStaging   = "staging"
	PhaseSwapping  = "swapping"
	PhasePost      = "post"
)

// AccountCount is one account's local row counts shown at consent.
type AccountCount struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Messages int    `json:"messages"`
	Drafts   int    `json:"drafts"`
}

// AccountRef names an account.
type AccountRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Consent records what the user was shown when confirming.
type Consent struct {
	At           int64          `json:"at"`
	Accounts     []AccountCount `json:"accounts"`
	AddedSince   []string       `json:"addedSince"`
	RemovedSince []string       `json:"removedSince"`
}

// Intent is the commit marker of one restore.
type Intent struct {
	Version           int     `json:"version"`
	ID                string  `json:"id"`
	Phase             string  `json:"phase"`
	SnapshotPath      string  `json:"snapshotPath"`
	SnapshotSHA256    string  `json:"snapshotSha256"`
	SnapshotCreatedAt int64   `json:"snapshotCreatedAt"`
	AccountID         string  `json:"accountId"`
	BackupPath        string  `json:"backupPath"`
	Undo              bool    `json:"undo"`
	Consent           Consent `json:"consent"`
	StagedAt          int64   `json:"stagedAt"`
}

// Result is the outcome shown to the user after startup.
type Result struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // restored | failed | intent-discarded
	Phase      string `json:"phase"`
	Error      string `json:"error"`
	Undo       bool   `json:"undo"`
	BackupPath string `json:"backupPath"`
	// SnapshotCreatedAt is the time the database was restored to.
	SnapshotCreatedAt int64 `json:"snapshotCreatedAt"`
	FinishedAt        int64 `json:"finishedAt"`
	// HeldActions counts API actions held for review; nothing is replayed.
	HeldActions int `json:"heldActions"`
	// QueuedSubmissions counts submission attempts that need review; none is sent.
	QueuedSubmissions int `json:"queuedSubmissions"`
	// NeedsReauth lists accounts whose credentials are missing or invalid.
	NeedsReauth []string `json:"needsReauth"`
	// OrphanedAccounts exist only in the pre-restore backup; their credentials
	// may remain in the keyring until cleaned up.
	OrphanedAccounts []AccountRef `json:"orphanedAccounts"`
	Notice           string       `json:"notice"`
}

// ErrExclusiveAccess is returned when another process holds the database.
var ErrExclusiveAccess = errors.New("database is in use by another process")

// Hooks lets tests inject faults at named points (<phase>:start, <phase>:end,
// copy:mid). A returned error aborts as a crash would: nothing is cleaned up.
type Hooks struct{ Fault func(point string) error }

func (h Hooks) at(point string) error {
	if h.Fault == nil {
		return nil
	}
	return h.Fault(point)
}

// HasPending reports whether a restore intent exists.
func HasPending(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, intentFile))
	return err == nil
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// syncDir flushes a directory entry change; platforms that cannot are skipped.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	_ = d.Sync()
	return nil
}

func readIntent(dataDir string) (*Intent, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, intentFile))
	if err != nil {
		return nil, err
	}
	var in Intent
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("unreadable restore intent: %w", err)
	}
	if in.Version != intentVer || in.ID == "" || in.SnapshotPath == "" {
		return nil, errors.New("restore intent is incomplete")
	}
	return &in, nil
}

func saveIntent(dataDir string, in *Intent) error {
	return writeJSONAtomic(filepath.Join(dataDir, intentFile), in)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeResult(dataDir string, r Result) error {
	r.FinishedAt = time.Now().Unix()
	return writeJSONAtomic(filepath.Join(dataDir, resultFile), r)
}

// ReadResult returns the last restore result, or nil.
func ReadResult(dataDir string) (*Result, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, resultFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r Result
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// DismissResult removes the stored result once the user has seen it.
func DismissResult(dataDir string) error {
	err := os.Remove(filepath.Join(dataDir, resultFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ClearOrphans empties the stored orphan list once their credentials are cleaned up.
func ClearOrphans(dataDir string) error {
	r, err := ReadResult(dataDir)
	if err != nil || r == nil {
		return err
	}
	r.OrphanedAccounts = []AccountRef{}
	return writeJSONAtomic(filepath.Join(dataDir, resultFile), r)
}
