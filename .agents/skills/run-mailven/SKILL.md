---
name: run-mailven
description: Start, drive, screenshot, and stop Mailven for manual UI verification. Use when asked to run the app or check its running UI.
---

Mailven is a Wails v2 Go/Svelte app. The dev URL is `http://localhost:34115`.
Use headless Playwright for repeatable checks or an available browser connector for exploration. If a connector stalls, use Playwright.

## Paths and prerequisites

Resolve the active checkout with `git rev-parse --show-toplevel` and run commands below from it, including in worktrees. Skill discovery does not change cwd. Scripts resolve sibling resources from their own location. Resolve user output paths from the invocation cwd before changing directories; quote paths and arguments.

Check `command -v go node npm wails make curl setsid pgrep pkill python3`; add the actual Go bin directory to PATH if Wails is installed there. Install locked frontend dependencies with `cd frontend && npm ci` if absent and authorized. For Playwright, check `node -e "require('./.agents/skills/run-mailven/pw.js')"`. Explicit `PLAYWRIGHT_MODULE` and `PLAYWRIGHT_CHROMIUM` paths override detected installations. A missing dependency should be reported or installed within the task's authorization, never added to frontend dependencies for UI testing.

The native backend requires a working display even for headless browser checks. Probe the existing display (for example `xdpyinfo` when available); environment variable names alone do not establish availability. If unavailable, check `command -v Xvfb`, choose an unused display number, launch Xvfb through the host background facility, record its PID/session, and set DISPLAY for the dev process. Do not kill an existing display. Native window/dock checks cannot be claimed under Xvfb.

## Safeguards

Dev mode uses real accounts, syncs real mail, and persists UI settings. Capture `GetUIState()` before changing the active view, widths, or collapsed sidebars; restore that state before stopping. Wait at least 1.5 seconds for debounced saves before comparing persisted state. Avoid opening real messages (marks read). Never open a real composer (may save a draft); composer tests require the supplied fakes installed before app mount.

For fake-data checks block every non-read binding before interacting, including extension-prefixed verbs. Use `screenshot-fakes.js` and, for compose/send, `composer-fakes.js`. Do not send, delete, or modify real account data. Store setters also persist settings: use them only with non-read bindings blocked. Reload to remove fakes, and confirm original persisted state. Fake runtime event dispatch must stay inside its frame so it cannot trigger actions in the real parent app.

## Run and select detail

```bash
.agents/skills/run-mailven/dev.sh status
.agents/skills/run-mailven/dev.sh start
.agents/skills/run-mailven/dev.sh wait
```

The helper starts its own session. Use a host background handle for waits that exceed the tool's foreground limit. Record whether this task started the session; do not stop a pre-existing one. Finish Go edits first: backend hot reload replaces pages and invalidates fake frames.

Try `make test-ui` first: its backend-free mode (Vite plus fake bindings, no Wails session or display) covers the UI audit matrix and chat regressions deterministically; see `tests/ui/README.md`. Use a dev session for what it cannot reach.

Read only the reference needed for the check:

- [Playwright](references/playwright.md): repeatable screenshots, theme/layout matrix, fake server and headless setup.
- [Browser](references/browser.md): interactive navigation, store inspection, viewport and focus checks using the host's available browser APIs.
- [Screenshot](references/screenshot.md): retaking `docs/ss.png` with made-up data only.
- [Composer](references/composer.md): composer/undo-send checks with blocked real writes; also read Playwright for launcher setup.
- [Troubleshooting](references/troubleshooting.md): hidden tabs, OAuth failures, dev themes, bindings and shutdown problems, only when encountered.

Dismiss an OAuth-credentials notice if present. The left rail switches Mail, Contacts and Calendar; verify via `GetUIState().activeExtension`. Dev mode forces Dark; theme checks require the documented fake setup.

### Accounts column check

Every visual review covers Mail with Accounts expanded and collapsed. Use **Hide sidebar** / **Show sidebar**, or `layout.toggleActiveSidebar()` with safe bindings, and confirm `GetUIState().collapsedSidebars`. Restore the original state. At narrow widths where no toggle exists, check both states at a full-width viewport and report the narrow-layout limitation separately.

## Cleanup and report

Restore saved state and verify it, remove injected markup/fakes by reloading, then close tabs created for this task. Stop the fake server and Xvfb only by their recorded PID/session handles. If this task started the dev session, run:

```bash
.agents/skills/run-mailven/dev.sh stop
```

The helper reverts mode-only generated runtime changes and preserves content edits. Inspect the final diff. Report screenshots, checks and limitations, including any state restoration failure or missing UI coverage. Never leave task-started servers running.
