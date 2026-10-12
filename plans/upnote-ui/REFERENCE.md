# UpNote vs MailPong reference (M0, 2026-10-11)

Measured by M0. No note titles or content are copied here.

## Method and caveats

- **UpNote source:** four owner screenshots of UpNote desktop (Linux, titles-only note list; two dark, two light; one pair with the sidebar collapsed), 2880x1800 / 2879x1697 px. Colours are exact pixel values (`magick ... %[hex:p{x,y}]`, and the most common colours inside a crop, so anti-aliasing is excluded). Sizes are screenshot pixels measured with `magick -fuzz 20% -trim` on word crops, and are **not CSS px**: the device scale is unknown (the owner said 100% scaling, but 2880 px wide suggests 1.5). Ratios are scale-independent and are what the plan should use. The note list in these shots shows titles only, so previews, thumbnails and the normal mode were **not** measured.
- **MailPong source:** code (`frontend/src/themes/built-in.css`, `app.css`, components). Colours are the theme's hex comments. Rendered MailPong shots are in `~/.cache/mailpong-upnote-ui/before/`.
- **Contrast:** computed by `node plans/upnote-ui/contrast.mjs <fg> <bg> ...` (WCAG 2.x). Output below is from this command:
  `node plans/upnote-ui/contrast.mjs '#DEDEDE' '#202222' '#A8A8A8' '#202222' '#848487' '#202222' '#202020' '#FFFFFF' '#818181' '#FFFFFF' '#818181' '#F8F8F8' '219 28% 88%' '220 16% 22%' '219 28% 95%' '220 16% 22%' '220 17% 36%' '219 28% 95%' '220 16% 22%' '219 28% 95%' '219 28% 88%' '222 16% 28%' '220 17% 36%' '219 27% 92%' '219 28% 95%' '222 16% 28%' '220 16% 22%' '219 27% 92%'`
  The `--muted` rows use the HSL token values from `built-in.css` (hex nord values give slightly different ratios because of rounding). Accent text ratios (5.14, 4.74) come from `'#4E90FE' '#202222' '#0077C3' '#FFFFFF'`.

## Surfaces (UpNote, exact pixels)

| Area | UpNote Dark | UpNote Light | MailPong Dark | MailPong Light | Source |
|---|---|---|---|---|---|
| Sidebar (Accounts / notebooks) | `#202222` | `#FFFFFF` | nord0 `#2E3440` | nord6 `#ECEFF4` | pixel / code |
| Note list / chat list | `#202222` | `#FFFFFF` | nord0 | nord6 | pixel / code |
| Editor / conversation | `#202222` | `#FFFFFF` | nord0 | nord6 | pixel / code |
| Chrome bars (list header, editor toolbar, format bar) | `#2C2C2F` | `#F8F8F8` | nord0 (same as panes) | nord6 | pixel / code |
| Title bar | `#2D2D30` | `#FFFFFF` | nord0 | nord6 | pixel / code |
| Selected row | `#313235` | `#E5F0FD` (blue tint) | nord1 `bg-muted` | nord5 | pixel / code |
| Selected sidebar item | `#313235` | `#F3F3F3` | nord1 | nord5 | pixel / code |
| Pane border | `#16161B` (darker than panes) | `#EBEBEB` | nord3 `#4C566A` (lighter than panes) | nord4 `#D8DEE9` | pixel / code |
| Row divider (inset, not edge to edge) | `#3A3A43` | `#EBEBEB` | none (selection only) | none | pixel / code |
| Title rule under editor title | `#515151` | `#D6D6D6` | n/a | n/a | pixel |
| Accent (links, section label, New Note) | `#4E90FE` | `#0077C3` | nord8 `#88C0D0` | nord10 `#5E81AC` | pixel / code |

## Text (UpNote, exact pixels; contrast from `contrast.mjs`)

| Role | UpNote Dark | Ratio | UpNote Light | Ratio |
|---|---|---|---|---|
| Titles, sidebar items, editor text (one colour for all) | `#DEDEDE` on `#202222` | 11.89:1 | `#202020` on `#FFFFFF` | 16.29:1 |
| Toolbar icons | `#A8A8A8` on `#2C2C2F` | 6.72:1 (on `#202222`) | `#818181` on `#F8F8F8` | 3.67:1 |
| Search placeholder | `#848487` on `#202222` | 4.29:1 | `#818181` on `#FFFFFF` | 3.90:1 |
| Accent text | `#4E90FE` on `#202222` | 5.14:1 | `#0077C3` on `#FFFFFF` | 4.74:1 |

MailPong tiers today (code, ratios from `contrast.mjs`):

| Tier | Dark | Ratio | Light | Ratio |
|---|---|---|---|---|
| `--foreground` on `--background` | nord6 on nord0 | 10.88:1 | nord0 on nord6 | 10.88:1 |
| `--muted-foreground` on `--background` | nord4 on nord0 | 9.08:1 | nord3 on nord6 | 6.47:1 |
| `--muted-foreground` on `--muted` | nord4 on nord1 | 7.34:1 | nord3 on nord5 | 6.00:1 |
| `--foreground` on `--muted` | nord6 on nord1 | 8.79:1 | nord0 on nord5 | 10.09:1 |

