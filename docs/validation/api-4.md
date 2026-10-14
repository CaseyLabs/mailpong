# API-4 validation — 2026-10-14

[API-4](../../PLAN.md#api-4) / F5b-2 (resolved). Base `732472f`.

## Behavior

`apimigrate.Store.Correlate` never guesses identity. An API row receives local state only when it has a Message-ID and exactly one IMAP row matches (Message-ID without angle brackets, date to the second, same folder type preferred) and no other API row claims that IMAP row. Otherwise it stays uncorrelated:

- API row without Message-ID: counted in `Uncorrelated` and the new `NoMessageID`. IMAP rows without Message-ID are never candidates.
- Several IMAP rows match: counted in `Ambiguous` (existing behavior).
- Several API rows match one IMAP row (duplicate API messages): all are counted in `Ambiguous` and none receives state. Previously each copied it.

Uncorrelated messages appear once from the API without cached body, answered/forwarded flags, verification results or attachments; the IMAP rows stay hidden and intact. User guidance: [MAIL_APIS](../MAIL_APIS.md), [PROVIDERS](../PROVIDERS.md).

## Evidence

- `TestCorrelateCarriesStateOnceAndKeepsIMAPRows` (existing) and `TestCorrelateLeavesAbsentAndSharedIdentityUncorrelated` (absent IDs, shared IMAP match, empty IMAP ID, exact counts, no carried state).
- `go test ./internal/apimigrate` — PASS; `gofmt -l` clean.

## Limits

Fixtures only; no live migration. The migration UI does not show the result counts, so no UI change or check.
