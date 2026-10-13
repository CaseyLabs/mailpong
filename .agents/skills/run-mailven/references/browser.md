# Browser reference

Paths in commands are relative to the active checkout root; helper filenames refer to `.agents/skills/run-mailven/`. Read the core SKILL.md safeguards first.

Read state in the page with the available page-evaluation tool. The dev server serves the source
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
`sqlite3 -readonly ~/.local/share/mailven/mailven.db "select value from app_state where key='ui_state'"`.
