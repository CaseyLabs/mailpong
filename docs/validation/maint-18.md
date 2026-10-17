# MAINT-18 — Chat code cleanups

Owner approved MAINT-18 on 2026-10-16 (unattended `make milestone` run, this milestone only). [Roadmap](../../PLAN.md#maint-18); base `cbcafa3`, branch `milestone/maint-18`.

## Result

- `chatThread.svelte.ts` types `smimeResults` and `pgpResults` with the generated `app.SMIMEViewResult` and `app.PGPViewResult`; its hand-copied `SMIMEViewResult`, `PGPViewResult` and `DecryptedAttachment` interfaces are removed (nothing else imported them).
- `subjectSections` in `chatCompact.ts` tests `entry.kind === 'message'` once, together with the subject check.
- MA10 is resolved (the shared `copyText` and `formatAddress` came with MAINT-14). No behavior change.

## Verification

- `npm run lint`, `npm run check` (0 errors, 0 warnings) and `npm run build` in `frontend/`.
- `make test-ui`: 142 passed, 0 failed, 0 skipped.

## Limits

Chromium with fake data only; no native WebKitGTK check.
