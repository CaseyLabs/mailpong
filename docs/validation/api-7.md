# API-7 validation — 2026-10-15

[API-7](../../PLAN.md#api-7) / [MA8](../../ISSUES.md#ma8), [MA7](../../ISSUES.md#ma7), [SY9](../../ISSUES.md#sy9). Base `730ddcf`. Fixtures only; no live account access was authorized.

## Decisions

- **MA8, settings on an API route.** Special-folder mappings name IMAP paths, and activation hides the IMAP rows. `folder.Store.ListHidden(accountID, folder.RowsIMAP)` returns the hidden rows with their original type and path. `App.mappingFolders` uses it while the route is active (falling back to the visible rows before an activation has swapped them); `UpdateAccount`, `GetAccountFoldersForMapping` and `GetAutoDetectedFolders` all read through it. `UpdateAccount` checks every non-empty path against that one list, so an unknown path is still rejected. An API-routed account no longer triggers an IMAP folder sync from the mapping dialog.
- **MA7, interrupted activation.** `account_mail_api_routes.activating` (migration 65, with rewind lines) is set before the route goes active and cleared once the outcome settles: a refusal, a revert, or a finished reconcile. A failed clear is logged, not returned. A revert whose swap back fails keeps the mark. At startup, before IDLE and the sync scheduler, `recoverInterruptedAPIActivations` returns every marked account to IMAP: it restores the IMAP rows if they are still hidden, sets the route to staged, and clears the mark. Every step is safe to repeat. Finishing the reconcile at startup was rejected: it would block startup on the network, and [MAIL_APIS](../MAIL_APIS.md) promises the account stays on IMAP when activation does not finish.
- **SY9, stuck account removal.** `imap.Client.ForceClose` sends LOGOUT in a goroutine and waits at most `forceCloseLogoutGrace` (500 ms) before closing the socket. Closing fails the stuck command, which releases the command lock that LOGOUT waits on. A healthy connection still sends LOGOUT at once. The pool calls `ForceClose` while holding its lock, so each connection with a stuck command can delay other pool callers by up to 500 ms. Before this fix, that delay had no limit.

## Evidence

- `app/api_route_settings_test.go` `TestUpdateAccountOnActiveAPIRouteKeepsIMAPMappings`: on an active route, a sync-period change saves and keeps `[Gmail]/Starred`; an unknown path is rejected; the mapping list and auto-detection return the IMAP rows; after deactivation the same mapping saves. Without the fix it fails with "starred folder not found: [Gmail]/Starred".
- `app/api_route_recovery_test.go`:
  - `TestRecoverInterruptedAPIActivationReturnsToIMAP` covers an interruption after each step: after the mark, after going active, during the reconcile, and after the folders returned. In every case recovery leaves the route staged, the IMAP rows visible and the mark cleared. A second recovery changes nothing.
  - `TestAPIActivationOutcomeClearsMark`: a refused activation and a successful one both leave no mark, and recovery keeps the finished activation active with the API rows visible.
- `internal/imap/force_close_test.go` `TestForceCloseDoesNotWaitForStuckCommand`: a raw TCP server never answers an APPEND. `ForceClose` returns in about 0.5 s, and the stuck APPEND returns. Without the fix, `ForceClose` blocks until the 5 s timeout.
- `go test -race` on the focused app, imap, folder and account tests: pass.
- `internal/database` migration rewind test: pass.
- Full run: `python3 tools/project-check.py --base main --extra-command '["make", "lint-go"]' --run` passed whitespace, docs, tooling, skill paths, brand, `make test` and `make lint-go` (run `run-ls61531s`).
- `internal/sync/imap_older_test.go` was gofmt-formatted; the SYNC-7 commit left it unformatted.

## Limits

Fixtures only. The live Gmail checks (Edit Account on the dev account, quitting during activation, removing an account during a sync) are unrun. A desktop check of the Edit Account dialog is also unrun. Startup recovery returns an interrupted activation to IMAP rather than finishing it, so the user activates again. The activation tests that wait out the real API backoff are slow ([TF2](../../ISSUES.md#tf2)).
