# PLAN: Mail UI polish — chat list and reading pane

## Scope

Improve visual hierarchy, density, focus visibility and theming in the live
chat-style mail UI (`frontend/src/lib/components/chat/*`, plus
`viewer/EmailBody.svelte` and `viewer/AttachmentList.svelte`, which
`ChatBubble.svelte` renders). Apple Mail on macOS is the reference for
principles, not platform chrome. Keep the Tailwind + theme-token system
(`frontend/src/themes/*.css`, HSL vars like `--primary`, `--muted`) and the
existing kit components. English locale only (`frontend/src/lib/i18n/locales/en.json`).

Out of scope: the classic `list/MessageList`, `list/ConversationRow` and
`viewer/ConversationViewer` components. `App.svelte` mounts `ChatList` and
`ChatView` only; deleting the classic code is ISSUES.md U1.

Baseline (code review; measure heights in DevTools before M1 and M4):

- `ChatRow.svelte` is about 100px with the preview on: people line (20px),
  subject `line-clamp-2 text-xs leading-5 min-h-10` (40px), snippet `truncate
  text-sm` (20px), `py-2.5`. The subject is smaller than the snippet.
  `contain-intrinsic-size` claims 68px.
- Selection is `bg-primary/20` whether or not the list has focus. The listbox
  sets `data-focused` (`ChatList.svelte:426`) but nothing styles it. Hover is
  `hover:bg-muted/60`.
- No `focus-visible` styles in `chat/` except the quote button in
  `ChatBubble.svelte`. The listbox has `outline-hidden`; rows use
  `aria-activedescendant` with `tabindex="-1"`.
- `ChatRow` dates use `formatRelativeDate` (`lib/utils/date.ts`): "just now",
  "5m", "3h" today.
- `ChatListHeader.svelte` has no total/unread counts.
- `ChatSecurityBanners.svelte` hard-codes 28 Tailwind palette colors (the
  `TONES` map and the read-receipt button). `EmailBody.svelte:747–764` has a
  `yellow-*` remote-images banner. Signed and encrypted (positive) states each
  take a full banner.
- Full email bodies sit in a `rounded-xl border` box (`ChatBubble.svelte:229`).
- `ChatView.svelte` is 499 lines, at the 500-line guideline.

Already done in the chat UI, so not planned: grouped toolbar with separators
and an overflow menu (`ChatViewHeader.svelte`), row `aria-label` summary, hover
action buttons with `aria-label`s and keyboard shortcuts.

## Milestones

### M1 — Chat list row and header

1. Row density and hierarchy (`ChatRow.svelte`):
   - Subject on one line (`truncate`, drop `min-h-10`); snippet stays one line.
   - Subject at least the snippet's size, with people > subject > snippet in
     weight or color.
   - Update `contain-intrinsic-size` to the measured height.
2. Hover tint visible on light themes (for example `hover:bg-muted` instead of
   `/60`).
3. Focus visibility: `focus-visible` rings on every `ChatListHeader` button,
   filter chip and the scope trigger. The active row shows a ring or inset
   outline while the listbox has keyboard focus.
4. Dates: add `formatListDate` to `lib/utils/date.ts` (clock time today,
   "Yesterday", weekday within the last 7 days, then the existing `MMM d` /
   `MMM d, yyyy`) and use it in `ChatRow`. Leave `formatRelativeDate` for other
   callers.
5. Header subtitle in `ChatListHeader`: the current folder's
   `totalCount`/`unreadCount` (folder model, kept up to date by
   `accounts.svelte.ts`), with ICU plural keys in `en.json`. For the unified
   inbox, sum the inbox unread counts; hide the subtitle when no counts exist.

Acceptance criteria:

- With the preview on, a standard row measures 72–80px in DevTools; with the
  preview off it is shorter.
- Hover is visible on built-in light and Adwaita light (screenshot).
- Tabbing through the list header shows a ring on each control; arrowing in the
  focused list shows which row is active.
- Rows from today show a clock time (for example "14:05" or "2:05 PM" per
  locale); yesterday shows "Yesterday".
- The subtitle reads, for example, "1,204 messages, 3 unread" and updates when
  messages are read.
- Changed components stay under 500 lines.

Validation:

- `cd frontend && npm run lint && npm run check`.
- `run-aerion` skill: screenshots of the list on built-in (light and dark) and
  Adwaita light; keyboard pass through header and list. If no display is
  available, record that in Progress.

### M2 — Selection follows pane focus

1. Style the selected row from the listbox's `data-focused` (for example a
   `group/list` + `group-data-[focused]/list:` variant), falling back to
   `isPaneFocused('messageList')` (`lib/stores/keyboard.svelte.ts:29`) if the
   attribute does not cover all cases:
   - list focused: solid `bg-primary text-primary-foreground`; subject,
     snippet and time use `text-primary-foreground/80`;
   - list not focused: neutral `bg-muted`.
