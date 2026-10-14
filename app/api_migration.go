package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/message"
)

// Migration steps, in order. Each is safe to repeat after an interruption.
const (
	migrateStepSnapshot  = "snapshot"
	migrateStepActivate  = "activate"
	migrateStepCorrelate = "correlate"
	migrateStepComplete  = "complete"
)

// errMigrationInterrupted is returned by the test seam to stop a migration as
// a crash would: no cleanup runs and the record stays where it was.
var errMigrationInterrupted = errors.New("migration interrupted")

// MailAPIMigrationPlan is the read-only report shown before consent. The
// migration refuses to start while Blockers is not empty.
type MailAPIMigrationPlan struct {
	AccountID      string             `json:"accountId"`
	Route          string             `json:"route"`
	RouteState     string             `json:"routeState"`
	Counts         apimigrate.Counts  `json:"counts"`
	PendingActions int                `json:"pendingActions"`
	Blockers       []string           `json:"blockers"`
	Migration      *apimigrate.Record `json:"migration"`
}

// MailAPIRollbackReport lists API-side work that did not reach IMAP. Nothing
// is replayed or dropped: queued actions stay queued, hidden, until the route is
// activated again.
type MailAPIRollbackReport struct {
	AccountID    string                  `json:"accountId"`
	SnapshotPath string                  `json:"snapshotPath"`
	Unfinished   []MailAPIRollbackAction `json:"unfinished"`
	// AppliedSince counts actions applied at the provider after the migration
	// started; the IMAP reconcile picks their effect up from the server.
	AppliedSince int `json:"appliedSince"`
}

