// Package apiactions drains the local-first action queue of API-routed
// accounts: each user action is already applied locally and queued; the worker
// sends it to Gmail or Graph, in order per message, and settles it by the
// provider's answer. A request that may have been applied is verified by a
// read before any re-send and is never reverted on a guess.
package apiactions

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/message"
)

const (
	baseBackoff = 5 * time.Second
	maxBackoff  = 5 * time.Minute
)

// Worker drains the queue. Zero optional fields use defaults.
type Worker struct {
	Store *message.Store
	// Remote builds the provider client for an account's action.
	Remote func(accountID, provider string) (Remote, error)
	// Active reports whether the account still routes through its API; an
	// inactive account's queue is left untouched for a later activation.
	Active func(accountID string) bool
	// Emit reports user-visible problems as (event, payload).
	Emit func(event string, data any)
	// Changed is called after local state changed outside the user's action
	// (a revert or a removal), so the UI can refresh the account.
	Changed func(accountID string)
	Now     func() time.Time

	mu      sync.Mutex
	running map[string]bool
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func (w *Worker) emit(accountID string, a *message.APIAction, text string) {
	log.Warn().Str("account", accountID).Int64("action", a.ID).Str("op", a.Op).Str("error", text).Msg("api action problem")
	if w.Emit != nil {
		w.Emit("sync:error", map[string]any{"accountId": accountID, "error": "Could not " + describe(a) + ": " + text})
	}
}

func describe(a *message.APIAction) string {
	switch a.Op {
	case message.APIActionRead:
		return "update read status"
	case message.APIActionStar:
		return "update the star"
	}
	return "move a message"
}

// Recover marks interrupted in-flight actions of every account unknown. Call
// it once at startup before draining.
func (w *Worker) Recover() error {
	ids, err := w.Store.OpenAPIActionAccounts()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := w.Store.RecoverAPIActions(id); err != nil {
			return err
		}
	}
	return nil
}

// Drain runs every runnable action of the account in order and returns when
// none is due, the account is deactivated or ctx ends. It returns the time the
// next deferred action is due (zero if none) so the caller can reschedule.
// Only one Drain runs per account; a concurrent call returns at once.
func (w *Worker) Drain(ctx context.Context, accountID string) (time.Time, error) {
	w.mu.Lock()
	if w.running == nil {
		w.running = map[string]bool{}
	}
	if w.running[accountID] {
		w.mu.Unlock()
		return time.Time{}, nil
	}
	w.running[accountID] = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.running, accountID)
		w.mu.Unlock()
	}()

	for ctx.Err() == nil {
		if w.Active != nil && !w.Active(accountID) {
			return time.Time{}, nil
		}
		a, err := w.Store.NextAPIAction(accountID, w.now())
		if err != nil {
			return time.Time{}, err
		}
		if a == nil {
			return w.nextDue(accountID)
		}
		if err := w.run(ctx, a); err != nil {
			return time.Time{}, err
		}
	}
	return time.Time{}, ctx.Err()
}

func (w *Worker) nextDue(accountID string) (time.Time, error) {
	open, err := w.Store.ListAPIActions(accountID, message.APIActionPending, message.APIActionUnknown)
	if err != nil {
		return time.Time{}, err
	}
	var next time.Time
	for _, a := range open {
		if a.NextAttemptAt.After(w.now()) && (next.IsZero() || a.NextAttemptAt.Before(next)) {
			next = a.NextAttemptAt
		}
	}
	return next, nil
}

func backoff(attempts int) time.Duration {
	d := baseBackoff
	for i := 1; i < attempts && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}

// run settles one action. Store errors abort the drain; provider errors are
// settled into the action's state.
func (w *Worker) run(ctx context.Context, a *message.APIAction) error {
	if a.Op == message.APIActionMove && a.Payload.MovedTo != "" {
		// The provider already applied this move; only the local re-key is left.
		if err := w.Store.SetAPIActionStatus(a.ID, message.APIActionInFlight, ""); err != nil {
			return err
		}
		a.Attempts++
		return w.rekey(a, a.Payload.MovedTo, a.Payload.MovedContainers)
	}
	remote, err := w.Remote(a.AccountID, a.Provider)
	if err != nil {
		// No client (no grant, no token): nothing was sent this time, but an
		// unknown action may already have taken effect and must be verified
		// before it is replayed.
		if a.Status == message.APIActionUnknown {
			return w.stayUnknown(a, err.Error())
		}
		return w.retryLater(a, err.Error())
	}
	if a.Status == message.APIActionUnknown {
		done, err := w.verify(ctx, remote, a)
		if err != nil || done {
			return err
		}
	}
	if err := w.Store.SetAPIActionStatus(a.ID, message.APIActionInFlight, ""); err != nil {
		return err
	}
	a.Attempts++
	res, err := remote.Apply(ctx, a)
	return w.settle(a, res, err)
}

