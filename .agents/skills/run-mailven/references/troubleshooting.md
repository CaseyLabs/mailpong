# Troubleshooting reference

Paths in commands are relative to the active checkout root; helper filenames refer to `.agents/skills/run-mailven/`. Read the core SKILL.md safeguards first.

## Gotchas

- **Dev mode always renders Dark** (`applyTheme` in
  `theme.svelte.ts` forces it under `import.meta.env.DEV`), so the theme picker
  has no visible effect in dev. Backend-free mode (`window.__fakeBackend`, used
  by `tests/ui/`) is exempt: it applies the theme `GetThemeMode` returns.
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
  the page body with it from the available page-evaluation tool) gets its own viewport, so the
  app switches layout mode there and `iframe.contentWindow.go` has the bindings;
  restore saved UI state afterwards. Ask the user to
  resize the window holding the browser tab only as a last resort (it may
  be minimized or behind other windows). To test a narrow layout
  without a resize, set a narrower width on the pane's element from
  the available page-evaluation tool. That checks how the pane's own content wraps, but it
  doesn't switch the app into its narrow layout mode.
- **Sweeping window widths.** Use the iframe probe in
  [Testing window widths](browser.md#testing-window-widths).
- **The tab is often `hidden`, and the first screenshot after a hot reload
  often times out.** In a hidden tab `requestAnimationFrame` never fires and
  timers are throttled, so never `await` an animation frame in
  the available page-evaluation tool (it hangs), and treat pauses seen only there as the
  background tab, not the app. Retry a timed-out screenshot after a few
  seconds, or measure with `getBoundingClientRect()` instead.
  A closed bits-ui dialog also stays mounted there (`data-state=closed`),
  because its exit animation waits on a frame. The dialog guard stays on and
  swallows keyboard shortcuts, so reload the page (and reinstall any binding
  fakes) after closing a dialog.
- **Faking the composer.** Use `composer-fakes.js` (see
  [Composer and undo-send checks](composer.md)) rather than
  hand-written fakes: a `SaveDraft` of the wrong shape makes the composer send
  at once with no undo, which looks like an app bug. Scope locators to
  `getByRole('region', { name: 'Email composer' })` (the chat reply box has
  its own **Send**) and pick the editor with `[contenteditable=true]:visible`.
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

- **Backend logs are quiet**: only fatal errors are logged by default. Start a task-owned session with `MAILVEN_DEBUG=1 .agents/skills/run-mailven/dev.sh start` for sync, IDLE and shutdown detail. Strip ANSI colors before searching the log.

- **`dev.sh stop` prints "still up"**: the dev processes outlived the kill. List
  them with `ps -o pid,pgid,sid,cmd -p $(pgrep -d, -f "wails dev|node_modules/.bin/vite|build/bin/[^ ]*-dev-linux")`
  and, only for the session this task started, set `session_id` to its verified ID and run `pkill -TERM -s "$session_id"`. Vite runs in its own process group, so kill by
  session ID, not process group.
- **`git status` shows `frontend/wailsjs/runtime/*` changes after a dev run**:
  `make dev` regenerates those files with different file modes. `dev.sh stop`
  reverts them when only modes changed and warns if their content changed.
