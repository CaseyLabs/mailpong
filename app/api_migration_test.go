package app

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/platform"
)

// newMigrationRig is a route rig whose IMAP message matches the API message by
// Message-ID and date and has a cached body to carry over.
func newMigrationRig(t *testing.T) *routeRig {
	t.Helper()
	r := newRouteRig(t)
	r.app.db = r.db
	r.app.paths = &platform.Paths{Data: t.TempDir()}
	date := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	if _, err := r.db.Exec(`UPDATE messages SET message_id='<m1@example.com>', date=?, body_text='cached imap body', body_fetched=1, is_answered=1 WHERE id='imap-1'`, date); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *routeRig) count(t *testing.T, q string) int {
	t.Helper()
	var n int
	if err := r.db.QueryRow(q, r.acc.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrationPlanIsReadOnlyAndReportsBlockers(t *testing.T) {
	r := newMigrationRig(t)
	plan, err := r.app.PlanMailAPIMigration(r.acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blockers) != 0 || plan.Counts.IMAPMessages != 1 || plan.Counts.APIMessages != 0 || plan.RouteState != account.MailRouteStateStaged {
		t.Fatalf("plan = %+v", plan)
	}
	if r.state(t) != account.MailRouteStateStaged || r.count(t, `SELECT COUNT(*) FROM api_migrations WHERE account_id=?`) != 0 {
		t.Fatal("plan changed state")
	}

	queueAPIFlagAction(t, r, message.APIActionUnknown)
	plan, _ = r.app.PlanMailAPIMigration(r.acc.ID)
	if len(plan.Blockers) != 1 || !strings.Contains(plan.Blockers[0], "unknown outcome") {
		t.Fatalf("blockers = %v", plan.Blockers)
	}
	if _, err := r.app.MigrateMailAPIAccount(r.acc.ID); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("Migrate = %v", err)
	}

	r2 := newMigrationRig(t)
	if _, err := r2.db.Exec(`INSERT INTO submission_attempts (id,message_id,account_id,draft_id,fingerprint,outcome,stage,created_at,updated_at)
		VALUES ('s1','m',?, 'd','f','unknown','data_body',1,1)`, r2.acc.ID); err != nil {
		t.Fatal(err)
	}
	if plan, _ = r2.app.PlanMailAPIMigration(r2.acc.ID); len(plan.Blockers) != 1 || !strings.Contains(plan.Blockers[0], "submission") {
		t.Fatalf("submission blockers = %v", plan.Blockers)
	}
}

func TestMigrationIMAPOnlyAccountHasNoPlan(t *testing.T) {
	r := newMigrationRig(t)
	if err := r.app.accountStore.ClearMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.PlanMailAPIMigration(r.acc.ID); err == nil {
		t.Fatal("IMAP-only account has a migration plan")
	}
	if _, err := r.app.RollbackMailAPIMigration(r.acc.ID); err == nil {
		t.Fatal("rollback of an unmigrated account succeeded")
	}
}

func TestMigrationSnapshotCorrelatesAndPreserves(t *testing.T) {
	r := newMigrationRig(t)
	if _, err := r.db.Exec(`INSERT INTO drafts (id, account_id, subject) VALUES ('d1', ?, 'keep me')`, r.acc.ID); err != nil {
		t.Fatal(err)
	}
	rec, err := r.app.MigrateMailAPIAccount(r.acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != apimigrate.StateCompleted || rec.Correlated != 1 || rec.Uncorrelated != 0 {
		t.Fatalf("record = %+v", rec)
	}
	st, err := os.Stat(rec.SnapshotPath)
	if err != nil || st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("snapshot %q: %v %v", rec.SnapshotPath, st, err)
	}
	if r.state(t) != account.MailRouteStateActive {
		t.Fatal("route not active")
	}
	var body string
	var answered int
	if err := r.db.QueryRow(`SELECT m.body_text, m.is_answered FROM messages m JOIN api_message_rows l ON l.message_id=m.id WHERE m.account_id=?`, r.acc.ID).Scan(&body, &answered); err != nil {
		t.Fatal(err)
	}
	if body == "" || answered != 1 { // API body kept where it has one; local-only state carried
		t.Fatalf("carried state = %q %d", body, answered)
	}
	if n := r.count(t, `SELECT COUNT(*) FROM drafts WHERE account_id=?`); n != 1 {
		t.Fatalf("drafts = %d", n)
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "api" {
		t.Fatalf("visible = %v; no duplicate copy may show", got)
	}
	if _, err := r.app.MigrateMailAPIAccount(r.acc.ID); err == nil {
		t.Fatal("second migration of a migrated account succeeded")
	}
}

func TestMigrationResumesAfterInterruptionAtEveryStep(t *testing.T) {
	for _, step := range []string{migrateStepSnapshot, migrateStepActivate, migrateStepCorrelate, migrateStepComplete} {
		t.Run(step, func(t *testing.T) {
			r := newMigrationRig(t)
			r.app.apiRoutes.migrationStep = func(s string) error {
				if s == step {
					return errMigrationInterrupted
				}
				return nil
			}
			if _, err := r.app.MigrateMailAPIAccount(r.acc.ID); !errors.Is(err, errMigrationInterrupted) {
				t.Fatalf("interrupted Migrate = %v", err)
			}
			r.app.apiRoutes.migrationStep = nil
			rec, err := r.app.MigrateMailAPIAccount(r.acc.ID)
			if err != nil {
				t.Fatalf("resume: %v", err)
			}
			if rec.State != apimigrate.StateCompleted || rec.Correlated != 1 {
				t.Fatalf("record = %+v", rec)
			}
			if n := r.count(t, `SELECT COUNT(*) FROM api_message_rows WHERE account_id=?`); n != 1 {
				t.Fatalf("api rows = %d, want 1 (no duplicates)", n)
			}
			if n := r.count(t, `SELECT COUNT(*) FROM messages m WHERE account_id=? AND NOT EXISTS (SELECT 1 FROM api_message_rows l WHERE l.message_id=m.id)`); n != 1 {
				t.Fatalf("imap rows = %d, want 1 (none lost)", n)
			}
			if rec.SnapshotPath == "" {
				t.Fatal("no snapshot recorded")
			}
		})
	}
}

func TestMigrationFailureStaysOnIMAPAndIsRetryable(t *testing.T) {
	r := newMigrationRig(t)
	r.gmail.fail = true
	rec, err := r.app.MigrateMailAPIAccount(r.acc.ID)
	if err == nil || !strings.Contains(err.Error(), "stays on IMAP") {
		t.Fatalf("Migrate = %v", err)
	}
	if rec == nil || rec.SnapshotPath == "" || rec.Error == "" {
		t.Fatalf("record = %+v", rec)
	}
	if r.state(t) != account.MailRouteStateStaged || len(r.visibleSubjects(t)) != 1 || r.visibleSubjects(t)[0] != "imap" {
		t.Fatal("failed migration did not leave the account on IMAP")
	}
	r.gmail.fail = false
	if rec, err = r.app.MigrateMailAPIAccount(r.acc.ID); err != nil || rec.State != apimigrate.StateCompleted {
		t.Fatalf("retry = %+v, %v", rec, err)
	}
}

func TestMigrationCorrelationFailureReturnsToIMAP(t *testing.T) {
	r := newMigrationRig(t)
	r.app.apiRoutes.migrationStep = func(s string) error {
		if s == migrateStepComplete {
			return errors.New("disk full")
		}
		return nil
	}
	if _, err := r.app.MigrateMailAPIAccount(r.acc.ID); err == nil {
		t.Fatal("Migrate succeeded")
	}
	if r.state(t) != account.MailRouteStateStaged || r.visibleSubjects(t)[0] != "imap" {
		t.Fatal("account not returned to IMAP")
	}
	if rec, _ := r.app.migrationStore().Get(r.acc.ID); rec.State != apimigrate.StateFailed {
		t.Fatalf("record = %+v", rec)
	}
}

func TestMigrationRollbackRestoresIMAPAndReportsQueuedActions(t *testing.T) {
	r := newMigrationRig(t)
	if _, err := r.app.MigrateMailAPIAccount(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	queueAPIFlagAction(t, r, message.APIActionPending)
	rep, err := r.app.RollbackMailAPIMigration(r.acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unfinished) != 1 || rep.Unfinished[0].Status != message.APIActionPending || rep.SnapshotPath == "" {
		t.Fatalf("report = %+v", rep)
	}
	if r.state(t) != account.MailRouteStateStaged {
		t.Fatal("route still active")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("visible = %v", got)
	}
	if acts, _ := r.app.messageStore.ListAPIActions(r.acc.ID, message.APIActionPending); len(acts) != 1 {
		t.Fatal("queued action was dropped")
	}
	if rec, _ := r.app.migrationStore().Get(r.acc.ID); rec.State != apimigrate.StateRolledBack {
		t.Fatalf("record = %+v", rec)
	}
	if _, err := r.app.RollbackMailAPIMigration(r.acc.ID); err == nil {
		t.Fatal("second rollback succeeded")
	}
	// A rolled-back account can migrate again from a fresh snapshot.
	if rec, err := r.app.MigrateMailAPIAccount(r.acc.ID); err != nil || rec.State != apimigrate.StateCompleted {
		t.Fatalf("re-migrate = %+v, %v", rec, err)
	}
}
