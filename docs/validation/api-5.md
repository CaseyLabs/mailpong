# API-5 rollback verification — 2026-10-14

[API-5](../../PLAN.md#api-5) / [F5b-3](../../ISSUES.md#f5b-3), [checkpoint](../plans/api-5.md). Base `0bc3916`; branch `api-5-rollback-sync`.

## Behavior and proofs

Rollback restores the staged protocol route and visible IMAP rows synchronously. `MailAPIRollbackReport.syncRequested` means only that the scheduler trigger was invoked. It does not establish start, completion or success. A missing scheduler or already inactive route reports no new request. Folder-transition failure preserves the active API route and makes no request.

`TestMigrationRollbackRequestsSyncAfterRouteTransition` verifies account ID, staged route, visible IMAP rows and one trigger; simulated reconciliation stays unfinished when rollback returns, and repeated rollback does not trigger again. Table-driven `TestMigrationRollbackSyncRequestLimits` covers absent scheduler, inactive route and injected folder swap failure. Existing focused tests retain queue, snapshot, migration retry, deactivation and clear behavior.

The standard UI rollback suite verifies cancel, deferred pending state, disabled confirmation, requested/unrequested copy and the protocol route display under fake data. Expanded/collapsed Accounts cases and the full Mail audit matrix passed. The UI never treats request invocation as reconciliation completion; an unrequested sync points to manual Sync.

## Commands and results

- `go test ./app -run 'Migration|APIRoute'`: PASS, 32.367s.
- `make generate`: PASS; Wails emitted non-fatal existing `Not found: time.Time` diagnostics. Reverted unrelated runtime mode churn; preserved baseline model blank-line whitespace. Generated bindings retain only the two new syncRequested field/constructor lines.
- In `frontend/`, `npm run lint`: PASS; `npm run check`: PASS, 0 errors/0 warnings; `npm run build`: PASS. Logs: `/tmp/mailven-api-5-{lint,check,build}.log` (ephemeral).
- `node tests/ui/run.mjs mail-api-migration`: PASS, 3/3.
- `make test-ui`: PASS, 50/50, 0 failed/skipped, 84.065s. Summary: `test-results/ui/results.json`; ephemeral log `/tmp/mailven-api-5-ui.log`.
- `git diff --check`: PASS. Local Markdown path/anchor scan: PASS (287 links; site `/docs/` routes resolved to user-guide sources). An initial scanner treated three existing site routes as literal filesystem paths; correcting that mapping resolved the false positives. `git diff --name-only -- archive LICENSE`: empty.
- Independent native `project-acceptance-checker`: PASS on “A test asserts rollback triggers IMAP sync; UI and API distinguish route transition from asynchronous reconcile completion.” Read-only source review used the parent-run checks; no overlapping test execution and no findings/stubs.
- Direct diff/correctness review substituted for a separate code-review tool. Simplify review removed a temporary type cast after generation; no additional simplification warranted.

## Limits

All final checks passed; the initial documentation scanner false positives are explained above. Full Go suite, standalone legacy migration tool, native window/backend and real-provider tests were unrun: the focused Go checks cover changed app callers and the backend-free UI suite covers this UI contract. No live accounts, consent, credential edits, test mail, route switches or migration occurred. A scheduler request can still be skipped or fail asynchronously; this milestone deliberately promises no sync progress/completion from rollback. Snapshot restoration and provider action reversal remain unavailable. No new confirmed findings were left unfixed.
