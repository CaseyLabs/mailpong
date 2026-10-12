# Plan: rename MailPong to Mailven

## Scope

Rename the project `MailPong` to `Mailven` and `mailpong` to `mailven`. The new repo is https://github.com/CaseyLabs/mailven.

Decisions:
- Persisted data is a clean break: no migration of the keyring service, data directories, database filename or lock files.
- The app ID becomes `io.github.caseylabs.Mailven`.
- Historical text (old release notes, `docs/PLAN-migration.md`, `ISSUES.md`) is renamed too, and README and AGENTS.md get a "formerly MailPong" note.
- Legacy identifiers listed in `AGENTS.md` stay untouched.
- The user renames the GitHub repo and repoints `origin`; see M4.
- You have my permission to use my `gh` CLI tool for changes.

Work happens on branch `rename-mailven`, one commit per milestone, then lands per Landing a Branch.

## Milestones

### M1 — Go module, imports, identifiers
- `go.mod` module becomes `github.com/caseylabs/mailven`; rewrite all imports.
- Rename persisted identifiers: keyring `serviceName` (`internal/credentials/store.go`, `internal/keyring/keyring.go` and their tests), `mailven.db`, `~/.config|.local/share|.cache/mailven`, `Application Support/Mailven`, `%APPDATA%\Mailven`, single-instance paths, `windowsAppID`, DBus handle tokens and commandline, `MAILVEN_DEBUG`, build tag `mailven_creds` (`build/gencreds.go`, `internal/oauth2`), `MailvenNotificationDelegate`, test-key strings.
- Acceptance: `go build ./...`, `make test` and `make fmt` are clean; `git grep -i mailpong -- '*.go'` is empty.

### M2 — Build, packaging, file renames
- `git mv` the `run-mailpong` skill, `brand/mailpong.svg`, `build/linux/mailpong.png`, the Flatpak manifests, icons, `.desktop` and metainfo files to the new names.
- Non-Go leftovers from M1 that must land here: Makefile `BUILD_TAGS` (`mailpong_creds` -> `mailven_creds`, also docs/BUILD.md, docs/EXTENSIONS.md), rename `cmd/mailpong-creds` and its `.gitignore` entry, `MAILPONG_DEBUG` in the run skill and troubleshooting doc, and `StartupWMClass`, `Info.plist` `CFBundleIdentifier`, icon names and Flatpak command so they match the Go-side `io.github.caseylabs.Mailven` / `mailven`. Until M2 lands, `make build` and Linux/macOS autostart are mismatched on this branch (squash-merged, so never released).
- Update `wails.json`, `Makefile`, `.github/` workflows and issue templates, `frontend/package.json` and lockfile, `tools/brand/render-icons.sh`, Flatpak manifests, `.claude/` references, `.env.example`.
- Acceptance: `make build` produces `build/bin/mailven`; `make lint` passes; manifest, desktop file and metainfo agree on the new app ID.

### M3 — Frontend, extensions, docs, in-app text
- Replace user-visible text in `frontend/src`, `extensions/*`, `app/*`, `docs/**`, README, CONTRIBUTING, AGENTS.md, CLAUDE.md, ISSUES.md; GitHub URLs point to `CaseyLabs/mailven`.
- Add the "formerly MailPong" note; keep "derived from Aerion" statements and keep `tools/brand-check.allow` valid.
- Acceptance: `cd frontend && npm ci && npm run lint && npm run check && npm run build` pass; `git grep -i mailpong` shows only intentional hits.

### M4 — Verify and land
- UI check with the `run-mailven` skill: Mail view with Accounts column expanded and collapsed; title and About show "Mailven".
- Full `make test` and `make lint`; final sweeps of `git grep -il mailpong`, `git ls-files | grep -i mailpong`, and variants (`Mail Pong`, `mail-pong`).
- Acceptance check by `project-acceptance-checker`, then land the branch.
- User steps: rename the repo on GitHub, run `git remote set-url origin git@github.com:CaseyLabs/mailven.git`, update any Flathub or OAuth registrations, rename the local directory.

## Progress

- [x] M1 — Go module, imports, identifiers
- [x] M2 — Build, packaging, file renames
- [x] M3 — Frontend, extensions, docs, in-app text
- [x] M4 — Verify and land

## Surprises & Discoveries

- Survey: about 381 tracked files and 1600 matches; 20+ file names contain the old name.
- M1 renamed `go.mod` and all `*.go` mechanically (mailpong/MailPong/MAILPONG). Non-Go references (build tag in Makefile, `cmd/mailpong-creds`, `MAILPONG_DEBUG` docs, packaging IDs) are left for M2/M3; the review found them, and M2 now lists them. Also renamed `notifier_darwin.m` and gave `windowsGUID` a fresh GUID (clean break; the old one would share a COM CLSID with MailPong). `provider_microsoft_test.go` had a baseline gofmt drift, fixed with the rename.
- M2 renamed everything outside docs/frontend/extensions mechanically (build, .github, .claude, Makefile, wails.json, tools, cmd, brand). `build/bin` still holds stale `mailpong` artifacts (gitignored). `make lint` failed on a baseline brand-check hit in PLAN.md, fixed with an allow entry. AGENTS.md/CLAUDE.md still link `run-mailpong`; M3 fixes the remaining docs; NOTICE, DISCLAIMER.md, SECURITY.md, THIRD_PARTY_NOTICES.md and CHANGELOG/metainfo wording for 0.4.0 also need M3 attention.

- M3 renamed all remaining text mechanically (MailPong/mailpong/MAILPONG -> Mailven/mailven/MAILVEN, GitHub URLs to CaseyLabs/mailven), then added the "formerly MailPong" notes to README and AGENTS.md. Remaining `git grep -i mailpong` hits are those notes and PLAN.md. The calendar extension localStorage keys (`mailven:calendar:*`) changed too (clean break).

## Decision Log

- Clean break for persisted data (no migration): the app has no installed base to preserve.
- App ID `io.github.caseylabs.Mailven`.

## Outcomes & Retrospective

Rename landed as one squash commit on `main`. Verified: `make test`, `make lint`, UI check (title and About show "Mailven"; Accounts column checked expanded and collapsed, restored to collapsed), sweeps show only the "formerly MailPong" notes and PLAN.md. Not pushed: the user still has to rename the GitHub repo, run `git remote set-url origin git@github.com:CaseyLabs/mailven.git`, update Flathub/OAuth registrations and rename the local directory.
