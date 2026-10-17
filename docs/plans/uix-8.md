# UIX-8 — Signed and encrypted mail as chat content

Roadmap: [UIX-8](../../PLAN.md#uix-8). Authorization: owner requested on 2026-10-16 (plan and implement).

## Goal

Signed (S/MIME, PGP) and decrypted mail renders as the full sandboxed body in the chat. Show it as a
sanitized bubble or reader content like other mail, keeping the security badges and banners.

Owner decisions: cover signed mail and decrypted mail (image-only HTML keeps its card); extend the existing
`chat_rich_style` select rather than adding a toggle: "Email layout" with Readable text (default) and
Original layout. Persisted values `reader`/`card` are unchanged.

## Approach

- `SMIMEViewResult` and `PGPViewResult` gain `Chat *message.ChatText` (`json:"chat,omitempty"`), from
  `message.ExtractChatText` on the unwrapped body. Computed per call and not put in the shared chat-text
  cache, so decrypted plaintext is not kept beyond the view result. Absent when decryption fails.
- `ChatBubble`: `chat` = the security result's chat for signed/encrypted mail, else `msg.chat`. Signed or
  encrypted mail uses it only with Readable text; Original layout keeps the full body. Show original
  is available whenever the chat view is.
- Settings label/description renamed; user guide.

## Progress

- [x] Backend fields + Go test
- [x] Bindings regenerated
- [x] Frontend bubble + settings strings
- [x] UI tests (fake security results), full `make test-ui`
- [x] Docs, PLAN, validation evidence, commit

## Checkpoint

Done 2026-10-16 on branch `feat/uix-8-chat-all-mail` (base `80107a7`); [evidence](../validation/uix-8.md). Native WebKitGTK and real signed or encrypted mail unchecked.

## Decision Log

- The view result leaves `chat` unset when the signature cannot be unwrapped (review finding), so the raw envelope never shows as chat text.
- Bad signatures still get the chat view; the banner carries the warning.
