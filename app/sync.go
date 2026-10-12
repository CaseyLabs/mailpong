package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	gosync "sync"
	"time"

	"github.com/caseylabs/mailpong/internal/certificate"
	"github.com/caseylabs/mailpong/internal/folder"
	"github.com/caseylabs/mailpong/internal/logging"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Sync API - Exposed to frontend via Wails bindings
// ============================================================================

// SyncFolder synchronizes messages for a folder with the IMAP server
func (a *App) SyncFolder(accountID, folderID string) error {
	const debounceMs = 500
	log := logging.WithComponent("app")

	// Use composite key to allow multiple folders to sync concurrently
	syncKey := accountID + ":" + folderID

	a.syncMu.Lock()

	// Check debounce - if last request for this folder was within 500ms, skip
	if lastReq, exists := a.syncLastRequest[syncKey]; exists {
		if time.Since(lastReq) < time.Duration(debounceMs)*time.Millisecond {
			a.syncMu.Unlock()
			log.Debug().Str("account", accountID).Str("folder", folderID).Msg("SyncFolder debounced, skipping")
			return nil // Silently ignore
		}
	}
	a.syncLastRequest[syncKey] = time.Now()

	if _, exists := a.syncContexts[syncKey]; exists {
		log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Cancelling existing sync for restart")
	}
	ctx, release, ok := a.replaceFolderSyncLocked(syncKey, slotReleaseWait)

	a.syncMu.Unlock()
	if !ok {
		log.Debug().Str("account", accountID).Str("folder", folderID).Msg("SyncFolder skipped, app is shutting down")
		return nil
	}

	// The body fetch goroutine releases the slot when it finishes; earlier
	// returns release it here.
	bodyFetchStarted := false
	defer func() {
		if !bodyFetchStarted {
			release()
		}
	}()

	// Get account to determine sync period
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	syncPeriodDays := 30 // default (0 means all messages)
	if acc != nil {
		syncPeriodDays = acc.SyncPeriodDays
	}

	// If one of our own delete/move EXPUNGEs is still in flight, wait it out
	// before reconciling (racing it resurrects the just-deleted messages).
	// Emit the engine's initial progress event first so the sidebar spinner
	// appears immediately — otherwise the wait looks like a dead sync button.
	if a.recentOwnExpunge(accountID) {
		wailsRuntime.EventsEmit(a.ctx, "sync:progress", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
			"fetched":   0,
			"total":     0,
			"phase":     "messages",
		})
		a.waitForExpungeQuiet(ctx, accountID)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Use ctx (not a.ctx) for sync operations so they can be cancelled
	err = a.syncEngine.SyncMessages(ctx, accountID, folderID, syncPeriodDays, false)
	if err != nil {
		// Check if this was a cancellation
		if ctx.Err() != nil {
			log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Sync cancelled")
			return ctx.Err()
		}
		// Check for certificate error - emit special event for TOFU dialog
		var certErr *certificate.Error
		if errors.As(err, &certErr) {
			log.Warn().Str("folder", folderID).Str("fingerprint", certErr.Info.Fingerprint).Msg("Untrusted certificate during sync")
			wailsRuntime.EventsEmit(a.ctx, "certificate:untrusted", map[string]interface{}{
				"accountId":   accountID,
				"certificate": certErr.Info,
			})
			return err
		}

		// Actual error - emit error event
		log.Error().Err(err).Str("folder", folderID).Msg("Header sync failed")
		wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
			"error":     err.Error(),
		})
		return err
	}

	// Checkpoint WAL after heavy sync operation
	if checkpointErr := a.db.Checkpoint(); checkpointErr != nil {
		log.Warn().Err(checkpointErr).Msg("WAL checkpoint after SyncFolder failed")
	}

	// Emit folder count change event so frontend updates sidebar
	if folderObj, folderErr := a.folderStore.Get(folderID); folderErr == nil && folderObj != nil {
		log.Debug().
			Str("folderID", folderID).
			Int("unreadCount", folderObj.UnreadCount).
			Msg("Emitting folders:countsChanged after sync")
		wailsRuntime.EventsEmit(a.ctx, "folders:countsChanged", map[string]int{
			folderID: folderObj.UnreadCount,
		})
	}

	// Start background body fetching (emits progress events for "bodies" phase)
	// Pass ctx so body fetch can also be cancelled
	bodyFetchStarted = true
	go func(syncCtx context.Context, syncDays int) {
		defer release()

		// Panic recovery - ensure we always emit an event so UI doesn't get stuck
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("folder", folderID).Msg("Body fetch goroutine panicked")
				wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     fmt.Sprintf("body fetch panic: %v", r),
				})
			}
		}()

		bodyErr := a.syncEngine.FetchBodiesInBackground(syncCtx, accountID, folderID, syncDays)

		if bodyErr != nil {
			if syncCtx.Err() != nil {
				// Cancelled - not an error, still emit synced so spinner stops
				log.Debug().Str("folder", folderID).Msg("Background body fetch cancelled")
				wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
				})
			} else {
				// Actual error - emit error event instead of synced
				log.Error().Err(bodyErr).Str("folder", folderID).Msg("Background body fetch failed")
				wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     bodyErr.Error(),
				})
			}
		} else {
			// Success
			wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
				"accountId": accountID,
				"folderId":  folderID,
			})
		}
	}(ctx, syncPeriodDays)

	return nil
}

