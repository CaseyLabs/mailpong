# UIX — bb-inspired UI refresh

[Project](../../PLAN.md#uix). Status: proposed design plan, no milestone selected. Owner direction recorded 2026-10-14. Executing any UIX milestone requires explicit selection under [the lifecycle rules](../PROJECT_MANAGEMENT.md).

## Goal and inspiration

Refine Mailven's chat-style Mail view toward the calm, low-noise look of [bb](https://getbb.app) ([source](https://github.com/get-bb/bb), MIT). The owner supplied a private screenshot of bb's thread view; it is not committed. What it shows:

- **Sidebar:** a single quiet surface. At the top are labeled primary actions with icons ("New thread", "Automations"). Below them are small grey section labels ("Pinned", "All Threads") and a thread list whose rows each have a right-aligned status glyph (running spinner, done check, needs-input question mark). Settings sits as a lone cog at the bottom.
- **Header:** the thread title, a `…` overflow menu next to the title, one bordered primary button ("Commit") and a panel toggle.
- **Transcript:** flat, bubble-less text in generous type. Tool activity appears as one-line, muted, expandable rows ("› Ran 48 tests", "› Edited promo.test.ts").
- **Composer:**
  - A collapsible summary card ("Working tree · Uncommitted · 6 files, +124 −18") sits above it.
  - The composer itself is a large rounded card with a hint placeholder, an expand control, and an in-card bottom toolbar: a picker on the left, attach/voice/send on the right.
  - A row of context chips sits below the card (project, worktree, branch, access level).

Mailven borrows only these patterns: quiet low-chroma surfaces, airy type, labeled primary actions, status glyphs, a minimal header, compact expandable rows, and a carded composer with chips. It does not borrow bb code, assets, branding or agent concepts: no nested sub-threads, no model picker semantics, and no automations.

## Current state

Mailven already has the same skeleton (see [screenshot](../ss.png)):

- **Shell:** `frontend/src/App.svelte` (about 1690 lines; not restructured here). It holds `rail/ExtensionRail.svelte`, the Accounts column `sidebar/Sidebar.svelte` (about 660 lines), `chat/ChatList.svelte` (499 lines, at the limit) and `chat/ChatView.svelte`.
- **Rows:** `chat/ChatRow.svelte` shows lock, attachment, snooze and pin icons inline; the unread state is a dot.
- **Header:** `chat/ChatViewHeader.svelte` has up to five icon buttons (Archive, Delete, Snooze, Pin, `⋯`). Its `compact`/`narrow` width logic collapses them.
- **Stream:** `chat/ChatStream.svelte`, `ChatBubble.svelte`, `chatThread.svelte.ts` and `chatScroll.svelte.ts`. Only quoted text collapses.
- **Reply:** `chat/ChatComposer.svelte` puts a "Reply to · recipients · Topic" line above a single-row input with attach, expand and send.
- **Themes:** Nord light/dark tokens live in `frontend/src/themes/built-in.css`, with radius and font tokens in `app.css`; they are documented in [UI_TOKENS](../UI_TOKENS.md). Icons come from Iconify offline collections.
- **Settings:** persisted per key (`internal/settings/store.go`, `app/settings.go`, `frontend/src/lib/stores/settings.svelte.ts`); `chat_reading_width` is the model for a new chat display setting.

## Owner decisions (2026-10-14)

- Scope covers all four areas: composer card and chips, conversation compaction, list and sidebar polish, header simplification.
- Retune the existing Nord light/dark defaults rather than adding a theme; this absorbs [UI1](../../ISSUES.md#ui1).
- Bubbles stay the default; add an optional flat transcript mode.
- Track as the proposed `UIX` project in PLAN.md with this plan.

## Design by area

### Sidebar and list (UIX-3)

```
┌ Accounts ───────────────┐┌ Chats ─────────────────────────────┐
│ ✎  New message          ││ Pinned                             │
│ ⌕  Search               ││ Priya Shah, Sam Rivera   00:20   ◌ │  ◌ sending
│                         ││ Weekend trip to the coast          │
│ Personal                ││ Chats                              │
│   Inbox            12   ││ Jamie Chen, Lena Fischer 00:05   ● │  ● unread
│   Archive               ││ Omar Haddad          Yesterday   ? │  ? needs reply
│ Work                    ││ Harbour Dental       Yesterday   ⏰ │  ⏰ snoozed
│ ⚙                       ││                                    │
└─────────────────────────┘└────────────────────────────────────┘
```

- **Primary actions:** labeled "New message" and "Search" rows at the top of the expanded Accounts sidebar. With the sidebar collapsed, the icon-only compose button in the list header stays.
- **Section labels:** small and muted, without heavy weight or uppercase.
- **Status glyph:** one right-aligned slot per row showing the highest-priority state: sending/queued, then needs reply, then snoozed, then unread. Each glyph has an accessible name. Lock, attachment and pin icons stay in the meta line.
- **Needs reply:** shown when the latest message is not from the user and is addressed to them directly. Confirm during UIX-3 whether `ChatItem` already exposes this or whether the backend summary needs a field.

### Header (UIX-2)

```
Priya Shah, Sam Rivera  ⋯                              [ Archive ]
Weekend trip to the coast · snoozed until Fri
```

- Title and subtitle are unchanged, with the `⋯` menu placed next to the title.
- Archive is the single bordered primary action.
- Delete, Snooze, Pin, Spam, Forward, Mark read and the sender actions live in the menu (the existing shared context menu via `openMenu`).
- Keyboard shortcuts are unchanged.
- Width-based icon collapsing is removed where the menu supersedes it.

### Conversation compaction (UIX-5)

```
› 6 earlier messages
Sam Rivera  02:45
  Saturday works for me…
› Notice from Harbour Dental
Priya Shah  00:15
  Yes please!
```

- Runs of older, already-read messages collapse into one muted, expandable row ("› N earlier messages").
- Messages from low-priority senders inside a conversation collapse to "› Notice from X".
- The latest message and any unread message never collapse.
- Grouping is pure frontend logic in `chatThread.svelte.ts`.
- Expanded state is per view and resets on navigation.
- Rows are buttons with `aria-expanded`.
- Scroll anchoring (`chatScroll.svelte.ts`) holds when a row expands.

### Composer (UIX-4)

```
┌ › 3 attachments in this conversation ─────────────────────── ⌄ ┐
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│ Type a reply…                                                ⤢ │
│                                                                 │
│ From: me@personal.example ⌄                         📎    [ ➤ ] │
└─────────────────────────────────────────────────────────────────┘
  Reply to: All ⌄   2 recipients   Topic: Weekend trip to the coast
```

- The input becomes a rounded card with an in-card toolbar. The From identity picker sits on the left, reusing the CHAT-1 sender data and review flow. Attach and send sit on the right; expand moves to the top right.
- The "Reply to / recipients / Topic" line becomes a chip row below the card, with the same controls and accessible names.
- An optional collapsed summary card above the composer lists the conversation's attachments.
- Send-key behavior and sender review are unchanged. [CH1](../../ISSUES.md#ch1) is adjacent but out of scope.

### Palette and type (UIX-1)

- Lower the chroma of the Nord surface tokens (`--surface-rail/sidebar/list/content`, `--selected`, `--bubble-own`, `--divider`) toward near-neutral greys, keeping the Nord Frost and Aurora accents.
- Darken the light `--primary` and retune light `--selected` so the header Compose label and the selected sidebar item reach 4.5:1 (UI1).
- Use a slightly larger base text size and line height and a slightly softer `--radius`; spacing comes from tokens, not per-component values.

### Flat transcript mode (UIX-6)

- New setting `chat_display_style`: `bubbles` (default) or `flat`.
- In flat mode, `ChatBubble.svelte` renders a name/time line and the body without a bubble background. Own messages are distinguished by name and a subtle accent rule, not alignment.
- Compaction rows and the status glyphs work identically in both modes.

## Milestones

Order: UIX-1 first, because later milestones consume its tokens. UIX-2 to UIX-5 can run in any order, one per session. UIX-6 comes last so flat mode is built on the final stream.

| ID | Scope | Acceptance | Validation |
| --- | --- | --- | --- |
| UIX-1 | Nord retune and type/spacing tokens; update UI_TOKENS | UI1 entry removed from `tests/ui/ui-audit-allow.cjs`; contrast check and UI audit pass; UI1 resolved | `make lint` (contrast check), `node tests/ui/run.mjs ui-quality`, full `make test-ui`, visual check of both themes with Accounts expanded and collapsed |
| UIX-2 | Header: title, `⋯`, Archive primary | Every header action reachable by mouse, menu and keyboard; tests updated for moved actions | Header/triage regression tests, frontend lint/check, full UI suite |
| UIX-3 | Labeled sidebar actions, section labels, row status glyphs; extract `ChatListSection` first | Glyph priority covered by a regression test; `ChatList.svelte` under 500 lines; audit passes in both Accounts states | New glyph test, frontend lint/check, full UI suite |
| UIX-4 | Composer card, in-card From picker, chip row, attachment summary card | `chat-compose` suite passes with the reply-line locator updated deliberately; send key and sender review unchanged | `node tests/ui/run.mjs chat-compose`, full UI suite, real-backend send check only with authorization |
| UIX-5 | Compaction rows for earlier read and low-priority messages | Latest/unread never collapse; keyboard expand/collapse; scroll position holds | New compaction tests on fakes, full UI suite |
| UIX-6 | `chat_display_style` setting and flat bubble variant | Both modes pass the audit; Go settings table test; user guide updated | `go test ./internal/settings`, `make generate` per [CONTRIBUTING](../../CONTRIBUTING.md#regenerating-wails-bindings), frontend checks, full UI suite in both modes |

## Cross-cutting constraints

- Preserve keyboard accessibility and visible focus; new collapsible rows and chips are real buttons.
- Keep components under 500 lines. Extract before adding to `ChatList.svelte`, `ChatComposer.svelte` and `Sidebar.svelte`.
- Update English locale files only; follow [LANGUAGE](../LANGUAGE.md) for new strings.
- Preserve test-facing names and selectors, or change them deliberately in the same milestone with a stated reason. Watch these:
  - Names: "Toggle sidebar", "Settings", "Back", "Show quoted text", "Hide quoted text", "Low priority (2)".
  - Selectors: `data-chat-row`, `data-message-id`, `.bg-bubble-own`, and the reply-line `border-t` locator in `tests/ui/chat-compose.test.cjs`.
- Read [Writing tests](../../tests/ui/README.md#writing-tests) before changing fakes or UI tests. Turn each confirmed bug into a regression test.
- Visual checks cover the Mail view with the Accounts column expanded and collapsed, in both themes. Use the `run-mailven` skill for native/WebKitGTK checks the Chromium suite cannot make.
- `make lint` includes the brand check: use Mailven naming and never mention bb in UI strings.

## Risks and open questions

- **Needs reply:** the signal may need a backend summary field; if so, that extends UIX-3 into Go and needs focused tests.
- **Compaction thresholds:** how many older read messages remain visible is unresolved. Start with "collapse all but the last two read messages before the first unread or the latest" and tune with the owner in UIX-5.
- **Header:** moving Delete into the menu adds a click; confirm with the owner in UIX-2 whether Delete keeps a button.
- **WebKitGTK:** the UI suite runs in Chromium, so rendering differences in WebKitGTK (fonts, radius, blur) need a native check per milestone, or a recorded unrun check.
- **Viewer focus:** [UI4](../../ISSUES.md#ui4) (hidden viewer focus) touches the same panes. Fixing it is not in scope, but no milestone may regress it further.

## Progress

- [x] Design plan and proposed milestones recorded (2026-10-14).
- [ ] UIX-1 to UIX-6: not selected.

## Decision Log

- 2026-10-14: Retune Nord instead of adding a theme (owner). Keep bubbles as the default and add optional flat mode (owner). Excluded: nested sub-threads, a new theme, redesign of the `Composer.svelte` modal and separate-window composer, and restructuring `App.svelte`.

## Surprises & Discoveries

- Mailven already matched bb's overall layout, so the work is refinement rather than re-layout.
