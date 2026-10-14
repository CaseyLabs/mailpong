# SEND-1 — Autosave encryption failure policy

## Scope and authorization

[SEND-1](../../PLAN.md#send-1) / [CD2](../../ISSUES.md#cd2) selected and reopened by the user on 2026-10-14. Clean base `96198ebf3cbd689e78088ed78abc63c51783279c` matched local origin/main; implementation branch `fix/send-1-autosave-encryption` (deleted after landing).

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

Current step: complete. Implementation squash commit `91a5146832f8a721e47313605e5f17320351dccd` landed on `main` and was pushed to `origin/main`; clean tracking state verified at handoff. No remaining SEND-1 work or active milestone. Changed files: app/draft.go, app/draft_encryption_test.go, docs/SENDING.md, docs/validation/send-1.md, PLAN.md, ISSUES.md and this plan.
Passed command: `go test ./app -run 'Test(Draft|Autosave|RecoveryDraft)' -count=1`.
Verified: missing and unusable keys return errors without new rows or changes to existing rows; ordinary plaintext behavior stays intact. Frontend source/bindings/prompts were unchanged; lint/check and Accounts expanded/collapsed UI checks were not run. Go 1.26 CI verification remains unrun locally (installed Go 1.27.1-X:nodwarf5).

## Surprises & Discoveries

Both composers and recovery share the helper. Recovery has an additional downgrade guard, retained unchanged.

## Outcomes & Retrospective

Verified acceptance; CD2 resolved. [Exact results and limitations](../validation/send-1.md). Independent acceptance passed; simplification required no code change. No frontend changes or live compatibility claims. No next milestone authorized.

## Next-session handoff

No next milestone is authorized. Recheck checkout state, read AGENTS.md, docs/PROJECT_MANAGEMENT.md and PLAN.md Portfolio/Progress, then obtain selection. SYNC-1 is the recommended candidate; API-1/API-2 design and other proposed work remain unselected. SEND-2/3 require separate live authorization; G4/D2 remain deferred. Use project-next-milestone only after a concrete milestone is selected. Preserve the disposable-only scope; no real drafts, providers, email sends, credentials, migration or dependency changes are authorized. This handoff only corrects the completed checkpoint; no product code or validation results changed.
