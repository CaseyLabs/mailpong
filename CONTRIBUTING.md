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
`sh .agents/skills/project-next-milestone/plan-outline.sh PLAN.md`.
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
- `make build`: produce the desktop application.
- `make generate`: regenerate bindings after changing the Wails API.

Start with checks relevant to the change. Add Go tests for meaningful behavior and edge cases, using table-driven tests where appropriate. No frontend test script or numeric coverage threshold is configured. Manually check affected UI flows and report checks that could not run.

## Localization and Security

The UI is English-only; see [docs/LANGUAGE.md](docs/LANGUAGE.md) for how strings and locales work. Report vulnerabilities according to [SECURITY.md](SECURITY.md).

## Aerion Upstream and License

Upstream Aerion fixes can be cherry-picked into Mailven. In a clone that lacks the remote, run `git remote add upstream https://github.com/hkdb/aerion.git`, then `git fetch upstream` and `git cherry-pick <commit>`. Renamed paths and identifiers can make these conflict, so resolve them by hand. For changes meant for Aerion itself, consult Aerion's current contribution rules separately.

Replace "Aerion" only where it names this app. Keep it in attribution, history, `LICENSE`, `archive/`, statements of origin, and the persisted legacy identifiers listed in [AGENTS.md](AGENTS.md). Preserve the repository's Apache License 2.0 terms and the [NOTICE](NOTICE).
