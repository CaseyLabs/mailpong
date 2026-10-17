# MAINT-15 — Day label clear of the sticky heading

Owner approved MAINT-15 on 2026-10-16 (unattended `make milestone` run, this milestone only). [Roadmap](../../PLAN.md#maint-15); base `e46af4c`, branch `milestone/maint-15`.

## Result

- `chatScroll.svelte.ts` `apply()` anchors the day label row above the first unread message (when it starts a day, else the message) at the rendered sticky heading's height plus 16 px, replacing the fixed 48 px.
- The scroller's `scroll-padding-top` is set from the same heading height plus 8 px on each resize, replacing the fixed `scroll-pt-10` class in `ChatView.svelte`.
- Independent acceptance check (`project-acceptance-checker`): all three criteria pass. The sender line is covered because it is inside the message element, which the test asserts is below the label. Authorization: owner-started runner, 2026-10-16, MAINT-15 only; no live or native step involved.
- UI8 is resolved.

## Verification

- `tests/ui/chat-anchor.test.cjs` (new): a trip chat whose unread message opens a second subject on the next day, 500 px high, in both Accounts states; the day label's top is at or below the heading's bottom and the message follows its label. Both cases fail on the previous code (label top 112.75 against heading bottom 133) and pass now.
- `cd frontend && npm run lint && npm run check`: 0 errors, 0 warnings.
- `tools/project-check.py --base e46af4c --run` (lint, check, build, `make test-ui`): passed; `make test-ui`: 138 passed, 0 failed, 0 skipped.

## Limits

Chromium with fake data only; no native WebKitGTK check.
