# PLAN: Visual refinement of MailPong, informed by UpNote

**Status:** Proposed, 2026-10-11. M0 can start immediately. Milestones marked
**Approval gate** need the owner's explicit go-ahead on before/after
screenshots before they are committed.
**Path:** `PLAN.md` (root). The earlier Aerion-to-MailPong migration plan was
removed (`19d6a89`), so a bare `/project-next-milestone` selects this plan.
Working files this plan creates (`contrast.mjs`, `REFERENCE.md`) live in
`plans/upnote-ui/`.
**Run with:** `/project-next-milestone`, one milestone per session. That skill
ends by running `/project-finish-milestone`.
**Branch:** `feat/ui-polish-update` (created from `main` at `162e5a2`; land
per AGENTS.md "Landing a Branch" after the last milestone).
**Themes:** since `162e5a2` there are exactly two themes, Dark and Light
(the former Nord Dark / Nord Light palettes, both in
`frontend/src/themes/built-in.css`), plus System, which resolves to one of
them. `nord.css` no longer exists.

## Scope

The owner likes the UI of the UpNote note-taking app. Study it, decide which of
its lessons fit MailPong's chat-style mail UI (visual hierarchy, spacing, sizes,
heights, widths, fonts, colour layering), and implement the ones that survive
that review, in small reversible steps.

### Objectives

1. Capture a measured reference of UpNote and a baseline of MailPong, so every
   change is argued from numbers and screenshots rather than impressions.
2. Give MailPong a small, named set of design tokens (type scale, spacing,
   control heights, surfaces, text tiers) and route the chat UI through them.
3. Apply the UpNote lessons that fit: clear three-tier text hierarchy, tonal
   pane surfaces instead of hard borders, calmer list rows, quieter chrome, and
   a readable, user-adjustable reading width.
4. Keep both built-in themes (Dark and Light) legible and at WCAG AA contrast.

### Out of scope

- Copying UpNote's assets, icons, fonts, colours or layout pixel-for-pixel.
  UpNote is closed-source; lessons are taken as principles and proportions.
- New mail or chat features. M7 (focus/reading options) is optional and needs
  separate approval.
- Changes to the composer window (`ComposerApp.svelte`, `composer/`) beyond
  inheriting shared tokens, the extensions' own views (Contacts, Calendar)
  beyond inheriting tokens, and email body rendering (`viewer/EmailBody.svelte`).
- Dependency changes, except a bundled font if the owner picks one in M3.
- Wails, Svelte or Tailwind upgrades.

## Claude Code execution contract

- Read `AGENTS.md`, `.claude/rules/working-style.md`, `ISSUES.md` and the
  `run-mailpong` skill before M0. Repository instructions win over this file.
- Read only the current milestone's section plus the Decision Log and
  Surprises entries it depends on. Use `rg` and `sed -n` on large files.
- Values in this plan are **starting targets**. M0's measurements and the
  owner's choices override them; record every override in the Decision Log.
- Prefer token changes in `frontend/src/app.css` and `frontend/src/themes/*.css`
  over per-component edits. Keep components under 500 lines (AGENTS.md).
- Preserve keyboard accessibility: focus rings, `focus-visible` reveal of
  hover-only controls, and every shortcut in `docs/KEYBOARD_SHORTCUTS.md`.
- Every UI milestone ends with the screenshot set in "Visual check protocol"
  below. Store screenshots in the session scratchpad, not the repo, except the
  README shot in M8.
- Dev builds force Dark (`import.meta.env.DEV` in `stores/theme.svelte.ts`);
  set `data-theme="light"` on `<html>` by hand for Light shots, as
  `headless-shot.js` does. Light coverage has been partial before (root
  PLAN.md M12), so check Light shots individually.
- If a milestone changes a Go binding (M6), stop the dev app (`dev.sh stop`)
  before `make generate`, then revert unrelated churn in
  `frontend/wailsjs/runtime/*` (AGENTS.md "Layout Notes").
- Approval gates: post before/after crops to the owner with AskUserQuestion
  (recommended option first) and wait. Don't commit a gated milestone first.
- Don't touch the persisted Aerion identifiers listed in AGENTS.md.

### Visual check protocol (every UI milestone)

Use Playwright (`ui-matrix.js`), not Claude in Chrome: it writes PNGs from
one Bash call and `compare` checks them without loading images into context,
while every Claude in Chrome screenshot is returned as an image plus several
calls per shot (resize, theme, hover, focus).

**Detect the environment first.** Claude Code sets `CLAUDE_CODE_REMOTE=true`
in cloud sessions; it is unset locally. Decide once per session:

```bash
if [ "$CLAUDE_CODE_REMOTE" = true ] || [ -d /opt/node-tools/node_modules/playwright ]; then echo cloud; else echo local; fi
```

