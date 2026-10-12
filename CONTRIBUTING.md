# Contributing to Mailven

Mailven is an independent project derived from [Aerion](https://github.com/hkdb/aerion). This repository is the home of Mailven; it is not a fork of Aerion on GitHub. Aerion's own contribution rules apply only to submissions made to Aerion.

## Workflow

- Keep changes focused and explain their purpose.
- Use [Mailven's issues](https://github.com/CaseyLabs/mailven/issues) for bugs, ideas, and questions. If a bug also exists in Aerion, mention it; upstream fixes may be cherry-picked (see below).
- Base changes on `main` unless the task specifies another target.
- Describe the behavior change, relevant issues, checks performed, and known limitations. Include screenshots for visible UI changes when useful.
- Use descriptive commit messages with prefixes such as `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, and `chore:`.

## AI project guidance

Shared policy lives in `AGENTS.md`; `CLAUDE.md` imports it. Edit project skills
and their resources in `.agents/skills/`. The single relative directory symlink
`.claude/skills -> ../.agents/skills` exposes the same files to Claude Code.
Start either CLI normally from the checkout; no plugin or custom discovery flag
is needed. Claude uses `/skill-name`; Codex uses `$skill-name` or `/skills`.
Keep shared workflows independent of host-specific tools and preprocessing.
The remaining portability work and verification are tracked in `PLAN.md`.

Clones must preserve Git symlinks. On Windows, enable Developer Mode or use
appropriate privileges and clone with `git -c core.symlinks=true clone <url>`.
Verify `.claude/skills` is a directory symlink, not a text file containing its
target. Do not replace it with a copied skill tree. Relative links also keep
linked Git worktrees pointing to their own skill files.

## Development Setup

Use Go 1.26, Node.js 24 (matching CI), and the Wails v2 CLI. On Linux, install GTK 3 and WebKitGTK 4.1 development libraries.

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
