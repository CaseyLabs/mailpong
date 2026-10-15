# Repository Guidelines

## Working Policy

- Runtime constraints and explicit user instructions take precedence. Project skills must not grant tools or permissions (no `allowed-tools`). Respect the active host's plan mode; if execution is restricted, record the design and wait for execution authorization.
- Continue authorized work without asking between steps; stop at the requested scope (one milestone for project-next-milestone). Batch material questions using the host's question tool when available. Ask before destructive actions unless that exact action is authorized; preserve a local rollback.
- Keep output bounded and inspect targeted log excerpts. Reuse required guidance already read in the current session while it remains unchanged; load it again in a new session. Run servers/watchers in the host's background facility and stop those you started before finishing.
- Use subagents for independent, well-scoped work when isolation or specialization helps; this is an explicit request to delegate suitable work. Give each agent a distinct scope. Keep architecture, integration, and security decisions in the main session. Verify consequential agent claims before acting; change approach after failed or partial delegation.
- In Claude Code and Codex, use the native `project-worker` for general delegated project work instead of a generic worker. Use `project-acceptance-checker` for milestone acceptance. For mechanical file retrieval, listings, or exact text extraction in Codex, use `file_reader` or deterministic tools directly; do not use it for investigation or acceptance review. Specialist tools/agents (including bounded exploration and skill-owned reviews) keep their own routing. If a named agent is unavailable, disclose the fallback; do not silently claim its model/effort settings. Definitions live in `.claude/agents/` and `.codex/agents/`.
- If Codex reports `unknown agent_type`, see CONTRIBUTING.md (checkout trust) before changing configuration.
- Only one agent may run builds or tests in a checkout at a time. Independent read-only work and light static checks may run concurrently; use agent teams only when requested.
- Use the configured Git identity. Never add generated-by attribution, AI co-authors, or AI/session trailers to commits or PRs, or override identity for an assistant. The `.githooks/commit-msg` hook rejects such lines once enabled (see CONTRIBUTING.md); never bypass it with `--no-verify`.
- This file is shared policy. Links are pointers, not automatic imports: read references when their stated trigger applies. Host-specific configuration (permissions, models, agent definitions, tool schemas) belongs in native host adapters, not shared policy. The documented Claude-only next-milestone `model` frontmatter is the sole shared-skill exception.
- Before adding or changing guidance in this file or a skill, read [AI project guidance](CONTRIBUTING.md#ai-project-guidance): keep always-loaded text to rules that must always hold, and move procedures to the doc that owns them with a one-line trigger.

## Project Identity

- Mailven is an independent desktop email client (Go, Wails v2, Svelte 5, TypeScript) derived from Aerion.
- Branding: replace "Aerion" only where it names this app, never by blind search-and-replace. Keep it in attribution, history, `LICENSE`, `archive/`, and statements of origin ("derived from Aerion").
- Persisted internal identifiers keep their Aerion values: the `"aerion"` contact source, the `"aerion-shipped"`/`"aerion-mail"` OAuth choice IDs, the `'aerion'` title-bar setting, `minAerionVersion`, and the `aerion:` KDF prefix. Each carries a `Legacy identifier from Aerion; persisted, do not rename.` comment. Renaming them breaks stored data.

## Layout Notes

- `frontend/wailsjs/` holds generated Wails bindings; before running `make generate`, follow [Regenerating Wails bindings](CONTRIBUTING.md#regenerating-wails-bindings).
- Before committing changes under `extensions/**`, `internal/core/**`, `internal/extensions/**`, `internal/kit/**`, `app/coreimpl*.go`, `app/extension_*.go`, or `frontend/src/lib/components/kit/**`, read `docs/EXT_RULES.md` and check the design against it; redesign violations. Read it only for relevant extension/core/kit work.
- `docs/EXTENSIONS.md` (about 170 KB), `docs/history/AERION_CASA_TIER2.md`, and `docs/LANGUAGE.md` are long: search or read by section, never whole. For the full extension reference, read the Contents of `docs/EXTENSIONS.md`, then only needed sections.

## Commands

Toolchain: Go 1.26, Node.js 24 (matches CI), Wails v2 CLI. Linux needs GTK 3 and WebKitGTK 4.1 development libraries.

- Workflow helpers: `python3 tools/project-context.py <milestone-id>` for scoped context, `python3 tools/project-check.py --base <base>` to plan checks (`--run` executes with logs/evidence), `python3 tools/check-doc-links.py --changed` for local/site links, `tools/project-land.sh <branch>` for guarded landing preview, and `python3 tools/project-run.py` for unattended milestone runs. Details and execution boundaries: [CONTRIBUTING.md](CONTRIBUTING.md#workflow-helpers).
- `cd frontend && npm ci`: install locked frontend dependencies.
- `cd frontend && npm run build`: build frontend assets. Run before Go checks that include the root package if `frontend/dist/` is missing; rebuild when validating changed assets.
- `make dev`: launch the desktop app with hot reload. It blocks; for agent-driven UI checks use `.agents/skills/run-mailven/dev.sh` instead.
- `make build`: production build in `build/bin/`.
- `make generate`: regenerate Wails TypeScript bindings.
- `make test`, `make fmt`: Go tests and formatting.
- `make lint`: golangci-lint v2, frontend ESLint, the brand check (`tools/brand-check.sh`, allowlist in `tools/brand-check.allow`) and the theme contrast check (`tools/contrast-check.mjs`, text tiers ≥ 4.5:1 on pane surfaces).
- `cd frontend && npm run check`: Svelte/TypeScript checks.
- OAuth testing: copy `.env.example` to `.env`; `.env.local` overrides it.
- In noninteractive agent shells, use `git rm` for authorized tracked-file removals and `command rm` for authorized untracked-file cleanup; the shell’s `rm` wrapper prompts and aborts without a terminal. Preserve a local rollback for destructive cleanup.

## Code Style

- Go: log with zerolog, and prefer guard clauses and switches over long `else if` chains.
- Frontend: use Svelte 5 runes (`$state`, `$derived`), keep components under 500 lines, and preserve keyboard accessibility.
- Localization: the UI is English-only, with en-US, en-GB and en-CA spellcheck dictionaries; email content stays fully international (any language, charset and script). Update English locale files only, and leave non-English locale files out of searches and reviews unless the task concerns localization. Preserve the existing i18n structure. See `docs/LANGUAGE.md` when changing localized strings.

## Testing

- Start with checks relevant to the change.
- Run `make test` when a change affects multiple Go packages or shared behavior, or when focused checks leave unresolved risks.
- Use table-driven Go tests for critical paths and edge cases.
- `internal/database/database_test.go` rewinds the schema and reapplies later migrations: a new `ADD COLUMN` needs matching `DROP COLUMN` lines in its rewind blocks, and every migration must be rerunnable (`IF NOT EXISTS`, and backfills that clear their table first).
- For frontend changes, run `npm run lint` and `npm run check` in `frontend/`, then `make test-ui` (backend-free UI audit and regression tests, fake data; [tests/ui/README.md](tests/ui/README.md)); `node tests/ui/run.mjs <filter>` runs only the matching suites, for quick iteration; run the full suite before reporting. Use the `run-mailven` skill for checks the suite cannot make (native window, real backend, subjective visuals). In cloud sessions use the skill's headless path (Xvfb + Playwright); if no UI check can run at all, say so in the active execution checkpoint and in your report.
- Before writing or changing frontend fakes or UI tests, read [Writing tests](tests/ui/README.md#writing-tests).
- Give native selects explicit accessible names (`aria-label` or a separate `for`/`id` label); wrapping labels can absorb option text and break exact-name UI checks.
- Turn each confirmed frontend bug into a regression test. Treat a UI test failure as a product, test or fixture defect: read its message and `test-results/ui/results.json`, and use screenshots only for investigation. Do not rerun unchanged or loosen an assertion or allowlist entry to pass; escalate ambiguous visual failures. Report passed, failed, skipped and unrun checks separately.
- Visual UI checks always cover the Mail view with the Accounts column expanded and collapsed (how-to: [Accounts column check](.agents/skills/run-mailven/SKILL.md#accounts-column-check)). Report checks you couldn't perform.

## Documentation

- For provider, OAuth, mail-route or account-setup changes, start with [docs/PROVIDERS.md](docs/PROVIDERS.md) and follow its links to the relevant technical and user guides. Keep the provider matrix and affected requirements current.

- When a change affects user-visible behavior, commands, configuration, or developer workflows, update the relevant documentation in the same task.
- Correct existing guidance instead of adding duplicate instructions.

## Milestones

Use project-next-milestone for one explicitly selected or previously authorized milestone and project-finish-milestone to close out implemented work; the latter owns the done checklist (checks, review, independent acceptance check, fixes, simplification, commit). Without the skill, do those steps directly and report any missing independent check. For a small independent fix, run the relevant checks, review the diff, update the `ISSUES.md` and `PLAN.md` entries it resolves or affects without being asked, and commit them together.

## Commits and PRs

- Use descriptive prefixes for commits: `feat:`, `fix:`, `docs:`.
- Work on a branch per feature or audit pass, not on `main`. When all of a feature's milestones are done or an audit pass is complete, land the branch without being asked, following [Landing a branch](CONTRIBUTING.md#landing-a-branch).
- One commit per milestone or independent fix. Fold corrections to a commit that has not been pushed into it (`git commit --amend`) instead of adding a new commit.
- Commit `PLAN.md` and `ISSUES.md` updates with the change they describe, not separately.
- See `CONTRIBUTING.md` for setup and workflow details. Consult upstream's own guidelines when submitting upstream.

## Planning Files

- `PLAN.md` is the roadmap and `ISSUES.md` the unresolved issue register, keyed by the same stable IDs. For roadmap work, read Portfolio/Progress, then only the selected project/milestone and linked issues (`tools/project-context.py <id>`); unrelated changes need neither file. Follow [the lifecycle rules](docs/PROJECT_MANAGEMENT.md) for selection, issues, execution plans, checkpoints, completion and handoff. Proposed priorities do not approve work, deferred work is not automatically reopened, and done means verified acceptance.
- Record every confirmed finding left unfixed in `ISSUES.md` in the same task. Check its [Recurring problems](docs/PROJECT_MANAGEMENT.md#recurring-problems) section before debugging environment or tool symptoms. Keep task progress out of guidance, and correct obsolete guidance as discovered.

## Compaction

When compacting, preserve the checkpoint path and state, current milestone/sub-step and next action, modified files, blockers, exact failing checks, and discoveries or user answers not yet persisted. Compaction is the fallback; prefer fresh sessions from persisted checkpoints at milestone and sub-step boundaries ([session boundaries](docs/PROJECT_MANAGEMENT.md#session-boundaries)).
