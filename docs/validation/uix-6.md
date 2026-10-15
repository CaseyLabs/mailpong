# UIX-6 — Optional flat transcript mode

Owner selected UIX-6 on 2026-10-15. [Roadmap and inline checkpoint](../../PLAN.md#uix-6); [design](../plans/ui-refresh.md#flat-transcript-mode-uix-6); pre-commit base `24e37b4`, branch `feat/uix-1-nord-retune`.

## Result

- `chat_display_style` setting (`bubbles` default, `flat`): `Store.Get/SetChatDisplayStyle` validate the value; `App` bindings regenerated (`make generate`, runtime and `models.ts` churn reverted); the backend-free defaults fixture updated.
- Settings → Chat → Message style select (`ChatTab.svelte`, English strings only); `Select.Trigger` now forwards `aria-label`.
- `settings.svelte.ts` holds `getChatDisplayStyle()`; `ChatBubble.svelte` in flat mode drops the bubble background, avatar column and right-hand alignment, shows the name line for every message and marks my messages with a left accent rule. Cards, full bodies, compaction rows and chat-row glyphs are unchanged.
- User guide: `settings.md` (Message Style) and `email-management.md`.

## Verification

- `go test ./internal/settings`: pass (new `TestSetGetChatDisplayStyle`; `TestFrontendFakeDefaults` fixture regenerated).
- `npm run lint` clean; `npm run check` 0 errors.
- `node tests/ui/run.mjs chat-flat`: 3 passed (default bubbles, flat structure, settings save).
- `make test-ui`: 85 passed, 0 failed, 0 skipped. The audit matrix gains eight flat configurations (light/dark × 1400/900 px × Accounts expanded/collapsed, labelled `style=flat`); the existing matrix covers bubbles. The Accounts-collapsed flat entries were added after the independent acceptance check flagged their absence.

## Limits

Chromium with synthetic data only; one screenshot reviewed (light, 1400 px, flat). No native WebKitGTK check; flat mode is not audited at the 700 px overlay width or with previews on.
