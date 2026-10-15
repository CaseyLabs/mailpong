# UIX-4 — Composer card and chips

Owner selected UIX-4 on 2026-10-15. [Roadmap and inline checkpoint](../../PLAN.md#uix-4); [design](../plans/ui-refresh.md#composer-uix-4); pre-commit base `23d40a4`, branch `feat/uix-1-nord-retune`.

## Result

- `ChatComposer.svelte`: the input is a rounded card. The textarea fills it, Expand sits top right, and a toolbar holds the sender label (left), Attach and Send (right). Attachment chips stay above the card. The Reply to / recipients / Topic line is a chip row below it with unchanged controls and names.
- Send key, draft handling and sender review are untouched (no change to `chatComposerState`).
- Deviations: the From is a read-only label, not a picker (the backend resolves the alias; picking needs a backend change). The optional attachment summary card is deferred (attachments load per message; no thread-level source).
- Tests: the `chat-compose` reply-line locator became `[data-chat-composer]` deliberately (the old `border-t` ancestor no longer wraps the chips); new test checks sender/Attach/Send/Expand sit inside the card and the chip row sits below.

## Verification

- `npm run lint` clean; `npm run check` 0 errors.
- `node tests/ui/run.mjs chat-compose`: 13 passed, 0 failed.
- Full UI suite (`make test-ui`): 68 passed, 0 failed, 0 skipped.

## Limits

Chromium with synthetic data only. No screenshot review, native WebKitGTK or real-backend send was done.
