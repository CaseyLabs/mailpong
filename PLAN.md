# Plan: issues round 3

## Scope

Fix these ISSUES.md entries:

- **M10**: syncs that are still running when the app shuts down.
- **M7**: chat state that is lost when combining is turned off.
- **M6**: Sent mail from other clients, fixed with IDLE on the Sent folder.

Then do the open checks:

- **M8**: chat-key timing.
- **M9**: running-app checks of the chat UI.

Out of scope:

- **S6**: left open by owner decision.
- **A4/C12**: architectural.
- **U1/U3**: upstream independence.
- **D2**: blocked on peer ranges.
- **C17**: not chosen for this round.

Remove each ISSUES.md entry in the same commit as its fix.

## Milestones

### 1. M10: stop syncs before the pool and DB close

`Shutdown` (`app/app.go`) closes `imapPool` and `db` while folder-sync goroutines can still be running:

- the body fetch in `SyncFolder` (`app/sync.go`)
- the IDLE body fetch (`app/background.go`)
- `syncSentAfterIdle` (`app/idle_sent.go`)

A fetch retry can then ask the pool for a connection, and its credential lookup reads a closed DB.

Changes:

- `internal/imap/pool.go`:
  - Add `Pool.Close()`. It marks the pool closed, then calls `CloseAll`.
  - `GetConnection` checks the closed flag before it looks up credentials, and returns a new `ErrPoolClosed`.
  - Leave `CloseAll` unchanged. `CancelAllSyncs` uses it on network changes, and the pool must stay usable after that.
- `app/sync.go`:
  - Add `shutdownSyncs(wait)`. Under `syncMu` it sets a `shuttingDown` flag and collects each slot's `done` channel. It then calls `CancelAllSyncs` and waits, bounded by `slotReleaseWait`, for the slots to release. It logs any slot that does not release.
  - `beginFolderSync` and `replaceFolderSyncLocked` refuse to start a sync while `shuttingDown` is set.
- `Shutdown`:
  - Call `shutdownSyncs` after the scheduler, IDLE and monitors stop, and before extensions are unregistered.
  - Replace `imapPool.CloseAll()` with `imapPool.Close()`.
- `app/compose.go` runs a Sent body fetch on `a.ctx` without a slot. Either run it in a slot, or confirm that `ErrPoolClosed` is enough to stop it.

Acceptance:

- After `Pool.Close`, `GetConnection` returns `ErrPoolClosed` without calling `getCredentials`.
- `shutdownSyncs` waits for a running slot to release, and returns within the bound when a slot never releases.
- No sync can start after shutdown begins.

Validation:

- Table-driven tests in `internal/imap/pool_test.go`, and an app test for `shutdownSyncs`.
- `go test ./internal/imap/ ./app/`, then `make test`.
- Manual: quit during a large sync, then check that the debug log has no `database is closed`.

### 2. M7: hand split sender-chat state to a thread

In `StaleChatKeys` (`internal/message/chat_store.go`), a sender chat whose sender no longer combines yields `ChatKeyMove{To: ""}`, so `MoveChatState` and `draftStore.MoveChatLink` drop its pin, snooze and draft link.

Changes:

- A split row resolves `To` to the thread key of the sender's newest inbox message. It matches the sender's address the same way `senderCombinedExpr` and `chatKeyRows` do. If the sender has no inbox mail, `To` stays empty and the state is dropped.
- Reuse `MoveChatState` as it is: state already on `To` wins.
- Check how `MoveChatLink` handles a link that already exists on `To`, and keep the target's link.
- Update the comments on `ChatKeyMove` and `moveStaleChatState` (`app/chat.go`).

Acceptance:

- Turning combining off for a sender moves the sender chat's pin, snooze and chat draft link to the thread of the sender's newest inbox message.
- State already on that thread is kept.
- When the sender has no inbox mail, the state is dropped, as before.

Validation:

- Table-driven cases in `internal/message/chat_store_test.go`, plus a draft-link case.
- `go test ./internal/message/ ./app/`.
- In the app: pin a combined sender chat, turn combining off, and check that the newest thread is pinned.

### 3. M6: IDLE on the Sent folder

IDLE watches INBOX only, so `app/idle_sent.go` infers Sent activity from inbox events and allows at most one Sent sync per 30 s. A new message sent from another client therefore waits for the next scheduled sync.

Changes:

- `internal/imap/idle.go` `IdleManager`:
  - Key connections by account and folder.
  - `StartAccount(accountID, accountName string, folders ...string)` starts one connection per folder, and watches INBOX when no folders are given.
  - `StopAccount` stops all of the account's connections.
  - `newIdleConnection` takes the folder.
- App call sites (`app/account.go`, `app/background.go`):
  - Pass INBOX and the Sent path from `GetSpecialFolder(accountID, folder.TypeSent)`.
  - On the first sync the Sent folder is not known yet. Call `RestartAccount` once the folder list sync finds it.
- `app/background.go` IDLE loop:
  - Route events of any type whose `Folder` is not INBOX to `syncSentAfterIdle`, debounced per account with `idleFlagResyncDebounce`.
  - These events must not reach the inbox handlers for new mail, expunge or flags. That way they raise no notifications and start no inbox syncs.
  - Remove the inferred `scheduleSentSyncAfterIdle` calls and the throttle state: `idleSentLast`, `idleSentPending` and `idleSentMinInterval`.
  - When a sync of the folder is already running, keep retrying, as a short re-debounce.
