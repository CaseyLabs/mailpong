---
name: run-mailpong
description: Run, start, launch, and drive the MailPong desktop email client for manual UI verification — start the dev server, open it in Chrome, click through Mail/Contacts/Calendar, read UI or store state, take screenshots, then stop it cleanly. Use when asked to run MailPong, verify a UI change in the running app, or screenshot it.
---

MailPong is a Wails v2 app (Go backend, Svelte 5 frontend). `make dev` serves the
frontend with the Go bindings at `http://localhost:34115`. An agent drives that
URL in Chrome with the Claude-in-Chrome tools. `.claude/skills/run-mailpong/dev.sh`
starts, waits for and stops the dev session. All paths are relative to the repo
root.

## Prerequisites

The toolchain is installed. Run `cd frontend && npm ci` first if
`frontend/node_modules` is missing.

## Run (agent path)

```bash
.claude/skills/run-mailpong/dev.sh start   # make dev in its own session; prints the log path
.claude/skills/run-mailpong/dev.sh wait    # polls http://localhost:34115 for HTTP 200 (first build takes ~1 min); fails early if make dev exits
```

The Go backend logs only fatal errors unless debug mode is on. To read sync,
IDLE or shutdown lines in the log, start with `MAILPONG_DEBUG=1 .claude/skills/run-mailpong/dev.sh start`.
The log has ANSI colors, so strip them (`sed 's/\x1b\[[0-9;]*m//g'`) before grepping.

`make dev` also opens a native app window on the user's desktop. Ignore it and
drive the browser tab:

1. `tabs_context_mcp` with `createIfEmpty: true`, then `navigate` that tab to
   `http://localhost:34115`.
2. If an "OAuth credentials missing" notice is showing, click its **OK** button.
   It only shows sometimes, and it can come back once after you dismiss it.
3. Interact with `computer` clicks and keys, and check results with
   `javascript_tool`. Pass `save_to_disk: true` on a `computer` screenshot to
   keep the image file.

Read state in the page with `javascript_tool`. The dev server serves the source
modules, so importing a store returns the same instance the app is using.
After a hot reload the app may hold the module under a `?t=<timestamp>` URL,
and a plain import then gives a separate copy. Import the exact URL listed in
`performance.getEntriesByType('resource')` instead:

```js
const ui = await window.go.app.App.GetUIState()
// Resolve the URL the app actually loaded (it may carry ?t=<timestamp>).
const url = (name) =>
  performance.getEntriesByType('resource').map((e) => new URL(e.name).pathname + new URL(e.name).search)
    .filter((u) => u.includes(name)).pop() ?? `/src/lib/stores/${name}`
const kb = await import(url('keyboard.svelte.ts'))
const layout = await import(url('layout.svelte.ts'))
;({ active: ui.activeExtension, collapsed: ui.collapsedSidebars, pane: kb.getFocusedPane(), mode: layout.getLayoutMode(), width: innerWidth })
```

The left rail switches views (Mail, Contacts, Calendar from the top). After a
click, confirm the switch with `(await window.go.app.App.GetUIState()).activeExtension`.
Wait about 1.5 s before reading saved state, because UI-state saves are debounced
by 1 s.

### Accounts column check

For any visual review, check the change with the Accounts column (the Mail
sidebar) both expanded and collapsed. Toggle it with the **Hide sidebar** /
**Show sidebar** button, or call `layout.toggleActiveSidebar()`, and confirm
with `ui.collapsedSidebars`. Restore the user's original state afterwards.

### Stop

When finished, put back anything you changed (see Gotchas), close the tab, then:

```bash
.claude/skills/run-mailpong/dev.sh stop    # kills wails, vite and the app, then reverts mode-only changes in frontend/wailsjs/runtime
```

## Gotchas

- **Dev mode always renders Dark** (`applyTheme` in
  `theme.svelte.ts` forces it under `import.meta.env.DEV`), so the theme picker
  has no visible effect in dev.
- **Dev mode uses the user's real accounts and saved UI state.** It syncs real
  mail. Collapsed sidebars, the active view and widths are saved for real. Note
  the starting `GetUIState()` and restore it before stopping. The calendar
  view mode is not saved (`calendarView.svelte.ts` starts in month view on
  every load). Never open the
  composer, which could save a draft to the user's account, and avoid opening
  real messages, which marks them read.
