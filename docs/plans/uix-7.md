# UIX-7 — Readable newsletters in chat

Roadmap: [UIX-7](../../PLAN.md#uix-7). Authorization: owner selected and authorized plan + implementation on 2026-10-16.

## Goal

Rich mail (newsletters, receipts, very long text) currently renders in the chat as a collapsed `ChatRichCard`
whose expansion shows the full sandboxed HTML. Show it instead as sanitized, chat-like content: headings,
paragraphs, list items, links and content images, folded after a few blocks.

Owner decisions: all rich mail (not only bulk-flagged); images follow the existing remote-image rules
(allowlist, always-load setting, Load images, Ctrl+L); on by default with a Settings → Chat option to keep the card.

## Approach

- `internal/message/chatreader.go`: `extractReader(doc)` walks the parsed HTML (reusing `chattext.go`
  helpers `isHidden`, `isTrackingPixel`, `isLayoutTable`, `attr`) into `[]ReaderBlock`
  (`heading`, `text`, `item`, `image`), each text block holding `[]ReaderSpan` runs with an optional
  sanitized `href` (http, https, mailto). Images keep http(s) and `data:image/*` sources only; `cid:` and
  small icons are dropped (alt text kept when inside a link). Data tables join cells per row; layout
  table cells are block boundaries. Long plain text becomes paragraph blocks.
- `ChatText.Reader` (`json:"reader,omitempty"`) is filled only when `IsRich`; computed on read, cached
  with the rest of `ChatText`, never persisted.
- Frontend: `ChatReaderMessage.svelte` renders the blocks; `ChatBubble` gains mode `reader` (rich +
  blocks + setting `reader`). Images use `isImageAllowedSync`, `getAlwaysLoadImages`, the
  `load-remote-images` event and `EmailBodyImageBanner`; `<img referrerpolicy="no-referrer">`.
- Setting `chat_rich_style` (`reader` default, `card`), following UIX-6's `chat_display_style` wiring.

## Progress

- [x] Backend extraction + Go tests
- [x] Setting (store, app binding, test, bindings regenerated, fake defaults regenerated)
- [x] Frontend component, mode, settings UI, i18n
- [x] UI suite `chat-reader` + reader audit variants; full `make test-ui` 119 pass; `make lint`, `npm run check`, `make test` pass
- [x] Docs (user guide, UI test README)
- [x] Close-out: acceptance check, review, validation evidence, commit

## Checkpoint

Done 2026-10-16 on branch `feat/uix-7-reader-view` (base `2461e93`); [evidence](../validation/uix-7.md). Native WebKitGTK and real-backend checks unrun.

## Surprises & Discoveries

- The UI audit caught the message hover actions covering the image button's Always load chevron; the reader pads that row (`pr-16`), as the card's title row does.
- An image-only HTML body with no text never reaches the reader (chat text extraction falls back to plain text), so it keeps the card.

## Decision Log

- Footer text (unsubscribe, address) is kept, not stripped: it is useful and hard to detect reliably.
- Inline `data:` images are capped at 256 KB each and 1 MiB per message (review finding), so a message with many inline images cannot bloat the conversation payload.

## Outcomes & Retrospective

Shipped as planned. The review added the inline-image size budget; the acceptance check passed every criterion (audit via the full UI suite). Remaining limit: no native WebKitGTK or real-newsletter check.
