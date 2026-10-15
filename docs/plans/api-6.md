# API-6 — Shared mailbox API route

[API-6](../../PLAN.md#api-6) / [MA4](../../ISSUES.md#ma4), MA5 (resolved); [MA1](../../ISSUES.md#ma1) stays the sending privacy gate. Design: [Shared mailbox API design](../SHARED_MAILBOX_API_DESIGN.md). Base `1ff3d26`, branch `api-6-shared-mailbox-route`. Recorded authorization: user selected API-6 on 2026-10-14 via /project-next-milestone. Local implementation only: no live provider access, credentials change or tenant check is authorized, so shared migration/activation must not be exposed in the UI (live gate).

## Progress

- [x] 1. Migration 62 (`mailbox_address`, `grant_account_id`), `MailAPIRoute` fields, store validation, dependents query, rewind lines, tests.
- [x] 2. Owner resolver `routeGrantOwner` and token/status/clear wiring through the owner slot (B9, B12); owner clear refused while children reference it.
- [x] 3. Consent for a linked child (B1, B2, D3 scope upgrade, identity against the parent).
- [x] 4. Sync, read, write, send wiring (B3, B5, B6, B8) with fake-Graph fixtures.
- [x] 5. Plan blockers (B7), activation gate (B4) held closed by the live gate, MA5 lifecycle fix (parent removal).
- [x] 6. UI: child section message and `make test-ui` regression (52 passed). Sender-review wording unchanged: no live Sent/Bcc evidence, MA1 stays the gate.
- [x] 7. Docs, validation record, finish workflow.

## Decision Log

Live gate: activation/migration of a shared route stays refused by a single recorded-evidence switch until live evidence is recorded in `docs/validation/`; fixtures drive the code path through a test-only override.

## Checkpoint

Current step: done. All checks passed (Go targeted + make test, make lint, make test-ui 52 passed, doc links, diff --check); independent acceptance: all criteria pass, live checks unverifiable by design.

Owner decision 2026-10-14: remaining Microsoft 365 and regular/personal Outlook.com work, including shared-mailbox live validation and rollout, is deferred. Local implementation and prior evidence remain preserved; explicit reopening is required. See [roadmap](../../PLAN.md#api-6).