// ForceSyncFolder clears body content and attachments for a folder, then re-syncs.
// This is useful when attachments weren't extracted properly (e.g., after a fix)
// or when message content needs to be re-parsed.
func (a *App) ForceSyncFolder(accountID, folderID string) error {
	log := logging.WithComponent("app")
	log.Info().Str("accountID", accountID).Str("folderID", folderID).Msg("Starting force re-sync of folder")

	// Step 1: Clear body content for all messages in the folder
	bodiesCleared, err := a.messageStore.ClearBodiesForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to clear bodies: %w", err)
	}
	log.Info().Int64("bodiesCleared", bodiesCleared).Msg("Cleared message bodies")

	// Step 2: Delete attachments for all messages in the folder
	attachmentsDeleted, err := a.attachmentStore.DeleteAttachmentsForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to delete attachments: %w", err)
	}
	log.Info().Int64("attachmentsDeleted", attachmentsDeleted).Msg("Deleted attachments")

	// Step 3: Trigger normal folder sync (which will re-fetch bodies and extract attachments)
	return a.SyncFolder(accountID, folderID)
}

// SyncAccountComplete performs a comprehensive sync of an account:
// 1. Syncs folder list from IMAP
// 2. Syncs core folders' messages (Inbox, Drafts, Sent)
func (a *App) SyncAccountComplete(accountID string) error {
	log := logging.WithComponent("app.masterSync")
	log.Info().Str("accountID", accountID).Msg("Starting complete account sync")

	// Check if sync was cancelled before starting
	a.syncMu.Lock()
	cancelled := a.syncCancelled
	a.syncMu.Unlock()
	if cancelled {
		return fmt.Errorf("sync cancelled")
	}

	// 1. Sync folder list first (required for message sync)
	if err := a.SyncFolders(accountID); err != nil {
		return fmt.Errorf("folder sync failed: %w", err)
	}

	// 2. Determine which folders to sync based on account settings
	foldersToSync, err := a.getSyncFolders(accountID)
	if err != nil {
		return fmt.Errorf("failed to determine sync folders: %w", err)
	}

	log.Info().Str("accountID", accountID).Int("folders", len(foldersToSync)).Msg("Syncing folders")

	// Sync with semaphore (max 2 concurrent to leave pool room for user actions)
	sem := make(chan struct{}, 2)
	var syncErrors []string
	var errorsMu gosync.Mutex
	var wg gosync.WaitGroup

	for _, f := range foldersToSync {
		// Check if sync was cancelled between folders
		a.syncMu.Lock()
		cancelled := a.syncCancelled
		a.syncMu.Unlock()
		if cancelled {
			log.Info().Str("accountID", accountID).Msg("Sync cancelled, stopping folder loop")
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(f *folder.Folder) {
			defer recoverPanic("app.sync", "sync folder")
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			log.Info().Str("path", f.Path).Str("id", f.ID).Msg("Syncing folder")
			if syncErr := a.SyncFolder(accountID, f.ID); syncErr != nil {
				log.Warn().Err(syncErr).Str("folder", f.Path).Msg("Message sync failed")
				errorsMu.Lock()
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", f.Path, syncErr))
				errorsMu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	if len(syncErrors) > 0 {
		return fmt.Errorf("some folders failed to sync: %s", strings.Join(syncErrors, "; "))
	}

	log.Info().Str("accountID", accountID).Msg("Complete account sync finished")
	return nil
}

// getSyncFolders returns the folders that should be automatically synced for an account.
// Three-way logic:
//   - SyncAllFolders=true → all folders
//   - SyncFoldersEnabled=true → subscribed folders (respects IMAP subscriptions)
//   - default → core only (Inbox, Drafts, Sent) — backward compatible
func (a *App) getSyncFolders(accountID string) ([]*folder.Folder, error) {
	acct, err := a.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if acct.SyncAllFolders {
		return a.folderStore.List(accountID)
	}
	if acct.SyncFoldersEnabled {
		return a.folderStore.ListSubscribed(accountID)
	}
	return a.getCoreOnlyFolders(accountID)
}

// getCoreOnlyFolders returns core folders (Inbox, Drafts, Sent) — the default sync behavior.
func (a *App) getCoreOnlyFolders(accountID string) ([]*folder.Folder, error) {
	coreTypes := []folder.Type{folder.TypeInbox, folder.TypeDrafts, folder.TypeSent}
	var folders []*folder.Folder
	for _, ft := range coreTypes {
		f, err := a.folderStore.GetByType(accountID, ft)
		if err != nil {
			continue
		}
		if f != nil {
			folders = append(folders, f)
		}
	}
	return folders, nil
}

// SyncAllComplete syncs all accounts completely, then syncs CardDAV sources.
// Email sync runs first (most important), then CardDAV sync runs after to avoid
// database contention (SQLITE_BUSY errors).
// This is the master sync function called from the sidebar sync button.
func (a *App) SyncAllComplete() error {
	log := logging.WithComponent("app.masterSync")

	// Reset cancellation flag for this sync run
	a.syncMu.Lock()
	a.syncCancelled = false
	a.syncMu.Unlock()

	// Skip if we know we're offline — avoids connection errors and the
	// error indicator that would appear on the sidebar account menu.
	// Emit folder:synced for each account so the frontend clears its
	// syncing state (otherwise it stays stuck on the spinner).
	if a.networkMonitor != nil && !a.networkMonitor.IsConnected() {
		log.Info().Msg("Skipping complete sync — offline")
		accounts, listErr := a.accountStore.List()
		if listErr == nil {
			for _, acc := range accounts {
				if !acc.Enabled {
					continue
				}
				inbox, inboxErr := a.folderStore.GetByType(acc.ID, folder.TypeInbox)
				if inboxErr == nil && inbox != nil {
					wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
						"accountId": acc.ID,
						"folderId":  inbox.ID,
					})
				}
			}
		}
		return nil
	}

	log.Info().Msg("Starting complete sync of all accounts and contacts")

	accounts, err := a.accountStore.List()
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	var errors []string

	// First: Sync each email account (sequentially to avoid overwhelming IMAP)
	// Email sync is the primary use case and runs without database contention
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}

		// Check if sync was cancelled between accounts
		a.syncMu.Lock()
		cancelled := a.syncCancelled
		a.syncMu.Unlock()
		if cancelled {
			log.Info().Msg("Sync cancelled, stopping account loop")
			break
		}

		if err := a.SyncAccountComplete(acc.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", acc.Email, err))
			// Continue with other accounts
		}
	}

	// Then: Sync CardDAV contacts (after email sync completes)
	// This avoids SQLITE_BUSY errors from concurrent writes
	a.syncMu.Lock()
	cancelled := a.syncCancelled
	a.syncMu.Unlock()
	if !cancelled {
		if err := a.SyncAllContactSources(); err != nil {
			errors = append(errors, fmt.Sprintf("contacts: %v", err))
		}
	}

	// Restart IDLE connections — they may have died during network changes
	// or exhausted their reconnect attempts. StartAccount is a no-op for
	// accounts that already have a healthy IDLE connection.
	a.restartIDLE()

	if len(errors) > 0 {
		return fmt.Errorf("sync errors: %s", strings.Join(errors, "; "))
	}

	log.Info().Msg("Complete sync of all accounts and contacts finished")
	return nil
}