2. The unread pill, status icons and hover actions stay legible on the solid
   selection (invert the pill).

Acceptance criteria:

- Clicking the reading pane switches the selected row from solid to neutral;
  clicking or keyboard-focusing the list switches it back.
- `--primary-foreground` on `--primary` is at least 4.5:1, and the `/80`
  secondary text is at least 4.5:1, for `_defaults.css` and all 11 themes in
  light and dark.

Validation:

- `npm run lint && npm run check`.
- A scratch script that parses each theme's `--primary`/`--primary-foreground`
  HSL values and computes the contrast ratios (including the 80% blend);
  record the lowest ratio in the Progress entry.
- `run-aerion`: screenshots for built-in, Adwaita, Dracula, Nord and GitHub,
  focused and unfocused. If no display is available, record that in Progress.

### M3 — ChatView extraction (refactor, no behavior change)

1. Move a self-contained part of `ChatView.svelte` (message loading/state or
   keyboard handling) into a `.svelte.ts` module or child component so M4 has
   room.

Acceptance criteria:

- `ChatView.svelte` is at most about 400 lines; every new file is under 500.
- Behavior is unchanged: opening a thread, j/k between messages, reply,
  archive, and the security banners all work as before.

Validation:

- `npm run lint && npm run check`.
- `run-aerion`: walk through the listed behaviors. If no display is available,
  record that in Progress.

### M4 — Reading pane polish

1. Record the baseline (spacing between messages, header height) in this plan
   before changing anything.
2. Full-body messages (`ChatBubble.svelte:229`) drop the bordered box and sit
   on the pane background, separated by a hairline (`border-border`) between
   messages.
3. Per-message actions and the focused-bubble ring use `focus-visible`
   instead of appearing on any focus.
4. `ChatViewHeader`: the subject is the primary heading, with participants
   secondary.

Acceptance criteria:

- Measured spacing matches the values recorded in step 1's target.
- Mouse clicks on a bubble show no ring; keyboard focus does.
- Changed components stay under 500 lines.

Validation:

- `npm run lint && npm run check`.
- `run-aerion`: before/after screenshots of a long HTML thread and a short
  plain-text thread. If no display is available, record that in Progress.

### M5 — Themed status tokens

1. Add `--info`, `--success`, `--warning` and their `-foreground` pairs to
   `themes/_defaults.css` and each theme file, and map them in the `app.css`
   `@theme inline` block.
2. Rewrite the `ChatSecurityBanners` `TONES` map and the read-receipt button,
   and the `EmailBody` remote-images banner, to use the tokens (`bad` uses
   `--destructive`).
3. Positive states (signed, encrypted) become a compact badge in the bubble's
   meta line; failures and warnings stay full banners.
4. Check the token change against `docs/EXT_RULES.md`.

Acceptance criteria:

- `grep -nE '(blue|green|amber|red|yellow)-[0-9]' frontend/src/lib/components/chat/ChatSecurityBanners.svelte frontend/src/lib/components/viewer/EmailBody.svelte`
  prints nothing.
- Banner text on its token background is at least 4.5:1 for all themes in
  light and dark.
- A signed message shows a badge, not a banner; a bad signature still shows a
  banner.

Validation:

- `npm run lint && npm run check`; the grep above.
- The M2 contrast script, extended to the new tokens.
- `run-aerion`: signed, encrypted, bad-signature and remote-images messages on
  three themes. If no display is available, record that in Progress.

## Progress

- [x] M1 — Chat list row and header (visual checks outstanding: dev mode forces Nord Dark on the real mailbox, so row heights, light-theme hover and screenshots need the owner)
- [x] M2 — Selection follows pane focus (contrast verified by script, lowest ratio 4.61 across all 28 theme variants after `_selection.css`; screenshots and the click-to-switch check not done, no display in this session, so the owner needs to check them)
- [x] M3 — ChatView extraction (moved scroll/anchor/message focus to `chatScroll.svelte.ts` and triage actions to `chatActions.svelte.ts`; ChatView 499 → 397 lines; lint and check pass; walkthrough not done, no display in this session)
- [x] M4 — Reading pane polish (full-body messages drop the border box and get a `border-t border-border` hairline unless they start a day (the day separator divides there); ring and per-message actions use `group-focus-visible` / `group-has-focus-visible`, so the unused `focused` prop is gone; header shows subject as `text-base` heading with participants below, sender chats unchanged; lint and check pass; no display in this session, so before/after screenshots and the measured spacing are not done and need the owner)
- [x] M5 — Themed status tokens (`themes/_status.css` defines the six tokens for light and dark via `:root`/`.dark`, mapped in `app.css`; `ChatSecurityBanners` and the `EmailBody` banner use them, grep prints nothing; signed/encrypted are badges in the meta line via new `chatSecurity.ts`; banner text ≥ 5.17:1 on every theme background and card by `/tmp/contrast/status.py`; lint, check and build pass; no display in this session, so the signed/encrypted/bad-signature/remote-images checks on three themes need the owner)

