# SYNC-1 validation — 2026-10-14

[SYNC-1](../../PLAN.md#sync-1) / [R3](../../ISSUES.md#r3). Base `eae50f0`.

## Behavior

When a provider move returns a new remote ID and the local re-key (`ApplyAPIMove`) fails, `internal/apiactions/worker.go` `rekey` keeps the move action open: it records `MovedTo`/`MovedContainers` in the action payload (`message.Store.SetAPIActionPayload`) and reschedules it with backoff. Retries repeat only the re-key and never resend the provider request. Each retry counts as an attempt, so backoff grows to its cap. A retry that finds the old ref gone and the new ref holding the destination row treats the re-key as already done, so an unrecorded earlier success cannot stall the queue. Later queued actions for the message stay behind the open action, so none runs with the stale ID, receives a 404 or reaches `gone()`.

## Evidence

- `go test ./internal/apiactions ./internal/message ./internal/database` — PASS.
- `TestWorkerMoveRekeyRetryAcceptsUnrecordedRekey` covers an already re-keyed message with `MovedTo` set: the action settles as applied with no provider request.
- `go test ./internal/...` — PASS; `go vet ./internal/apiactions ./internal/message .` — clean.
- `TestWorkerMoveRekeyFailureRetriesWithoutDeleting` injects the fault (the new ID already exists locally): the move is sent once, the queued read action is held and the message row remains. After the fault is cleared and backoff elapses, the re-key succeeds and the read action runs as `read:r2`.

- Independent project-acceptance-checker: both criteria PASS. It reported two retry-path defects, a possible stall after an unrecorded re-key and backoff that did not grow; both were fixed and the tests rerun (PASS).
- Simplify review was done directly: the change is small and needed no behavior-preserving edits.

## Limits

Local fixtures only; no live Graph or Gmail access. If persisting the recorded new ID itself fails (a database error), the drain aborts and the action stays in flight; after a restart it is treated as unknown and verified against the provider by its old ID, which a Graph move may no longer answer. This narrow double fault is not handled. `ApplyAPIMove` is called with `FromFolderID` equal to `ToFolderID` (unchanged from before), because the local row already moved at enqueue time.