// syncSlot is one registered folder sync. Its pointer is the ownership token
// release checks, so a replaced sync never removes its successor's entry.
type syncSlot struct {
	key    string // "accountID:folderID"
	cancel context.CancelFunc
	done   chan struct{} // closed by the slot's release

	cancelledAt time.Time // set by cancelSlotLocked
}

// cancelSlotLocked cancels slot and records it in cancelledSlots until its
// release, so shutdownSyncs still waits for it once it leaves syncContexts.
// Every cancel of a registered slot goes through here. The caller holds syncMu.
func (a *App) cancelSlotLocked(slot *syncSlot) {
	slot.cancel()
	slot.cancelledAt = time.Now()
	if a.cancelledSlots == nil {
		a.cancelledSlots = make(map[*syncSlot]struct{})
	}
	a.cancelledSlots[slot] = struct{}{}
}

// slotReleaseWait caps how long SyncFolder waits for a cancelled sync to
// release its slot before replacing it anyway.
const slotReleaseWait = 5 * time.Second

// pendingSlotLocked returns the slot that holds key: the registered one, or
// one cancelled less than slotReleaseWait ago that has not released yet, so a
// new sync doesn't overlap the winding-down one. A slot stuck past that bound
// no longer counts. It returns nil when key is free. The caller holds syncMu.
func (a *App) pendingSlotLocked(key string) *syncSlot {
	if slot, ok := a.syncContexts[key]; ok {
		return slot
	}
	for slot := range a.cancelledSlots {
		if slot.key == key && time.Since(slot.cancelledAt) < slotReleaseWait {
			return slot
		}
	}
	return nil
}

