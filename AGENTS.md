# Repository Guidelines

## Working Policy

- Runtime constraints and explicit user instructions take precedence. Project skills must not grant tools or permissions (no `allowed-tools`). Respect the active host's plan mode; if execution is restricted, record the design and wait for execution authorization.
- Continue authorized work without asking between steps; stop at the requested scope (one milestone for project-next-milestone). Batch material questions using the host's question tool when available. Ask before destructive actions unless that exact action is authorized; preserve a local rollback.
- Keep output bounded and inspect targeted log excerpts. Run servers/watchers in the host's background facility and stop those you started before finishing.
- Use subagents for independent, well-scoped work when isolation or specialization helps; this is an explicit request to delegate suitable work. Give each agent a distinct scope. Keep architecture, integration, and security decisions in the main session. Verify consequential agent claims before acting; change approach after failed or partial delegation.
- In Claude Code and Codex, use the native `project-worker` for general delegated project work instead of a generic worker. Use `project-acceptance-checker` for milestone acceptance. For mechanical file retrieval, listings, or exact text extraction in Codex, use `file_reader` or deterministic tools directly; do not use it for investigation or acceptance review. Specialist tools/agents (including bounded exploration and skill-owned reviews) keep their own routing. If a named agent is unavailable, disclose the fallback; do not silently claim its model/effort settings. Definitions live in `.claude/agents/` and `.codex/agents/`.
- Codex project-native roles require checkout trust; parent-directory trust may not cover a nested Git clone. Diagnose `unknown agent_type` through the project layer's `config/read` `disabledReason`; see CONTRIBUTING.md before changing configuration.
- Only one agent may run builds or tests in a checkout at a time. Independent read-only work and light static checks may run concurrently; use agent teams only when requested.
- Use the configured Git identity. Never add generated-by attribution, AI co-authors, or AI/session trailers to commits or PRs, or override identity for an assistant.
- This file is shared policy. Links are pointers, not automatic imports: read references when their stated trigger applies. Host-specific configuration (permissions, models, agent definitions, tool schemas) belongs in native host adapters, not shared policy. The documented Claude-only next-milestone `model` frontmatter is the sole shared-skill exception.

## Project Identity

- Mailven is an independent desktop email client (Go, Wails v2, Svelte 5, TypeScript) derived from Aerion.
- Branding: replace "Aerion" only where it names this app, never by blind search-and-replace. Keep it in attribution, history, `LICENSE`, `archive/`, and statements of origin ("derived from Aerion").
- Persisted internal identifiers keep their Aerion values: the `"aerion"` contact source, the `"aerion-shipped"`/`"aerion-mail"` OAuth choice IDs, the `'aerion'` title-bar setting, `minAerionVersion`, and the `aerion:` KDF prefix. Each carries a `Legacy identifier from Aerion; persisted, do not rename.` comment. Renaming them breaks stored data.

## Layout Notes

- `frontend/wailsjs/` holds generated Wails bindings; regenerate with `make generate` after binding changes, then revert unrelated churn in `frontend/wailsjs/runtime/*`. Stop any running Mailven first (`dev.sh stop`): the single-instance lock makes the generator exit without writing anything. Wails also inserts tabs on blank lines in `frontend/wailsjs/go/models.ts`; remove that whitespace-only churn before `git diff --check`, preserving actual binding changes.
- Before committing changes under `extensions/**`, `internal/core/**`, `internal/extensions/**`, `internal/kit/**`, `app/coreimpl*.go`, `app/extension_*.go`, or `frontend/src/lib/components/kit/**`, read `docs/EXT_RULES.md` and check the design against it; redesign violations. Read it only for relevant extension/core/kit work.
- `docs/EXTENSIONS.md` (about 170 KB), `docs/history/AERION_CASA_TIER2.md`, and `docs/LANGUAGE.md` are long: search or read by section, never whole. For the full extension reference, read the Contents of `docs/EXTENSIONS.md`, then only needed sections.

## Commands