- **Sync fails with `unauthorized_client`.** The account's refresh token was
  issued to a different OAuth client than the one in `.env`/`.env.local`.
  Only the owner can fix it, by clicking **Re-authorize** on the account in
  Settings → Accounts (see `CONTRIBUTING.md`). Check first with
  `await window.go.app.App.TestOAuthConnection(id)`.
- **Fake data instead of real data.** Bindings are looked up on
  `window.go.app.App` at call time, so a page script can replace one (for
  example a calendar or contacts list call) to return made-up records. While
  doing this, block every binding that isn't a read, so a stray click can't
  change real data. Extension bindings are named `Calendar_<Verb>` and
  `Contacts_<Verb>`, so test the verb after the prefix:
  `for (const k of Object.keys(window.go.app.App)) if (!/^(Get|List|Search|Is|Has|Find)/.test(k.replace(/^[A-Za-z]+_/, ''))) window.go.app.App[k] = () => Promise.reject(new Error('blocked: ' + k))`.
  Call the store's reload (or switch views) to pick up the fake data,
  and reload the page to undo it. To check a mail layout without opening a
  message, add a copy of the markup to the page and measure it.
- **Layout modes come from `matchMedia` on the viewport**: narrow below 768px,
  medium up to 1024px, otherwise full. The owner allows resizing the Chrome
  window yourself. `resize_window` has had no effect under the user's window
  manager, so try it, check `innerWidth`, and if it didn't change, try another
  way. A same-origin `<iframe>` of the dev URL at the width you need (replace
  the page body with it from `javascript_tool`) gets its own viewport, so the
  app switches layout mode there and `iframe.contentWindow.go` has the bindings;
  restore saved UI state afterwards. Ask the user to
  resize the window holding Claude's tab group only as a last resort (it may
  be minimized or behind other windows). To test a narrow layout
  without a resize, set a narrower width on the pane's element from
  `javascript_tool`. That checks how the pane's own content wraps, but it
  doesn't switch the app into its narrow layout mode.
- **Sweeping window widths.** Use the iframe probe in
  [Testing window widths](#testing-window-widths).
- **The tab is often `hidden`, and the first screenshot after a hot reload
  often times out.** In a hidden tab `requestAnimationFrame` never fires and
  timers are throttled, so never `await` an animation frame in
  `javascript_tool` (it hangs), and treat pauses seen only there as the
  background tab, not the app. Retry a timed-out screenshot after a few
  seconds, or measure with `getBoundingClientRect()` instead.
  A closed bits-ui dialog also stays mounted there (`data-state=closed`),
  because its exit animation waits on a frame. The dialog guard stays on and
  swallows keyboard shortcuts, so reload the page (and reinstall any binding
  fakes) after closing a dialog.
- **Stores can be driven from the page.** `await import('/src/lib/stores/settings.svelte.ts')`
  returns the app's own module instance, so its setters (for example
  `setChatAutoAdvance`) change the running app. They also save the setting, so
  call them only while non-read bindings are blocked.
- **Clicks use the screenshot's coordinate frame**, which is reported with every
  screenshot (e.g. 1568×778 for a 2560px-wide viewport), not CSS pixels. In that
  frame, the rail buttons are at about x=15, y=42 / 70 / 100.
- **The key tool can't send `` Ctrl+` ``.** Use the rail buttons. `ctrl+Tab`,
  `ctrl+shift+b`, `alt+h` and similar combinations work.
- **Kit panes (Contacts, Calendar) take DOM focus, mail panes don't.** Read
  `kb.getFocusedPane()` instead of `document.activeElement` to check pane focus.

## Testing window widths

Note `GetUIState()` first, then replace the page with an iframe and define a probe:

```js
document.body.innerHTML = '<iframe id=f src="/" style="width:1024px;height:700px;border:0"></iframe>'
const f = document.getElementById('f')
await new Promise((r) => (f.onload = r)); await new Promise((r) => setTimeout(r, 6000))
window.probe = async () => {
  const w = f.contentWindow
  const u = w.performance.getEntriesByType('resource').map((e) => new URL(e.name))
    .map((x) => x.pathname + x.search).filter((x) => x.includes('layout.svelte.ts')).pop()
  const L = await w.eval(`import(${JSON.stringify(u)})`)
  const r = w.document.querySelector('main')?.getBoundingClientRect()
  return `${f.style.width} mode=${L.getLayoutMode()} view=${L.getResponsiveView()} main=${Math.round(r?.left)}+${Math.round(r?.width)}`
}
```

