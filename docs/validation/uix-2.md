# UIX-2 — Header simplification

Owner selected UIX-2 on 2026-10-14. [Roadmap and inline checkpoint](../../PLAN.md#uix-2); [design](../plans/ui-refresh.md#header-uix-2); pre-commit base `ddd94f6`, branch `feat/uix-1-nord-retune`.

## Result

- `frontend/src/lib/components/chat/ChatViewHeader.svelte`: the title block is followed by the `⋯` "More actions" button; the right side holds Compose (wide panes), a bordered Archive button and a ghost Delete button. The owner chose to keep Delete as a button.
- Snooze and Pin are menu items at every width (previously icon buttons, with Pin moving to the menu only on narrow panes). Snooze opens the presets dropdown from a zero-size anchor (`ChatSnoozeMenu` `hiddenTrigger`), the same anchor the H key uses. Width-based Pin collapsing is removed; `compact` still shrinks Archive, Delete and Compose labels to icons.
- Keyboard shortcuts are unchanged. A snoozed chat still shows its time in the subtitle; the pinned state no longer has a header indicator (the list shows it).
- New `tests/ui/chat-header.test.cjs`: at 1200/700/500 px the header buttons are Archive, Delete and More actions only; the menu offers Snooze, Pin/Unpin and Delete; Pin from the menu calls `PinChat`; Snooze opens its presets; Archive runs from the keyboard.

## Verification

- `python3 tools/project-check.py --base ddd94f6 --run` after the final edit: whitespace, documentation links, frontend lint, Svelte/TypeScript check (0 errors/warnings), production build and the full UI suite pass. UI result: 62 passed, 0 failed, 0 skipped, including the UI audit across both themes, 700/900/1400 px and Accounts expanded, collapsed and overlay.
- Independent acceptance check: all three criteria pass. It reran `chat-header` (4/0) and `inbox-triage` (6/0) itself. Two paths are verified by code reading only, with no UI test: the H shortcut (`App` → `ChatView.openSnooze` → header `openSnooze` → hidden `ChatSnoozeMenu`) and Forward for multi-message chats in the `⋯` menu.

## Limits

Chromium with synthetic data only. No native WebKitGTK, screenshot review of the new header or real-backend run was done; these remain unrun.
