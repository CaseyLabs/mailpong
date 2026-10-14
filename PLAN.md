# Deferred work

## Scope

No implementation milestone is currently queued. Provider work F1–F5 is complete for its local scope; current behavior belongs in [PROVIDERS.md](docs/PROVIDERS.md), [MAIL_APIS.md](docs/MAIL_APIS.md) and [SENDING.md](docs/SENDING.md). Historical checks and remaining verification limits are in [the provider implementation record](docs/validation/provider-implementation.md). Completion does not establish live provider compatibility.

Retain the selected direction of full Gmail API and delegated Microsoft Graph, including future shared-mailbox support and explicit opt-in migration. Shared API activation is currently unsupported. Existing accounts keep their configured routes until explicitly migrated; there is no automatic switch or fallback.

Select and refine acceptance criteria and validation before starting one newly requested milestone. G4/AI5 and D2 remain deferred; do not resume them automatically. No live provider access, test emails, provider registrations, tenant/DNS changes or dependency changes are authorized by this plan.

## Milestones

No active implementation milestone. Deferred work:

- **G4 / AI5 — cross-host continuation:** only when explicitly resumed, refine and verify Claude runtime compliance/routing, bidirectional Codex/Claude continuation (including uncommitted work, checkpoint provenance and blockers), and comparable context savings. Retain G1–G3 evidence in [the verification record](docs/validation/shared-skills-g3.md); those remaining claims are unverified.
- **D2 — TypeScript 7 compatibility:** recheck both tooling peer ranges using CONTRIBUTING.md before proposing an upgrade; the last recorded incompatibility remains in ISSUES.md.
- **Provider follow-ups:** select from ISSUES.md, including shared-mailbox activation and snapshot restoration. Live compatibility checks require separate authorization specifying accounts, From identities, controlled recipients and cases; use [API verification limits](docs/MAIL_APIS.md#live-verification-limits) and [P2 parity scenarios](docs/validation/provider-transport-recommendations.md).

## Progress

No active milestone. Deferred items are not instructions to resume work.

## Surprises & Discoveries

None for an active milestone. Completed implementation discoveries are retained in the technical references and Git history.

## Decision Log

G4/AI5 remains deferred by the user (2026-10-12); D2 remains blocked by the peer ranges recorded in ISSUES.md. Do not alter personal CLI settings as part of those follow-ups.

## Outcomes & Retrospective

No active milestone. Completed provider verification is retained in [the implementation record](docs/validation/provider-implementation.md); G1–G3 evidence remains in [the shared-skills verification record](docs/validation/shared-skills-g3.md).