- Our own sends echo back through Sent IDLE, so each send runs one extra incremental sync next to the one in `compose.go`. This is acceptable.
- Update docs that describe IDLE scope or Sent sync latency (grep `docs/` and `README.md`).

Acceptance:

- While Aerion is idle, mail sent from another client appears in Sent and in its chat within seconds.
- Sent events never raise new-mail notifications or start inbox syncs.
- Each account opens exactly one extra IDLE connection.
- Removing an account stops both of its connections.

Validation:

- Unit tests for `IdleManager` keying and for start and stop.
- `make test`.
- Manual, with a real account: send from a web client, then watch the log and the chat list.

### 4. M8 timing

- Add a Go benchmark. It builds about 50k inbox messages with combining on by default, then times the chat list, count and new-mail lookups that go through `chatKeyRows`.
- If a call takes more than about 50 ms, add an index or cache the thread starters.
- Record the numbers in the Decision Log.

Acceptance:

- The benchmark numbers are recorded.
- The M8 entry is gone from ISSUES.md.

Validation:

- `go test -bench` for the new benchmark.
- `make test` and `make lint`.

### 5. M9 running-app checks

Needs a running app with Google OAuth credentials in `.env.local`. Kept last so the other milestones are not blocked.

With the `run-aerion` skill, check:

- light themes (built-in and Adwaita), including hover and selection
- row heights of 72–80px with a preview
- selection switching between solid and neutral on pane focus
- signed, encrypted and bad-signature messages
- the Settings > Chat combine switch
- the list giving way to the reading pane as the window narrows

Fix small defects here. File larger ones in ISSUES.md.

Acceptance:

- Every item is checked, and either passes or is fixed or filed.
- The M9 entry is gone from ISSUES.md.

Validation:

- `make test` and `make lint`.
- If the frontend changes, `npm run lint` and `npm run check` in `frontend/`.

## Progress

- [x] 1. M10: stop syncs before the pool and DB close (manual quit-during-sync log check still open; needs a running app)
- [x] 2. M7: hand split sender-chat state to a thread (in-app pin check still open; needs a running app)
- [x] 3. M6: IDLE on the Sent folder (real-account check still open; needs a running app and Google OAuth credentials in .env.local)
- [x] 4. M8 timing
- [x] 5. M9 running-app checks (done on fake data; real-mail leftovers in ISSUES M9)

## Surprises & Discoveries

- 2026-10-10: M10 review found syncs orphaned by an earlier `CancelAllSyncs` are not awaited by `shutdownSyncs`; logged as ISSUES M11. Declined: `SyncFolder` returning nil on shutdown (the Complete loops already stop on `syncCancelled`).
- 2026-10-10: M7 undo of a split restores the sender chat's state but does not take it back from the thread it moved to; accepted.

- 2026-10-10: Running-app checks (M10, M7, M6, and milestone 4) are blocked: the dev app has no Google OAuth client ID, so the only account cannot sign in. Owner chose to skip setting up `.env.local` for now.
- 2026-10-10: M9 ran on fake data (page-level binding fakes, writes blocked). Row heights, built-in and Adwaita light themes, solid/neutral selection on pane focus, signed/encrypted/bad-signature banners (S/MIME and PGP), the combine switch and the narrow list-only layout all passed; no defects. Resize had no effect, so the narrow layout was checked in a 700px same-origin iframe (mode `narrow`).

## Decision Log

- 2026-10-10: M7 target is the newest inbox thread the sender started (not the thread of their newest message): a thread another sender started maps into that sender's chat, so the state would not show or would move again at startup.
- 2026-10-10: M10 compose.go `syncSentFolder` now takes the Sent folder's slot (review fix): without one it raced the IDLE-triggered Sent sync and was not cancelled at shutdown. Slotless IDLE handlers and `SyncFolders` register with `trackSync`, which `shutdownSyncs` waits on within the same bound.
- 2026-10-10: M10 `replaceFolderSyncLocked` now returns `ok`; false means shutdown has begun and `SyncFolder` returns nil.
- 2026-10-10: Leave S6 (revocation checks) open by owner decision.
- 2026-10-10: For M6, use a second IDLE connection on Sent rather than a shorter Sent poll. It costs one extra connection per account. Inbox activity still syncs Sent, throttled to once per 30 s, only while the Sent IDLE connection is not running (IDLE unsupported or reconnecting).

- 2026-10-10: M8 timing (50k inbox messages, 16-core Ryzen, `go test -bench ChatQueries`): `ListChats` 382 ms combined / 319 ms split; `CountChats` 253 ms / 109 ms; `ListNewMail` (50 ids) 15 ms / 0.4 ms. Combining adds about 60 ms to the list, 140 ms to the count and 15 ms to new-mail lookups. No index or cache added: the window sort is an expression partition no index can serve, the baseline grouped query already costs over 300 ms, and a thread-starter cache is a schema change out of scope here. Filed as ISSUES M12.

## Outcomes & Retrospective

- M7: `StaleChatKeys` and the in-app split (`splitSenderChat`) hand state to `NewestInboxThreadKey` (newest thread the sender started); `MoveChatLink` already kept the target's link.
- M10: `Pool.Close` and `shutdownSyncs` land; `Close` also runs inside `shutdownSyncs` so retries fail fast, and a dial finishing after `Close` is force-closed.
- M9: no defects found on fake data; real-mail leftovers stay in ISSUES M9, with the real-account checks for M10, M7 and M6 (need Google OAuth credentials in `.env.local`).
