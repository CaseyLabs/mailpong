# SYNC-2 validation — 2026-10-14

[SYNC-2](../../PLAN.md#sync-2) / [R4](../../ISSUES.md#r4). Base `d2ea490`.

## Behavior

`app/api_actions.go` `apiMove` now returns the messages it queued before any failure. `apiMoveLocal` emits `messages:moved` and refreshes folder counts for those rows, then returns the batch error, so the caller still sees the failure. If nothing was moved it returns the error with no events.

## Evidence

- `TestAPIMovePartialBatchRefreshesMovedRowsAndReportsFailure`: a two-row batch whose second row has a missing source folder emits `messages:moved` for the first row only, moves it locally and returns the second row's error.
- `go test ./app` — PASS; `go vet ./app` — clean.

## Limits

Backend-only change; the frontend is unchanged, and the guarded Mail UI check (Accounts expanded/collapsed) was not run. Local fixtures only; no live Gmail or Graph access.
