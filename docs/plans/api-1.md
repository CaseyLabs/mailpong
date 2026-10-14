# API-1 — Shared mailbox activation design

[API-1](../../PLAN.md#api-1) / [MA4](../../ISSUES.md#ma4). Design only. Base `ca973f3`, branch `api-1-shared-mailbox-design`. Recorded authorization: user selected API-1 on 2026-10-14; user requested continuation on this branch on the same date.

## Scope and decisions

Produce a bounded shared identity/schema/delegate/read-write design; no implementation, live access, route switch, credentials change or migration. Maintained proposal: [Shared mailbox API design](../SHARED_MAILBOX_API_DESIGN.md). Shared support remains unavailable; MA4 stays deferred. API-6 is proposed only. API-5 is not an established design dependency.

## Progress

- [x] Reconcile existing draft against account, credential, consent, Graph and UI code.
- [x] Correct route-mode versus scope semantics and explicit grant-owner storage/locking.
- [x] Specify parent identity, snapshot authority, removal/failure handling and validation gates.
- [x] Record existing best-effort parent deletion risk as MA5.
- [x] Documentation checks, independent acceptance review and final closeout.

## Acceptance and verification

Acceptance: “Proposal maps all shared route blockers and data/security invariants with validation and migration gates; do not claim shared support from design alone.” Validation: read-only trace of apiRemote/activation, Graph clients, credential ownership and provider architecture; official Microsoft requirements review; local Markdown links and `git diff --check`. No runtime code changed, so builds, Go tests and UI checks are unrun and unnecessary for this milestone; later implementation requires the checks in the proposal.

## Surprises & Discoveries

The draft conflated route mode with granted scopes; owner refresh locking already exists but owner storage resolution does not. Parent removal ignores enumeration errors and continues after child deletion failures; it is not an enforced database cascade (MA5).

## Decision Log

Retain persisted route flags, block legacy shared rows without identity, preserve parent `/me` through scope expansion. Keep one owner grant/snapshot; refuse owner clear with references and prevent owner removal on dependent deletion failures. Require separately authorized live identity, delegation, sender, Sent-copy and Bcc evidence before exposing shared migration.

## Checkpoint

Current step: completed design; commit/landing follows repository policy. Changed files: this checkpoint, design reference, PLAN.md, ISSUES.md, MAIL_APIS.md and validation record. No implementation selected. Source review and independent acceptance passed; local links and whitespace checks passed. Builds/tests/UI/live checks unrun for this documentation-only scope. Exact results and limits: [validation](../validation/api-1.md). Resume by selecting a new milestone in a fresh session, not by implementing API-6 automatically.

## Outcomes & Retrospective

Design completed and reviewed. Simplification review found no code to simplify; technical proposal and checkpoint are separated to keep status in the roadmap. Existing MA4 remains deferred; newly confirmed MA5 remains open. No live or runtime support claim.
