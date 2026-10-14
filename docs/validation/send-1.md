# SEND-1 validation — 2026-10-14

[SEND-1](../../PLAN.md#send-1) / [CD2](../../ISSUES.md#cd2). Base `96198ebf3cbd689e78088ed78abc63c51783279c`; policy described in [SENDING](../SENDING.md).

## Evidence

- `go test ./app -run 'Test(Draft|Autosave|RecoveryDraft)' -count=1` — PASS (`ok github.com/caseylabs/mailven/app 1.093s`). Independent project-acceptance-checker reran it: PASS (0.996s).
- `git diff --check` — PASS.
- Missing and malformed certificate/public-key fixtures cover S/MIME and PGP, main and detached composers, new and existing drafts (16 cases). Body HTML/text and base64 attachment sentinels are supplied. Failures return errors and nil payload/results; create leaves no row; update leaves the entire prior encrypted row unchanged, including sync linkage/status. Detached currentDraft remains unchanged.
- Successful S/MIME/PGP fixtures persist no plaintext body or separate attachment column, then recover the original bodies and attachment bytes. Ordinary unencrypted payload and existing identity, ownership, sync and recovery tests also pass.
- Independent read-only acceptance: PASS, no correctness findings. Added-stub scan only matched a comment describing the disposable SMTP fixture; no incomplete implementation.
- Direct diff review and simplify skill review: shared guard clauses are sufficiently simple; no behavior-preserving code changes warranted. Read-only `archive/` remained clean.

## Limits and excluded work

All data and keys are disposable; the success/recovery fixture starts a loopback SMTP simulator but these tests do not submit mail. No real drafts, live providers, real credentials, migrations or dependencies changed. Failure injection covers missing and malformed keys, not every possible cryptographic/I/O failure; both encryptor error branches return before storage for all returned errors.

No frontend source, bindings or prompts changed. Existing save-error status and Save & Close failure handling were inspected, not changed; frontend lint/check and run-mailven UI (including Accounts expanded/collapsed) were not run. No native UI or live provider compatibility is claimed. Installed Go is 1.27.1-X:nodwarf5; tests were not repeated under the project's Go 1.26 CI toolchain.

Failed development checks were fixture setup errors (embedded draftOps type, required account fields, nonexistent folder FK), corrected before passing runs; no baseline product failure was found. No unrelated or subsequent milestone was selected.