| | Local (owner's desktop) | Cloud (Claude Code on the web) |
|---|---|---|
| Playwright module | `~/.cache/mailpong-upnote-ui/node_modules/playwright` (1.63.0; reinstall per M0 step 2 if missing) | `/opt/node-tools/node_modules/playwright` |
| Chromium | Playwright's own (`~/.cache/ms-playwright/chromium-1243`), so no `executablePath` | `executablePath: '/opt/pw-browsers/chromium'`, as in `headless-shot.js` |
| Display for `dev.sh start` | the real desktop; no Xvfb (not installed) | `Xvfb :99 -screen 0 1600x1000x24` with `DISPLAY=:99`, plus `PATH=$PATH:$HOME/go/bin` |
| Claude in Chrome | available, fallback only | not available |
| `before` set | `~/.cache/mailpong-upnote-ui/before/` persists across sessions | container is fresh each session: rebuild it (below) |

Then follow the `run-mailpong` skill: start Xvfb (cloud only), `dev.sh start`
and `serve-fakes.py` with `run_in_background`, never with `&`, then
`dev.sh wait`. `$SCRATCH` below means the session scratchpad directory from
the system prompt; substitute the literal path.

M0 adds `.claude/skills/run-mailpong/pw.js`, a small helper that does the
detection above and exports a `launch()` returning a Playwright browser:
`$PLAYWRIGHT_MODULE` (and optional `$PLAYWRIGHT_CHROMIUM`) override both
paths; otherwise it picks the cloud or local row of the table, and if the
chosen module is missing it exits with the reinstall command rather than
falling back silently. Switch `headless-shot.js` to it too, so that script
also runs locally. Never add Playwright to `frontend/package.json`.

`headless-shot.js` only captures the README shot and 520×330
title-bar/sidebar crops per theme, so it does not cover this matrix. M0 also
adds `.claude/skills/run-mailpong/ui-matrix.js` (uses `pw.js`, same page
setup as `headless-shot.js`), taking `<outdir> <prefix>` and writing
`<prefix>-<theme>-<width>-<accounts>-<previews>.png` plus hovered and focused
row crops. Every later milestone runs it as
`node .claude/skills/run-mailpong/ui-matrix.js "$SCRATCH" after` and compares
against the `before` set.

Before/after pairs must come from the same environment, because fonts and
rendering differ between the desktop and the cloud image. Locally, use M0's
set in `~/.cache/mailpong-upnote-ui/before/`. In the cloud, or locally when
that directory is missing, rebuild `before` first: on the clean checkout,
before editing anything, run
`node .claude/skills/run-mailpong/ui-matrix.js "$SCRATCH" before`, then make
the changes and take `after`.

Keep context small when inspecting: run `compare -metric AE` on every
before/after pair first and Read only the pairs that differ (or their
`diff.png`), preferring the row crops over full windows. Use Claude in Chrome
(local only) if Playwright cannot run, and then only for the shots needed to
answer a specific question. Its screenshots are not saved as PNG files, so
they can never be the before/after set. Cover:

- Window widths 1400 px (full layout) and 900 px (medium, 768–1024) and 700 px
  (narrow), in CSS pixels.
- Accounts column expanded and collapsed (AGENTS.md requirement).
- Dark and Light, every time (there are only two).
- Chat list with and without message previews (Settings → General), and a
  selected, a hovered and a keyboard-focused row.
- Stop everything you started: `dev.sh stop`, `pkill -f 'serve-fake[s]'`,
  and in the cloud `pkill Xvfb`.

Frontend checks for every milestone: `cd frontend && npm run lint && npm run check && npm run build`.
Run `make lint` before committing (it includes the brand check).

## Findings so far (2026-10-11, revalidate in M0)

### MailPong today (from code at `162e5a2` and the owner's screenshot)

The owner's screenshot is a native window at about 1.75 device pixels per CSS
pixel (the 48 px rail measures about 84 device px). All numbers below are CSS px.

| Area | Current value | Source |
|---|---|---|
| Font family | `system-ui` stack, so it differs per Linux desktop | `app.css` `--font-sans` |
| Type usage in components | `text-sm` 275, `text-xs` 209, `text-[10px]` 29, `text-lg` 15, `text-[11px]` 2, `text-[13px]` 1, `text-base` 1 | `rg` over `lib/components` |
| Extension rail | 48 px wide (`w-12`), 48 px buttons | `rail/ExtensionRail.svelte` |
| Accounts column | default 240, min 180, max 400 | `stores/uiState.svelte.ts` |
| Chat list | default 420, min 280, max 600 | same |
| Reading column | bubbles 40rem (`--container-chat-column`), cards 46rem (`--container-chat-stream`), centred | `app.css` |
| Title bar | 40 px (`h-10`) | `common/TitleBar.svelte` |
| Chat row | 44 px avatar, `px-3 py-2 gap-3`, name `text-sm`, preview `text-sm`, time `text-xs`; about 60 px tall with preview, 44 without | `chat/ChatRow.svelte` |
| Row selection | `bg-muted`, `rounded-lg`, inset `mx-1.5` | same |
| Bubbles | `text-sm`, `px-3.5 py-2`, `rounded-2xl`, max `min(75%,36rem)` | `chat/ChatBubble.svelte` |
| Day separator | `text-xs`, `pt-4 pb-1.5`, hairline in `--divider` | `chat/ChatDaySeparator.svelte` |
| Conversation header | `px-3 py-2`, name and subject both `text-sm` | `chat/ChatViewHeader.svelte` |
| List header | title `font-semibold` at 16 px, icon buttons `p-2`, search `py-1.5 text-sm`, filter tabs `text-xs` | `chat/ChatListHeader.svelte` |

What the screenshot shows (Dark, then named Nord Dark):

1. **One flat surface.** In Dark, `--background`, `--card` and `--popover`
   are all nord0, so rail, list and conversation read as one plane split by
   1 px nord3 borders. Light has the same problem: all three are nord6.
2. **Weak text hierarchy.** `--muted-foreground` is nord4 against nord6
   foreground, so previews, timestamps and meta lines sit almost as bright as
   names and message text.
3. **Loud chrome.** The solid primary compose button is the brightest element
   on screen; the conversation header spends width on text labels
   ("Archive", "Delete").
4. **Busy bubbles.** Every bubble shows its "…" menu at rest (50% opacity).
5. **Large, similar avatars.** 44 px avatars with 30% tints look alike in
   Dark; they dominate the list.
6. **No scale discipline.** Arbitrary sizes (`text-[10px]`, `[11px]`, `[13px]`)
   sit next to the Tailwind scale.
7. **Unused density setting.** `MessageListDensity`
   (`micro|compact|standard|large`) is stored and offered in Settings →
   General, and `kit/Avatar.svelte` has density sizes, but `ChatRow.svelte`
   ignores it.
8. **Focus mode exists in part.** `Shift+F` toggles focus mode for a message
   (`docs/KEYBOARD_SHORTCUTS.md`). It lives in `App.svelte` (`focusMode:
   'off' | 'thread' | 'message'`) and was built for the old viewer overlay,
   not `ChatView`.

### UpNote (public sources; M0 measured the owner's screenshots)

M0 measured four owner screenshots (titles-only list, Linux desktop): see
`plans/upnote-ui/REFERENCE.md`. Its key result: UpNote's panes share one
colour (`#202222` Dark, `#FFFFFF` Light) with a darker hairline, lighter chrome
bars (`#2C2C2F`, `#F8F8F8`), inset row dividers and a single text colour. The
bullets below are the public-source hypotheses.

- Three panes: a sidebar (All Notes views, a **Quick Access** section, nested
  notebooks with icons and note counts, tags, templates), a note list, and the
  editor.
- Note list entries show a title, a text snippet and, when the note has one,
  its first image as a small thumbnail. A **compact mode** shows titles only.
  Pinned notes stay on top.
- The editor has user settings for **font**, **font size**, **line spacing**,
  **paragraph spacing** and a **line length** slider (Settings → Editor).
- **Focus mode** (Ctrl+Shift+F) hides the sidebar and note list but keeps the
  top bar and formatting toolbar.
- Themes and dark mode; reviewers consistently describe it as clean, simple
  and minimal. One user's criticism: sidebars could use softer colours and the
  New Note button is distracting, which applies equally to MailPong's
  compose button.

Lessons to test against M0's measurements (hypotheses, not facts):

| UpNote trait | MailPong translation |
|---|---|
| Three text tiers (title / snippet / date) separated by weight and grey | Name / preview / time tiers with distinct tokens and AA contrast |
| Panes separated by tone more than by lines | `--surface-*` tokens per pane; borders demoted to hairlines or removed |
| Calm list rows, optional titles-only mode | Wire the existing density setting into `ChatRow`; smaller avatars at compact densities |
| Quiet chrome, content first | Tonal compose button, icon-only header actions with tooltips, hover-revealed bubble actions |
| Line length slider | A reading-width setting mapped to `--container-chat-column` / `--container-chat-stream` |
| Consistent typeface across platforms | Optional bundled UI font (owner decision, M3) |
| Header rows aligned across panes | One shared header height for list header and conversation header |

## Milestones

### M0. Baseline and UpNote reference (no code changes)

Goal: replace the hypotheses above with measurements.

Steps:

1. Confirm you are on `feat/ui-polish-update` and the tree is clean apart
   from `PLAN.md` and `plans/upnote-ui/`.
2. Detect the environment ("Visual check protocol"). Locally, Playwright is
   already installed (2026-10-11):
   `~/.cache/mailpong-upnote-ui/node_modules/playwright` at 1.63.0, which
   matches the cached `chromium-1243`; a headless launch and screenshot
   worked. If it is missing, reinstall with
   `npm install --prefix ~/.cache/mailpong-upnote-ui --no-save playwright@1.63.0`
   (never into `frontend/package.json`). In the cloud, the image provides it.
   Write `.claude/skills/run-mailpong/pw.js` and `ui-matrix.js`, switch
   `headless-shot.js` to `pw.js`, and in the skill's SKILL.md replace the
   cloud-only "Headless (cloud) path" with one Playwright section that
   gives the local/cloud table and the detection command. Run
   `headless-shot.js` once locally to confirm the switch. Take the MailPong
   baseline with prefix `before`. Copy the `before` set to a stable path
   outside the session scratchpad (`~/.cache/mailpong-upnote-ui/before/`), since
   later sessions get a new scratchpad; record the path in Surprises &
   Discoveries. Relative timestamps in fake data change between runs; note
   which regions differ run-to-run by taking the set twice.
3. Measure UpNote. Ask the owner (AskUserQuestion) which source to use:
   - **Web app with Claude in Chrome (recommended):** the owner signs in at
     `https://web.getupnote.com` in their Chrome; open it in a new tab and run
     one `javascript_tool` call per theme that returns a compact JSON of
     `getComputedStyle` values (no screenshots; at most one small one to
     confirm which elements were picked). Chrome is the right tool here
     despite the screenshot rule above: it reuses the owner's sign-in, and
     text results cost about the same as a Playwright script would. Elements
     to measure: representative ones (sidebar section label,
     sidebar item, item count, note-list header, note title, snippet, date,
     selected row, list divider, editor body, editor title, toolbar button).
     Record `font-family`, `font-size`, `font-weight`, `line-height`,
     `letter-spacing`, `color`, `background-color`, padding, height, border,
     border-radius, and pane widths, for light and dark themes.
   - **Owner screenshots:** UpNote desktop at 100% display scaling, light and
     dark, with the note list in normal and compact mode. Measure with Read.
   Don't type into or change the owner's notes, and don't copy note titles or
   content into any file or screenshot kept outside the scratchpad.
4. Write `plans/upnote-ui/contrast.mjs` (plain Node, no dependency): given
   foreground/background colour pairs (hex, `rgb()` or HSL triples), print
   WCAG ratios. M2 grows it into the theme checker under `tools/`.
5. Write `plans/upnote-ui/REFERENCE.md`: one table per area (UpNote vs
   MailPong, with the measurement method for each value), the contrast ratio
   of every text tier against its surface in both apps, and the final list of
   lessons, each marked adopt / adapt / reject with one line of reasoning.
6. Update "Findings so far" and the target values in M1–M6 if the
   measurements disagree.

Acceptance criteria:

- `REFERENCE.md` exists and every row names its source (computed style,
  screenshot measurement, or code).
- Contrast ratios are computed by `contrast.mjs` (not estimated) and its
  command is recorded.
- Each lesson in the table above has an adopt / adapt / reject verdict.
- `ui-matrix.js` produces the full `before` set as PNG files on disk, and is
  documented in the `run-mailpong` skill. Running
  `compare -metric AE` on two back-to-back `before` runs works and its
  run-to-run differences are recorded. If Playwright cannot be made to run
  here, stop and ask the owner: M1's acceptance check depends on it.
- No app code changed.

Validation: `git status --short` shows only `PLAN.md`, `plans/upnote-ui/` and
`.claude/skills/run-mailpong/`.

### M1. Design tokens (no intended visual change)

Goal: a named vocabulary the later milestones change in one place.

Files: `frontend/src/app.css`, `frontend/src/themes/_defaults.css`, new
`docs/UI_TOKENS.md`, and components that use arbitrary sizes.

Steps:

1. Add tokens (names are proposals, values match today's rendering). This is
   Tailwind 4: a variable in `@theme` creates utilities only in its
   namespace (`--text-*` = font size, `--color-*` = colour, `--spacing-*` /
   `--height-*` = sizes), so names must not collide with existing utilities
   such as `text-primary` and `text-secondary` (from `--color-primary`,
   `--color-secondary`).
   - Type roles in `app.css` `@theme inline`: `--text-meta` (12 px, today's
     `text-xs`), `--text-body` (14 px, `text-sm`), `--text-title` (16 px),
     each with a `--text-<role>--line-height`. Keep `--text-2xs` for badges
     and map the `text-[10px]` uses to it; map `[11px]` and `[13px]` to the
     nearest role or a named token.
   - Control heights in `@theme inline`: `--height-control-sm` (28 px),
     `--height-control` (32 px), `--height-header` (today's list and
     conversation header heights; measure first, they currently differ),
     giving `h-control`, `h-control-sm`, `h-header`. Check in the built CSS
     that the utilities are emitted; if not, use `--spacing-*` instead.
   - Surfaces: raw HSL triples per theme beside the existing colour
     variables (`--surface-rail`, `--surface-sidebar`, `--surface-list`,
     `--surface-content` in `themes/_defaults.css`, overridden in
     `built-in.css` from M2), each defaulting to the colour that pane uses
     today, and mapped in `@theme inline` as
     `--color-surface-rail: hsl(var(--surface-rail))` etc. for
     `bg-surface-*`.
   - Text tiers, same pattern: `--fg-primary` (= `--foreground`),
     `--fg-secondary` (= `--muted-foreground`), `--fg-tertiary` (new; default
     `--muted-foreground`), mapped as `--color-fg-*` for `text-fg-*`.
   Later milestones refer to these as the surface tokens, text tiers
   (`fg-primary/secondary/tertiary`), type roles and control heights.
2. Point the rail, Accounts column, chat list, conversation pane and their
   headers at the surface and size tokens.
3. Replace arbitrary `text-[Npx]` classes in `chat/`, `rail/`, `sidebar/`,
   `kit/` and `common/` with tokens (6 uses at `162e5a2`). The other 32, in
   `settings/`, `composer/`, `viewer/`, `list/` and elsewhere, are out of
   scope; leave them unless one is a one-line swap in a file this plan
   already touches.
4. Write `docs/UI_TOKENS.md` (one table: token, role, default, where used) and
   link it from `frontend/README.md`.

Acceptance criteria:

- `rg -n 'text-\[[0-9]+px\]' frontend/src/lib/components/{chat,rail,sidebar,kit,common}`
  prints nothing.
- Before/after screenshots at 1400 px in Dark and Light are pixel-identical
  apart from the run-to-run regions noted in M0, or differ only where an
  arbitrary size snapped to a token (list each such place in Surprises &
  Discoveries). Compare with
  `compare -metric AE before.png after.png diff.png` (ImageMagick).
- Lint, svelte-check and build pass.

### M2. Text hierarchy and tonal surfaces — **Approval gate**

Goal: UpNote's calm, layered look: three readable text tiers and panes told
apart by tone.

Files: `frontend/src/themes/built-in.css` (both themes),
`frontend/src/app.css`, `tools/contrast-check.mjs` (grown from M0's
`plans/upnote-ui/contrast.mjs`, plain Node, no new dependency).

Steps:

1. Per theme, set `--surface-*` so the rail and Accounts column are one step
   off the list, and the conversation pane is the calmest surface (match
   UpNote's ordering from M0). Dark starting point: rail/sidebar nord1,
   list nord0, content nord0, with borders between same-tone panes only.
   Note `--divider` and `--muted` are both nord1 in Dark, so a nord1 surface
   hides dividers; pick a different divider tone there. (M0: UpNote keeps panes
   one colour and uses lighter header bars and a darker hairline, so keep the
   tone step subtle and consider a lighter header band.) Light has the same
   flat nord6 plane: starting point rail/sidebar nord5, list and content
   nord6 (white-ish), and `--divider` is nord5 there too, so the same
   divider caveat applies.
2. Demote pane borders to `--divider` hairlines where tones already separate
   panes; remove them where they double up.
3. Set `--fg-secondary` and `--fg-tertiary` per theme so primary >
   secondary > tertiary is visible, each ≥ 4.5:1 on every surface it sits on
   (tertiary ≥ 4.5:1 too; it carries times and counts). Dark today has
   nord4 secondary on nord0, which is high contrast but too close to nord6;
   try a mix toward nord3/nord4 and verify the ratio.
4. Use the tiers: names and message text primary; previews, subjects and
   folder names secondary; times, counts, day separators and meta lines
   tertiary.
5. Write the contrast script: parses the theme CSS, resolves `hsl()` triples,
   prints a pass/fail table for every (text tier, surface) pair per theme, and
   exits 1 on a failure. Add it to `make lint` only if it runs in under a
   second; otherwise document the command in `docs/UI_TOKENS.md`.

Acceptance criteria:

- The contrast script exits 0 for all themes and the output is pasted into
  Progress.
- In screenshots, the three tiers are distinguishable in every theme, and the
  rail, Accounts column, list and conversation are distinguishable without
  relying on a border.
- Owner approved the before/after crops.

### M3. UI typeface — **Approval gate (owner decision)**

Goal: decide whether MailPong bundles a UI font so metrics stop varying by
Linux desktop, as UpNote's do across platforms.

Steps:

1. From M0, record UpNote's UI font. Present options via AskUserQuestion:
   - Keep `system-ui` (no download size, native feel; metrics vary).
   - Bundle a variable, OFL-licensed UI font (for example Inter; or the
     UpNote family if it is OFL), WOFF2, latin + latin-ext subsets only,
     `font-display: swap`, served from `frontend/src/assets/fonts/`.
   Give the size cost of each in KB. Mention that Nunito
   (`assets/fonts/nunito-v16-latin-regular.woff2`, regular weight only,
   latin only) is already shipped but unused: it is a rounded display face
   and lacks the weights and latin-ext coverage a UI font needs. Unless the
   owner picks it, delete the file, its `OFL.txt` and its
   `THIRD_PARTY_NOTICES.md` entry in this milestone (keep `OFL.txt` if the
   new font is OFL and reuses it).
2. If bundled: `@font-face` in `app.css`, update `--font-sans` with the system
   stack as fallback, add the licence to `THIRD_PARTY_NOTICES.md`, enable
   `tabular-nums` where times and counts align, and check that non-Latin email
   content still falls back to system fonts (AGENTS.md: email content stays
   fully international).
3. Re-measure line heights; adjust `--text-<role>--line-height` so row heights from
   M1 don't drift by more than 1 px unintentionally.

Acceptance criteria:

- Decision and reasoning in the Decision Log.
- If bundled: no network request for fonts (works offline and in Flatpak),
  licence entry present, a CJK and an Arabic sample subject render with a
  fallback font in the chat list, and `npm run build` output size change is
  recorded.

### M4. Chat list rows

Goal: UpNote-style list rows that scan quickly, and a density setting that
actually works.

Files: `chat/ChatRow.svelte`, `chat/ChatGroupHeader.svelte`,
`chat/ChatLowGroupHeader.svelte`, `chat/ChatList.svelte`, `kit/Avatar.svelte`, `settings/GeneralTab.svelte`
(labels only if needed), `lib/i18n/locales/en.json` (English only).

Steps:

1. Wire `getMessageListDensity()` into `ChatRow` (avatar size, padding, gap,
   preview lines) and update the `contain-intrinsic-size` values per density.
   Starting targets, standard density: 36 px avatar, row ≈ 56 px with preview;
   compact: 28 px avatar, ≈ 44 px; micro: no preview, ≈ 32 px; large:
   today's 44 px avatar. Settle against M0.
2. Hierarchy inside the row: name `--text-body` semibold when unread, medium
   otherwise; preview `--text-body` or `--text-meta` in `--fg-secondary`;
   time `--text-meta` `--fg-tertiary` with `tabular-nums`. Unread shows by
   weight plus the existing dot, not by brightening everything.
3. Selection: keep the inset rounded pill; use a surface-relative tint so it
   reads on the new `--surface-list`. Keep the keyboard focus ring.
4. Group headers ("Chats", low priority): small caps-style label in
   `--fg-tertiary` with `tracking-wide`, like UpNote's section labels.
5. There is no virtualizer: rows use `content-visibility: auto` with
   `contain-intrinsic-size` 60/44 px (`ChatRow.svelte` styles). Keep those
   estimates in step with each density, and check scroll position and
   keyboard selection survive a density change at runtime.

Acceptance criteria:

- Changing density in Settings changes row height live, for all four values,
  with no overlapping or clipped rows (screenshots of each).
- Rows with previews off match the `micro`/titles-only look.
- Keyboard navigation (arrow keys, Enter) and the focused ring still work.
- `ChatRow.svelte` stays under 500 lines.

### M5. List header, search, filters and Accounts column — **Approval gate (compose button)**

Goal: quieter chrome, consistent control heights, aligned headers.

Files: `chat/ChatListHeader.svelte`, `common/ComposeButton.svelte`,
`sidebar/Sidebar.svelte`, `sidebar/AccountSection.svelte`,
`sidebar/FolderTreeItem.svelte`, `sidebar/UnifiedInboxSection.svelte`,
`rail/ExtensionRail.svelte`, `rail/RailButton.svelte`, `kit/Sidebar*.svelte`.

Steps:

1. Header height = `--height-header` for the list header and (in M6) the
   conversation header, so their bottom edges line up across panes.
2. Compose button: tonal (primary at low alpha with primary text) or ghost
   with an accent icon, still the obvious primary action; keep its label for
   screen readers. Present both to the owner in the gate screenshot.
3. Search field and icon buttons at `--height-control` (32 px); filter tabs at
   `--text-meta` with the active tab in `--fg-primary`.
4. Accounts column: item height ≈ 32 px, 16–18 px icons, counts right-aligned
   in `--fg-tertiary` with `tabular-nums`, section labels as in M4 step 4.
5. Rail: icons at 20–22 px on 40–44 px hit targets; active state by tint, not
   only by colour of the icon.

Acceptance criteria:

- List header and conversation header bottoms align within 1 px at 1400 px
  (measure in a screenshot).
- All interactive controls in these files are at least 28 px tall
  (`getBoundingClientRect` check recorded).
- Accounts column check passes expanded and collapsed.
- Owner approved the compose-button variant (record it in the Decision Log).

### M6. Conversation pane and reading width

Goal: content-first conversation view with UpNote's adjustable line length.

Files: `chat/ChatViewHeader.svelte`, `chat/ChatBubble.svelte`,
`chat/ChatDaySeparator.svelte`, `chat/ChatComposer.svelte`,
`chat/ChatStream.svelte`, `chat/ChatRichCard.svelte`, `app.css`,
`settings/ChatTab.svelte`, `settings/SettingsDialog.svelte` (loads and
saves every chat setting through its `Get*`/`Set*` imports),
`stores/settings.svelte.ts`,
`internal/settings/store.go` (new `chat_reading_width` key, beside
`KeyChatSendKey`), `app/settings.go` (Get/Set methods like
`GetChatSendKey`), regenerated `frontend/wailsjs/go/app/*` (`make generate`,
see the execution contract), English locale.

Steps:

1. Header: `--height-header`; name `--text-body` semibold, subject
   `--fg-secondary`. Archive and Delete become icon buttons with tooltips and
   `aria-label`s at every width (today they collapse only below
   `COMPACT_WIDTH = 860`); ask the owner if they want labels kept at wide
   widths.
2. Bubble actions ("…"): hidden at rest, shown on row hover, on
   `focus-visible`, and while the menu is open. Check touch/keyboard reach.
3. Bubble text: test `--text-body` at 14 px vs 15 px with line-height 1.45;
   pick by screenshot with the owner.
4. Day separators and timestamps in `--fg-tertiary`; keep the hairline.
5. Reading width setting (UpNote's line length): Settings → Chat → "Reading
   width": Narrow / Standard / Wide, mapping `--container-chat-column` to
   about 34rem / 40rem / 48rem and `--container-chat-stream` to at least
   46rem (a 600 px HTML email must still fit without horizontal scrolling;
   see the comment in `app.css`). Persist through the Go settings store like
   the other chat settings; Standard is today's value.
6. Composer meta line in `--fg-tertiary`; input at `--height-control` minimum
   height.

Acceptance criteria:

- With Reading width at Narrow and Wide, bubbles and the composer track the
  column and a 600 px HTML email card never scrolls horizontally at 1400 px.
- The new setting round-trips through restart (Go test for the store, like
  existing settings tests; `go test ./internal/settings/...`).
- Bubble actions are reachable by keyboard (Tab to the bubble, menu opens).
- `docs/` updated where the setting is described; `make test` passes.

### M7. Optional: reading options — **Approval gate (scope)**

Only with separate owner approval; skip and tick as "declined" otherwise.

Candidates, each independently approvable:

- **UI text size** (UpNote's font size setting): Small / Default / Large,
  implemented as a root `font-size` change since Tailwind sizes are rem-based;
  check pixel-sized values (`contain-intrinsic-size`, `Avatar` px table, pane
  widths) scale consistently.
- **Conversation focus mode** (UpNote's focus mode): hide Accounts column and
  chat list for the current conversation. `Shift+F` already toggles a message
  focus mode, but it is `App.svelte`'s `focusMode` state built for the old
  viewer overlay (`viewerIsOverlay`), not `ChatView`. First check what
  `Shift+F` does in the chat view today; then extend that state to the chat
  layout or reuse the shortcut, rather than adding a conflicting one, and
  update `docs/KEYBOARD_SHORTCUTS.md`.

Acceptance criteria: per approved item, behaviour works at all three widths,
persists if it is a setting, and is documented.

### M8. All-theme pass, docs and landing

Steps:

1. Full screenshot matrix: Dark and Light × wide/medium/narrow × Accounts
   column expanded/collapsed × previews on/off, plus Contacts and Calendar
   views. Inspect each crop; fix defects.
2. Contrast script passes for both themes.
3. Retake `docs/ss.png` per the `run-mailpong` skill ("Retaking the README
   screenshot", 1040×660, Dark, fake data only).
4. `CHANGELOG.md` entry; `docs/UI_TOKENS.md` reflects final values;
   `REFERENCE.md` gains a short "what we adopted" section.
5. Fill Outcomes & Retrospective; remove `plans/upnote-ui/` working notes that
   no longer need referencing (AGENTS.md "Planning Files").
6. Land the branch per AGENTS.md "Landing a Branch".

Acceptance criteria:

- `make lint`, `make test`, frontend lint/check/build pass on the rebased
  branch.
- Screenshot matrix reviewed with no open defects (or each logged in
  `ISSUES.md` with severity, location, failure).
- README screenshot shows only fake data.

## Risks

- **Taste is subjective.** Mitigation: approval gates on M2, M3, M5 and
  owner choices recorded in the Decision Log.
- **Theme regressions.** Dark and Light share tokens; a surface change tuned
  in Dark can break Light (dev builds hide Light by default). Mitigation:
  contrast script and Light shots in every milestone.
- **Layout shift from density and font changes** can break virtual scrolling
  or `contain-intrinsic-size` estimates. Mitigation: M4 step 5 and runtime
  density switching in the check.
- **Extensions inherit tokens.** Contacts and Calendar use `kit/` components;
  renaming or retargeting tokens may restyle them. Check both views in each
  milestone's screenshots and follow `docs/EXT_RULES.md` for kit changes.
- **UpNote access.** If neither a signed-in web session nor screenshots are
  available, M0 records the gap and later milestones proceed on MailPong's own
  baseline with the owner judging screenshots.

## Progress

- [x] M0. Baseline and UpNote reference (2026-10-11; `REFERENCE.md`, `pw.js`, `ui-matrix.js`, `before` set in `~/.cache/mailpong-upnote-ui/before/`)
- [x] M1. Design tokens (2026-10-11; `docs/UI_TOKENS.md`, tokens in `app.css` / `_defaults.css`)
- [x] M2. Text hierarchy and tonal surfaces (2026-10-11; per-theme surfaces and tiers in `built-in.css`, `tools/contrast-check.mjs` in `make lint` and CI, crops in `~/.cache/mailpong-upnote-ui/m2/`; contrast: tertiary 4.80-7.27 Dark, 4.82-5.97 Light)
- [x] M3. UI typeface (2026-10-11; owner kept `system-ui`; unused Nunito font, `OFL.txt` and notices entry removed)
- [x] M4. Chat list rows (2026-10-11; per-density table in `ChatRow.svelte`, rows 32/44/60-44/76-60 px, group labels in caps; shots in the session scratchpad)
- [x] M5. List header, search, filters and Accounts column (2026-10-11; owner chose tonal Compose; list top row `min-h-header` with divider, bottoms align at 99 px)
- [x] M6. Conversation pane and reading width (2026-10-11; `chat_reading_width` setting, `data-reading-width` on `<html>`, header labels kept at wide widths, 15 px bubble text; shots in the session scratchpad)
- [x] M7. Optional reading options (2026-10-11; declined by owner, no code changes)
- [x] M8. All-theme pass, docs and landing (2026-10-11; matrix, Contacts and Calendar shots clean, `docs/ss.png` retaken, CHANGELOG, `docs/UI_TOKENS.md`, REFERENCE)

## Surprises & Discoveries

- 2026-10-11 (M1): after-vs-before at 1400/900/700 px, Dark and Light: only differences are 20-283 px of relative-time text (same as M0 noise); no `text-[Npx]` snapped, so no visual change.

- 2026-10-11 (M0): UpNote's panes are the same colour in both themes
  (`#202222` / `#FFFFFF`); separation is a hairline, lighter header bars and
  inset row dividers. Its text uses one colour. MailPong's Dark
  `--muted-foreground` (nord4) is only 9.08:1 vs 10.88:1 for `--foreground`,
  which explains the weak hierarchy (`REFERENCE.md`). Screenshots' device
  scale is unknown.
- 2026-10-11 (M0): `before` set path: `~/.cache/mailpong-upnote-ui/before/`
  (36 PNGs). Run-to-run noise: at most about 198 differing pixels per image
  (relative times in the fake data, and a 9x9 px spot at 700/900 px). At 700 px
  there is no Accounts toggle, so only `expanded` shots exist. An unquoted
  `==` in zsh `echo` aborts the command.

- 2026-10-11 (planning): the density setting (`MessageListDensity`) is stored
  and shown in Settings but not used by the chat list. Nord Dark uses the
  same colour for background, card and popover. The owner's screenshot is at
  about 1.75× device scale; measure in CSS px.
- 2026-10-11 (plan review): `162e5a2` cut the themes to Dark and Light and
  deleted `nord.css`; the plan was updated to match. `headless-shot.js`
  does not cover this plan's screenshot matrix, hence `ui-matrix.js` in M0.
  This machine has ImageMagick (`compare`, `magick`) but no Xvfb.
- 2026-10-11 (readiness review): the Playwright Node module is missing on
  this desktop (only its browsers are cached), and Claude in Chrome
  screenshots are not saved as files. Fixed the same day: Playwright 1.63.0
  installed to `~/.cache/mailpong-upnote-ui/` (matches `chromium-1243`). Nunito is bundled but unused (handled in M3).
  Light is as flat as Dark (background, card and popover all nord6).

## Decision Log

- 2026-10-11 (M7): owner declined both optional reading options (UI text size,
  conversation focus mode). No code changes; M8 is next.
- 2026-10-11 (M3): owner chose to keep the `system-ui` stack (0 KB added;
  metrics vary per desktop, accepted). No font is bundled, so `--font-sans`,
  line heights and `tabular-nums` are unchanged. The unused Nunito woff2, its
  `OFL.txt` and its `THIRD_PARTY_NOTICES.md` entry were deleted (-19 KB).
  Inter (variable, latin + latin-ext, est. 100-130 KB) was declined.

- 2026-10-11 (M2): Dark surfaces step rail 16 / sidebar 18.5 / list 20.5 / content 22 % lightness (rail darkest), Light 89 / 92 / 95 / 97.5 %; `--divider` is a hairline darker than all panes (13 % Dark, 84 % Light), so every `border-divider` user now draws it (composer, headers, attachment cards): accepted, matches UpNote's darker hairline. Tiers: Dark 95 / 85 / 72 %, Light 22 / 30 / 39 %; Dark tertiary was raised to 72 % so it holds 4.5:1 on the selected row (`--muted`). Read rows: subject secondary, preview tertiary; unread: subject primary, preview secondary. Sidebar rows other than folders (account headers, unified inbox) still use legacy tokens; M5 reworks that column.

- 2026-10-11 (M1): `--height-header` is 59 px, the measured conversation header (list top bar has no fixed height; M5 aligns it); applied as `min-h-header`. Surface defaults use `var(--muted) / 0.3` so the translucent rail and sidebar render identically. Type roles `meta/body/title` are defined but not yet applied (equivalent to `text-xs/sm/base`); `[11px]`/`[13px]` became named tokens `text-caption`/`text-compact` (no line height) so nothing snaps. Review: search field uses `min-h-control`; ComposerApp's glyphs also use `text-2xs`; extension panes not tokenised (ISSUES U1).

- 2026-10-11 (M0): UpNote measured from owner screenshots, not Chrome (owner's
  choice). Its panes are **not** tonally layered, so M2's surfaces are
  "adapt": a subtle tone step for the Accounts column and lighter header bars
  rather than three distinct pane tones, plus hairlines kept (Dark hairline
  darker than panes). UpNote rows are taller than MailPong's, so M4 must not
  shrink the 44 px no-preview row below what the density setting asks for.
  The compose-button critique stands (M5). A line-length slider is deferred to
  M7. Scale of the screenshots is unknown, so only ratios are used.

- 2026-10-11: plan started at `plans/upnote-ui/PLAN.md` and moved to the root
  `PLAN.md` once the migration plan was removed; only its working files
  (`contrast.mjs`, `REFERENCE.md`) stay under `plans/upnote-ui/`.
- 2026-10-11 (planning): lessons are applied through tokens first (M1), then
  visual changes behind approval gates, so each step is reversible by
  reverting token values.
- 2026-10-11 (readiness review): token names follow Tailwind 4 namespaces.
  Text tiers are `--fg-*` / `--color-fg-*` (not `--text-*`, which would
  create font-size utilities clashing with `text-primary`/`text-secondary`),
  surfaces are raw triples mapped to `--color-surface-*`, and control heights
  are `--height-*`.
- 2026-10-11 (readiness review): MailPong screenshots use Playwright
  (`ui-matrix.js`) on desktop and cloud, because it uses far fewer tokens
  than Claude in Chrome (files on disk, `compare` before Read). Claude in
  Chrome is kept only for reading UpNote's computed styles as text in the
  owner's signed-in session.
- 2026-10-11 (readiness review): Playwright's location is chosen per
  environment by `pw.js`, detecting the cloud by `CLAUDE_CODE_REMOTE=true`
  (or `/opt/node-tools/node_modules/playwright` existing). Before/after
  screenshots always come from the same environment; the cloud rebuilds
  `before` each session because its container does not persist.

- 2026-10-11 (M4): `ChatRow` reads the density from a table (avatar, padding, text size, intrinsic size) and sets `contain-intrinsic-size` inline, replacing the two CSS classes. Measured rows (Playwright with fakes, live density switch, Dark and Light, both Accounts states): micro 32 px (one line: name, subject, time), compact 44 (name, then subject with the preview inline after " — "), standard 60 with preview / 44 without (36 px avatar), large 76 / 60 (44 px avatar). Standard is 60, not the 56 target: three 20/16/16 px lines. Previews off gives the two-line name + subject layout at compact, standard and large; micro stays one line whatever the setting (so "previews off matches micro" holds loosely: the same titles-only content, two lines above micro), except a search match shows its snippet inline (two lines). Review fixes: status icons stay visible at compact with previews, intrinsic size derives from avatar and text height, account badge shrinks below 32 px avatars. Subject is `fg-secondary` (medium when unread) and preview `fg-tertiary` (contrast-checked tier), not secondary as planned, so the two lines stay distinct; unread = name weight + dot. Selection keeps `bg-muted`. Group labels: caption-size uppercase `tracking-wide` `fg-tertiary`.

- 2026-10-11 (M6): reading width is `data-reading-width` on `<html>` (set by `setChatReadingWidth` in the settings store) selecting plain `--chat-column` / `--chat-stream` variables in `app.css` (Narrow 34 / Standard 40 / Wide 48 rem column; stream 46 / 46 / 54 rem). They replace the `--container-chat-*` `@theme` tokens, because `@theme inline` bakes values into the utilities (`max-w-(--chat-column)` now). Owner answers: Archive/Delete keep their labels at wide widths (icon-only below `COMPACT_WIDTH`, as before); bubble text is 15 px with line-height 1.45 (bubble and pending bubble). Bubble actions were already hover/focus-visible only; added `has-data-[state=open]` so they stay while the menu is open. Tab moves between bubbles (existing shortcut), which reveals the actions, and Shift+F10 opens the menu; Playwright confirmed. Measured Dark and Light at 1400 / 900 px: column 544 / 640 / 768 px, stream 736 / 736 / 864 px, no horizontal overflow; fakes have no 600 px HTML email, so that is argued from stream width, not rendered. The 15 px and label changes came after those shots.

- 2026-10-11 (M5): owner approved the **tonal** Compose button (`ToolbarButton` gained `tonal`: tinted fill, accent icon, `fg-primary` label because primary text on the tint was ~3:1 in Light; height `min-h-control`; solid and ghost were dropped after the gate). Review fixes: `COMPACT_WIDTH` 364 (row now includes its padding), selected-row counts use `fg-secondary`, rail settings button 44 px, All Inboxes toggle labelled; account menu overlap logged as ISSUES U2. The list header's top row (scope, sync, Compose) is `min-h-header` with a divider so it aligns with the 59 px conversation header (both bottoms at y=99 at 1400 px, Dark and Light, Accounts expanded and collapsed); search and filter tabs sit below it. Accounts column rows are `min-h-control` (32 px) with unread counts as plain `text-meta tabular-nums fg-tertiary` (no pill); rail buttons 48x44 with a primary left indicator on the active one. Account header names are not capitalised (user content). Measured controls in the touched files are all >= 28 px; only the window buttons (title bar) and the conversation's "Show fewer" (24 px, M6) are smaller.

## Outcomes & Retrospective

- Result (2026-10-11): all milestones M0-M6 and M8 done, M7 declined. The chat list, text hierarchy, tonal surfaces, header band, Compose button and reading width now follow the UpNote findings in `plans/upnote-ui/REFERENCE.md` ("What we adopted"). Final matrix (Dark and Light, 1400 / 900 / 700 px, Accounts expanded and collapsed, previews on and off, plus Contacts and Calendar) showed no open defects; `tools/contrast-check.mjs` passes both themes and runs in `make lint` and CI.
- Went well: tokens first (M1) made later surface and tier changes small; Playwright matrix kept screenshots out of context.
- Would change: M2's "tonal layers" premise was only partly supported by UpNote; M0 measurements caught that early. Contacts and Calendar panes still do not use the surface tokens (see ISSUES.md).


- M1 (2026-10-11): tokens added, no visual change; 6 arbitrary sizes replaced.

- M0 (2026-10-11): reference at `plans/upnote-ui/REFERENCE.md`, baseline of 36 shots in `~/.cache/mailpong-upnote-ui/before/`, tooling in `run-mailpong` (`pw.js`, `ui-matrix.js`). Baseline `tools/brand-check.sh` failure is logged as ISSUES T1.
