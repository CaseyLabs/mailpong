# SEND-1 — Autosave encryption failure policy

## Scope and authorization

[SEND-1](../../PLAN.md#send-1) / [CD2](../../ISSUES.md#cd2) selected and reopened by the user on 2026-10-14. Clean base `96198ebf3cbd689e78088ed78abc63c51783279c` matched local origin/main; branch `fix/send-1-autosave-encryption`.

Disposable fixtures only; no live providers, email sends, real credentials, account migrations or dependency upgrades. Preserve recovery protections and other draft behavior.

## Decision Log

Fail closed in shared `encryptDraftBody`: return an error and no payload on requested encryption failure. Both composers stop before persistence, ownership changes or new sync. Keep the previous saved draft unchanged; new edits remain in the open composer until saved successfully. Existing frontend error handling is unchanged.

## Progress

- [x] Inspect roadmap, CD2, encryption callers and existing recovery/missing-key fixtures.
- [x] Implement fail-closed helper.
- [x] Verify both protocols, both composers, failed create/update, bodies and attachments.
- [x] Focused draft/recovery tests, independent acceptance, diff review and simplification.
- [x] Document evidence, resolve CD2 and land.

## Validation mapping / checkpoint

Current step: implementation and acceptance complete; ready for repository-policy landing. Final commit/push status is reported with the Git handoff. Changed files: app/draft.go, app/draft_encryption_test.go, docs/SENDING.md, docs/validation/send-1.md, PLAN.md, ISSUES.md and this plan.
Passed command: `go test ./app -run 'Test(Draft|Autosave|RecoveryDraft)' -count=1`.
Prove missing and unusable keys return errors without new rows or changes to existing rows; ordinary plaintext behavior stays intact. No frontend changes planned, so frontend/UI checks are conditional.

## Surprises & Discoveries

Both composers and recovery share the helper. Recovery has an additional downgrade guard, retained unchanged.

## Outcomes & Retrospective

Verified acceptance; CD2 resolved. [Exact results and limitations](../validation/send-1.md). Independent acceptance passed; simplification required no code change. No frontend changes or live compatibility claims. No next milestone authorized.
