# API-3 snapshot restoration validation — 2026-10-14

Base `6156c3d`; branch `api-3-snapshot-restore`. [Milestone](../../PLAN.md#api-3), [checkpoint](../plans/api-3.md), [design](../API_SNAPSHOT_RESTORE_DESIGN.md), [behavior](../MAIL_APIS.md#snapshot-restoration).

## Implemented

`internal/snapshotrestore` (preflight, intent, phased apply, post-restore rules), `app/snapshot_restore.go` (Wails methods), startup hook in `main.go`, `FinishPost` in `App.Preflight`, consent dialog in the account Mail API section and a result banner with undo.

## Passed

- Fixtures (`internal/snapshotrestore`): preflight report and refusals (missing file, missing/wrong manifest, hash mismatch, corrupt, newer schema, account absent); older schema migrated on a copy only; credential key present, missing and mismatched; full restore with post-restore rules; crash at 8 phase boundaries then resume with the live database whole at each point; crash between post-restore commit and intent removal reruns with an identical result; stale WAL not replayed; database in use aborts; unwritable backup aborts; WAL without database refused; corrupt intent set aside; undo through the pre-restore backup; second stage refused.
- App (`app/snapshot_restore_test.go`): no migration record, snapshot outside the managed directory and missing acknowledgement are refused without staging; undo without a restore is refused.
- UI (`tests/ui/snapshot-restore.test.cjs`, Accounts expanded and collapsed): consent lists every-account scope, per-account counts, added accounts, that provider changes are not undone and re-sync, backup location and that Mailven closes; the dated restore button stays disabled until acknowledged; cancel calls nothing. The result banner reports held actions, unsent submissions, sign-in and orphaned credentials, and undo requires its own acknowledgement.
- `python3 tools/project-check.py --base 6156c3d --extra-command '["make","lint"]' --run`: whitespace, doc links, frontend lint/check/build, `make test`, `make test-ui` (full suite) and `make lint` pass.
- Independent `project-acceptance-checker`: PASS on all five points of the criterion. Its one gap (no crash inside `FinishPost`) led to the rerun fix and fixture above.
- Direct review (no code-review tool available in this session): rerun defect fixed; UI test selector simplified. No other changes warranted.

## Deviations from the design

- Mailven quits and the user reopens it; it does not relaunch itself, because a relaunch races the single-instance lock. The consent copy says so.
- No account-level needs-reauthorization state exists, so affected accounts are recorded in `restore-result.json` and the banner; `enabled` is untouched and no schema changed.
- The "Rules" count was dropped (no rules table). Held queue means API actions become `unknown`; submission attempts are counted and never auto-sent.

## Not verified

Native-window restore, a real account, Windows replace semantics and Flatpak data paths. Live and native restore stay unauthorized; fixtures prove the file and database behavior only.