Toolchain: Go 1.26, Node.js 24 (matches CI), Wails v2 CLI. Linux needs GTK 3 and WebKitGTK 4.1 development libraries.

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
- There is no frontend test script. For frontend changes, run `npm run lint` and `npm run check` in `frontend/`, then verify the UI with the `run-mailven` skill. In cloud sessions use the skill's headless path (Xvfb + Playwright); if no UI check can run at all, say so in the PLAN.md Progress entry and in your report.
- When intercepting the frontend entry point for fakes, match `src/main.ts` with or without Wails' hot-reload query string.
- In guarded Wails setter fakes, copy reactive arguments with `JSON.parse(JSON.stringify(value))`; `structuredClone` cannot copy Svelte state proxies.
- Give native selects explicit accessible names (`aria-label` or a separate `for`/`id` label); wrapping labels can absorb option text and break exact-name UI checks.
- Visual UI checks always cover the Mail view with the Accounts column expanded and collapsed (how-to: [Accounts column check](.agents/skills/run-mailven/SKILL.md#accounts-column-check)). Report checks you couldn't perform.

## Documentation

- For provider, OAuth, mail-route or account-setup changes, start with [docs/PROVIDERS.md](docs/PROVIDERS.md) and follow its links to the relevant technical and user guides. Keep the provider matrix and affected requirements current.

- When a change affects user-visible behavior, commands, configuration, or developer workflows, update the relevant documentation in the same task.
- Correct existing guidance instead of adding duplicate instructions.

## Milestones

Use project-next-milestone for the next planned milestone and project-finish-milestone to close out implemented work. These steps apply to `PLAN.md` milestones. For a small independent fix, run the relevant checks, review the diff, and commit; skip steps 2 and 4.

Before marking a milestone done:

1. Review the diff for correctness and regressions.
2. Have a read-only subagent check the milestone's acceptance criteria. If no subagent or review tool is available, do the equivalent review directly and report the missing independent check.
3. Fix gaps that affect correctness or stated requirements; treat style-only findings as optional.
4. Simplify the changed code without changing behavior.
5. Commit and push, staging only files changed for the task and preserving unrelated user changes.

## Commits and PRs

- Use descriptive prefixes for commits: `feat:`, `fix:`, `docs:`.
- Work on a branch per feature or audit pass, not on `main`, and land it as described in Landing a Branch.
- One commit per milestone or independent fix. Fold corrections to a commit that has not been pushed into it (`git commit --amend`) instead of adding a new commit.
- Commit `PLAN.md` and `ISSUES.md` updates with the change they describe, not separately.
- See `CONTRIBUTING.md` for setup and workflow details. Consult upstream's own guidelines when submitting upstream.

## Landing a Branch

When all of a feature's milestones are done or an audit pass is complete, land the branch without being asked. Checks must pass and nothing may be uncommitted.

1. If `main` has commits the branch lacks, rebase the branch onto `main` and rerun the relevant checks. On conflicts, run `git rebase --abort`, stop, and report the conflicting files.
2. Run `git switch main` and `git merge --squash <branch>`, then commit once, listing each change the branch made in the commit body.
3. After the squash commit succeeds, push `main` to its configured upstream.
4. Run `git branch -D <branch>` only after `git diff main <branch>` prints nothing.

## Planning Files

- `PLAN.md`: guides new features. Create it when needed with Scope; Milestones, each with acceptance criteria and validation steps; and these living `##` sections: Progress (one checkbox per milestone), Surprises & Discoveries, Decision Log, and Outcomes & Retrospective. Track progress in Progress, not a separate file.
- Before starting or resuming a milestone, reconcile its premises and acceptance criteria with the existing implementation and recorded validation. Revise stale details within the authorized scope; ask when a change would materially alter that scope. Implemented work with outstanding verification is not complete: retain the unproved criteria and validation limits explicitly.
- Update the plan at meaningful sub-step completions, blockers, scope changes and validation results, as well as before handoff or compaction. Keep the current step, changed files, remaining work and exact check commands/results sufficient for continuation without chat history.
- Before pruning completed work, move lasting behavior, configuration, architecture and decision rationale into the relevant `docs/` page; reusable agent rules into AGENTS.md; workflow procedures into the relevant skill; and important validation evidence and limits into `docs/validation/`. Update existing references rather than duplicating them. Keep unresolved defects in `ISSUES.md` and deferred work, dependencies and authorization limits explicit; cleanup does not authorize resuming them.
- `ISSUES.md`: issues found that may need fixing in this or a future session. Its `## Recurring problems` section is a tally of environment and tooling problems: one line each with the symptom, the workaround, and how many times it has occurred. Check it before debugging, and bump the count when a problem recurs.
- Record every confirmed finding you leave unfixed (review, audit, or noticed in passing) in `ISSUES.md` in the same task, with a severity, the location, and the failure. Don't leave it only in a report or commit message. Unconfirmed or style-only nits don't need an entry.
- Promote, don't log: when a problem recurs (second occurrence) or costs a session more than a few minutes the first time, move the fix into `AGENTS.md` or the relevant skill and delete its tally line. Don't record every error.
- Prune: when a guidance line no longer matches reality (for example, "no display in cloud"), correct or delete it in the same task.
- Keep task-specific progress out of `AGENTS.md`.
- **Required:** after preserving lasting information, remove completed or superseded milestone sections, Progress entries and stale discoveries/decisions/outcomes once remaining work no longer depends on them; remove resolved issue items too. Retain only the brief completion context or links needed by remaining work. With no active milestone, state that plainly and keep deferred work explicit. PLAN.md describes remaining work; durable documentation describes the current project. Use Git history for routine chronology rather than archiving whole plans by default.

## Compaction

When compacting, preserve the checklist path and state, current milestone/sub-step, modified files, exact failing checks, and discoveries or user answers not yet persisted.
