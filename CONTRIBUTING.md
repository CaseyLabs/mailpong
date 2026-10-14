# Contributing to Mailven

Mailven is an independent project derived from [Aerion](https://github.com/hkdb/aerion). This repository is the home of Mailven; it is not a fork of Aerion on GitHub. Aerion's own contribution rules apply only to submissions made to Aerion.

## Workflow

- Keep changes focused and explain their purpose.
- Use [Mailven's issues](https://github.com/CaseyLabs/mailven/issues) for bugs, ideas, and questions. If a bug also exists in Aerion, mention it; upstream fixes may be cherry-picked (see below).
- Base changes on `main` unless the task specifies another target.
- Describe the behavior change, relevant issues, checks performed, and known limitations. Include screenshots for visible UI changes when useful.
- Use descriptive commit messages with prefixes such as `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, and `chore:`.

## AI project guidance

Shared policy, including extension/core/kit review triggers, lives in
`AGENTS.md`; `CLAUDE.md` imports it. Read linked references only when their
stated trigger applies; they are not automatic imports. Edit project skills
and their resources in `.agents/skills/`. The single relative directory symlink
`.claude/skills -> ../.agents/skills` exposes the same files to Claude Code.
Start either CLI normally from the checkout; no plugin or custom discovery flag
is needed. Claude uses `/skill-name`; Codex uses `$skill-name` or `/skills`.
Keep shared workflows independent of host-specific tools and preprocessing.
Native model/effort settings live in `.claude/agents/` and `.codex/agents/`;
both adapters read the same worker/reviewer briefs in `.agents/skills/`.
Use `project-acceptance-checker` for milestone acceptance and `project-worker`
for general delegated project work. If the host cannot load them, disclose the
fallback and that the configured model/effort was not applied. Host permissions,
model availability and explicit user choices still take precedence.

Codex loads project-scoped native agents only for a trusted checkout. Review and
trust a new clone through the normal Codex onboarding before expecting these
roles to load; trust in a parent directory does not cover a nested Git checkout
in CLI 0.162.1. Skill discovery can still pass while native roles return
`unknown agent_type`. To diagnose this, inspect the project layer
`disabledReason` with app-server `config/read` (`cwd` set to the checkout,
`includeLayers: true`). Do not copy agents into personal configuration or add
discovery overrides to bypass trust. See the [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

| Native agent | Claude Code | Codex |
| --- | --- | --- |
| `project-acceptance-checker` | Sonnet, low effort | GPT-6 Luna, medium effort |
| `project-worker` | Sonnet, medium effort | GPT-6 Luna, medium effort |
| `file_reader` | Not defined; retain Claude routing | GPT-6 Luna, low effort |

Codex primary sessions inherit the personal GPT-6.1 Sol Medium default; this
checkout has no session-wide model override. The project-local `file_reader`
handles only mechanical retrieval, listings, and exact text extraction, including
independent steps within skill workflows. Use deterministic tools directly when
delegation adds no benefit. Investigation, implementation, synthesis, and
acceptance checks retain Medium reasoning. Explicit user choices take precedence.

The G3 verification record describes earlier settings; the table above describes
the current routing. Start a fresh session to load changed role definitions.

The one shared-frontmatter exception is `model: sonnet` in
`project-next-milestone/SKILL.md`: it declares a supported Claude turn-scoped override,
with no skill-specific effort override. Codex retains its selected session model
and effort; its 0.162.1 native loader accepts the extra field. Keep this exception
explicit rather than adding a duplicate Claude skill or changing the model for
all project sessions. The generic skill-creator validator rejects this Claude
extension; native loading checks supplement that narrower schema check.
Runtime model/effort selection is verified separately from parsing and skill
discovery. See the [verification record](docs/validation/shared-skills-g3.md).

Resolve the active checkout with `git rev-parse --show-toplevel`; never reuse a
skill path from another checkout. User-supplied relative plan/output paths resolve
from the invocation working directory (default plan: `PLAN.md` there), before
changing directory. Quote resolved paths. For example, from the checkout root:
`python3 tools/project-context.py --plan PLAN.md`; add the authorized milestone ID for its linked issues/checkpoint. `plan-outline.sh` remains available for legacy outline-only use.
Read Portfolio/Progress and the explicitly selected or already authorized milestone, its linked issues and execution checkpoint only.
Follow the shared [project-management lifecycle](docs/PROJECT_MANAGEMENT.md): `PLAN.md` owns the roadmap, `ISSUES.md` owns findings, and substantial selected milestones checkpoint in `docs/plans/<milestone-id>.md`. Reconcile before work and preserve lasting evidence before pruning. A bare continuation request with no authorized active milestone lists candidates for selection; it does not execute proposed/deferred work. Keep outstanding
verification, deferred work and authorization limits explicit.
The run-mailven core links optional UI references by task; its safeguards always
apply. Handoffs persist checkpoints before committing and include a host-neutral
prompt. Remaining G4 runtime/context verification and future provider work are tracked
in [AI / G4](PLAN.md#g4) and [the provider projects](PLAN.md#api); completed provider checks are retained in
[the implementation validation record](docs/validation/provider-implementation.md).
The [verification record](docs/validation/shared-skills-g3.md) documents completed
Codex and guarded UI checks. [Bidirectional handoff trials](docs/validation/shared-handoff-ai5.md)
verified fresh Codex/Claude continuation with dirty work, checkpoint provenance
and blockers. Broader Claude workflow compliance/routing and comparable context
savings remain unverified and deferred until requested on a new branch; their
deferral does not prevent landing the completed scope.
The screenshot helpers install fake bindings before mount and verify persisted
UI and extension settings stay unchanged; follow the run-mailven safeguards.
Claude's import adapter is intentional compatibility guidance; do not remove it
based solely on support for native AGENTS.md in another version or configuration.

Clones must preserve Git symlinks. On Windows, enable Developer Mode or use
appropriate privileges and clone with `git -c core.symlinks=true clone <url>`.
Verify `.claude/skills` is a directory symlink, not a text file containing its
target. Do not replace it with a copied skill tree. Relative links also keep
linked Git worktrees pointing to their own skill files.

## Workflow helpers

These standard-library helpers reduce repeated shell recipes and returned context. They preserve the existing lifecycle and authorization rules; none selects a milestone, performs review, authorizes live work or marks acceptance complete. Required guidance already read in a session may be reused while unchanged; new sessions load it again.

From the checkout root:

```bash
python3 tools/project-context.py                  # Portfolio/Progress and Git facts
python3 tools/project-context.py API-5            # Selected project, milestone, linked issues/checkpoint
python3 tools/check-doc-links.py --changed        # Staged/unstaged/untracked Markdown
python3 tools/project-check.py --base main        # Check plan only
python3 tools/project-check.py --base main --run  # Serial checks and private evidence/logs
```

`project-context.py` accepts `--plan`/`--issues`, `--json` and `--max-lines`; source line pointers identify bounded excerpts. User-supplied paths resolve from the invocation directory, including spaces and worktrees. Missing/ambiguous milestones and plans are errors. Context describes proposed/deferred status without authorizing it.

`check-doc-links.py` accepts explicit Markdown paths, `--changed`, `--base` and `--json`. It validates local destinations, anchors, reference links and `/docs/` routes against user-guide sources; it does not fetch external sites. Deleted sources are skipped during changed-file scans, but links to missing targets fail. Ordinary relative links may browse existing repository directories without an index; `/docs/` site routes require a `.md`, `.mdx` or `index.md` page. Code examples are ignored. Use `--max-errors` to bound console diagnostics and `--json-output` for a new full report file (existing files are refused). Keep archive/origin files outside checks unless the task changes or explicitly selects them.

`project-check.py` infers scopes from changes against the common ancestor of the supplied base (default `HEAD`) and the current branch and unions any `--scope docs|tooling|go|frontend` additions. Unsupported changed paths require manual validation; it never silently assumes they passed. Add milestone-specific checks with `--extra-command '["python3", "path/to/check.py"]'` (argv only, checkout-root cwd). Frontend includes lint/check/build and the full UI suite; Go includes `make test` and prepares missing embedded frontend assets. Tooling runs the Python helper tests, skill path checks and brand check. Existing native/live gates remain separate. Only one helper check/landing operation may run in a checkout at a time.

Execution saves complete logs and `evidence.json` in a new private directory under `~/.cache/mailven-project-checks/` (or `$XDG_CACHE_HOME`); `--output-dir` chooses a new directory outside the checkout. Console output is bounded by `--tail-lines` (default 6). Failure stops the batch, preserves logs and identifies unrun checks. Source fingerprints cover tracked and nonignored untracked files, their content, symlinks and executable modes; changing source invalidates evidence, while committing identical content does not. Review logs before sharing them. Tests/builds may legitimately change generated source, in which case review and rerun rather than claiming fresh evidence.

For the standard authorized squash landing, commit all milestone files, complete independent acceptance and review, and preview:

```bash
tools/project-land.sh feature-branch --evidence /absolute/path/evidence.json
# Supply a reviewed title and body file; --execute performs the authorized action.
tools/project-land.sh feature-branch --evidence /absolute/path/evidence.json \
  --title 'feat: describe the completed work' --body-file /absolute/path/body.txt --execute
```

Landing requires the feature checkout to be clean, checks/logs to match its source and scopes, and main to have a configured upstream. Evidence must start at the landing merge base or an earlier ancestor and cover every changed feature path; a later feature commit cannot serve as its baseline. Use `project-check.py --base main --run` for the whole branch, including when main has diverged; its recorded baseline is the common ancestor. Older evidence recorded directly against diverged main must be regenerated. It retains `recovery/project-land-...-feature` and `...-main` local branches before changes, rebases when needed and reruns the recorded checks, aborts on conflicts, respects configured identity/hooks, pushes main without force, and deletes the feature only after a successful push and equal-tree comparison. A main branch already checked out in another worktree requires landing from a checkout where it is available. Failure retains the feature/recovery branches and reports the Git state; fix the stated problem without bypassing safeguards. Recovery branches are intentionally not removed automatically.

Validate changes to helpers with `python3 -m unittest discover -s tools/tests -p 'test_*.py'` and `python3 .agents/skills/project-next-milestone/tests/paths.py`. These tests use disposable repositories/remotes and fake commands, never production accounts or project pushes.

## Development Setup

Use Go 1.26, Node.js 24 (matching CI), and the Wails v2 CLI. On Linux, install GTK 3 and WebKitGTK 4.1 development libraries.

Enable the tracked Git hooks once per clone:

```bash
git config core.hooksPath .githooks
```

`commit-msg` rejects AI co-author lines, assistant session trailers, and "Generated with/by" attribution, for people and agents alike.

The frontend uses TypeScript 6, the newest major accepted by both the lint and Svelte check tools.
Before upgrading TypeScript, check the published peer ranges of both tools:

```bash
npm view typescript-eslint@latest version peerDependencies --json
npm view svelte-check@latest version peerDependencies --json
```

Upgrade only to a version accepted by both, then run frontend lint, check, and build.

```bash
git clone https://github.com/CaseyLabs/mailven.git
cd mailven
go mod download
cd frontend
npm ci
npm run build
cd ..
make dev
```

For OAuth testing, copy `.env.example` to `.env` and configure your own credentials. `.env.local` overrides `.env`. Do not commit credentials or include private mail data in logs or issues.

`make dev` uses the same data directory (`~/.local/share/mailven` on Linux) and OS keyring as a native build. Refresh tokens belong to the OAuth client that issued them, so after the client ID in `.env`/`.env.local` changes, or for accounts added by a build with other credentials, sync fails with `token refresh failed: unauthorized_client`. Re-authorize the account once in the dev app (edit it in Settings → Accounts, then **Re-authorize**) to fix it.

`make dev` always renders the Dark theme, whatever theme is saved in Settings. Theme choices are still saved and apply in production builds.

Debug logging is off by default. Start Mailven with `--debug` or `MAILVEN_DEBUG=1` to log at debug level to the console and to `mailven.log` in the data directory (`~/.local/share/mailven/` natively, `~/.var/app/io.github.caseylabs.Mailven/data/mailven/` under Flatpak). The log rotates to `mailven.log.1` at 10 MB.

## Code and Validation

Prefer small changes, explicit Go error handling, and guard clauses. Format Go with `gofmt`, document exported functions, and use zerolog for logging. Frontend code uses two-space indentation, single quotes, no semicolons, PascalCase component names, and Svelte 5 runes. Keep components focused and under 500 lines. Preserve keyboard accessibility, resource efficiency, and user privacy.

- `make fmt`: format Go code.
- `make lint`: run golangci-lint v2, frontend ESLint, the theme contrast check (`tools/contrast-check.mjs`) and the brand check (`tools/brand-check.sh`; add a reasoned line to `tools/brand-check.allow` for any legitimate "Aerion" mention).
- `cd frontend && npm run check`: check Svelte and TypeScript.
- `make test`: run Go tests; build the frontend first because Go embeds `frontend/dist/`.
- `make test-ui`: run the backend-free UI audit and frontend regression tests headlessly with fake data; needs Playwright outside `frontend/package.json` (see `tests/ui/README.md`).
- `make build`: produce the desktop application.
- `make generate`: regenerate bindings after changing the Wails API.

Start with checks relevant to the change. Add Go tests for meaningful behavior and edge cases, using table-driven tests where appropriate. Frontend regression tests live in `tests/ui/`; add one for each confirmed frontend bug. No numeric coverage threshold is configured. Manually check affected UI flows the suite does not cover and report checks that could not run.

## Localization and Security

The UI is English-only; see [docs/LANGUAGE.md](docs/LANGUAGE.md) for how strings and locales work. Report vulnerabilities according to [SECURITY.md](SECURITY.md).

## Aerion Upstream and License

Upstream Aerion fixes can be cherry-picked into Mailven. In a clone that lacks the remote, run `git remote add upstream https://github.com/hkdb/aerion.git`, then `git fetch upstream` and `git cherry-pick <commit>`. Renamed paths and identifiers can make these conflict, so resolve them by hand. For changes meant for Aerion itself, consult Aerion's current contribution rules separately.

Replace "Aerion" only where it names this app. Keep it in attribution, history, `LICENSE`, `archive/`, statements of origin, and the persisted legacy identifiers listed in [AGENTS.md](AGENTS.md). Preserve the repository's Apache License 2.0 terms and the [NOTICE](NOTICE).
