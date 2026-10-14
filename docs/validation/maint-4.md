# MAINT-4 validation — 2026-10-14

[MAINT-4](../../PLAN.md#maint-4) ([execution plan](../plans/maint-4.md)). Base `afe1684`, branch `feat/maint-4-ui-regression-ci`. How to run and extend the suite: [tests/ui/README.md](../../tests/ui/README.md).

## Evidence

- `make test-ui` — PASS, 41 passed, 0 failed, 0 skipped, about 76 s. Run three times on the final suite with identical results, and once more by the independent acceptance check. There are 20 audit configurations (light/dark × 1400/900/700 px × Accounts expanded/collapsed × previews on/off, where the layout has the toggle) and 21 regression tests: logic 5, chat presentation 4, inbox triage 6 and chat composition 6. `test-results/ui/results.json` summary: `{"passed":41,"failed":0,"skipped":0,"todo":0}`. `junit.xml` is written alongside.
- `go test ./internal/settings -run TestFrontendFakeDefaults` — PASS. It pins `settings-defaults.json` to the Go defaults.
- `make test` — PASS (exit 0). `make lint` — PASS (golangci-lint 0 issues, ESLint, brand check, contrast check).
- `cd frontend && npm run lint` — PASS. `npm run check` — 0 errors, 0 warnings over 1216 files.
- `.github/workflows/ci.yml`: `actionlint` reports no findings, and the file parses as YAML. Existing jobs and steps are unchanged; the `ui` job is added alongside them.
- Product source changed in one place: `applyTheme` (`theme.svelte.ts`) keeps the requested theme in backend-free mode (`window.__fakeBackend`) instead of forcing Dark, so the light audit configurations use the real theme path. Wails dev and production builds behave as before. The fake data fix is in `screenshot-fakes.js`, where Low priority chats now appear in the `all` section, as the backend returns them.
- No real data touched: the suite starts no Go process and opens no profile. Bindings other than reads reject with `blocked:` unless a test allows them, and allowed writes resolve inside the page. No files under `~/.config/mailven`, `~/.local/share/mailven` or `~/.local/share/mailven-dev-linux-amd64` changed during the work.

## Failure detection (reversible mutations)

Each mutation was applied to `frontend/src`, run, then reverted with `git checkout --`. `frontend/src` was clean afterwards.

| Mutation | Result |
| --- | --- |
| A: remove the chat Send button's `aria-label` (`ChatComposer.svelte`) | ui-quality failed 20/20: `no-name: button near[Send (Enter to send, Shift+Enter for a …] in main.pane-detail — button has no accessible name`, with configuration and screenshot per test |
| B: drop Cc from Reply All recipients (`chatFormat.ts`) | `logic.test.cjs:36` failed: "Reply All adds To and Cc, without me". A + B together gave 20 passed, 21 failed, exit 1 |
| C: uppercase the queued reply text (`chatComposerState.svelte.ts`) | chat-compose failed 3/6, including `chat-compose.test.cjs:31` (reply text) and `:65` ("Undo puts the text back in the box") |

With `GITHUB_ACTIONS=1` and a scratch `GITHUB_STEP_SUMMARY`, the runner emitted `::error file=…,line=…` annotations and a summary table showing each failure's first finding. Mutation A first showed two diagnostic gaps: an unnamed icon button was described only as `button in main.pane-detail`, and the summary row showed only the finding count. Both were fixed before the final runs. The audit now adds the nearest ancestor tooltip, and the summary shows the first finding.

## Existing Go coverage (not duplicated in the browser)

Classification, sender combine/split, snooze, pin, archive/undo, flags, quote extraction, reply assembly and the sender gate are covered by Go tests, for example `TestClassifyBulk`, `TestGetSenderConversation`, `TestMigrationV50_SenderChatCombined`, `TestDueSnoozesAndNextWake`, `TestListChatsFlags`, `TestChatStateCommandUndo`, `TestCanUndo`, `TestQueueFlagCoalesces`, `TestGenerateSnippet_QuotedLines`, `TestAssembleChatReply`, `TestBuildChatReply`, `TestChatReplyFrom` and `TestCheckSender` (`app/chat_reply_test.go`, `app/reply_identity_test.go`, `internal/message/chat_*_test.go`). The browser tests cover presentation and the frontend wiring to these bindings.

## Findings recorded

The audit and tests found [UI1](../../ISSUES.md#ui1) (light-theme contrast), [UI2](../../ISSUES.md#ui2) (closed narrow sidebar stays in the Tab order) and [UI3](../../ISSUES.md#ui3) (Archive listed twice in the chat row menu). They are open, not fixed, because they are UX changes outside this milestone. UI1 and UI2 are allowlisted in `tests/ui/ui-audit-allow.cjs` with their issue IDs. Delete those entries when the issues are fixed.

## Code review fixes

A `/code-review` of the milestone commit found ten defects in the test tooling. All ten were fixed before push:
- Vite is stopped on SIGINT/SIGTERM/SIGHUP, on exit and on runner exceptions. Checked: SIGINT exits 130 and leaves no Vite, and `stop()` returns after Vite is SIGKILLed (it used to hang).
- Focus baselines are recorded with nothing hovered or focused, and topped up after each Tab step. A control focused without a baseline is reported as `focus-unchecked`. That covered six chat-bubble controls per view that the audit had skipped (they have indicators).
- The light theme is applied through `applyTheme`, so the theme store agrees with the page.
- The UI2 allowlist entry matches only findings inside the narrow sidebar overlay, by landmark. The matrix fails on allowlist entries that match nothing.
- The call recorder tolerates arguments JSON cannot copy. The chat-compose waits poll the reply box, not the first textarea. AGENTS.md says a runner filter runs only the matching suites.

After the fixes, `make test-ui` passed (42 passed, 0 failed, 0 skipped: the 20 audit configurations, the allowlist staleness check and the 21 regression tests). `make lint`, `npm run lint` and `npm run check` passed too.

## Limits

- CI has not run on GitHub. The `ui` job was validated only statically (actionlint, YAML). Its Playwright install path (`npm install --prefix ../playwright`, `playwright install --with-deps chromium` on ubuntu-22.04) was not executed. Locally, Playwright 1.63.0 and Chromium came from the existing `~/.cache` install.
- The audit runs in headless Chromium against Vite, not in WebKitGTK inside Wails. Engine-specific rendering, native menus, window chrome and real backend timing are not covered. The Wails-session scripts (`ui-matrix.js`, `headless-shot.js`, `undo-send-check.js`) were not run, because they need a dev session. Their shared fakes changed only in the `all` section.
- Contrast is sampled against the rendered background under each text element. Images, gradients and text over translucent layers are approximated.
- Node v26.11.1 was used locally; CI uses Node 24. No real accounts, providers, credentials or mail were used.

## Acceptance review

Independent read-only `project-acceptance-checker` reran `make test-ui` (41/41), `go test ./internal/settings`, `npm run check` and actionlint. It confirmed that the `ui` job adds to CI without changing the existing jobs, that the frontend has no dependency or source changes, and that the AGENTS.md guidance is concise. It reported every criterion it could verify as PASS. It did not repeat the mutation demo, and ran the suite once. Its one finding was a `make test-ui ui-quality` reproduction in ISSUES.md, which the Makefile target does not support. That finding is fixed: the reproduction now reads `node tests/ui/run.mjs ui-quality`.
