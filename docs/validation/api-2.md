# API-2 design validation — 2026-10-14

Base `ea0c754`; branch `api-2-snapshot-restore-design`. [Milestone](../../PLAN.md#api-2), [checkpoint](../plans/api-2.md), [design](../API_SNAPSHOT_RESTORE_DESIGN.md).

## Passed

- Read-only source review: `internal/apimigrate` `Snapshot` (whole-database `VACUUM INTO`), `internal/database` `Open`/`Migrate`/`ErrSchemaTooNew`, `internal/credentials` store, `internal/platform` paths, `app/api_migration.go` rollback, single-instance lock in `main.go`.
- `python3 tools/project-check.py --base ea0c754 --run`: whitespace PASS, docs links PASS (6 paths, 0 errors).
- Independent `project-acceptance-checker`: content PASS for the local-restore/remote distinction, database replacement, credentials and data-loss UX; validation criterion PASS; all code claims verified; no stubs. It flagged that the single-instance lock does not cover detached composers (separate processes on the same database), and the design now requires a refusal while a composer is connected plus a verified exclusive-access check at startup. It also flagged “Approved” as unmet; the owner approved the design on 2026-10-14 when asked.

## Limits

Documentation only: builds, Go/frontend tests and UI checks unrun. No database replacement, fault injection, credential change, provider call or migration. Windows replace semantics, Flatpak paths and crash behavior are design assumptions until API-3 fixtures run. F5b-1 stays open until API-3.
