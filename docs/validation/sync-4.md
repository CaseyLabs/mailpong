# SYNC-4 validation — 2026-10-14

[SYNC-4](../../PLAN.md#sync-4) / [R2](../../ISSUES.md#r2), [SY3](../../ISSUES.md#sy3), [SY4](../../ISSUES.md#sy4), [SY5](../../ISSUES.md#sy5). Base `92b8eb9`.

## Measurements and decisions

- **R2, Sent-copy reconciliation.** Each send ran `ReconcileCopies`, whose observation UPDATE and result query covered every accepted attempt of the account, so cost grew with the journal. `saveSentCopies` now calls `Store.ReconcileAttempt(accountID, attemptID)`, which scopes both statements to the new attempt (`(?='' OR attempt_id=?)`). `ReconcileCopies` keeps the whole-account scan for `GetSubmissionCopies`, a user-requested view.
- **SY4, Gmail reconcile memory.** The ID slice must stay: nothing commits until enumeration completes. The duplicate `map[string]bool` of every remote ID is replaced by `localOnly`, which sorts the slice in place and binary-searches it. `BenchmarkGmailDiff` (1M remote IDs, 10k vanished, `-benchtime 3x`): map 56.6 MB/op, 162 ms; sorted 16.7 MB/op including a 16 MB benchmark-only clone of the slice, 75 ms.
- **SY3, Graph delta reconcile.** `deltaRun` filled a `seen` map on every run, though only a reconcile reads it. It now collects IDs into a slice only when `start == ""` and diffs with `localOnly`. Verifying each missed local ID with a GET stays: it is what keeps a moved message from being deleted, and its count is bounded by the number of missed IDs.
- **SY5, Gmail reset requests and 5xx.** Policy, not a defect. The reconcile list returns IDs only, so comparing versions costs one `minimal` GET per known message, with no raw fetch for unchanged ones. A 5xx is treated like a throttle: at most `apiRetryLimit` (3) retries with capped waits, then the error returns and no authorization failure is raised. Both are now pinned by tests; no code change.

## Evidence

- `TestReconcileAttemptScope`: three accepted attempts with synced copies; `ReconcileAttempt` returns and observes only the named one, the others stay `pending`, and a full `ReconcileCopies` then observes all.
- `TestLocalOnly` (empty, all remote, none remote, duplicates); existing Gmail and Graph reconcile, move and removal tests still pass.
- `TestGmailAPIResetRequestCost`: 25 known messages on a reset make exactly 25 minimal GETs and no raw GET. `TestGmailAPIServerErrorBounded`: a persistent 503 makes `apiRetryLimit+1` attempts and no authorization failure.
- `go test -race ./internal/sync ./internal/submission` — PASS; `go vet ./internal/sync ./internal/submission ./app` — clean.

## Limits

Synthetic fixtures and an in-process benchmark only; no live mailbox benchmarking. The benchmark covers the ID diff, not the end-to-end memory of a real sync. The Graph change has no dedicated memory measurement; it reuses the benchmarked `localOnly` and the existing reconcile fixtures. The remaining `remote` ID slice still grows with mailbox size.