Then set `f.style.width`, wait about a second and call `await probe()` for each
width, going both narrow-to-wide and wide-to-narrow. In a `hidden` tab the
iframe gets no `matchMedia` change events, so the layout mode never follows the
width: take a screenshot first to bring the tab forward, and confirm with
`f.contentWindow.matchMedia('(max-width: 1024px)').matches`. Kit views (Contacts,
Calendar) render their own detail pane instead of `main`.

Widths worth testing: 767/768 and 1024/1025 (the mode breakpoints); where the
list (280–600 px, `LIST_MIN`/`LIST_MAX` in `uiState.svelte.ts`) or sidebar
(180–400 px) hits a limit, which depends on the saved widths; and 360, the
native window's `MinWidth` in `main.go` (500 for the composer). The desktop
window has 2 device pixels per CSS pixel, so in native screenshots or
recordings the breakpoints sit at 1536 and 2050 device pixels. Glitches that
only show while the native window resizes (paint lag, the window background
showing through) don't reproduce in Chrome; check `main.go` window options for
those.

If the page loses its copy of the starting state, read the saved state directly:
`sqlite3 -readonly ~/.local/share/mailpong/mailpong.db "select value from app_state where key='ui_state'"`.

## Playwright checks (local and cloud)

Headless Playwright writes screenshots from one Bash call, so it needs neither
Claude-in-Chrome nor a visible window. Detect the environment once:

```bash
if [ "$CLAUDE_CODE_REMOTE" = true ] || [ -d /opt/node-tools/node_modules/playwright ]; then echo cloud; else echo local; fi
```

| | Local (desktop) | Cloud |
|---|---|---|
| Playwright module | `~/.cache/mailpong-upnote-ui/node_modules/playwright` (1.63.0) | `/opt/node-tools/node_modules/playwright` |
| Chromium | Playwright's own, no `executablePath` | `/opt/pw-browsers/chromium` |
| Display for `dev.sh start` | the real desktop | `Xvfb :99 -screen 0 1600x1000x24`, `DISPLAY=:99`, `PATH=$PATH:$HOME/go/bin` |

`pw.js` does this selection and exports `launch()`. `$PLAYWRIGHT_MODULE` and
`$PLAYWRIGHT_CHROMIUM` override the paths. If the module is missing it exits
with the reinstall command
(`npm install --prefix ~/.cache/mailpong-upnote-ui --no-save playwright@1.63.0`;
never add Playwright to `frontend/package.json`).

```bash
# cloud only: (Xvfb :99 -screen 0 1600x1000x24 &)  and  export PATH=$PATH:$HOME/go/bin DISPLAY=:99
.claude/skills/run-mailpong/dev.sh start          # run_in_background; then dev.sh wait
python3 .claude/skills/run-mailpong/serve-fakes.py   # run_in_background
node .claude/skills/run-mailpong/headless-shot.js "$SCRATCH"          # ss.png plus vc-<theme>-<state>.png
node .claude/skills/run-mailpong/ui-matrix.js "$SCRATCH" <prefix>     # the full matrix
```

`headless-shot.js` dismisses the first-run dialogs, enables Contacts and
Calendar (their rail icons only show once enabled), runs the fakes from
"Retaking the README screenshot", and writes the README shot plus the
dark/light x Accounts-column expanded/collapsed crops. Copy `ss.png` to
`docs/ss.png` as is.

`ui-matrix.js <outdir> <prefix>` uses the same page setup and writes 36 PNGs:
`<prefix>-<dark|light>-<1400|900|700>-<expanded|collapsed>-<previews|nopreviews>.png`
(at 700 px the app is in overlay layout with no Accounts toggle, so only the
`expanded` file exists), plus `<prefix>-<theme>-<state>-<previews|nopreviews>-row-<hover|focus>.png`
crops at 1400 px. Compare a before and an after set without loading images:

