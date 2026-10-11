---
name: run-aerion
description: Run, start, launch, and drive the Aerion desktop email client for manual UI verification — start the dev server, open it in Chrome, click through Mail/Contacts/Calendar, read UI or store state, take screenshots, then stop it cleanly. Use when asked to run Aerion, verify a UI change in the running app, or screenshot it.
---

Aerion is a Wails v2 app (Go backend, Svelte 5 frontend). `make dev` serves the
frontend with the Go bindings at `http://localhost:34115`. An agent drives that
URL in Chrome with the Claude-in-Chrome tools. `.claude/skills/run-aerion/dev.sh`
starts, waits for and stops the dev session. All paths are relative to the repo
root.

## Prerequisites

The toolchain is installed. Run `cd frontend && npm ci` first if
`frontend/node_modules` is missing.

## Run (agent path)

```bash
.claude/skills/run-aerion/dev.sh start   # make dev in its own session; prints the log path
.claude/skills/run-aerion/dev.sh wait    # polls http://localhost:34115 for HTTP 200 (first build takes ~1 min); fails early if make dev exits
```

The Go backend logs only fatal errors unless debug mode is on. To read sync,
IDLE or shutdown lines in the log, start with `AERION_DEBUG=1 .claude/skills/run-aerion/dev.sh start`.
The log has ANSI colors, so strip them (`sed 's/\x1b\[[0-9;]*m//g'`) before grepping.

`make dev` also opens a native Aerion window on the user's desktop. Ignore it and
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

For any visual review, check the change with the Accounts column (the Mail
sidebar) both expanded and collapsed. Toggle it with the **Hide sidebar** /
**Show sidebar** button, or call `layout.toggleActiveSidebar()`, and confirm
with `ui.collapsedSidebars`. Restore the user's original state afterwards.

When finished, put back anything you changed (see Gotchas), close the tab, then:

```bash
.claude/skills/run-aerion/dev.sh stop    # kills wails, vite and the app, then reverts mode-only changes in frontend/wailsjs/runtime
```

## Gotchas

- **Dev mode always renders Nord (Dark)** (`applyTheme` in
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

## Troubleshooting

- **`dev.sh stop` prints "still up"**: the dev processes outlived the kill. List
  them with `ps -o pid,pgid,sid,cmd -p $(pgrep -d, -f "wails dev|node_modules/.bin/vite|aerion-dev-linux")`
  and run `pkill -TERM -s <SID>`. Vite runs in its own process group, so kill by
  session ID, not process group.
- **`git status` shows `frontend/wailsjs/runtime/*` changes after a dev run**:
  `make dev` regenerates those files with different file modes. `dev.sh stop`
  reverts them when only modes changed and warns if their content changed.
