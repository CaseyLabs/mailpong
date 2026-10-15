# UIX-3 — List and sidebar polish

Owner selected UIX-3 on 2026-10-15. [Roadmap and inline checkpoint](../../PLAN.md#uix-3); [design](../plans/ui-refresh.md#sidebar-and-list-uix-3); pre-commit base `23f529a`, branch `feat/uix-1-nord-retune`.

## Result

- `ChatListSection.svelte` (new) renders a group header plus its rows; `ChatList.svelte` uses it (499 → 490 lines). A flat list no longer gets an empty-label group.
- `chatStatus.ts` (new) picks one status per row: sending, then snoozed, then unread. `ChatRow` shows it in a right-aligned slot with `data-row-status`; the snooze icon and unread dot left the meta line. Sending follows the composer's pending queue (`isSendingChat`), and the row's accessible name says "Sending".
- Section labels (`ChatGroupHeader`, `ChatLowGroupHeader`) use normal weight.
- `SidebarActions.svelte` (new): labeled "New message" and "Search" rows at the top of the expanded Accounts sidebar. The list-header Compose remains for the collapsed column.
- Deferred by owner decision: the needs-reply glyph. `ChatItem` has no latest-sender field and Go changes were out of scope.
- Tests: `logic` (glyph priority), `chat-compose` (sending glyph appears and clears on Undo), new `sidebar-actions` (Search focuses the list search, New message opens the composer, collapsed keeps Compose).

## Verification

- `npm run lint` clean; `npm run check` 0 errors; `tools/brand-check.sh` and `tools/contrast-check.mjs` pass.
- `python3 tools/project-check.py --base 23f529a --run` after the final edit: whitespace, documentation links, frontend lint, Svelte/TypeScript check, production build and the full UI suite pass.
- Full UI suite: 67 passed, 0 failed, 0 skipped, including the UI audit in both Accounts states.

- Independent acceptance check: all three criteria pass (`logic` 6/0, `sidebar-actions` 3/0, `ui-quality` 21/0 with Accounts expanded and collapsed). `Sidebar.svelte` was already 664 lines at the base and is now 671; splitting it is outside this milestone.
- Simplification review: no change warranted.

## Limits

Chromium with synthetic data only. No screenshot review, native WebKitGTK or real-backend run was done.
