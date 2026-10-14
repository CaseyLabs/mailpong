# SYNC-3 validation — 2026-10-14

[SYNC-3](../../PLAN.md#sync-3) / [SY2](../../ISSUES.md#sy2), [SY6](../../ISSUES.md#sy6). Base `7a4728f`.

## Behavior

- **Oversize Graph message (SY2).** `graphapi.Reader.GetRaw` returns `ErrMessageTooLarge` past `MaxRawBytes` (default 64 MiB) instead of a generic error. `GraphSync.process` then keeps the message as a header-only row built from metadata (`subject`, `from`, `receivedDateTime` are now selected), logs a warning and counts it in `GraphResult.Oversize`. The folder cursor advances; the row is unfetched, and opening it reports "this message is too large to download" (`app/api_read.go`).
- **Missing Date (SY6).** `parseAPIRaw` takes the provider receive time (Gmail `internalDate`, Graph `receivedDateTime`) and applies it with `applyInternalDate`: it sets `ReceivedAt` and stands in for a missing `Date`. `time.Now()` remains only when the provider gives no time. On-demand body parsing (`ParseAPIBody`) passes none; `StoreAPIBody` does not touch dates.

## Evidence

- `TestGraphSyncOversizeMessageKeepsFolderMoving`: with a 2 KiB cap, a big and a small message both land, `Oversize == 1`, the cursor is stored, the big row has provider date and sender and is unfetched, and an idle rerun does nothing.
- `TestGraphSyncMissingDateUsesReceivedTime`, `TestGmailAPIMissingDateUsesInternalDate`: `Date` and `ReceivedAt` equal the provider time.
- `go test ./internal/sync ./internal/graphapi ./internal/gmailapi ./app/...` — PASS; `go vet` on those packages — clean.

## Limits

Rows ingested before this change keep their stored dates (no backfill). Fixtures only; no live Graph or Gmail access. The oversize cap path was exercised with a lowered cap, not a real 64 MiB message. No UI check run: the oversize row is not surfaced beyond the snippet and the open-time error.