// verify reads the provider after an unknown outcome. It reports done when the
// action is settled or rescheduled; otherwise the caller re-sends it, because
// the provider still shows the state before the action.
func (w *Worker) verify(ctx context.Context, remote Remote, a *message.APIAction) (bool, error) {
	v, err := remote.Verify(ctx, a)
	switch {
	case errors.Is(err, ErrGone) && a.Op != message.APIActionMove:
		return true, w.gone(a)
	case err != nil:
		return true, w.stayUnknown(a, err.Error())
	case v == Matches:
		return true, w.Store.SetAPIActionStatus(a.ID, message.APIActionApplied, "")
	case v == Differs:
		return false, nil
	}
	const text = "the provider's state cannot be verified"
	if a.Error != text {
		w.emit(a.AccountID, a, text)
	}
	return true, w.stayUnknown(a, text)
}

func (w *Worker) settle(a *message.APIAction, res Result, err error) error {
	if err == nil {
		return w.applied(a, res)
	}
	if errors.Is(err, ErrGone) {
		return w.gone(a)
	}
	switch apiwrite.OutcomeOf(err) {
	case apiwrite.Rejected:
		return w.reject(a, err.Error())
	case apiwrite.NotSent:
		return w.retryLater(a, err.Error())
	case apiwrite.Reauthorize:
		w.emit(a.AccountID, a, "the account needs to be reauthorized")
		return w.retryLater(a, err.Error())
	}
	// Unknown: the provider may have applied it. Verify before re-sending.
	return w.stayUnknown(a, err.Error())
}

func (w *Worker) applied(a *message.APIAction, res Result) error {
	if a.Op == message.APIActionMove && res.NewRemoteID != "" && res.NewRemoteID != a.RemoteID {
		return w.rekey(a, res.NewRemoteID, res.Containers)
	}
	return w.Store.SetAPIActionStatus(a.ID, message.APIActionApplied, "")
}

// rekey gives the moved message its new remote ID locally. The move already
// happened at the provider, so a failure keeps the action open with the new ID
// recorded: later actions on the message stay queued behind it instead of
// using the stale ID, and each retry repeats only the re-key.
func (w *Worker) rekey(a *message.APIAction, newID string, containers []string) error {
	err := w.Store.ApplyAPIMove(message.APIMove{AccountID: a.AccountID, Provider: a.Provider,
		OldRemoteID: a.RemoteID, NewRemoteID: newID, Containers: containers,
		FromFolderID: a.Payload.ToFolderID, ToFolderID: a.Payload.ToFolderID})
	if err != nil && a.Payload.MovedTo != "" {
		// An earlier retry may have re-keyed without recording it.
		old, _, oerr := w.Store.GetAPIRef(a.AccountID, a.Provider, a.RemoteID)
		moved, rows, nerr := w.Store.GetAPIRef(a.AccountID, a.Provider, newID)
		if oerr == nil && nerr == nil && old == nil && moved != nil && rows[a.Payload.ToFolderID] != "" {
			err = nil
		}
	}
	if err == nil {
		return w.Store.SetAPIActionStatus(a.ID, message.APIActionApplied, "")
	}
	log.Warn().Err(err).Int64("action", a.ID).Msg("could not re-key moved api message; will retry")
	p := a.Payload
	p.MovedTo, p.MovedContainers = newID, containers
	if perr := w.Store.SetAPIActionPayload(a.ID, p); perr != nil {
		return perr
	}
	return w.retryLater(a, "could not update the moved message locally: "+err.Error())
}

func (w *Worker) gone(a *message.APIAction) error {
	if err := w.Store.RemoveAPIMessage(a.AccountID, a.Provider, a.RemoteID); err != nil {
		return err
	}
	if err := w.Store.SetAPIActionStatus(a.ID, message.APIActionRejected, "message no longer exists"); err != nil {
		return err
	}
	w.emit(a.AccountID, a, "the message no longer exists on the server")
	w.changed(a.AccountID)
	return nil
}

func (w *Worker) reject(a *message.APIAction, text string) error {
	var err error
	if a.Op == message.APIActionMove {
		err = w.Store.RevertAPIMove(a)
	} else {
		err = w.Store.RevertAPIFlag(a)
	}
	if err != nil {
		return err
	}
	if err := w.Store.SetAPIActionStatus(a.ID, message.APIActionRejected, text); err != nil {
		return err
	}
	w.emit(a.AccountID, a, text)
	w.changed(a.AccountID)
	return nil
}

func (w *Worker) changed(accountID string) {
	if w.Changed != nil {
		w.Changed(accountID)
	}
}

// retryLater returns an action that reached nobody to pending with backoff.
func (w *Worker) retryLater(a *message.APIAction, text string) error {
	return w.Store.ScheduleAPIAction(a.ID, message.APIActionPending, w.now().Add(backoff(max(a.Attempts, 1))), text)
}

// stayUnknown keeps an action whose effect is unconfirmed, due again later.
func (w *Worker) stayUnknown(a *message.APIAction, text string) error {
	return w.Store.ScheduleAPIAction(a.ID, message.APIActionUnknown, w.now().Add(backoff(max(a.Attempts, 1))), text)
}
