package snapshotrestore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/crypto"
	"github.com/caseylabs/mailven/internal/database"
)

// keyFile is the encryptor key beside the database (internal/crypto).
const keyFile = "device.key"

// Report is what preflight proved about a snapshot; the consent dialog shows it.
type Report struct {
	SnapshotPath string         `json:"snapshotPath"`
	SHA256       string         `json:"sha256"`
	CreatedAt    int64          `json:"createdAt"`
	AccountID    string         `json:"accountId"`
	Accounts     []AccountCount `json:"accounts"`
	// AddedSince are accounts in the live database that the snapshot lacks;
	// RemovedSince are accounts the snapshot has and the live database lacks.
	AddedSince   []string `json:"addedSince"`
	RemovedSince []string `json:"removedSince"`
	BackupDir    string   `json:"backupDir"`
	Undo         bool     `json:"undo"`
}

// Options adjusts preflight. Undo restores a pre-restore backup, which has no
// manifest.
type Options struct{ Undo bool }

// Preflight validates a snapshot without changing it or the live database:
// files present, hash, integrity, forward migration and credential key, all on
// a private copy. live may be nil (no comparison). It never stages anything.
func Preflight(dataDir, snapshotPath string, live *sql.DB, opt Options) (*Report, error) {
	info, err := os.Stat(snapshotPath)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("snapshot is missing: %s", snapshotPath)
	}
	rep := &Report{SnapshotPath: snapshotPath, BackupDir: filepath.Join(dataDir, backupDir), Undo: opt.Undo}
	if !opt.Undo {
		var man apimigrate.Manifest
		data, err := os.ReadFile(strings.TrimSuffix(snapshotPath, ".db") + ".json")
		if err != nil {
			return nil, fmt.Errorf("snapshot manifest is missing: %w", err)
		}
		if err := json.Unmarshal(data, &man); err != nil || man.Version != 1 || man.AccountID == "" {
			return nil, errors.New("snapshot manifest is unreadable or has an unknown version")
		}
		if man.Database != "" && man.Database != filepath.Base(snapshotPath) {
			return nil, errors.New("snapshot manifest does not describe this database")
		}
		rep.AccountID, rep.CreatedAt = man.AccountID, man.CreatedAt
	} else {
		rep.CreatedAt = info.ModTime().Unix()
	}
	if rep.SHA256, err = fileSHA256(snapshotPath); err != nil {
		return nil, fmt.Errorf("snapshot is unreadable: %w", err)
	}

	tmp, err := os.MkdirTemp(dataDir, "restore-preflight-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a scratch copy: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := os.Chmod(tmp, 0o700); err != nil {
		return nil, err
	}
	cp := filepath.Join(tmp, "copy.db")
	if err := copyFile(snapshotPath, cp); err != nil {
		return nil, fmt.Errorf("cannot copy the snapshot (disk space or permissions): %w", err)
	}
	db, err := database.Open(cp)
	if err != nil {
		return nil, fmt.Errorf("integrity check failed: snapshot cannot be opened: %w", err)
	}
	defer db.Close()
	if err := integrityCheck(db.DB); err != nil {
		return nil, fmt.Errorf("integrity check failed: %w", err)
	}
	if err := db.Migrate(); err != nil {
		var tooNew *database.ErrSchemaTooNew
		if errors.As(err, &tooNew) {
			return nil, err
		}
		return nil, fmt.Errorf("snapshot cannot be migrated to the current schema: %w", err)
	}
	if err := probeCredentials(db.DB, dataDir); err != nil {
		return nil, err
	}
	if rep.Accounts, err = accountCounts(db.DB); err != nil {
		return nil, err
	}
	if !opt.Undo && !hasAccount(rep.Accounts, rep.AccountID) {
		return nil, errors.New("snapshot does not contain the migrated account")
	}
	if live != nil {
		liveAcc, err := accountEmails(live)
		if err != nil {
			return nil, err
		}
		snapAcc := map[string]string{}
		for _, a := range rep.Accounts {
			snapAcc[a.ID] = a.Email
		}
		for id, email := range liveAcc {
			if _, ok := snapAcc[id]; !ok {
				rep.AddedSince = append(rep.AddedSince, email)
			}
		}
		for id, email := range snapAcc {
			if _, ok := liveAcc[id]; !ok {
				rep.RemovedSince = append(rep.RemovedSince, email)
			}
		}
		sort.Strings(rep.AddedSince)
		sort.Strings(rep.RemovedSince)
	}
	return rep, nil
}

