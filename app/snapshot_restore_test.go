package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/snapshotrestore"
)

func TestSnapshotRestoreGuards(t *testing.T) {
	r := newMigrationRig(t)
	a := r.app
	st := apimigrate.NewStore(a.db)

	if _, err := a.PreviewMailAPISnapshotRestore(r.acc.ID); err == nil {
		t.Fatal("preview without a migration record must fail")
	}

	// A snapshot outside the managed directory is never restorable.
	outside := filepath.Join(t.TempDir(), "x.db")
	if err := st.Put(apimigrate.Record{AccountID: r.acc.ID, State: apimigrate.StateSnapshotted, SnapshotPath: outside}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PreviewMailAPISnapshotRestore(r.acc.ID); err == nil {
		t.Fatal("snapshot outside the managed directory must be refused")
	}

	// A real snapshot previews, but staging needs the acknowledgement.
	path, err := st.Snapshot(filepath.Join(a.paths.Data, snapshotRestoreDir), r.acc.ID, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Put(apimigrate.Record{AccountID: r.acc.ID, State: apimigrate.StateSnapshotted, SnapshotPath: path}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RestoreMailAPISnapshot(r.acc.ID, false); err == nil {
		t.Fatal("restore without acknowledgement must fail")
	}
	if snapshotrestore.HasPending(a.paths.Data) {
		t.Fatal("refused restore must not stage an intent")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestUndoNeedsARestoredResult(t *testing.T) {
	r := newMigrationRig(t)
	if _, err := r.app.PreviewUndoSnapshotRestore(); err == nil {
		t.Fatal("undo with no restore must fail")
	}
}