type MailAPIRollbackAction struct {
	Op     string `json:"op"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (a *App) migrationStore() *apimigrate.Store { return apimigrate.NewStore(a.db) }

// migrationBlockers lists why a fresh migration cannot start.
func (a *App) migrationBlockers(route *account.MailAPIRoute, counts apimigrate.Counts, open int) []string {
	var out []string
	accountID := route.AccountID
	if route.Shared {
		out = append(out, "Shared mailbox routes cannot be migrated yet.")
	}
	if status, err := a.mailAPIOps().status(accountID); err != nil {
		out = append(out, "The API authorization could not be checked: "+err.Error())
	} else if !status.Authorized {
		out = append(out, fmt.Sprintf("API authorization is not usable (%s); reauthorize first.", status.Failure))
	}
	if open > 0 {
		out = append(out, fmt.Sprintf("%d API action(s) are in flight or of unknown outcome.", open))
	}
	if counts.QueuedSubmissions > 0 {
		out = append(out, fmt.Sprintf("%d message submission(s) are queued or of unknown outcome; review them first.", counts.QueuedSubmissions))
	}
	if a.imapSyncBusy(accountID) {
		out = append(out, "An IMAP sync is in progress.")
	}
	if mu := a.apiRoutes.lock(accountID); !mu.TryLock() {
		out = append(out, "An API sync is in progress.")
	} else {
		mu.Unlock()
	}
	return out
}

// PlanMailAPIMigration reports what migrating the account would involve. It
// changes nothing.
func (a *App) PlanMailAPIMigration(accountID string) (*MailAPIMigrationPlan, error) {
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	switch {
	case err != nil:
		return nil, err
	case route == nil:
		return nil, errors.New("account has no API route; authorize one first")
	}
	st := a.migrationStore()
	counts, err := st.Count(accountID)
	if err != nil {
		return nil, err
	}
	rec, err := st.Get(accountID)
	if err != nil {
		return nil, err
	}
	var open, pending int
	actions, err := a.messageStore.ListAPIActions(accountID, message.APIActionPending, message.APIActionInFlight, message.APIActionUnknown)
	if err != nil {
		return nil, err
	}
	for _, act := range actions {
		if act.Status == message.APIActionPending {
			pending++
		} else {
			open++
		}
	}
	plan := &MailAPIMigrationPlan{AccountID: accountID, Route: route.Route, RouteState: route.State,
		Counts: counts, PendingActions: pending, Migration: rec, Blockers: []string{}}
	if route.State == account.MailRouteStateActive && (rec == nil || rec.State == apimigrate.StateCompleted) {
		plan.Blockers = append(plan.Blockers, "The account already uses its API route.")
		return plan, nil
	}
	if inProgress(rec) {
		return plan, nil // resuming needs no fresh preconditions
	}
	plan.Blockers = append(plan.Blockers, a.migrationBlockers(route, counts, open)...)
	return plan, nil
}

func inProgress(rec *apimigrate.Record) bool {
	return rec != nil && (rec.State == apimigrate.StateSnapshotted || rec.State == apimigrate.StateActivated)
}

// MigrateMailAPIAccount moves an account onto its staged API route: snapshot,
// activation (with its API reconcile), then correlation of the API rows with
// the existing IMAP rows. Accounts, credentials, identities, drafts, queued
// mail, rules and contacts are untouched; IMAP rows stay hidden for rollback.
// After an interruption the same call resumes. An error returns the account to
// IMAP with its data intact.
func (a *App) MigrateMailAPIAccount(accountID string) (*apimigrate.Record, error) {
	rs := &a.apiRoutes
	rs.transition.Lock()
	defer rs.transition.Unlock()
	plan, err := a.PlanMailAPIMigration(accountID)
	if err != nil {
		return nil, err
	}
	if len(plan.Blockers) > 0 {
		return nil, fmt.Errorf("migration blocked: %s", plan.Blockers[0])
	}
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	if err != nil || route == nil {
		return nil, errors.Join(err, errors.New("account has no API route"))
	}
	st := a.migrationStore()
	rec := plan.Migration
	step := func(name string) error {
		if rs.migrationStep != nil {
			return rs.migrationStep(name)
		}
		return nil
	}
	save := func(r apimigrate.Record) error {
		if err := st.Put(r); err != nil {
			return err
		}
		rec = &r
		return nil
	}

	// Snapshot, unless a snapshot of the untouched account already exists.
	snapshotted := rec != nil && rec.State == apimigrate.StateSnapshotted && rec.SnapshotPath != ""
	if snapshotted {
		if _, err := os.Stat(rec.SnapshotPath); err != nil {
			snapshotted = false
		}
	}
	if route.State != account.MailRouteStateActive && !snapshotted && (rec == nil || rec.State != apimigrate.StateActivated) {
		if err := step(migrateStepSnapshot); err != nil {
			return rec, err
		}
		if a.paths == nil {
			return nil, errors.New("data directory is not available for the snapshot")
		}
		routeJSON, err := json.Marshal(route)
		if err != nil {
			return nil, err
		}
		path, err := st.Snapshot(filepath.Join(a.paths.Data, "migration-snapshots"), accountID, routeJSON, time.Now())
		if err != nil {
			return nil, fmt.Errorf("snapshot failed; account stays on IMAP: %w", err)
		}
		if err := save(apimigrate.Record{AccountID: accountID, State: apimigrate.StateSnapshotted, Route: route.Route, SnapshotPath: path}); err != nil {
			return nil, err
		}
	}
	if rec == nil { // active route adopted without a snapshot: record it
		if err := save(apimigrate.Record{AccountID: accountID, State: apimigrate.StateActivated, Route: route.Route}); err != nil {
			return nil, err
		}
	}

	fail := func(cause error) (*apimigrate.Record, error) {
		if errors.Is(cause, errMigrationInterrupted) {
			return rec, cause
		}
		if r, err := a.accountStore.GetMailAPIRoute(accountID); err == nil && r != nil && r.State == account.MailRouteStateActive {
			if err := a.deactivateAPIRoute(accountID); err != nil {
				cause = errors.Join(cause, fmt.Errorf("could not return to IMAP: %w", err))
			}
		}
		next := *rec
		next.State, next.Error = apimigrate.StateFailed, cause.Error()
		if rec.State == apimigrate.StateSnapshotted {
			next.State = apimigrate.StateSnapshotted // nothing changed yet; stays resumable
		}
		_ = save(next)
		return rec, cause
	}

	if route.State != account.MailRouteStateActive {
		if err := step(migrateStepActivate); err != nil {
			return fail(err)
		}
		if err := a.activateAPIRoute(accountID); err != nil {
			return fail(err)
		}
	}
	if rec.State == apimigrate.StateSnapshotted {
		next := *rec
		next.State, next.Error = apimigrate.StateActivated, ""
		if err := save(next); err != nil {
			return fail(err)
		}
	}
	if err := step(migrateStepCorrelate); err != nil {
		return fail(err)
	}
	res, err := st.Correlate(accountID)
	if err != nil {
		return fail(fmt.Errorf("correlating existing messages failed; account returned to IMAP: %w", err))
	}
	if err := step(migrateStepComplete); err != nil {
		return fail(err)
	}
	next := *rec
	next.State, next.Error, next.Correlated, next.Uncorrelated = apimigrate.StateCompleted, "", res.Correlated, res.Uncorrelated
	if err := save(next); err != nil {
		return fail(err)
	}
	a.refreshAPIAccount(accountID)
	return rec, nil
}

// RollbackMailAPIMigration returns the account to IMAP: the hidden IMAP
// folders and rows come back, an IMAP reconcile runs, and nothing is replayed
// to the provider. API-side actions that did not reach IMAP are reported and
// stay queued. Restoring the full pre-migration snapshot is a separate choice
// that would lose later local changes; the report names the snapshot.
func (a *App) RollbackMailAPIMigration(accountID string) (*MailAPIRollbackReport, error) {
	rs := &a.apiRoutes
	rs.transition.Lock()
	defer rs.transition.Unlock()
	st := a.migrationStore()
	rec, err := st.Get(accountID)
	if err != nil {
		return nil, err
	}
	route, err := a.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return nil, err
	}
	active := route != nil && route.State == account.MailRouteStateActive
	if rec == nil && !active {
		return nil, errors.New("account has not been migrated")
	}
	if rec != nil && rec.State == apimigrate.StateRolledBack && !active {
		return nil, errors.New("migration was already rolled back")
	}
	if active {
		if err := a.deactivateAPIRoute(accountID); err != nil {
			return nil, err
		}
	}
	report := &MailAPIRollbackReport{AccountID: accountID, Unfinished: []MailAPIRollbackAction{}}
	started := int64(0)
	if rec != nil {
		report.SnapshotPath, started = rec.SnapshotPath, rec.StartedAt
	}
	actions, err := a.messageStore.ListAPIActions(accountID)
	if err != nil {
		return nil, err
	}
	for _, act := range actions {
		switch act.Status {
		case message.APIActionApplied:
			if act.CreatedAt.Unix() >= started {
				report.AppliedSince++
			}
		case message.APIActionRejected:
		default:
			report.Unfinished = append(report.Unfinished, MailAPIRollbackAction{Op: act.Op, Status: act.Status, Error: act.Error})
		}
	}
	next := apimigrate.Record{AccountID: accountID, State: apimigrate.StateRolledBack}
	if route != nil {
		next.Route = route.Route
	}
	if rec != nil {
		next = *rec
		next.State = apimigrate.StateRolledBack
	}
	if err := st.Put(next); err != nil {
		return report, err
	}
	return report, nil
}