func hasAccount(accs []AccountCount, id string) bool {
	for _, a := range accs {
		if a.ID == id {
			return true
		}
	}
	return false
}

func integrityCheck(db *sql.DB) error {
	var res string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return errors.New(res)
	}
	return nil
}

// probeCredentials proves the encryptor key decrypts the snapshot's fallback
// secrets. The key file is never created here: a missing key means every
// restored fallback secret would be unreadable.
func probeCredentials(db *sql.DB, dataDir string) error {
	var samples []string
	for _, col := range []string{"encrypted_password", "encrypted_access_token", "encrypted_refresh_token"} {
		var v sql.NullString
		err := db.QueryRow(`SELECT ` + col + ` FROM accounts WHERE ` + col + ` IS NOT NULL AND ` + col + ` <> '' LIMIT 1`).Scan(&v)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("credential check failed: %w", err)
		}
		samples = append(samples, v.String)
	}
	if len(samples) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dataDir, keyFile)); err != nil {
		return errors.New("credential check failed: the encryption key file is missing, so stored secrets in the snapshot would be unreadable")
	}
	enc, err := crypto.NewEncryptor(dataDir)
	if err != nil {
		return fmt.Errorf("credential check failed: %w", err)
	}
	for _, s := range samples {
		if _, err := enc.Decrypt(s); err != nil {
			return errors.New("credential check failed: the encryption key does not match the snapshot's stored secrets")
		}
	}
	return nil
}

func accountCounts(db *sql.DB) ([]AccountCount, error) {
	rows, err := db.Query(`SELECT a.id, a.email,
		(SELECT COUNT(*) FROM messages m WHERE m.account_id=a.id),
		(SELECT COUNT(*) FROM drafts d WHERE d.account_id=a.id)
		FROM accounts a ORDER BY a.order_index, a.email`)
	if err != nil {
		return nil, fmt.Errorf("cannot read snapshot accounts: %w", err)
	}
	defer rows.Close()
	var out []AccountCount
	for rows.Next() {
		var c AccountCount
		if err := rows.Scan(&c.ID, &c.Email, &c.Messages, &c.Drafts); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func accountEmails(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT id, email FROM accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, err
		}
		out[id] = email
	}
	return out, rows.Err()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Stage records consent and writes the pending-restore intent. It does not
// touch the live database; the caller then quits so Apply runs at startup.
func Stage(dataDir string, rep *Report, backupID string, now int64) (*Intent, error) {
	if HasPending(dataDir) {
		return nil, errors.New("a restore is already staged")
	}
	in := &Intent{
		Version: intentVer, ID: backupID, Phase: PhaseStaged,
		SnapshotPath: rep.SnapshotPath, SnapshotSHA256: rep.SHA256, SnapshotCreatedAt: rep.CreatedAt,
		AccountID: rep.AccountID, Undo: rep.Undo, StagedAt: now,
		BackupPath: filepath.Join(dataDir, backupDir, "pre-restore-"+backupID+".db"),
		Consent:    Consent{At: now, Accounts: rep.Accounts, AddedSince: rep.AddedSince, RemovedSince: rep.RemovedSince},
	}
	if err := os.MkdirAll(filepath.Join(dataDir, backupDir), 0o700); err != nil {
		return nil, err
	}
	if err := saveIntent(dataDir, in); err != nil {
		return nil, fmt.Errorf("cannot stage restore: %w", err)
	}
	return in, nil
}