// folderBusyLocked reports whether a sync holds key (see pendingSlotLocked).
// The caller holds syncMu.
func (a *App) folderBusyLocked(key string) bool {
	return a.pendingSlotLocked(key) != nil
}

// beginFolderSync claims the sync slot for key unless a sync already holds
// it. release cancels ctx and frees the slot if this call still owns it.
func (a *App) beginFolderSync(key string) (ctx context.Context, release func(), ok bool) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.shuttingDown {
		return nil, nil, false
	}
	if a.folderBusyLocked(key) {
		return nil, nil, false
	}
	ctx, release = a.registerFolderSyncLocked(key)
	return ctx, release, true
}

// registerFolderSyncLocked registers a sync under key, cancelling and
// replacing any existing entry, and returns its context and release func. The
// caller holds syncMu.
func (a *App) registerFolderSyncLocked(key string) (context.Context, func()) {
	if old, exists := a.syncContexts[key]; exists {
		a.cancelSlotLocked(old)
	}
	ctx, cancel := context.WithCancel(a.ctx)
	slot := &syncSlot{key: key, cancel: cancel, done: make(chan struct{})}
	a.syncContexts[key] = slot
	release := gosync.OnceFunc(func() {
		cancel()
		a.syncMu.Lock()
		defer a.syncMu.Unlock()
		if a.syncContexts[key] == slot {
			delete(a.syncContexts, key)
		}
		delete(a.cancelledSlots, slot)
		close(slot.done)
	})
	return ctx, release
}

// replaceFolderSyncLocked cancels the sync registered under key, if any, and
// waits up to maxWait for it, and for a cancelled sync of key still winding
// down, to release its slot, so the syncs don't overlap; then it registers a
// new sync as registerFolderSyncLocked does.
// The caller holds syncMu, which is dropped while waiting. ok is false when
// shutdown has begun, before or during the wait; nothing is registered then.
func (a *App) replaceFolderSyncLocked(key string, maxWait time.Duration) (ctx context.Context, release func(), ok bool) {
	if a.shuttingDown {
		return nil, nil, false
	}
	timeout := time.NewTimer(maxWait)
	defer timeout.Stop()
	// Re-check after each wait: another sync may have claimed the slot.
	for old := a.pendingSlotLocked(key); old != nil; old = a.pendingSlotLocked(key) {
		if a.syncContexts[key] == old {
			a.cancelSlotLocked(old)
		}
		a.syncMu.Unlock()
		select {
		case <-old.done:
			a.syncMu.Lock()
		case <-timeout.C:
			a.syncMu.Lock()
			return a.registerIfRunningLocked(key)
		}
		if a.shuttingDown {
			return nil, nil, false
		}
	}
	return a.registerIfRunningLocked(key)
}

// registerIfRunningLocked registers a sync unless shutdown has begun. The
// caller holds syncMu.
func (a *App) registerIfRunningLocked(key string) (context.Context, func(), bool) {
	if a.shuttingDown {
		return nil, nil, false
	}
	ctx, release := a.registerFolderSyncLocked(key)
	return ctx, release, true
}

// trackSync registers a sync that runs without a slot, so shutdownSyncs
// waits for it; done must be called when it ends. ok is false once shutdown
// has begun, and the sync must not run.
func (a *App) trackSync() (done func(), ok bool) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	return a.trackSyncLocked()
}

