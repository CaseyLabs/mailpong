package app

import (
	"time"

	"github.com/caseylabs/mailpong/internal/folder"
	"github.com/caseylabs/mailpong/internal/logging"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ensureSentIdle makes the account's Sent IDLE connection follow its current
// Sent folder: a connection on a stale path is stopped and a missing one is
// started. A connection that already exists is left alone even if it died, so
// scheduled syncs don't re-dial a server that rejects it; restartIDLE and
// network recovery revive those, and syncSentIfUnwatched covers Sent until
// then. Called after folder syncs.
func (a *App) ensureSentIdle(accountID string) {
	if a.idleManager == nil {
		return
	}
	a.idleAccountMu.Lock()
	defer a.idleAccountMu.Unlock()
	acc, err := a.accountStore.Get(accountID)
	if err != nil || acc == nil || !acc.Enabled {
		return
	}
	want := ""
	if sent, err := a.GetSpecialFolder(accountID, folder.TypeSent); err == nil && sent != nil && sent.Path != "INBOX" {
		want = sent.Path
	}
	found := false
	for _, f := range a.idleManager.Watched(accountID) {
		switch f {
		case "INBOX":
		case want:
			found = true
		default:
			a.idleManager.StopFolder(accountID, f)
		}
	}
	if want != "" && !found {
		a.idleManager.StartAccount(accountID, acc.Name, want)
	}
}

// startAccountIdle starts IDLE for an enabled account on INBOX and, when it is
// known, the Sent folder, reviving connections that died. Connections that
// already run are left alone. ensureSentIdle covers Sent once folder syncs
// have found it.
func (a *App) startAccountIdle(accountID string) {
	if a.idleManager == nil {
		return
	}
	a.idleAccountMu.Lock()
	defer a.idleAccountMu.Unlock()
	acc, err := a.accountStore.Get(accountID)
	if err != nil || acc == nil || !acc.Enabled {
		return
	}
	folders := []string{"INBOX"}
	if sent, err := a.GetSpecialFolder(accountID, folder.TypeSent); err == nil && sent != nil && sent.Path != "" && sent.Path != "INBOX" {
		folders = append(folders, sent.Path)
	}
	a.idleManager.StartAccount(acc.ID, acc.Name, folders...)
}

// stopAccountIdle runs change (the account's disable or deletion) and stops
// its IDLE connections as one step, so a concurrent ensureSentIdle or
// startAccountIdle sees the account either before both or after both.
func (a *App) stopAccountIdle(accountID string, change func() error) error {
	a.idleAccountMu.Lock()
	defer a.idleAccountMu.Unlock()
	if a.idleManager != nil {
		a.idleManager.StopAccount(accountID)
	}
	return change()
}

// idleSentMinInterval is the minimum gap between the fallback Sent syncs that
// inbox IDLE activity starts, so a stream of flag events (mail read on a
// phone) doesn't start a Sent sync for each one.
const idleSentMinInterval = 30 * time.Second

// syncSentIfUnwatched syncs Sent after inbox IDLE activity when the account's
// Sent IDLE connection isn't running (the server refused a second connection,
// or it gave up reconnecting). A reply sent from another client flags its
// inbox original \Answered, so inbox activity still catches most of them. It
// runs at most once per idleSentMinInterval per account; activity inside the
// interval coalesces into one trailing sync at its end.
func (a *App) syncSentIfUnwatched(accountID string) {
	if a.idleManager == nil {
		return
	}
	sent, err := a.GetSpecialFolder(accountID, folder.TypeSent)
	if err != nil || sent == nil || sent.Path == "" || sent.Path == "INBOX" {
		return
	}
	if a.idleManager.Running(accountID, sent.Path) {
		return
	}
	a.idleSentMu.Lock()
	defer a.idleSentMu.Unlock()
	now := time.Now()
	next := a.idleSentNext[accountID]
	if next.After(now) {
		return // a sync is already scheduled
	}
	at := next.Add(idleSentMinInterval)
	if at.Before(now) {
		at = now
	}
	a.idleSentNext[accountID] = at
	time.AfterFunc(at.Sub(now), func() { a.syncSentAfterIdle(accountID) })
}

// syncSentAfterIdle syncs the Sent folder after IDLE reports activity on it
// (or on the inbox; see syncSentIfUnwatched), so mail sent from another
// client (e.g. the Gmail web UI) reaches Sent, and the chat thread, within
// seconds. When a sync of the Sent folder is already running, it retries
// shortly rather than dropping the activity, since that sync may have listed
// Sent before the message arrived.
func (a *App) syncSentAfterIdle(accountID string) {
	defer recoverPanic("app.idle", "sync Sent after IDLE")
	log := logging.WithComponent("app.idle")

	if a.isShuttingDown() {
		return
	}
	sent, err := a.GetSpecialFolder(accountID, folder.TypeSent)
	if err != nil || sent == nil {
		return
	}
	syncKey := accountID + ":" + sent.ID
	ctx, release, ok := a.beginFolderSync(syncKey)
	if !ok {
		// Sent is busy (or shutdown began, which the retry's first check
		// catches). Keep one retry chain per account to cover all activity.
		if _, pending := a.idleSentRetry.LoadOrStore(accountID, struct{}{}); !pending {
			time.AfterFunc(idleFlagResyncDebounce, func() {
				a.idleSentRetry.Delete(accountID)
				a.syncSentAfterIdle(accountID)
			})
		}
		return
	}
	defer release()

	before := a.syncEngine.FolderChangeSeq(sent.ID)
	syncPeriodDays := a.syncPeriodDays(accountID)
	// Incremental, like the IDLE inbox sync; the scheduled sync stays the
	// full reconciliation.
	err = a.syncEngine.SyncMessages(ctx, accountID, sent.ID, syncPeriodDays, true)
	if err == nil {
		err = a.syncEngine.FetchBodiesInBackground(ctx, accountID, sent.ID, syncPeriodDays)
	}
	if err != nil && ctx.Err() == nil {
		log.Warn().Err(err).Str("accountID", accountID).Msg("Sent sync after IDLE failed")
	}

	// Cancelled means a manual sync replaced this one; it emits its own
	// completion, and emitting here would clear its progress indicator early.
	if ctx.Err() != nil {
		return
	}

	// Clears the Sent progress indicator and refreshes chat threads.
	wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
		"accountId": accountID,
		"folderId":  sent.ID,
	})
	// sent:synced reloads chat lists and open threads, so skip it when the
	// sync stored or removed nothing; IDLE flag activity triggers many no-op
	// syncs. Another writer moving the counter meanwhile only adds a reload.
	if a.syncEngine.FolderChangeSeq(sent.ID) == before {
		log.Debug().Str("accountID", accountID).Msg("Sent unchanged after IDLE sync; skipping sent:synced")
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "sent:synced", map[string]interface{}{
		"accountId": accountID,
	})
}
