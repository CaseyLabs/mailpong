# UIX-8 — Signed and encrypted mail as chat content

Owner requested UIX-8 on 2026-10-16. [Roadmap](../../PLAN.md#uix-8); [ExecPlan](../plans/uix-8.md); base `80107a7`, branch `feat/uix-8-chat-all-mail`.

## Result

- `app/message.go`: `SMIMEViewResult` and `PGPViewResult` carry `chat`, from `viewChatText` (`message.ExtractChatText` on the verified or decrypted body, outside `chatTextCache`). It stays unset when decryption fails or the signature cannot be unwrapped, so those keep the full body; an attachment-only body yields empty chat text.
- `ChatBubble.svelte`: signed or encrypted mail takes its chat text from the view result under Readable text; Original layout keeps it in full. Show original appears whenever the chat view does. The attachment-only check counts decrypted attachments.
- Settings → Chat → "Email layout" (Readable text, Original layout); `chat_rich_style` values unchanged. User and settings guides updated.

## Verification

- `go test ./app -run 'TestViewChatText|TestAttachChatText'`: pass.
- `node tests/ui/run.mjs chat-secure`: 6 pass (signed bubble with badge in both Accounts states, decrypted bubble and Show original, decrypted attachment-only mail, Original layout, failed decryption). The three new-behavior tests and the attachment-only test fail against the previous `ChatBubble` logic.
- `python3 tools/project-check.py --base 80107a7 --extra-command '["make","lint"]' --run`: whitespace, docs, frontend lint/check/build, `make test`, `make test-ui` (134 passed, 0 failed, 0 skipped; includes the audit in both Accounts states) and `make lint` all pass.
- Independent acceptance check (`project-acceptance-checker`): all criteria pass (before the review fixes; the fixes reran the suite).
- Code review: fixed unwrap-failure fallback, attachment-only decrypted mail, the decrypted-attachment `bare` check and a weak Show original assertion. Declined: full body for bad signatures (the banner flags them and sanitized text is safer than the tampered layout), `cid:` images in the reader (documented limit), the hand-copied view-result types (existing pattern), skipping extraction under Original layout (minor next to decryption).

## Limits

Chromium with faked S/MIME and PGP results only. No native WebKitGTK check and no real signed or encrypted mail; the unwrap-failure path is covered by review, not a test. Pictures embedded (`cid:`) in encrypted mail show only with Show original.
