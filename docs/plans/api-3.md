# API-3 — Snapshot restoration implementation

[API-3](../../PLAN.md#api-3) / [F5b-1](../../ISSUES.md#f5b-1). Authorized 2026-10-14 by the user selecting API-3 in this session. Branch `api-3-snapshot-restore`; base `6156c3d`. Implements only the approved [restore design](../API_SNAPSHOT_RESTORE_DESIGN.md). Excluded: live/native restore, real user databases, provider actions, dependency or application upgrades.

## Scope and approach

New package `internal/snapshotrestore` (preflight, intent, startup apply with phases and fault injection, post-restore rules, undo through the pre-restore backup). App methods expose preview/stage/result/undo; `main.go` applies a staged intent after the single-instance lock and before any database read. Frontend: consent dialog in the account Mail API section.

## Progress

- [x] Package: preflight, intent, phases, crash recovery, post rules + tests
- [x] App wiring, main.go startup hook, bindings
- [x] Frontend consent dialog + backend-free UI regression (Accounts expanded/collapsed)
- [x] Docs (MAIL_APIS), validation record, issue/plan reconcile
- [x] Checks, acceptance, simplify, commit

## Acceptance and proving checks

“Isolated restore/crash fixtures and UI verify approved recovery and loss semantics; provider-side changes remain accurately disclosed.” Proof: `go test ./internal/snapshotrestore ./app -run 'Restore|Migration'`; frontend `npm run lint`, `npm run check`, `npm run build`, `make test-ui`. Live migration/restore stays unauthorized.

## Decision Log

- No account-level needs-reauthorization state exists; restore records affected accounts in `restore-result.json` and the result notice, and leaves account `enabled` untouched so the existing auth-failure path prompts. (Deviation from "mark" in the design; schema change not warranted.)

- Mailven quits after staging and the user reopens it; auto-relaunch would race the single-instance lock.
- No "Rules" count (no rules table). Held queue: API actions become `unknown`; submission attempts are only counted, never auto-sent.
- Review found a rerun defect: a crash between the post-restore commit and intent removal re-ran the rules and reported 0 held actions. Fixed by counting held actions by their marker; `TestFinishPostRerunAfterCrashIsStable` covers it.

## Checkpoint

Done 2026-10-14: implemented, checked, acceptance PASS, committed on `api-3-snapshot-restore`. [Validation](../validation/api-3.md). No blockers; native and real-account restore remain unrun.
