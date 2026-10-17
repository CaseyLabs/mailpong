# UIX-7 — Readable newsletters in chat

Owner selected UIX-7 on 2026-10-16. [Roadmap](../../PLAN.md#uix-7); [ExecPlan](../plans/uix-7.md); base `2461e93`, branch `feat/uix-7-reader-view`.

## Result

- `internal/message/chatreader.go`: rich HTML reduces to `ChatText.reader` blocks (heading, text, item, image). Layout tables become block boundaries, data tables join cells per row; scripts, styles, hidden preheaders (display/visibility, zero opacity or font size, zero height with hidden overflow), quoted history, tracking pixels and icons under 48 px are dropped. Links keep only http(s) with a host and mailto; images keep http(s) and bounded raster `data:` sources (256 KB each, 1 MiB per message). Long plain text splits into paragraphs. Computed on read with the rest of `ChatText`, never persisted.
- `chat_rich_style` setting (`reader` default, `card`) with `App` bindings and the backend-free defaults fixture; Settings → Chat → "Newsletters and long mail".
- `ChatReaderMessage.svelte`: reader content in bubble and flat styles, Show more/less after 8 blocks, link chips, remote images gated by the allowlist, always-load setting, Load images and Ctrl+L / Ctrl+Shift+L, `referrerpolicy="no-referrer"`.
- User guide `email-management.md`; `tests/ui/README.md`.

## Verification

- `go test ./internal/message ./internal/settings`: pass (`TestExtractChatTextReader` table, `TestIsReaderHidden`, `TestSetGetChatRichStyle`).
- `python3 tools/project-check.py --base 2461e93 --extra-command '["make","lint"]' --run`: whitespace, doc links, tooling tests, skill paths, brand, frontend lint/check/build, `make test`, `make test-ui`, `make lint`: all pass; `make test-ui` 119 passed, 0 failed, 0 skipped.
- `node tests/ui/run.mjs chat-reader`: reader in both Accounts states, image gating, links via OpenURL, keyboard folding, Show original, flat mode, card setting.
- UI audit matrix gains four reader configurations (light/dark, 1400 px, Accounts expanded/collapsed).
- Independent acceptance check (`project-acceptance-checker`): all criteria pass; the audit criterion rests on the full UI suite run.

## Limits

Chromium with synthetic data only. No native WebKitGTK check, no real newsletters from live accounts, no audit at 900/700 px widths for the reader.
