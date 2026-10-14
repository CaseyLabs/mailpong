# Provider implementation validation record

## Scope and provenance

Provider milestones F1–F5 completed their local implementation scope on
2026-10-13. This record preserves the summary formerly in PLAN.md; the detailed
milestone specifications, checklists and per-milestone results remain in Git
history (the last pre-cleanup plan is at revision `714788e`). This is a historical
summary, not a fresh test run or evidence of live compatibility.

F1–F3 implemented provider safety, submission recovery, MIME/envelope preflight,
sender review, Sent-copy policy, linked authentication and setup diagnostics.
F4 implemented isolated API grants, submission, receive/sync, queued actions and
route activation. F5 implemented API reads, draft ownership correction, migration
planning/snapshots/correlation, route rollback and opt-in UI. Current behavior and
limitations belong in [MAIL_APIS.md](../MAIL_APIS.md), [SENDING.md](../SENDING.md)
and [PROVIDERS.md](../PROVIDERS.md). P1/P2 research evidence remains in the
[provider audit](provider-deliverability-review.md) and
[transport recommendations](provider-transport-recommendations.md).

## Recorded local checks

The completed-plan summary records passing focused fixtures, `make test`,
`make lint` and `git diff --check` with Go 1.26.8 and Node 24.21.0. Frontend
changes also passed `npm run lint`, `npm run check` and `npm run build`.
Independent acceptance reviews were recorded. F3c guarded diagnostics checks
(`tools/setup-diagnostic-check.cjs`) and F5c guarded migration checks
(`tools/mail-api-migration-check.cjs`) covered affected flows and Mail with
Accounts expanded/collapsed. These results have not been rerun for this
documentation cleanup.

## Outstanding verification

No live-provider tests or test emails were performed. Bcc privacy, signed/encrypted
MIME preservation, provider size limits, Sent behavior and real-account
migration/rollback remain live gates; use [the verification requirements](../MAIL_APIS.md#live-verification-limits)
and [ISSUES.md](../../ISSUES.md) before planning follow-ups. Shared-mailbox activation and full
snapshot restore remain unimplemented; route rollback retains API-era local
changes. Migration is not interruptible once started.

The F5c migration UI at narrow widths and F3c native save-picker/window, Windows
runtime and live diagnostics were not verified. Guarded UI checks do not close
those gaps. Preserve them until a follow-up records the platform, cases, revision
and results; separately authorize any live account access or mail.
