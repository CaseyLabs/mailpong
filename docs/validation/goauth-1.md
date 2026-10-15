# GOAUTH-1 — Disabled Google API diagnosis

Owner selected GOAUTH-1 on 2026-10-15. [Roadmap](../../PLAN.md#goauth-1); issues [GO1](../../ISSUES.md#go1) and [GO2](../../ISSUES.md#go2); pre-commit base `b6b0fbd`, branch `main`.

## Result

- `internal/kit/googleerr` (generic, importable by extensions) parses a Google `403` carrying `google.rpc.ErrorInfo` with `reason: SERVICE_DISABLED` into `*DisabledError{Service, Project}`. Service must match `*.googleapis.com` and the consumer a project ID or number; anything else is not classified. The Enable link (`console.developers.google.com/apis/api/<service>/overview?project=<project>`) is built from those validated fields, never copied from the response (`activationUrl` and message text are ignored). `Snippet` bounds and flattens bodies for error text.
- Gmail send-as (`internal/gmailapi/sendas.go`), People search and sync (`internal/contact/google.go`, `google_sync.go`), the contacts write path (`extensions/contacts/backend/google_write.go`) and every Calendar request (`extensions/calendar/backend/provider_google.go`, via `googleStatusError`) return the typed error. Sender review shows "Gmail API is disabled in Google Cloud project N. Enable it at …" instead of the re-authorize hint, which now appears only for scope denials.
- People search keeps returning empty results on a scope-denied 403, as before; only a disabled API now surfaces as an error.
- Docs: PROVIDERS.md, gmail.md and the custom-credentials guide say Gmail API must be enabled for every Google OAuth account, because sender review reads the send-as list.

## Verification

- `go test ./internal/kit/googleerr ./internal/gmailapi ./internal/contact ./extensions/calendar/... ./extensions/contacts/...`: pass. Table cases cover Gmail, People and Calendar disabled responses, missing scope, rate limit, malformed service/project, and a hostile `activationUrl`.
- `go test ./app -run 'SendAs|Sender'`: pass (disabled API message and link; scope denial keeps its message).
- `make lint`: clean. `python3 tools/check-doc-links.py --changed`: 0 errors. `make test` via `tools/project-check.py --run`: pass.

## Limits

Local fixtures only; no real disabled-API response was exercised in this milestone (the 2026-10-15 SEND-2 observation is the model for the fixture shape). The message is plain text in a toast: the Enable URL is shown, not a clickable link. GOAUTH-2 can add a clickable surface.
