# UI design tokens

Named values for type, sizes, pane surfaces and text tiers. Change a value in
one place and every use follows. Tokens live in `frontend/src/app.css`
(`@theme inline`) and `frontend/src/themes/_defaults.css`; themes override
the surface and text-tier triples in `frontend/src/themes/built-in.css`.

Tailwind 4 turns a `@theme` variable into utilities only inside its namespace
(`--text-*` is font size, `--color-*` colour, `--height-*` height), so the text
tiers are `--fg-*`, not `--text-*`; the latter would collide with
`text-primary` and `text-secondary`.

`app.css` also raises Tailwind's own `text-xs`, `text-sm` and `text-base` one
pixel (to 13, 15 and 17 px), so the many plain uses follow the role tokens.
No UI text is below 12 px except `text-2xs` (11 px). Chat bubbles use 16 px,
and the composer editor and email body default to 15 px.

| Token | Utility | Role | Default | Used in |
|---|---|---|---|---|
| `--text-meta` | `text-meta` | Timestamps, secondary labels, sentence-case section labels (semibold, `text-fg-tertiary`) | 13 px / 18 px (`text-xs`) | `ChatListHeader`, `ChatRow`, `ChatGroupHeader`, `ChatLowGroupHeader`, `SourceSidebar`, calendar sidebar, `FolderTreeItem`, `UnifiedInboxSection` |
| `--text-body` | `text-body` | Body and list text | 15 px / 22 px (`text-sm`) | `ChatRow`, `ChatViewHeader` |
| `--text-title` | `text-title` | Pane and dialog titles | 17 px / 24 px (`text-base`) | Not yet used; `text-base` is equivalent |
| `--text-caption` | `text-caption` | Chips and small notes | 12 px, no line height | `ChatBubble`, calendar day headers and attendee notes |
| `--text-compact` | `text-compact` | One step below body: quoted text, Standard-density list subjects | 14 px / 20 px | `ChatBubble`, `ChatRow` |
| `--text-2xs` | `text-2xs` | Badges, title-bar glyphs, dense calendar grid text | 11 px / 14 px | `TitleBar`, `ComposerApp`, `EditorToolbar` shortcut chips, `ImagesTab` and `AccountSecurityTab` badges, calendar grid and timeline text, attendee badges |
| `--height-control-sm` | `h-control-sm` | Small control | 28 px | `ChatListHeader` |
| `--height-control` | `min-h-control` | Standard control | 32 px | Accounts column rows, list search, composer, `ToolbarButton` |
| `--height-header` | `min-h-header` | Pane header bar | 61 px (conversation header: two 22 px lines) | `ChatViewHeader`, `ChatListHeader` top row, kit `ListHeader` and `DetailPane`, Calendar `ViewSwitcher` |
| `--surface-rail` | `bg-surface-rail` | Extension rail | `--muted` at 30 % | `ExtensionRail` |
| `--surface-sidebar` | `bg-surface-sidebar` | Accounts column | `--muted` at 30 % | `App.svelte`, `SidebarFrame` |
| `--surface-list` | `bg-surface-list` | Chat list, Contacts list | `--background` | `App.svelte`, `ContactList` |
| `--surface-content` | `bg-surface-content` | Conversation pane, Calendar views | `--background` | `App.svelte`, `DetailPane`, `CalendarPane` and its views |
| `--selected` | `bg-selected` | Selected chat row: a soft Frost tint | `--muted` | `ChatRow` |
| `--bubble-own` | `bg-bubble-own` | My chat bubbles: a faint Frost tint (others' use `bg-muted`) | `--primary` at 20 % | `ChatBubble`, `ChatPendingBubble` |
| `--fg-primary` | `text-fg-primary` | Main text | `--foreground` | Chat rows, headers, bubbles |
| `--fg-secondary` | `text-fg-secondary` | Secondary text | `--muted-foreground` | Previews, subjects, folder names, sender line |
| `--fg-tertiary` | `text-fg-tertiary` | Least prominent text | `--muted-foreground` | Times, day separators, group toggles |

Some pixel budgets are computed from these line heights, so update them when
the type scale changes: `--height-header`, `PILL_PX`/`BAND_PX` in Calendar's
`MonthView.svelte`, the row heights in `ChatRow.svelte`, and the default size
shown by the composer and signature toolbars.

Surface and text-tier variables hold raw HSL triples, like the existing
`--background`, and are wrapped in `hsl()` by the matching `--color-*`
variable. The rail and sidebar defaults carry their own `/ 0.3` alpha, so
don't add an opacity modifier (`bg-surface-sidebar/50`) to them. The sidebar
overlay of the narrow layout keeps `bg-background`; the viewer overlay uses
`bg-surface-content`. Contacts and Calendar use the same surface tokens and
header height as Mail.

## Theme values and the contrast check

`built-in.css` sets every surface and text tier per theme: surfaces step
from the rail (darkest in Dark, lowest in Light) up to the conversation pane,
and `--divider` is a hairline darker than all of them, so it is also the pane
border colour (`border-divider`). Every other `border-divider` user (composer,
headers, attachment cards) draws the same hairline.

`node tools/contrast-check.mjs` (also in `make lint` and CI) prints each text
tier against each pane surface, the hover and others'-bubble fill (`--muted`)
and the selected-row fill (`--selected`), plus primary text on my bubbles
(`--bubble-own`), for both themes, and exits 1 if any pair is below 4.5:1.
`--selected` and `--bubble-own` are the Frost accent mixed over
`--surface-list` and `--surface-content` (Dark nord8 at 14 % and 25 %, Light
nord10 at 14 % and 18 %, approximated on the low-chroma surfaces); keep that relation if the
surfaces change. The surfaces are near-neutral greys (10-14 % saturation) so the Frost and Aurora
accents carry the colour.

Light `--primary` (and `--ring`) is 213 40% 40%, a darker nord10 that gives 4.5:1 or more for
`--primary-foreground` on it and for primary text on `--selected` and the sidebar fills. The audit
checks both (UI1). Corner radius is `--radius: 0.625rem` in `app.css`; `rounded-lg`, `-md` and `-sm`
derive from it (10, 8 and 6 px). The base type sizes are unchanged: row and header heights are
computed from them.

Final values (HSL triples, `built-in.css`):

| Token | Dark | Light |
|---|---|---|
| `--surface-rail` | 220 10% 15% | 220 12% 90% |
| `--surface-sidebar` | 220 10% 18% | 220 12% 93% |
| `--surface-list` | 220 10% 20.5% | 220 14% 96% |
| `--surface-content` | 220 10% 22% | 220 14% 98% |
| `--divider` | 220 10% 12% | 220 10% 84% |
| `--selected` | 209 12% 27% | 216 18% 90% |
| `--bubble-own` | 205 13% 32.5% | 214 20% 90.5% |
| `--fg-primary` | 219 28% 95% | 220 16% 22% |
| `--fg-secondary` | 219 24% 85% | 220 17% 30% |
| `--fg-tertiary` | 219 14% 72% | 220 11% 39% |

Reading width is not a token: `data-reading-width` on `<html>` (the "Reading
width" chat setting) selects the plain variables `--chat-column` (Narrow 52 /
Standard 60 / Wide 72 rem, the centred column for bubbles, pending sends and
day separators), `--chat-bubble` (30 / 36 / 44 rem, the widest a bubble grows,
never past 75% of the column) and `--chat-stream` (46 / 46 / 54 rem, the
centred column for email cards and full bodies) in `app.css`. Inside the chat
column (24 px side padding) their messages sit on the left, mine on the right;
the conversation header and the composer sit in the same centred column.
