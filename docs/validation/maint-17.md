# MAINT-17 — Embedded pictures in decrypted reader mail

Owner approved MAINT-17 on 2026-10-16 (unattended `make milestone` run, this milestone only). [Roadmap](../../PLAN.md#maint-17); base `2fbe677`, branch `milestone/maint-17`.

## Result

- `internal/message`: `ExtractViewChatText` (used by the S/MIME and PGP view results through `viewChatText`) keeps `cid:` images as `cid:<id>` reader blocks (percent-decoded, angle brackets removed; empty or whitespace ids dropped). `ExtractChatText` and the chat text cache, which serve stored unencrypted mail, still drop them.
- `chatReaderCid.ts` resolves each `cid:` block from the result's `inlineAttachments` (content ID matched case-insensitively). A block is dropped when its content ID is unknown, its data is not a PNG, JPEG, GIF or WebP base64 data URL, it exceeds 256 KiB, or the message total would exceed 1 MiB (the limits stored `data:` images already use). The picture is a data URL, so nothing is requested.
- `ChatBubble.svelte` passes the PGP or S/MIME `inlineAttachments` to `ChatReaderMessage`; the user guide no longer says embedded pictures need Show original.
- UI10 is resolved.

## Verification

- `TestExtractViewChatTextCID` (table-driven): view extraction keeps `cid:` pictures (case, percent-encoding, brackets, links) and drops empty, spaced and icon-sized ones; stored-body extraction drops them all.
- `tests/ui/chat-reader-cid.test.cjs`: a faked PGP and a faked S/MIME result with one resolvable, one unknown and one over-budget picture show only the resolvable one, with no remote request and no remote-image banner.

## Limits

Chromium with fake data only; no native WebKitGTK check and no real encrypted mail.
