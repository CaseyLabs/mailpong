# MAINT-16 — Message actions clickable under the sticky heading

Owner approved MAINT-16 on 2026-10-16 (unattended `make milestone` run, this milestone only). [Roadmap](../../PLAN.md#maint-16); base `3a1fbc6`, branch `milestone/maint-16`.

## Result

- `ChatBubble.svelte`: the hover/focus action toolbar gets `z-20` only while shown (`group-hover`, `group-focus-visible`, `group-has-focus-visible`, open menu), above the sticky heading's `z-10`. A hidden toolbar keeps its normal stacking, so it never catches the pointer over the heading.
- Keyboard focus was never blocked (z-index does not affect tab order); the toolbar buttons stay in the tab order.
- UI9 is resolved.

## Verification

- `tests/ui/chat-anchor.test.cjs` (extended): a message scrolled so its top is 6 px under the heading, hovered; `elementFromPoint` on the Reply and Message actions buttons inside the heading's band returns those buttons, in both Accounts states. Both cases fail without the change (Reply hit the heading) and pass with it.
- `tools/project-check.py --base 3a1fbc6 --run` (whitespace, lint, check, build, `make test-ui`): passed; `make test-ui`: 140 passed, 0 failed, 0 skipped. (The tool also flagged "checks changed source files" because the evidence docs were edited while it ran; no check modified a source file.)

## Limits

Chromium with fake data only; no native WebKitGTK check.
