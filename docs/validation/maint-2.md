# MAINT-2 validation — 2026-10-14

[MAINT-2](../../PLAN.md#maint-2) / MA2, SY1 (resolved). Base `db18d02`.

## Behavior

- `internal/extensions/store.go` is gofmt-clean (blank line before `OpenStore`).
- `internal/apiread` owns the read-client GET: `Error` (the classified failure), `Client.Get` (401-once token refresh, response cap, `Retry-After`) and `ParseRetryAfter`. 429 and 5xx classify as rate limits; 401 and 403 classify as reauthorization and policy failures; a 403 is a rate limit only when the provider's `ForbiddenRateLimited` says so.
- `gmailapi.Reader.get` and `graphapi.Reader.fetch` configure it: Gmail sends `Accept`, a 40 MiB cap, names the path in transport errors and passes `isRateLimit` for 403; Graph sends `Prefer` and wraps `errTooLarge`. `gmailapi.ReadError` and `graphapi.ReadError` are aliases of `apiread.Error`, so `internal/sync` is unchanged. Error text is unchanged.

## Evidence

- `TestGet` (`internal/apiread`): ok, refresh once, repeated 401, 429 with `Retry-After`, 503, 403 policy, 403 quota, over-cap with the sentinel wrapped.
- `go test ./internal/apiread ./internal/gmailapi ./internal/graphapi ./internal/sync` and `make test` pass; the existing provider read and sync tests pass unmodified. `go vet ./...` and `gofmt -l internal app extensions` are clean.

## Limits

Fixtures only; no live provider calls. `apiwrite` keeps its own `parseRetryAfter` and classification (the write path is not part of SY1).