## Sizes (screenshot px; ratios to the editor body text)

| Measure | UpNote | Ratio | MailPong (CSS px, code) | Source |
|---|---|---|---|---|
| Sidebar item, list title, editor body: ascender height | 21 px (same for all three) | 1.0 | `text-sm` 14 px (name and preview) | trim / code |
| Editor title | 40 px incl. descender | about 1.5 | n/a | trim |
| Editor heading | 30 px incl. descender | about 1.15 | n/a | trim |
| Sidebar item pitch | 78 px | 1.5 x body line | n/a | pitch |
| List row pitch (titles only) | 82 px | 1.58 x body line | 44 / 60 px | pitch / code |
| Editor body line pitch | 52 px | 1.0 | `text-sm` leading | pitch |
| Sidebar : list : editor widths | 348 : 423 : rest px (about 1 : 1.2) | n/a | 240 : 420 : rest | pixel / code |
| Title bar | 58 px | n/a | 40 px | pixel / code |

If the capture scale is 1.5, UpNote's titles-only row is about 55 CSS px, which is **taller** than MailPong's 44 px row without previews. UpNote's list is airy, not dense.

## Findings that change the plan

1. **UpNote's panes are not tonally layered.** Sidebar, list and editor share one colour in both themes. Separation comes from (a) a hairline border that is *darker* than the panes in Dark (`#16161B`), (b) lighter chrome bars (`#2C2C2F`, `#F8F8F8`) at the top and bottom of panes, and (c) inset row dividers. M2's "tonal surfaces per pane" is therefore **adapt**, not copy: keep the idea, but UpNote's evidence supports bars and hairlines as much as tone.
2. **UpNote has one text colour**, with hierarchy from size, weight and icon grey, not from several text tiers. MailPong's problem is not a missing tier but that `--muted-foreground` is too close to `--foreground` in Dark (9.08 vs 10.88), so meta text looks as bright as names.
3. **UpNote's selected row in Light is tinted with the accent** (`#E5F0FD`); in Dark it is neutral.

## Lessons: verdicts

| Lesson (from PLAN.md) | Verdict | Reason |
|---|---|---|
| Three text tiers by weight and grey | **Adapt** | UpNote uses one text colour plus icon grey. MailPong keeps three tiers because chat rows carry name, preview and time, but widens the Dark primary/secondary gap. |
| Panes separated by tone more than lines | **Adapt** | Not supported as stated: UpNote uses equal tones, a dark hairline and lighter chrome bars. Use a subtle tone step for the Accounts column plus lighter header bars; keep hairlines. |
| Calm list rows, optional titles-only mode | **Adopt** | Matches the unused density setting. Do not shrink rows below today's 44 px: UpNote rows are taller. |
| Quiet chrome, content first | **Adopt** | UpNote's blue New Note button is its one loud element, which the owner's critique also named. Make MailPong's compose tonal (M5 gate). |
| Line length slider | **Reject for now** | Not measurable from these shots and low value before M6. Leave to M7 as optional. |
| Consistent typeface across platforms | **Adopt (option)** | UpNote's UI looks like Inter or a close neo-grotesque (unconfirmed by name). Decision stays with the owner in M3. |
| Header rows aligned across panes | **Adopt** | UpNote's list header, editor toolbar and sidebar section share one band at y=98-152 (display px). |
| Inset row dividers (new) | **Adopt** | Calm separation without boxing every row; fits MailPong's rounded selection. |
| Accent-tinted selection in Light (new) | **Adapt** | Consider a primary tint at low alpha for the selected row in Light; check contrast in M2. |

## Not measured

- Normal (non-compact) note list: snippets, dates, thumbnails.
- UpNote's font family, weights and letter spacing (no computed styles; screenshots only).
- Hover and focus states.

## What we adopted (M8, 2026-10-11)

- **Calm rows with a density table.** Chat rows are 32 / 44 / 60-44 / 76-60 px per density, with titles-only mode (M4).
- **Text hierarchy.** Three contrast-checked tiers (`--fg-primary/secondary/tertiary`, 4.5:1 minimum) instead of UpNote's single text colour (M2).
- **Tonal panes plus hairlines.** Surfaces step from rail to conversation pane with a darker `--divider` hairline, adapting UpNote's equal-tone panes and dark border (M2).
- **Aligned header band.** List and conversation headers share `--height-header`, 59 px (M5).
- **Quiet chrome.** Tonal Compose button, 32 px Accounts rows, plain unread counts (M5).
- **Reading width.** Narrow / Standard / Wide setting for the conversation column, 15 px bubble text (M6).
- **Kept `system-ui`** instead of a bundled neo-grotesque (owner decision, M3).
- **Not adopted:** UI text size and focus mode (M7, declined), accent-tinted selected row in Light (selection keeps `bg-muted`).

