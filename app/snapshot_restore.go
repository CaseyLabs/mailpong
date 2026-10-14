package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/snapshotrestore"
)

// snapshotRestoreDir is where API migration snapshots live; only files under it
// can be restored.
const snapshotRestoreDir = "migration-snapshots"

// PreviewMailAPISnapshotRestore proves the account's pre-migration snapshot can
// be restored and returns what the consent dialog must show. It changes nothing.
func (a *App) PreviewMailAPISnapshotRestore(accountID string) (*snapshotrestore.Report, error) {
	path, err := a.restoreSnapshotPath(accountID)
	if err != nil {
		return nil, err
	}
	return snapshotrestore.Preflight(a.paths.Data, path, a.db.DB, snapshotrestore.Options{})
}

// RestoreMailAPISnapshot stages a whole-database restore to the account's
// pre-migration snapshot and closes Mailven; the swap happens at next start.
// acknowledged must be true: the caller has shown the loss semantics.
func (a *App) RestoreMailAPISnapshot(accountID string, acknowledged bool) (*snapshotrestore.Report, error) {
	path, err := a.restoreSnapshotPath(accountID)
	if err != nil {
		return nil, err
	}
	return a.stageRestore(path, acknowledged, snapshotrestore.Options{})
}

// PreviewUndoSnapshotRestore checks the pre-restore backup of the last restore.
func (a *App) PreviewUndoSnapshotRestore() (*snapshotrestore.Report, error) {
	path, err := a.undoBackupPath()
	if err != nil {
		return nil, err
	}
	return snapshotrestore.Preflight(a.paths.Data, path, a.db.DB, snapshotrestore.Options{Undo: true})
}

// UndoSnapshotRestore restores the pre-restore backup through the same flow.
func (a *App) UndoSnapshotRestore(acknowledged bool) (*snapshotrestore.Report, error) {
	path, err := a.undoBackupPath()
	if err != nil {
		return nil, err
	}
	return a.stageRestore(path, acknowledged, snapshotrestore.Options{Undo: true})
}

// GetSnapshotRestoreResult returns the outcome of the last restore attempt, or nil.
func (a *App) GetSnapshotRestoreResult() (*snapshotrestore.Result, error) {
	res, err := snapshotrestore.ReadResult(a.paths.Data)
	if err != nil {
		return nil, nil //nolint:nilerr // no readable result means nothing to show
	}
	return res, nil
}

// DismissSnapshotRestoreResult hides the result banner.
func (a *App) DismissSnapshotRestoreResult() error {
	return snapshotrestore.DismissResult(a.paths.Data)
}

// CleanRestoreOrphanCredentials deletes stored credentials of accounts that
// existed only in the pre-restore database.
func (a *App) CleanRestoreOrphanCredentials() error {
	res, err := snapshotrestore.ReadResult(a.paths.Data)
	if err != nil || res == nil {
		return err
	}
	for _, o := range res.OrphanedAccounts {
		if a.accountStore != nil {
			if acc, _ := a.accountStore.Get(o.ID); acc != nil {
				continue // account exists again; never touch its credentials
			}
		}
		if err := a.credStore.DeleteAllCredentials(o.ID); err != nil {
			return err
		}
	}
	return snapshotrestore.ClearOrphans(a.paths.Data)
}

func (a *App) stageRestore(path string, acknowledged bool, opt snapshotrestore.Options) (*snapshotrestore.Report, error) {
	if !acknowledged {
		return nil, errors.New("restore needs explicit acknowledgement")
	}
	if n := a.GetConnectedComposers(); n > 0 {
		return nil, fmt.Errorf("close %d open composer window(s) first", n)
	}
	rs := &a.apiRoutes
	rs.transition.Lock()
	defer rs.transition.Unlock()
	rep, err := snapshotrestore.Preflight(a.paths.Data, path, a.db.DB, opt)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := snapshotrestore.Stage(a.paths.Data, rep, now.UTC().Format("20060102T150405"), now.Unix()); err != nil {
		return nil, err
	}
	go a.InitiateShutdown()
	return rep, nil
}

// restoreSnapshotPath resolves the migration snapshot of an account and
// requires it to sit under the managed snapshot directory.
func (a *App) restoreSnapshotPath(accountID string) (string, error) {
	rec, err := apimigrate.NewStore(a.db).Get(accountID)
	if err != nil {
		return "", err
	}
	if rec == nil || rec.SnapshotPath == "" {
		return "", errors.New("account has no migration snapshot")
	}
	root := filepath.Join(a.paths.Data, snapshotRestoreDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(rec.SnapshotPath), root) {
		return "", errors.New("snapshot is outside the managed snapshot directory")
	}
	return filepath.Clean(rec.SnapshotPath), nil
}

func (a *App) undoBackupPath() (string, error) {
	res, err := snapshotrestore.ReadResult(a.paths.Data)
	if err != nil || res == nil || res.Status != "restored" || res.Undo || res.BackupPath == "" {
		return "", errors.New("there is no restore to undo")
	}
	return res.BackupPath, nil
}

// restoreCredentialsOK reports whether an account's stored credentials survived
// a restore: a password for password accounts, tokens for OAuth accounts.
func (a *App) restoreCredentialsOK(accountID string) bool {
	acc, err := account.NewStore(a.db).Get(accountID)
	if err != nil || acc == nil {
		return false
	}
	if acc.AuthType == account.AuthOAuth2 {
		return a.credStore.HasOAuthTokens(accountID)
	}
	pw, err := a.credStore.GetPassword(accountID)
	return err == nil && pw != ""
}