// trackSyncUnlessBusy is trackSync for work that defers to a running sync of
// key: busy reports such a sync, and then done is nil. A nil done without
// busy means shutdown has begun.
func (a *App) trackSyncUnlessBusy(key string) (done func(), busy bool) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if !a.shuttingDown && a.folderBusyLocked(key) {
		return nil, true
	}
	done, _ = a.trackSyncLocked()
	return done, false
}

func (a *App) trackSyncLocked() (done func(), ok bool) {
	if a.shuttingDown {
		return nil, false
	}
	a.syncWork.Add(1)
	return a.syncWork.Done, true
}

// isShuttingDown reports whether shutdownSyncs has begun.
func (a *App) isShuttingDown() bool {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	return a.shuttingDown
}

// shutdownSyncs stops every running folder sync before the IMAP pool and the
// database close. It refuses new syncs, cancels the running ones, and waits up
// to wait for their slots to release and for tracked syncs (trackSync) to
// end, logging any that don't. Tracked syncs run on a.ctx, so the pool
// closing is what stops them.
func (a *App) shutdownSyncs(wait time.Duration) {
	log := logging.WithComponent("app")

	a.syncMu.Lock()
	a.shuttingDown = true
	pending := make([]*syncSlot, 0, len(a.syncContexts)+len(a.cancelledSlots))
	for _, slot := range a.syncContexts {
		pending = append(pending, slot)
	}
	for slot := range a.cancelledSlots {
		if a.syncContexts[slot.key] != slot { // not listed above
			pending = append(pending, slot)
		}
	}
	a.syncMu.Unlock()

	a.CancelAllSyncs()

	// Refuse new connections now, so retries inside cancelled syncs fail fast.
	if a.imapPool != nil {
		a.imapPool.Close()
	}

	// A context, not time.After: its Done stays closed for every wait below.
	waitCtx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for _, slot := range pending {
		select {
		case <-slot.done:
		case <-waitCtx.Done():
			select {
			case <-slot.done:
			default:
				log.Warn().Str("syncKey", slot.key).Msg("Sync did not stop before shutdown")
			}
		}
	}

	tracked := make(chan struct{})
	go func() {
		a.syncWork.Wait()
		close(tracked)
	}()
	select {
	case <-tracked:
	case <-waitCtx.Done():
		log.Warn().Msg("Syncs without a slot did not stop before shutdown")
	}
}

// syncPeriodDays returns the account's sync period, or the 30-day default
// when the account can't be read. 0 means all messages.
func (a *App) syncPeriodDays(accountID string) int {
	acc, err := a.accountStore.Get(accountID)
	if err != nil || acc == nil {
		return 30
	}
	return acc.SyncPeriodDays
}

// CancelFolderSync cancels a running sync for a specific folder
func (a *App) CancelFolderSync(accountID, folderID string) {
	log := logging.WithComponent("app")
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	syncKey := accountID + ":" + folderID
	if slot, exists := a.syncContexts[syncKey]; exists {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling folder sync")
		a.cancelSlotLocked(slot)
		delete(a.syncContexts, syncKey)
	}
}

// CancelAccountSync cancels any running syncs for the specified account (all folders)
func (a *App) CancelAccountSync(accountID string) {
	log := logging.WithComponent("app")
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	// Find and cancel all syncs for this account (keys are "accountID:folderID")
	prefix := accountID + ":"
	for key, slot := range a.syncContexts {
		if strings.HasPrefix(key, prefix) {
			log.Info().Str("syncKey", key).Msg("Cancelling folder sync")
			a.cancelSlotLocked(slot)
			delete(a.syncContexts, key)
		}
	}

}

// CancelAllSyncs cancels all running syncs and force-closes pool connections.
// Force-closing is needed because context cancellation cannot interrupt blocked
// TCP reads on dead sockets (e.g., after network changes). ForceClose kills the
// sockets immediately so goroutines unblock and emit folder:synced events.
func (a *App) CancelAllSyncs() {
	log := logging.WithComponent("app")
	a.syncMu.Lock()

	// Set cancellation flag so SyncAllComplete/SyncAccountComplete loops stop
	a.syncCancelled = true

	for syncKey, slot := range a.syncContexts {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling sync")
		a.cancelSlotLocked(slot)
	}
	a.syncContexts = make(map[string]*syncSlot)

	a.syncMu.Unlock()

	// Force-close all pool connections to unblock goroutines stuck on dead
	// TCP sockets. This uses ForceClose (no graceful logout) so it returns
	// instantly even if connections are unresponsive.
	if a.imapPool != nil {
		a.imapPool.CloseAll()
	}
}
