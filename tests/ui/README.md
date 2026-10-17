# UI audit and frontend regression tests

`make test-ui` (or `node tests/ui/run.mjs [filter...]`, which runs only the suites whose file names contain a filter) runs a deterministic UI audit and frontend regression
tests headlessly, without Wails, Go or a display. Vite serves the real frontend; `window.go` is a fake built
from the generated binding declarations (`.agents/skills/run-mailven/fake-backend.js`), filled with the
synthetic data in `screenshot-fakes.js`. Read bindings return that data or the settings defaults fixture
(`settings-defaults.json`, pinned to the Go defaults by `internal/settings` `TestFrontendFakeDefaults`).
Every other binding rejects with `blocked: <name>` unless a test allows it, so no test can reach real mail,
accounts or settings, and nothing persists between tests.

## Prerequisites

Node.js 24, `cd frontend && npm ci`, and Playwright 1.63.0 with Chromium installed outside
`frontend/package.json`:

```bash
npm install --prefix ~/.cache/mailven-upnote-ui --no-save playwright@1.63.0
~/.cache/mailven-upnote-ui/node_modules/.bin/playwright install chromium
```

`PLAYWRIGHT_MODULE` and `PLAYWRIGHT_CHROMIUM` override the detected paths (`pw.js`).

## Suites

| File | Covers |
| --- | --- |
| `ui-quality.test.cjs` | Audit matrix: light/dark × 1400/900/700 px × Accounts expanded/collapsed × previews on/off, plus flat-mode and reader-view variants |
| `sidebar-focus.test.cjs` | Mail and shared extension sidebars reject focus while closed at narrow widths; opening, closing and resizing preserve interaction |
| `viewer-focus.test.cjs` | Mail and shared detail-pane viewers reject focus while closed at medium and narrow widths; opening, Back, thread focus mode and resizing preserve interaction |
| `logic.test.cjs` | Pure chat logic (`chatFormat.ts`, `chatSnooze.ts`): reply recipients and default mode, reply identity, bubble grouping, subject headings, list groups, snooze presets |
| `chat-reader.test.cjs` | Rich mail in the reader view: sanitized blocks in a bubble or flat, blocked remote images and Load images, links through the app, Show more/less, Show original, the card setting (fake in `reader-fake.cjs`) |
| `chat-secure.test.cjs` | Signed and decrypted mail as chat bubbles with their badges from faked S/MIME and PGP view results, Show original, Original layout, failed decryption |
| `chat-presentation.test.cjs` | Incoming/outgoing bubbles and sender runs, quoted text and signature, sender chats with several subjects, combine/split with Undo |
| `inbox-triage.test.cjs` | Pin, snooze, archive, read/unread, the automatic Low priority group, sender priority overrides, Undo |
| `chat-compose.test.cjs` | Reply vs Reply All, draft autosave and restore, undo-send, reply identity, a refused reply (ISSUES.md R1) |
| `mail-api-migration.test.cjs` | Rollback pending state, confirmation cancellation, protocol route display, and both sync-request report outcomes |
| `google-setup-check.test.cjs` | Settings → Identity → Check Google setup: disabled, missing-scope, not-connected, error and OK states, Enable link, keyboard use, nothing runs until asked |

## Results

Each failure prints the test, `file:line`, the configuration (`dark 700px accounts=collapsed previews=on`) and
a concise message; audit findings read `rule: element — detail`, one line per distinct finding. Output goes to `test-results/ui/`:

- `results.json`: summary and one record per test (status, file, line, duration, error).
- `junit.xml`: native `node:test` JUnit report.
- `<test name>.png`: screenshot of each failure, for investigation only; the message is the diagnostic.

On GitHub Actions the runner adds `::error` annotations and a step summary, and the `ui` job in
`.github/workflows/ci.yml` uploads `test-results/ui` as `ui-test-results` when it fails.

## Audit rules and the allowlist

The rules are listed at the top of `ui-audit.cjs`: overflow, clipped text and controls, covered controls,
focusable controls inside hidden content, missing accessible names, contrast, and Tab-order checks (focus lost,
trapped, invisible, without an indicator, or unchecked for lack of an unfocused baseline). `ui-audit-allow.cjs`
lists the findings the audit tolerates. Each entry names a rule, an element pattern, optionally the landmark
around a focused element, the configurations it applies to, and a reason. A product defect stays allowlisted
only while its ISSUES.md entry is open. Delete the entry when the defect is fixed, so the audit guards the
fix: the matrix fails on an entry that no configuration matched. Intentional responsive behaviour belongs in the audit's logic or an allowlist entry
with its reason, never in a broad pattern.

## Writing tests

Use `ui.run(options, async (page, config) => { ... })` from `harness.cjs`. The options are:

- `width`, `height`, `theme`, `accounts`, `previews`
- `allow`: binding names that resolve instead of rejecting
- `beforeMount`: browser source that runs after the fakes and before the app, for example to wrap
  `window.go.app.App.GetConversation`
- `ready`: a selector to wait for

`ui.calls(page, name)` returns the recorded binding calls with JSON-copied arguments. Prefer importing pure
modules in the page (see `logic.test.cjs`) over driving the UI. Use the UI for wiring that logic tests cannot
reach. Use roles and accessible names for selectors, and keep test data synthetic.

- When intercepting the frontend entry point for fakes, match `src/main.ts` with or without Wails' hot-reload
  query string.
- In guarded Wails setter fakes, copy reactive arguments with `JSON.parse(JSON.stringify(value))`;
  `structuredClone` cannot copy Svelte state proxies.