## Surprises & Discoveries

- Sender chats by default: pins, snoozes and chat draft links are keyed by chat, so at startup and when **Combine messages from each sender** changes, `StaleChatKeys` moves a thread's state into the sender chat it now belongs to (state already on the sender chat wins) and drops state from sender chats whose sender no longer combines, as splitting does. Turning the default off doesn't hand a sender chat's state back to one of its threads. The list takes the window pass over the whole scope whenever the default is on or a sender is combined explicitly (`hasSenderChats`); not checked on a large real mailbox.
- "Combine messages from this sender" on a thread whose sender already combines (a thread someone else started) changes nothing: `SetSenderChat` returns false, the toast says so, and no undo is recorded. Undoing a combine or split restores the sender's previous choice, including "no choice" (follow the default).
- Visual pass 2026-10-10 (dev server, Nord Dark only, real mailbox; widths 668/900/1100px via an iframe because the Chrome window could not be resized): list, selection switching, header subject, day separators, remote-images banner and card expand look right. Fixed: toolbar and header buttons lacked focus-visible rings (Compose's ring was invisible on primary, now offset); no divider under the reading-pane subject, so scrolled messages clipped flush; `ChatRichCard`'s `block` overrode `line-clamp-3` (16-line preview) and the toggle plus `pr-20` squeezed it to a 162px column. Still unchecked: light themes, signed/encrypted/bad-signature messages (none in the mailbox), measured row heights.

- M5 review: `viewer/ConversationViewer.svelte` (classic, unmounted) still has hard-coded blue/green/amber banners and its own status mapping; left out of scope (ISSUES.md U1). The solid-button pairing (`bg-*-foreground` with `text-background`) is covered by the contrast script rows tagged `btn` (≥ 5.17).
- M3 review found that `_selection.css`'s `:root` rule gave the 7 unlisted themes the light purple instead of the `--primary` fallback; fixed with an explicit reset block there.
- `formatListDate` today branch uses `toLocaleTimeString` (OS locale) and goes stale after midnight; not fixed.
- `contain-intrinsic-size` (80px/60px) is computed from the row layout, not measured.
- `formatListDate` weekday and month names are English (date-fns default), like `formatRelativeDate`.

## Decision Log

- Owner request 2026-10-10: the list header no longer shows the message/unread counts subtitle (M1 item 5 reverted; `chat.listCounts` removed).
- Owner request 2026-10-10: the reading pane's flex basis is 30rem (was 20rem), so the list gives way before the reading pane drops below 480px; before, the list kept 420px while the pane shrank to 320px (checked with a headless Chromium layout of the same CSS).
- Owner request 2026-10-10: sender chats are on by default. Migration 50 adds `sender_chat.combined` (default 1, so earlier opt-ins stay combined); a row now records a per-sender choice and **Show as separate threads** stores 0. Threads started from the account's own addresses (account email and identities) or with no sender stay thread chats, so Sent folders are unaffected. Settings → Chat → **Combine messages from each sender** (`chat_combine_senders`, default on) sets the default for senders without a row; the SQL reads it from `settings` directly.
- M5: tokens live in one `_status.css` keyed on `:root` / `.dark` rather than in each of the 28 theme blocks (themes may still override them); `-foreground` is the tinted-banner text color, also used as a solid button background with `text-background`. Bad banners use `--destructive` for tint and border with `text-foreground` text, since several themes' destructive reds fail 4.5:1 as text. EXT_RULES.md has no theming rules and no extension code was touched.

- M4 baseline (from code, not measured): group gap `mt-3` (12px), same-sender `mt-0.5`; header block `pt-3 pb-1` with a 32px avatar row and `text-sm` title. Target: same gaps plus `pt-3` and a 1px hairline above full-body groups; header title `text-base`.

- M2: `--primary`/`--primary-foreground` and the 80% text both reach 4.5:1 in only 7 of the 28 theme variants (lowest 2.01, breeze). Rather than retune every theme's primary, which colors buttons everywhere, a new `--selection`/`--selection-foreground` pair (`themes/_selection.css`, falling back to the primary tokens in `app.css`) overrides 21 variants with a shifted lightness and a pure white or black foreground.

- Apple Mail is the reference for principles only. Theme tokens and kit
  components stay; no macOS-specific chrome.
- Target the chat UI: the classic list and viewer are unmounted dead code
  (ISSUES.md U1), so their density, unread-accent and star settings are not
  part of this plan.
- Rows keep a one-line snippet and a one-line subject, targeting 72–80px.
- Refactors get their own milestone (M3) so each milestone is one commit and
  visual diffs stay reviewable.

## Outcomes & Retrospective

_Not started._
