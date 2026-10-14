# API-5 — Rollback sync progress and coverage

[API-5](../../PLAN.md#api-5) / [F5b-3](../../ISSUES.md#f5b-3). Authorized 2026-10-14: user requested the recommended milestone after API-5 was recommended. Branch `api-5-rollback-sync`; base `0bc3916`.

## Scope and contract

Rollback synchronously restores the protocol route and visible IMAP rows. IMAP reconciliation is requested through the scheduler and runs separately; rollback does not promise sync start, completion or success. Add a narrow scheduler request seam and a report flag indicating only that TriggerSync was invoked. UI copy distinguishes that request from completion, including no scheduler/no request. Keep snapshot restore, API action replay and live migration outside scope.

## Progress

- [x] Trace rollback, deactivation, scheduler semantics and UI harness.
- [x] Add request seam/report flag and backend regression cases.
- [x] Update English UI copy, bindings and docs; add backend-free UI regression under Accounts expanded/collapsed.
- [x] Run checks, independent acceptance, simplify/review and reconcile status.
- [x] Milestone ready for commit/landing under repository policy.

## Acceptance and proving checks

“A test asserts rollback triggers IMAP sync; UI and API distinguish route transition from asynchronous reconcile completion.” Prove with `go test ./app -run 'Migration|APIRoute'`, including success ordering/exact trigger and unavailable/failure cases. Frontend: `npm run lint`, `npm run check`, `npm run build`; focused `node tests/ui/run.mjs mail-api-migration`, then full `make test-ui`. Binding change: stop Mailven (already down), `make generate`, revert unrelated runtime churn and remove whitespace-only models churn. Go app package checks cover call sites; broaden only if unresolved risks remain. Guarded fake UI supplies Mail Accounts expanded/collapsed; native/live checks are not required or authorized.

## Surprises & Discoveries

TriggerSync returns no success/completion status and launches a goroutine; the report can truthfully promise only invocation. The standalone migration UI tool is not in the standard regression runner; add the rollback contract there.

## Decision Log

Keep concrete global scheduler unchanged; inject only the route-deactivation request boundary. A syncRequested report flag must not be called syncStarted or syncCompleted. Failed or already inactive transitions must not report a new request.

## Checkpoint

Current step: complete; ready for milestone commit/landing. Exact checks: `go test ./app -run 'Migration|APIRoute'` PASS (32.367s); `make generate` PASS (non-fatal time.Time generator diagnostics), unrelated runtime modes restored and existing model whitespace preserved; focused `node tests/ui/run.mjs mail-api-migration` PASS (3/3); frontend `npm run lint` PASS, `npm run check` PASS (0 errors/warnings), `npm run build` PASS; full `make test-ui` PASS (50/50, zero skipped, 84.065s). `git diff --check` PASS. Independent native acceptance PASS, no findings/stubs; direct correctness review substituted for a separate code-review tool. Simplify review removed the temporary UI type cast after generated bindings; no further change warranted. Broad Go checks unrun: focused Migration/APIRoute covers all changed deactivation callers, including clear/failure rollback. Guarded Mail UI Accounts expanded/collapsed passed; native/live verification unrun, no provider actions. Changed files and durable evidence: [validation](../validation/api-5.md). No blockers. No milestone active; next action is selection in a fresh session.

## Outcomes & Retrospective

F5b-3 resolved with disposable fixtures and truthful request-only reporting. No new confirmed finding left unfixed; existing unrelated issues remain unchanged. Scope stayed API-5. No application/dependency upgrade or live migration.