```bash
for f in "$SCRATCH"/before/*.png; do n=$(basename "$f"); echo "$(compare -metric AE "$f" "$SCRATCH/after/${n/before/after}" null: 2>&1) $n"; done | sort -rn | head
```

Back-to-back runs on an unchanged tree differ by up to about 200 pixels per
image (relative times in the fake data, and a 9x9 px spot at the window
edge), so treat an AE under about 300 as unchanged. Before and after must
come from the same environment. The local `before` set lives in
`~/.cache/mailpong-upnote-ui/before/`; the cloud container is fresh each
session, so rebuild it from the clean checkout first.

- It sets `locale: 'en-GB'` so times are 24-hour, as in the old screenshot.
- Dev builds force Dark, so the scripts set `data-theme="light"` by hand.
- ImageMagick 7: `magick` and `compare` exist; `convert` is deprecated.
- `pkill -f serve-fakes.py` matches the shell running it and kills it (exit 144). Use
  `pkill -f 'serve-fake[s]'`, and run `dev.sh stop` (and in the cloud `pkill Xvfb`) as well.
- The native window and dock icon aren't visible under Xvfb; report that.
- zsh treats an unquoted `==` as a command: quote it in `echo`.

## Retaking the README screenshot

`docs/ss.png` (1040×660, Nord Dark, mail sidebar collapsed) must show only made-up
mail. `screenshot-fakes.js` fakes the read bindings: two accounts, a chat list
and a short group chat. It also blocks every write binding and saves nothing.
Edit the data there, not in the page.

1. Start the dev server and `serve-fakes.py` (both with `run_in_background`).
   `tabs_context_mcp` with `createIfEmpty`, then navigate to `http://localhost:34115`.
   The viewport must be at least 660 px tall; resize the Chrome window if it isn't.
2. Run this in `javascript_tool`. The page becomes a 1040×660 iframe, and the
   fakes go in before the app mounts, so its first load reads them:

   ```js
   await new Promise((r) => setTimeout(r, 2500))
   window.__ui0 = JSON.stringify(await window.go.app.App.GetUIState())
   ;(0, eval)(await (await fetch('http://127.0.0.1:8765/screenshot-fakes.js')).text())
   document.body.innerHTML = ''; document.body.style.margin = '0'
   const f = document.createElement('iframe')
   f.style.cssText = 'width:1040px;height:660px;border:0;position:fixed;left:0;top:0'
   document.body.appendChild(f); f.src = location.href
   const poll = setInterval(() => { try { const w = f.contentWindow
     if (w.location.href !== 'about:blank' && w.go?.app?.App && !w.__faked) { w.__faked = true; __installFakes(w); clearInterval(poll) } } catch {} }, 1)
   await new Promise((r) => setTimeout(r, 8000))
   ```
3. Hover outside the iframe (for example at x=1300) so no cursor shows, then
   take a `computer` `zoom` of region `[0, 0, 1040, 660]` with `save_to_disk`.
   It saves a PNG at about 1.3×. Scale it down:
   `magick <saved.png> -alpha off -filter Lanczos -resize 1040x660\! -strip PNG24:docs/ss.png`.
4. Check that `JSON.stringify(await window.go.app.App.GetUIState())` still equals
   `__ui0`. Then close the tab and stop both servers.

The fake data has to match what the components read, or the image looks wrong:

- Every message needs `chat: { text, hasQuoted: false, isRich: false }`.
  Without it the bubbles render in full mode and look like emails.
- `GetIdentities` must return my address, or "me" counts as a recipient.
- Keep the width at 1025 px or more. Below that the app uses its overlay
  layout and shows only one pane.

## Troubleshooting

- **`dev.sh stop` prints "still up"**: the dev processes outlived the kill. List
  them with `ps -o pid,pgid,sid,cmd -p $(pgrep -d, -f "wails dev|node_modules/.bin/vite|build/bin/[^ ]*-dev-linux")`
  and run `pkill -TERM -s <SID>`. Vite runs in its own process group, so kill by
  session ID, not process group.
- **`git status` shows `frontend/wailsjs/runtime/*` changes after a dev run**:
  `make dev` regenerates those files with different file modes. `dev.sh stop`
  reverts them when only modes changed and warns if their content changed.
