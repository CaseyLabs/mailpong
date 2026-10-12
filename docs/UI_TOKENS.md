# UI design tokens

Named values for type, sizes, pane surfaces and text tiers. Change a value in
one place and every use follows. Tokens live in `frontend/src/app.css`
(`@theme inline`) and `frontend/src/themes/_defaults.css`; themes override
the surface and text-tier triples in `frontend/src/themes/built-in.css`.

Tailwind 4 turns a `@theme` variable into utilities only inside its namespace
(`--text-*` is font size, `--color-*` colour, `--height-*` height), so the text
tiers are `--fg-*`, not `--text-*`; the latter would collide with
`text-primary` and `text-secondary`.

| Token | Utility | Role | Default | Used in |
|---|---|---|---|---|
| `--text-meta` | `text-meta` | Timestamps, secondary labels | 12 px / 16 px (`text-xs`) | `ChatListHeader`, `ChatRow`, `FolderTreeItem`, `UnifiedInboxSection` |
| `--text-body` | `text-body` | Body and list text | 14 px / 20 px (`text-sm`) | `ChatRow`, `ChatViewHeader` |
| `--text-title` | `text-title` | Pane and dialog titles | 16 px / 24 px (`text-base`) | Not yet used; `text-base` is equivalent |
| `--text-caption` | `text-caption` | Chips and section labels | 11 px, no line height | `ChatBubble`, `ChatGroupHeader`, `ChatLowGroupHeader`, `SourceSidebar` |
| `--text-compact` | `text-compact` | Quoted text | 13 px, no line height | `ChatBubble` |
| `--text-2xs` | `text-2xs` | Badges, title-bar glyphs | 10 px / 12 px | `TitleBar`, `ComposerApp` |
| `--height-control-sm` | `h-control-sm` | Small control | 28 px | `ChatListHeader` |
| `--height-control` | `min-h-control` | Standard control | 32 px | Accounts column rows, list search, composer, `ToolbarButton` |
| `--height-header` | `min-h-header` | Pane header bar | 59 px (conversation header) | `ChatViewHeader`, `ChatListHeader` top row |
| `--surface-rail` | `bg-surface-rail` | Extension rail | `--muted` at 30 % | `ExtensionRail` |
| `--surface-sidebar` | `bg-surface-sidebar` | Accounts column | `--muted` at 30 % | `App.svelte`, `SidebarFrame` |
| `--surface-list` | `bg-surface-list` | Chat list | `--background` | `App.svelte` |
| `--surface-content` | `bg-surface-content` | Conversation pane | `--background` | `App.svelte`, `DetailPane` |
| `--fg-primary` | `text-fg-primary` | Main text | `--foreground` | Chat rows, headers, bubbles |
| `--fg-secondary` | `text-fg-secondary` | Secondary text | `--muted-foreground` | Previews, subjects, folder names, sender line |
| `--fg-tertiary` | `text-fg-tertiary` | Least prominent text | `--muted-foreground` | Times, day separators, group toggles |

Surface and text-tier variables hold raw HSL triples, like the existing
`--background`, and are wrapped in `hsl()` by the matching `--color-*`
variable. The rail and sidebar defaults carry their own `/ 0.3` alpha, so
don't add an opacity modifier (`bg-surface-sidebar/50`) to them. The sidebar
overlay of the narrow layout keeps `bg-background`; the viewer overlay uses
`bg-surface-content`. Contacts and Calendar panes do not use the surface
tokens yet (see ISSUES.md).

## Theme values and the contrast check

`built-in.css` sets every surface and text tier per theme: surfaces step
from the rail (darkest in Dark, lowest in Light) up to the conversation pane,
and `--divider` is a hairline darker than all of them, so it is also the pane
border colour (`border-divider`). Every other `border-divider` user (composer,
headers, attachment cards) draws the same hairline.

`node tools/contrast-check.mjs` (also in `make lint` and CI) prints each text
tier against each pane surface and the selected-row fill (`--muted`) for both
themes, and exits 1 if any pair is below 4.5:1.

Final values (HSL triples, `built-in.css`):

| Token | Dark | Light |
|---|---|---|
| `--surface-rail` | 220 16% 16% | 219 27% 89% |
| `--surface-sidebar` | 220 16% 18.5% | 219 27% 92% |
| `--surface-list` | 220 16% 20.5% | 219 28% 95% |
| `--surface-content` | 220 16% 22% | 219 28% 97.5% |
| `--divider` | 220 16% 13% | 219 20% 84% |
| `--fg-primary` | 219 28% 95% | 220 16% 22% |
| `--fg-secondary` | 219 24% 85% | 220 17% 30% |
| `--fg-tertiary` | 219 14% 72% | 220 11% 39% |

Reading width is not a token: `data-reading-width` on `<html>` (the "Reading
width" chat setting) selects the plain variables `--chat-column` (Narrow 34 /
Standard 40 / Wide 48 rem) and `--chat-stream` (46 / 46 / 54 rem) in
`app.css`.
