# API-2 — Snapshot restoration design

[API-2](../../PLAN.md#api-2) / [F5b-1](../../ISSUES.md#f5b-1). Design only. Authorized 2026-10-14: user selected API-2 when asked. Branch `api-2-snapshot-restore-design`; base `ea0c754`. No implementation, database replacement, credential change or live migration.

## Progress

- [x] Trace `Snapshot`, database open/WAL, credential storage, migration gate and paths.
- [x] Write [design](../API_SNAPSHOT_RESTORE_DESIGN.md): offline whole-database swap, crash recovery, credentials, consent UX, failure table, API-3 validation.
- [x] Update PLAN, ISSUES, MAIL_APIS, validation record.
- [x] Checks and independent acceptance.

## Acceptance and proving checks

“Approved design distinguishes local snapshot restore from undoing remote actions and covers active database replacement, credentials and data-loss UX.” Proof: the design's sections “What a restore is”, “Decision”, “Post-restore rules”, “Data-loss consent UX”; local link scan and `git diff --check`. Builds/tests/UI unrun: documentation only.

## Surprises & Discoveries

The snapshot is a whole-database copy, so restore cannot be per-account; every account rolls back. Stale WAL frames are the main replay hazard. Queued actions in the snapshot may already be applied remotely.

## Decision Log

Offline swap at startup via an intent file; no in-process swap, no row merge. Restore never touches keyring; queues are held, never auto-sent; pre-restore backup makes the restore undoable.

## Checkpoint

Design complete; see [validation](../validation/api-2.md). Owner approved the design 2026-10-14. Detached-composer exclusivity was added after acceptance review. No milestone active after commit.
