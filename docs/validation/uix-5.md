# UIX-5 — Conversation compaction

Owner selected UIX-5 on 2026-10-15. [Roadmap and inline checkpoint](../../PLAN.md#uix-5); [design](../plans/ui-refresh.md#conversation-compaction-uix-5); pre-commit base `8e4a14b`, branch `feat/uix-1-nord-retune`.

## Result

- `chatCompact.ts`: pure `compactItems` folds runs of at least 3 older read messages into a `run` entry. The latest 5 messages, unread, draft and subject-heading messages, and IDs in `keep` never fold. The message after a folded run is shown with its sender.
- `ChatCompactRow.svelte`: muted one-line `aria-expanded` button, "N earlier messages" / "Hide N earlier messages" (new `chat.showEarlier`/`chat.hideEarlier` strings).
- `ChatStream.svelte`: per-view expanded set and a `keep` set of messages unread at open (so a message does not fold under the reader once auto-marked read); both reset when `resetKey` (the thread) changes.
- `chatScroll.svelte.ts`: `holdPosition` keeps the toggled row at its screen offset (also against the stay-at-bottom observer); Tab navigation skips folded messages and leaves compact rows to native Tab.
- Deviations: the "Notice from X" variant for low-priority senders is deferred (no per-message sender priority in the frontend model). A day separator on the first folded message is not shown while folded (the row stands in). Thresholds (5 / 3) differ from the design plan's tentative "all but the last two read" so existing short chats are unchanged; tune with the owner.

## Verification

- `npm run lint` clean; `npm run check` 0 errors.
- `node tests/ui/run.mjs chat-compaction`: 6 passed (logic, fold/never-fold, short chat, keyboard Enter/Space, read-while-open, position hold).
- Full UI suite (`make test-ui`): 74 passed, 0 failed, 0 skipped.

## Limits

Chromium with synthetic data only. No screenshot review or native WebKitGTK check. The low-priority variant is not implemented.
