# MAINT-4 — Deterministic UI audit, frontend regression tests and CI

Roadmap entry: [PLAN.md MAINT-4](../../PLAN.md#maint-4). Evidence: [docs/validation/maint-4.md](../validation/maint-4.md).

## Goal

Replace repeatable manual and AI-reviewed UI checking with deterministic, headless checks that run locally and in CI. AI review stays for ambiguous failures, subjective visuals and reasoning.

## Scope

- Backend-free UI test mode: plain Vite plus fake Wails bindings generated from `frontend/wailsjs/go/app/*.d.ts`. There is no Go process, profile, account or display, and nothing persists.
- UI quality audit across light/dark × 1400/900/700 px × Accounts expanded/collapsed (where the layout has the toggle) × previews on/off. It checks overflow, clipping, overlapping or covered controls, hidden or inaccessible focusables, accessible names, keyboard focus, responsive expectations and runtime text contrast.
- Frontend regression tests for chat presentation, inbox triage and chat composition, reusing the existing fake data.
- `make test-ui`, and a CI job with JUnit, JSON, GitHub annotations, step summary and failure screenshots.
- Workflow guidance in AGENTS.md and the run-mailven skill.

Out of scope: product behavior changes (bugs found are recorded, and fixed only when necessary and safe), new frontend dependencies, Wails/Svelte/Go upgrades, live accounts, and other roadmap milestones (G4 and D2 remain deferred).

## Authorization

Explicitly authorized by the user's 2026-10-14 request "Mailven: Automated UI Quality, Regression Testing, and CI Improvements", which covers this milestone only. Local commits on `feat/maint-4-ui-regression-ci` are allowed; merging and pushing need explicit approval, which overrides AGENTS.md "Landing a Branch".

## Context

- Every existing UI check (`ui-matrix.js`, `headless-shot.js`, `tools/*-check.cjs`) needs a live Wails dev session on :34115, so none could run in CI.
- The detached-composer fakes (`composer-fakes.js`) already proved that the frontend runs without a Wails backend.
- Go tests already cover classification, sender combine/split, snooze, pin, archive/undo, flags, quote extraction, reply assembly and the sender gate (see the validation doc). Frontend tests cover presentation and interaction, not backend logic.
- Runner: Node's built-in `node:test` (`run()` + `.compose()` reporters; `test:fail` carries `file`/`line`/`details.error`). Browser: Playwright 1.63.0 installed outside `frontend/package.json`, as the skill requires.

## Progress

- [x] 1. Prototype the backend-free page; settings defaults fixture pinned by `internal/settings` `TestFrontendFakeDefaults`.
- [x] 2. Harness and runner (`tests/ui/`): Vite lifecycle, page helper, failure screenshots, JSON/JUnit/annotations/summary.
- [x] 3. UI audit module and matrix test, with a documented allowlist.
- [x] 4. Regression tests: pure logic, chat presentation, inbox triage, chat composition.
- [x] 5. `make test-ui`, CI job, `.gitignore`.
- [x] 6. Docs: AGENTS.md, run-mailven skill, CONTRIBUTING.
- [x] 7. Verification, including the failure-detection demo; acceptance review; simplification; local commit.

## Acceptance → commands

| Criterion | Command / evidence |
| --- | --- |
| Repeatable PASS/FAIL UI audit over the full matrix; failures name element, configuration and reason | `make test-ui` (`tests/ui/ui-quality.test.cjs`), run twice |
| No false failures on expected responsive behavior | the matrix passes at 700/900/1400 with documented allowlist entries only |
| Synthetic data only; nothing persisted | backend-free mode has no backend; non-read bindings reject with `blocked:` |
| Critical chat behaviors covered | `tests/ui/chat-*.test.cjs`, `inbox-triage.test.cjs`, `logic.test.cjs` |
| Existing checks still pass | `make test`, `make lint`, `cd frontend && npm run check` |
| CI integration with diagnostics | `.github/workflows/ci.yml` `ui` job; YAML parses; local run produces `test-results/ui/{junit.xml,results.json}` |
| Failure detection | reversible mutation demo recorded in the validation doc |

## Surprises & Discoveries

- Settings getters return zero values from generated defaults, but Go defaults differ (for example, `GetChatShowLowGroup` is true). Without the fixture, the chat list asks for the `priority` section and renders empty.
- Dev builds force the Dark theme (`theme.svelte.ts` `applyTheme`), so the harness applies `data-theme` itself after mount.
- Transitions made contrast and focus visibility vary between runs (colours and hover-reveal opacity sampled mid-fade). The harness disables transitions and the audit waits for running animations before sampling and after each Tab.
- Responsive overlays (medium-layout viewer, narrow sidebar and scrim) cover the list on purpose; `covered-control` ignores controls under a large positioned layer stacked above them. Narrow layout opens on the selected chat with Back, and the list toggle there is "Toggle sidebar".
- The audit found two product defects, recorded as [UI1](../../ISSUES.md#ui1) (light-theme contrast) and [UI2](../../ISSUES.md#ui2) (closed narrow sidebar in Tab order) and allowlisted until fixed.
- The fake data left Low priority chats out of the `all` section; the backend includes them and the list groups them by `isLowPriority`. Fixed in `screenshot-fakes.js` (the README screenshot is unchanged: its Low group is collapsed).
- The chat row menu lists Archive twice (chat item and shared message item), recorded as [UI3](../../ISSUES.md#ui3); tests pick the chat-level item.
- Snooze presets depend on the browser's time zone; the harness pins `timezoneId: 'UTC'`.

## Decision Log

- Backend-free mode instead of a Wails session in CI: it needs no GTK, display, Go build or profile, and it is safe by construction. The Wails-session mode (`fake-page.js`) remains for checks against a real build.
- `node:test` instead of a new framework: no new dependency, native JUnit reporter, file/line in events.
- Diagnostics improved after the mutation demo: unnamed icon buttons are described by the nearest ancestor tooltip, and the CI step summary shows each failure's first finding.
- UI1 and UI2 are recorded and allowlisted, not fixed: both are UX changes outside the milestone's testing scope.
- Tests live in `tests/ui/` (repo tooling), not `frontend/` (no frontend dependency changes).

## Outcomes & Retrospective

Done. `make test-ui` runs 20 audit configurations and 21 regression tests headlessly in about 76 s with no backend; three runs gave identical 41/41 passes, and three reversible mutations were caught with actionable messages. The independent acceptance review passed every criterion it could verify. It found one doc error (an unsupported `make test-ui <filter>` form in ISSUES.md), which is fixed. Not proven: a real GitHub Actions run, including its Playwright install, and WebKitGTK rendering. Follow-ups are UI1–UI3.

## Checkpoint

- Completed on branch `feat/maint-4-ui-regression-ci` (from `main` at `afe1684`) as one local commit; not pushed or merged.
- Checks: see the [validation doc](../validation/maint-4.md). `make test-ui` 41/41 (three runs, plus one by the acceptance checker), `make test`, `make lint`, frontend lint/check, actionlint, `go test ./internal/settings` all pass.
- Unrun: GitHub CI, the CI Playwright install path, Wails-session scripts.
