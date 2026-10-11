# PLAN: Migrate the Aerion fork to MailPong

Status: **Approved by the owner on 2026-10-11.** Milestones marked
**Approval gate** still need an explicit go-ahead when they're reached. M1 is
already approved.

## Scope

Turn the customized Aerion fork (`clabs-public-forks/aerion`) into an
independently branded, public project called **MailPong**, hosted at
`caseylabs/mailpong`.

### Objectives

1. Create `caseylabs/mailpong` as a standalone repository with the full Git
   history and original authorship intact, leaving `clabs-public-forks/aerion`
   unchanged.
2. Give MailPong its own application identity: name, IDs, binaries, data
   directories, keyring service and packaging. That identity must never read,
   change or delete an existing Aerion installation or its data.
3. Replace Aerion branding and artwork. Keep Aerion references wherever
   licensing, attribution or history requires them.
4. Meet Apache-2.0 obligations, credit Aerion and [@hkdb](https://github.com/hkdb)
   prominently, and state clearly that MailPong is independent and has no
   upstream endorsement.
5. Ship an English-only UI with en-US, en-GB and en-CA spellcheck, while
   leaving international email content handling unchanged.
6. Update all documentation and agent instructions for an independent project.
7. Validate everything and prepare a release for approval, but don't publish it.

### Project identity (final, given by the owner)

| Item | Value |
|---|---|
| Name (user-facing) | MailPong |
| Machine name | `mailpong` |
| Tagline | Email, reimagined as chat. |
| GitHub description | MailPong is like a Messenger app for your mailbox: it simplifies your inbox by turning emails threads into combined chats. Built on the open-source Aerion client. |
| Repository | `caseylabs/mailpong` (GitHub user account `CaseyLabs`) |
| Status | Alpha / active development |
| Origin | Independent project derived from the open-source Aerion email client |

### Out of scope (future considerations)

- Nixpkgs packaging or publication.
- New email or chat features, and UI redesigns unrelated to branding.
- Removing the upstream Git remote or changing any fork network (see D13).
- Unrelated refactoring or dependency upgrades. Removing the non-English
  dictionaries and adding `dictionary-en-ca` are the only dependency changes,
  and the English-only and en-CA requirements need them.
- Publishing a release, submitting to Flathub, or getting Google/Microsoft
  OAuth apps verified.
- Any change to `clabs-public-forks/aerion`, including a README pointer or
  archiving it, unless the owner approves it separately (see M14).

## Initial Findings and Inventory

Collected on 2026-10-11 from the cloud checkout (branch
`fix/admiring-archimedes-0rwkyr`, HEAD `1c0445d` = `origin/main`) and the
GitHub connector. **The owner's local checkout is authoritative.** M0 checks
it for unpushed work before anything is copied.

### Git and GitHub state

- `clabs-public-forks/aerion`: public, **a GitHub fork** of `hkdb/aerion`,
  owned by an organization. It has 1 star, no releases, no tags, no open
  issues, one closed PR (#1), 52 tags inherited from upstream (`v0.1.x`–`v0.3.5`; no GitHub releases), Issues enabled, and Wiki, Projects and
  Discussions disabled. It has a single branch, `main`. The token holds admin
  on it.
- `caseylabs/mailpong` doesn't exist yet. `CaseyLabs` is a **user** account
  (the authenticated GitHub user), and it holds no fork of `hkdb/aerion`.
- History: `main` has 369 commits. 166 come from upstream (hkdb, plus
  `github-actions[bot]` Flathub manifest commits) and 203 are fork commits by
  "Casey Labs". The fork point is `f9bd8eb` (`v0.3.5-2`). Upstream `main`
  hasn't moved since, so the fork is 0 commits behind. Upstream has 75 tags
  (`v0.1.x`–`v0.3.5`) and per-version `-dev` branches. The fork has none.
- This cloud checkout is **shallow** (50 commits). Mirroring needs a full
  clone (see M1).
- The cloud checkout has only an `origin` remote. The owner's local checkout
  may have an `upstream` remote, which stays (D13).
- The local `main` ref in this checkout is stale (`347d178`), 3 commits
  behind `origin/main`.
- This container's git identity is `Claude <noreply@anthropic.com>`, which
  conflicts with `.claude/rules/working-style.md` (see Surprises).

### Name occurrences

`git grep -i aerion` matches **2,185 lines in 425 files**. They fall into
these groups:

| Category | Examples | Treatment |
|---|---|---|
| Go module path | `module github.com/hkdb/aerion`, every import (most Go files) | Rename to `github.com/caseylabs/mailpong` (M2) |
| App identity / OS integration | `internal/platform/paths.go` (`aerion`, `Aerion`, `aerion.db`), `internal/keyring/keyring.go` and `internal/credentials/store.go` (`serviceName = "aerion"`), `socketdir_unix.go`, `singleinstance_{darwin,windows}.go`, `ipc/server_windows.go` (pipe `aerion-*`), `autostart_{linux,darwin,windows}.go`, `notifier_windows.go` (AUMID `io.github.hkdb.Aerion`), `notifier_darwin.m`, `network_darwin.go`, `sleep_windows.go`, `main.go` (`ProgramName`, title, `AERION_DEBUG`) | Rename for full separation (M3) |
| Outgoing protocol identity | `smtp/message.go` `User-Agent: Aerion Email Client` and Message-ID fallback domain `aerion`, `smtp/mdn.go` `Reporting-UA`, calendar `PRODID -//Aerion//…` and new-event UID suffixes `@aerion-*`, OAuth callback HTML | Rename. New values apply only to newly created items (M3) |
| User-visible UI strings | `frontend/index.html` title and boot splash, `TitleBar.svelte`, `WhatsNewDialog.svelte`, `AboutTab.svelte`, `TermsDialog.svelte`, `OAuthMissingDialog.svelte`, `app/app.go` startup error dialogs, `gbmcheck_linux.go`, 18 strings in `en.json`, 6 in extension `en.json` files, OAuth picker labels `"Aerion - Google"` etc. | Rename (M4) |
| **Persisted internal values** | contact source id `"aerion"` (`internal/contact`, contacts extension), OAuth choice IDs `"aerion-shipped"`/`"aerion-mail"` (DB rows), title-bar setting value `'aerion'`, extension manifest field `minAerionVersion`, the `aerion:` crypto KDF domain string (`internal/crypto/crypto.go:126`), schema-migration comments | **Keep** as legacy identifiers (D6) |
| Web-only identifiers | DnD MIME `application/x-aerion-*`, localStorage keys `aerion:calendar:*` | Rename. The webview storage location changes with the app ID, so no migration is needed (M4) |
| Wails/Frontend metadata | `wails.json` (name, output, company, product, version 0.3.5, author hkdb), `frontend/package.json` name/version, `frontend/wailsjs/go/models.ts` (2) | Rename, version 0.4.0, regenerate bindings (M4) |
| Packaging | `build/linux/aerion.desktop` (11 translations, `X-Flatpak-RenamedFrom`), `build/flatpak/*` (app ID `io.github.clabs_public_forks.Aerion`, sources pointing at `hkdb/aerion`), metainfo (194 `xml:lang` entries), `build/linux/install.sh`/`uninstall.sh`, `build/darwin/Info*.plist` (`com.wails.{{.Name}}`), `build/windows/*` (NSIS uninstall key derived from company+product), `cmd/aerion-creds`, build tag `aerion_creds`, `Makefile` | Rename (M6, M7) |
| CI/CD | `release.yml`: artifact names, OAuth secrets, a Flatpak job that **pushes to `main`**, Flatpak sources from `hkdb/aerion` | Rework (M8) |
| Artwork | `brand/*`, `build/appicon.png`, `build/linux/aerion.png` and `icons/*`, `build/windows/icon.ico`, `frontend/src/assets/images/logo-universal.png`, `docs/ss.png` | Replace with new MailPong artwork (M5) |
| Docs | README, CONTRIBUTING, SECURITY, `docs/*` (BUILD, RELEASE, EXTENSIONS 69, CASAT2 41, SQL_ROLLBACK 24, LANGUAGE, PRIVACY 20, TERMS 15), user guide (~25 files), `.github/ISSUE_TEMPLATE/*` | Rewrite or update (M10, M11) |
| History / attribution | `LICENSE` appendix (`Copyright 2024-2025 Aerion Contributors`), `CHANGELOG.md` (160 entries), `archive/**`, `docs/CASAT2.md` | Keep. Move the changelog and CASAT2 to `docs/history/` (M10) |
| Agent tooling | `AGENTS.md` "Fork Scope", `CONTRIBUTING.md` fork workflow, `.claude/skills/run-aerion/*` (data path `~/.local/share/aerion`, log names, `pgrep` pattern), `.claude/rules/extensions.md` | Rewrite/rename (M2, M3) |

### Data-safety findings (important)

1. **Today, the fork and upstream Aerion already share user data.** The fork
   changed only the desktop/Flatpak app ID. Both builds still use
   `~/.local/share/aerion` (and the macOS/Windows equivalents), `aerion.db`,
   and keyring service `aerion`. MailPong separates all of these.
2. **The fork deletes Aerion files.** `autostart_linux.go` removes
   `io.github.hkdb.Aerion.desktop` from autostart. The Makefile `uninstall`
   target and `build/linux/uninstall.sh` remove Aerion desktop entries and
   icons, including the upstream `io.github.hkdb.Aerion` IDs and the old
   `aerion.desktop`. **MailPong must not touch any Aerion-named file**, so all
   of this legacy cleanup is removed (M3, M6).
3. The Windows notification AUMID (`io.github.hkdb.Aerion`) and the macOS
   launch agent label (`io.github.hkdb.Aerion`) are still upstream's IDs, and
   the macOS bundle ID is the Wails default `com.wails.aerion`. These collide
   with upstream installs today.
4. Encryption keys come from a per-install random salt plus machine data plus
   the literal `aerion:` prefix (`internal/crypto/crypto.go`). New data
   directories already produce a new salt, so separation doesn't depend on
   this string. Keeping the string makes an optional manual import possible
   (D6).
5. Wails stores webview data (localStorage) by program name on Linux and by
   executable name on Windows. Renaming either gives MailPong a fresh webview
   profile.
6. Today's `make dev` uses the same data directory and keyring as installed
   builds (`CONTRIBUTING.md`). After M3, developers using MailPong never touch
   their Aerion data.

### Licensing findings

- `LICENSE` is Apache-2.0, with the appendix line `Copyright 2024-2025 Aerion
  Contributors`. `docs/user-guide/LICENSE` is also Apache-2.0. The repo has
  no `NOTICE` file, and upstream has none either
  (`raw.githubusercontent.com/hkdb/aerion/main/NOTICE` returns 404).
- **Bundled spellcheck dictionaries carry mixed licenses**
  (`npm view dictionary-* license`):

  | Package | License |
  |---|---|
  | en, en-gb, en-ca (to add) | MIT AND BSD |
  | cs, nb | GPL-2.0 |
  | de | GPL-2.0 OR GPL-3.0 |
  | it | GPL-3.0 |
  | fr | MPL-2.0 |
  | nl | BSD-3-Clause OR CC-BY-3.0 |

  `copy-dictionaries.mjs` gzips them into the built app. Removing the
  non-English dictionaries (M9) removes the GPL and MPL material from the
  distribution, and doing so needs no legal conclusion.
- Bundled font: `frontend/src/assets/fonts/nunito-v16-latin-regular.woff2`
  (Nunito, generally SIL OFL-1.1). The license still has to be confirmed and
  credited.
- The OAuth consent screens, `docs/PRIVACY.md`, `docs/TERMS.md` and
  `docs/CASAT2.md` belong to upstream's Google-verified OAuth app and name
  upstream's contact (`aerion@3df.io`) and site (`aerion.3df.io`). They don't
  apply to MailPong as written.

### Localization inventory

- Core: `frontend/src/lib/i18n/locales/` has `en` plus 10 other locales: cs,
  de, fr, it, nb, pl, vi, zh-CN, zh-HK, zh-TW.
- Contacts extension: `en` plus the same 10. Calendar extension: `en` plus 9
  (no `pl`).
- `i18n/index.ts` handles registrations, `supportedLocales` and
  `detectSystemLocale` (which maps zh variants). `dateFnsLocale.ts` loads one
  locale per language. The language picker is in
  `settings/GeneralTab.svelte`, and `ComposerTab.svelte` uses
  `supportedLocales` for dictionary names.
- Spellcheck: `spellcheck/locales.ts` (`SPELLCHECK_DICTS`, `DICT_NAMES`,
  `APP_TO_DICT`), `scripts/copy-dictionaries.mjs`, and the `dictionary-*`
  packages in `package.json`.
- Platform: translations in the desktop entry (`GenericName`, `Comment`,
  `Keywords`, Action `Name`) and the metainfo (`xml:lang`).
- Docs and workflow: `docs/LANGUAGE.md` and `.github/ISSUE_TEMPLATE/translation.yml`.
- Email content handling (MIME, charsets, IDN, RFC 2047/2231) lives in the Go
  backend (`internal/sync/parse.go`, `internal/smtp`, …) and doesn't depend on
  UI locale files. It stays unchanged.

## Decisions

Owner-confirmed decisions are marked (Owner). The others are recommended
defaults that are open to change at approval.

- **D1 Repository strategy (recommended):** create a new, standalone
  `caseylabs/mailpong` and push the full history of `main` to it. Don't
  transfer or detach the existing fork. See "Repository Migration Strategy".
- **D2 App ID (Owner):** `io.github.caseylabs.MailPong`.
- **D3 OAuth (Owner):** bring your own credentials. Release builds ship with
  no bundled OAuth clients. Users enter their own in Settings or use IMAP
  passwords and app passwords.
- **D4 en-CA (Owner):** spellcheck dictionary only. The UI stays a single
  English locale. Bundle en-US, en-GB and en-CA.
- **D5 Aerion data (Owner):** documented manual import only. MailPong starts
  with empty data directories and its own keyring service, and it never reads
  Aerion data on its own.
- **D6 Persisted internal identifiers:** keep the values `"aerion"` (contact
  source), `"aerion-shipped"`/`"aerion-mail"`, the `'aerion'` title-bar
  setting, `minAerionVersion`, and the `aerion:` KDF prefix. They are internal
  storage and API values, never shown to users. Renaming them would need DB
  migrations and would break the manual import. Each gets a one-line
  `// Legacy identifier from Aerion; persisted, do not rename.` comment and an
  entry in the brand-check allowlist.
- **D7 Artwork (Owner):** Claude creates an **original** master SVG for
  MailPong (not derived from Aerion's artwork), and the owner approves it
  before it is used everywhere.
- **D8 Copyright holder / publisher (Owner):** "MailPong Contributors". Keep
  "Aerion Contributors" for inherited work.
- **D9 Version (Owner):** continue at **0.4.0**. Mitigation for confusion
  with Aerion: always write "MailPong 0.4.0", and docs and the README explain
  that the version numbering continues from the Aerion code base it derives
  from.
- **D10 Changelog (Owner):** move it unchanged to
  `docs/history/AERION_CHANGELOG.md`. Start a new `CHANGELOG.md` for MailPong
  that links to it.
- **D11 Branding scope:** replace "Aerion" wherever it names *this* app.
  Keep it in attribution, history, `LICENSE`, `archive/**`, persisted
  identifiers (D6) and statements of origin ("derived from Aerion"). Never
  run a blind search-and-replace. Every milestone edits only the files it
  lists.
- **D12 Work location:** all migration commits go to the new repository on
  branch `mailpong-migration`, one commit per milestone, and land on its
  `main` per AGENTS.md "Landing a Branch" after M12. The old fork receives no
  commits.
- **D13 Upstream tracking:** keep any `upstream` remote
  (`https://github.com/hkdb/aerion.git`) in local clones. Make no
  fork-network changes. Upstream fixes can still be cherry-picked; renamed
  paths will make conflicts more likely, and the plan accepts that. Stopping
  upstream tracking is a separate, later decision.
- **D14 Platform identifiers:**

  | Platform / item | Identifier |
  |---|---|
  | Go module | `github.com/caseylabs/mailpong` |
  | Wails `name` / `outputfilename` | `mailpong` (binary `mailpong`, `mailpong.exe`, `MailPong.app`) |
  | Flatpak app ID | `io.github.caseylabs.MailPong` |
  | Desktop entry, icon name, metainfo | `io.github.caseylabs.MailPong.desktop`, `…MailPong.png/.svg`, `io.github.caseylabs.MailPong.metainfo.xml` |
  | Linux `ProgramName` / WM class | `io.github.caseylabs.MailPong` (confirm `StartupWMClass` at runtime) |
  | macOS bundle ID | `io.github.caseylabs.MailPong`, and URL type name `io.github.caseylabs.MailPong.mailto` |
  | macOS launch agent label | `io.github.caseylabs.MailPong` |
  | Windows AUMID | `io.github.caseylabs.MailPong` |
  | Windows installer | product "MailPong", publisher "MailPong Contributors", `UNINST_KEY_NAME` "MailPong", install dir `$PROGRAMFILES64\MailPong` |
  | Windows autostart registry value | `MailPong` |
  | Data dirs: Linux | `~/.config/mailpong`, `~/.local/share/mailpong`, `~/.cache/mailpong`, DB `mailpong.db` |
  | Data dirs: macOS | `~/Library/Application Support/MailPong`, `~/Library/Caches/MailPong` |
  | Data dirs: Windows | `%APPDATA%\MailPong`, `%LOCALAPPDATA%\MailPong\Cache` |
  | Data dirs: Flatpak | `~/.var/app/io.github.caseylabs.MailPong/` |
  | Keyring service | `mailpong` |
  | IPC | `$XDG_RUNTIME_DIR/mailpong` (fallback `/tmp/mailpong-<uid>`), `\\.\pipe\mailpong-<user>` |
  | Debug env var | `MAILPONG_DEBUG=1` |
  | Creds shim / build tag | `cmd/mailpong-creds`, `/app/lib/mailpong/mailpong-creds`, `mailpong_creds` |
  | Release artifacts | `mailpong-vX.Y.Z-linux-{x86_64,aarch64}`, `mailpong-linux-{amd64,arm64}.tar.gz`, `MailPong-darwin-{amd64,arm64}.zip`, `MailPong-windows-{amd64,arm64}.exe`, `MailPong-windows-setup-*.exe`, `MailPong-vX.Y.Z-*.flatpak` |
  | Outgoing headers | `User-Agent: MailPong`, `Reporting-UA: MailPong/<version>`, Message-ID fallback domain `mailpong`, `PRODID:-//MailPong//Calendar//EN` |

- **D15 Legal documents:** keep `LICENSE` byte-identical. Add a `NOTICE`
  file, `THIRD_PARTY_NOTICES.md` and `DISCLAIMER.md`. Rewrite `TERMS.md` and
  `PRIVACY.md` for MailPong, without replacing or weakening Apache-2.0 §7–8.
  Every legal text is drafted for human review and is not legal advice.

## Repository Migration Strategy

### Options compared

| Option | History | Old repo intact | Standalone (not a fork) | Reversible | Notes |
|---|---|---|---|---|---|
| **A. New repo + push history** (recommended) | Full, unchanged SHAs and authors | Yes | Yes | Yes (delete the new repo) | Issues, PRs, stars and settings don't carry over. The fork has 0 issues and 1 closed PR, so nothing is lost |
| B. Transfer the fork to `CaseyLabs` and rename it | Full | **No** (the org loses it, and redirects go to the new location) | **No**, it stays in `hkdb/aerion`'s fork network | Partly | The org→user owner change needs admin on both. The repo still shows "forked from" |
| C. "Leave fork network" (Settings → Danger Zone), then transfer and rename | Full | **No** | Yes | **No**, it is permanent ([GitHub docs](https://docs.github.com/en/pull-requests/how-tos/work-with-forks/detaching-a-fork)) | Drops issues, PRs, stars and watchers. Changes the original repo irreversibly |
| D. Fresh repo with squashed or rewritten history | Lost or rewritten | Yes | Yes | Yes | Breaks the provenance and attribution of upstream commits. No compelling reason |

**Recommendation: A.** It is the only option that leaves
`clabs-public-forks/aerion` untouched, needs no permission on the org side
beyond read, keeps every commit SHA and author (hkdb's and yours), and can be
undone. GitHub won't show "forked from hkdb/aerion" on the new repo, so
attribution goes in the README, `NOTICE` and the GitHub description instead.

### Execution order (M1 has the exact commands)

1. Pre-flight (M0): the owner's local checkout has no unpushed commits or
   needed stashes, and `origin/main` is the agreed source.
2. Create `caseylabs/mailpong` as **private** and empty (no README, license or
   .gitignore). Use the GitHub connector `create_repository` (it creates
   under the authenticated user `CaseyLabs`) or the GitHub UI.
3. In a **new, full clone** (never this shallow checkout), push `main` only.
   Push **no tags**: upstream `v*` tags would fire `release.yml` and suggest
   that Aerion releases are MailPong releases.
4. Before the first push, disable the **Release** workflow in the new repo
   (Actions → Release → Disable workflow), or push `main` before Actions runs
   anything tag-based. CI (`ci.yml`) may run.
5. Set the remotes in the new clone: `origin` → `caseylabs/mailpong`,
   `aerion-fork` → `clabs-public-forks/aerion` (read-only by convention), and
   `upstream` → `hkdb/aerion`. In the owner's existing checkout, *add*
   `mailpong` as a remote; don't change `origin`. Retire the old checkout only
   after M13.
6. Do the rebranding on `mailpong-migration` in the new repo (M2–M12).
7. Make the repo public and configure it (M13).

Metadata that doesn't migrate, and what happens to it:

| Item | Action |
|---|---|
| Issues and PRs | None to move (0 issues; PR #1 is closed). `ISSUES.md` moves with the code |
| Releases and tags | No releases. The fork's 52 tags are inherited upstream tags: don't push them |
| Stars and watchers | Not transferable (1 star) |
| Branch protection and rulesets | Recreate in M13 |
| Actions secrets | None needed (D3). Only `GITHUB_TOKEN` is used |
| Labels and issue templates | Templates come with the code. Labels are created in M13 if needed |
| Topics, description, homepage | Set in M13 |
| Security settings (private vulnerability reporting, secret scanning, push protection, Dependabot alerts) | Enable in M13 |

### GitHub actions needing owner approval

- Create `caseylabs/mailpong` (private) and push `main` (M1).
- Disable the Release workflow until M13 (M1).
- Give later cloud sessions access to `caseylabs/mailpong` (environment / GitHub
  App installation on the `CaseyLabs` account) (M1).
- Change visibility to public; add description, topics, rulesets and security
  features; enable the Release workflow (M13).
- Any change at all to `clabs-public-forks/aerion` (M14, deferred).

## Milestones

Conventions for every milestone:

- Work in the `caseylabs/mailpong` clone on branch `mailpong-migration`,
  leaving unrelated user changes alone. Commit one milestone per commit with
  `feat:`, `fix:` or `docs:` prefixes, and update `PLAN.md` in the same
  commit.
- Finish each milestone with `/project-finish-milestone`. It runs the checks,
  `/code-review`, the acceptance checker and `/simplify`.
- "Standard checks" means:
  ```sh
  cd frontend && npm ci && npm run build && npm run lint && npm run check && cd ..
  make fmt && git diff --exit-code   # gofmt clean
  make lint
  make test
  ```
- The **UI visual check** uses the `run-mailpong` skill (`run-aerion` until
  M2; see its [Accounts column check](.claude/skills/run-mailpong/SKILL.md#accounts-column-check)). Check the affected screens with the Mail **Accounts column expanded and
  collapsed**, covering layout, interaction and responsiveness in each state.
  When no display is available (cloud), mark it **not performed**, never
  passed.
- **Brand check** (stale names). M12 makes this a script. Until then:
  ```sh
  git grep -n -I -i 'aerion' -- . ':!archive' ':!docs/history' ':!LICENSE' ':!NOTICE' \
    ':!THIRD_PARTY_NOTICES.md' ':!PLAN.md' ':!frontend/package-lock.json' \
    | grep -v -E 'Legacy identifier from Aerion|hkdb/aerion|derived from|Built on the open-source Aerion'
  ```
  Each milestone states which matches may remain afterwards.

---

### M0. Pre-flight and baseline (no code changes)

Goal: establish a clean starting point and a baseline for every later check.

Steps:
1. The owner runs these in their local checkout and reports the results:
   `git status`, `git stash list`, `git log --oneline origin/main..HEAD`,
   `git branch -vv`, `git remote -v`. Push or merge anything that should
   migrate into `clabs-public-forks/aerion` `main` first (normal workflow),
   or record that it stays behind.
2. Record the baseline on `origin/main` (`1c0445d` or newer): standard checks,
   `make build` on Linux, and brand-check counts (2,185 lines / 425 files).
3. Record which Aerion installs exist on the owner's machines (upstream,
   fork, Flatpak, or none). The owner records this; the agent doesn't
   inspect user data.

Acceptance criteria:
- The source commit for the migration is named, and the owner confirms
  nothing unpushed is lost.
- Baseline results are written to Surprises & Discoveries, including any
  check that already fails.

Validation: the commands above. Their output goes into the Progress entry.

### M1. Create `caseylabs/mailpong` and move remotes — **Approval gate**

Goal: a private, standalone copy of the full history, with the original
untouched.

Steps (in a directory outside the existing checkout):
```sh
git clone https://github.com/clabs-public-forks/aerion.git mailpong   # full, not shallow
cd mailpong
git rev-parse --is-shallow-repository          # expect: false
git rev-list --count main                      # expect: 369 or more (M0 baseline)
git remote rename origin aerion-fork
git remote add upstream https://github.com/hkdb/aerion.git
git remote add origin https://github.com/caseylabs/mailpong.git
# create the empty private repo first (connector create_repository: name=mailpong,
# private=true, autoInit=false, description=<GitHub description>), and
# disable the Release workflow as soon as the Actions tab exists
git push -u origin main                        # pushes main only, no tags
git switch -c mailpong-migration
# commit PLAN.md (this plan, with M0/M1 ticked) as the first migration commit
git push -u origin mailpong-migration
```
In the owner's existing checkout, run only
`git remote add mailpong https://github.com/caseylabs/mailpong.git`.

Acceptance criteria:
- `caseylabs/mailpong` exists, is private, is not a fork, has `main`, and has
  no tags or releases.
- `git rev-parse origin/main` in the new clone equals
  `git rev-parse aerion-fork/main`. Commit count and author list match the
  fork (`git log --format='%an' | sort | uniq -c`).
- `clabs-public-forks/aerion` is unchanged: same `main` SHA, settings,
  visibility, and still a fork (checked with the connector before and
  after).
- The Release workflow is disabled, and no workflow run was triggered by a
  tag.
- The `upstream` remote exists in the new clone. No fork-network change was
  made.

Rollback: delete `caseylabs/mailpong` (owner approval required). The
original is unaffected.

### M2. Agent instructions and dev tooling for MailPong

Goal: later milestones run under correct, non-fork guidance.

Changes:
- `AGENTS.md`: replace "Fork Scope" with "Project Identity". MailPong is an
  independent project derived from Aerion. Point to D11 for the branding and
  attribution rules and to the D6 allowlist. Drop "keep customizations
  focused to ease future upstream merges" and replace it with a short note
  that upstream fixes may be cherry-picked from the `upstream` remote. Add
  the persistent rule: *"Visual UI checks always cover the Mail view with the
  Accounts column expanded and collapsed. Report checks you couldn't
  perform."* Update the Localization bullet (English-only UI; the en-US,
  en-GB and en-CA dictionaries; email content stays fully international).
  Keep the commands, style, testing, milestone, landing and compaction rules.
- `CLAUDE.md`: keep it as a thin `@AGENTS.md` include plus the skill pointers.
  Don't duplicate rules.
- `.claude/skills/run-aerion/` → `.claude/skills/run-mailpong/` with
  `git mv`. Update `name:`, description, paths, `dev.sh` log and SID names
  (`mailpong-dev.log`, `MAILPONG_DEV_LOG`), and the `pgrep` pattern. The
  data-directory path and `MAILPONG_DEBUG` change in M3. Keep the existing
  Accounts-column section and link to it from the UI acceptance criteria.
- `.claude/rules/working-style.md`, `.claude/rules/extensions.md` and
  `.claude/agents/*`: rename Aerion where it names the app. Keep the
  workflows.
- `REVIEW.md`: rename where needed.
- Update references to `run-aerion` everywhere (`git grep run-aerion`).

Acceptance criteria:
- `git grep -n 'run-aerion'` returns nothing. The skill is listed as
  `run-mailpong`, and `.claude/skills/run-mailpong/dev.sh status` runs.
- AGENTS.md no longer calls the project a personal or downstream fork, and
  doesn't ask for upstream-merge compatibility.
- The Accounts-column rule appears once in AGENTS.md and once, in detail, in
  the skill. Nothing else duplicates it.
- Project-specific progress stays out of AGENTS.md.

Validation: `git grep -n -i 'fork' AGENTS.md CLAUDE.md .claude`, and run
`sh .claude/skills/project-next-milestone/plan-outline.sh` to confirm the
skills still parse.

### M3. Go module path and backend application identity

Goal: MailPong's backend has its own identity, and nothing in it can touch
Aerion files, data or keyring entries.

Changes:
1. Module path: in `go.mod`, change it to `github.com/caseylabs/mailpong` and
   rewrite imports mechanically
   (`go mod edit -module …` and `gofmt -r` / `sed` limited to `*.go`). Then
   `go build ./... && go vet ./...`.
2. Identity constants and strings per D14: `platform/paths.go` (`appName`,
   the macOS and Windows folder names, `mailpong.db`), `keyring.go` and
   `credentials/store.go` (`serviceName`, the test key name),
   `socketdir_unix.go`, `singleinstance_{darwin,windows}.go`,
   `ipc/server_windows.go`, `autostart_{linux,darwin,windows}.go`,
   `notifier_{windows.go,linux.go,darwin.m}`, `network_darwin.go`,
   `sleep_windows.go`, `filechooser_linux.go` handle tokens, `gbmcheck_linux.go`,
   `main.go` (`ProgramName`, window title, `MAILPONG_DEBUG`),
   `app/background.go` notifier name, `app/state.go` (name and website
   `https://github.com/caseylabs/mailpong`), `app/app.go` startup dialogs
   and `docsRollbackURL`, and the `internal/database/database.go` error URL.
3. **Remove all legacy Aerion cleanup.** In `autostart_linux.go`, drop
   `legacyAutostartFilename` and every read or delete of it.
4. Protocol identity: SMTP `User-Agent`, MDN `Reporting-UA`, Message-ID
   fallback domain, calendar `PRODID` and new-event UID suffixes, and the
   OAuth callback HTML.
5. OAuth (D3): rename `cmd/aerion-creds` → `cmd/mailpong-creds`, build tag
   `mailpong_creds`, the Flatpak creds path, `build/gencreds.go`, and picker
   labels "MailPong – Google/Microsoft". The `aerion-*` choice IDs stay
   (D6). Confirm that a build with no credentials shows only the Custom
   option and the "OAuth missing" guidance.
6. D6 comments on every kept identifier.
7. Update test fixtures and expectations that embed app names. Add
   table-driven tests:
   - `GetPaths` on each OS branch returns MailPong paths and never contains
     `aerion`.
   - The Linux autostart file name is `io.github.caseylabs.MailPong.desktop`,
     and enable/disable doesn't touch other files in a temp autostart dir
     seeded with `io.github.hkdb.Aerion.desktop` and
     `io.github.clabs_public_forks.Aerion.desktop`.
   - The keyring service name is `mailpong`.
8. In the `run-mailpong` skill and `CONTRIBUTING.md`, update the data path
   (`~/.local/share/mailpong/mailpong.db`) and `MAILPONG_DEBUG`.

Acceptance criteria:
- Standard checks pass. `go list -m` prints `github.com/caseylabs/mailpong`,
  and `git grep 'hkdb/aerion' -- '*.go' go.mod` is empty.
- Brand check: in `*.go`/`*.m`, only D6 legacy identifiers (with their
  comment) and attribution comments remain.
- New tests pass and fail when the constants are reverted. Prove this with a
  quick local revert.
- No code path reads, writes or deletes paths or keyring entries containing
  `aerion`/`Aerion`, apart from the `aerion:` KDF string, which is
  in-memory only.

Validation: standard checks,
`go test ./internal/platform/... ./internal/keyring/... ./internal/credentials/... ./internal/smtp/... ./extensions/calendar/...`,
and the brand check limited to Go.

### M4. Frontend, Wails metadata and UI branding

Goal: the app presents itself as MailPong everywhere in the UI.

Changes:
- `wails.json`: `name`/`outputfilename` `mailpong`, `companyName` and
  `author.name` "MailPong Contributors", `author.email` empty (O5), `productName` "MailPong", `productVersion` `0.4.0`, `copyright`
  "Copyright 2024-2025 Aerion Contributors; Copyright 2026 MailPong
  Contributors", `comments` "Email, reimagined as chat."
- `frontend/package.json` `name`/`version`. Regenerate the lockfile with
  `npm install --package-lock-only` (name and version only, no upgrades).
- `index.html` title and boot splash, `TitleBar.svelte`,
  `WhatsNewDialog.svelte` (replace the Aerion 0.3.5 notes with MailPong
  0.4.0 notes and link the new CHANGELOG), `AboutTab.svelte` (MailPong name,
  tagline, alpha status, and "Built on Aerion by @hkdb" with links),
  `TermsDialog.svelte` and `AboutTab.svelte` URLs → `caseylabs/mailpong`
  docs, `OAuthMissingDialog.svelte` URL → the MailPong docs path.
- English strings: 18 in `en.json` and 6 in extension `en.json` files. Keep
  the i18n keys; change only the values. The `settingsGeneral.titleBarAerion`
  key stays (it is a key, not shown to users) and its label becomes
  "MailPong title bar". Non-English files aren't edited because M9 deletes
  them.
- Rename DnD MIME types to `application/x-mailpong-*` and localStorage keys
  to `mailpong:calendar:*`. The webview profile is new (finding 5), so no
  migration is needed.
- `AerionCoreOAuthSection.svelte` → `CoreOAuthSection.svelte` with
  `git mv` (the component name is shown to developers only), and update the
  imports.
- `build/darwin/Info.plist` and `Info.dev.plist`: hard-code
  `CFBundleIdentifier` `io.github.caseylabs.MailPong` and the URL name, so
  the template no longer falls back to `com.wails.*`.
- Regenerate bindings (`make generate` after `dev.sh stop`), then revert
  unrelated churn in `frontend/wailsjs/runtime/*`.

Acceptance criteria:
- Standard checks pass. `make build` produces `build/bin/mailpong`.
- The brand check over `frontend/` and `extensions/*/frontend` shows only
  non-English locale files (deleted in M9) and D6 values.
- **UI visual check:** with the Accounts column expanded and with it
  collapsed, the window title, custom title bar, boot splash, About, Terms
  dialog, What's New and the OAuth-missing dialog all show MailPong. Mail
  list, chat view, Contacts and Calendar render as before.
- About credits Aerion and @hkdb with working links.

Validation: standard checks, `make build`, `run-mailpong` (start, then
screenshots of both column states, then stop), and
`grep -c '"name": "mailpong"' frontend/package.json`.

### M5. MailPong artwork — **Approval gate (artwork)**

Goal: original MailPong artwork replaces Aerion's.

Steps:
1. Draft `brand/mailpong.svg`, a mark that works in light and dark themes
   and stays legible at 16 px (done as the owner's blue envelope icon, redrawn
   as a vector; no separate glyph SVG). Show the owner a preview page or PNG before anything else.
2. After approval, generate the assets from the SVG with a committed script
   `tools/brand/render-icons.sh`. It uses ImageMagick `convert`, which this
   container already has; if a platform lacks it, `rsvg-convert` is an
   alternative. Outputs: `build/appicon.png` (1024), `build/linux/icons/{32,48,64,128,256}x…/io.github.caseylabs.MailPong.png`,
   `build/linux/icons/scalable/io.github.caseylabs.MailPong.svg`,
   `build/linux/mailpong.png` (256), `build/windows/icon.ico`
   (16–256 multi-size), and `frontend/src/assets/images/logo-universal.png`.
   Wails derives the macOS `.icns` from `build/appicon.png`.
3. Remove the Aerion artwork from the tree (`brand/icon*.png`,
   `brand/glyph-icon.*`, the old Linux icon names). Git history keeps it.
4. Refresh `docs/ss.png` with fake data only, following the skill's
   screenshot rules (`screenshot-fakes.js`).

Acceptance criteria:
- The owner has approved the master SVG.
- Every icon size listed above exists, comes from the SVG, and none is
  derived from Aerion artwork. `git grep -l -i 'aerion' -- '*.svg'` is empty.
- **UI visual check:** the logo and window icon in both Accounts-column
  states, light and dark themes.
- The `docs/ss.png` screenshot shows no real mail data.

Validation: `identify build/windows/icon.ico` (sizes), `file build/linux/icons/*/*`,
`make build`, and the visual check.

### M6. Linux desktop entry, AppStream and Flatpak

Goal: Linux packaging installs MailPong under its own ID without touching
Aerion.

Changes:
- `build/linux/aerion.desktop` → `build/linux/io.github.caseylabs.MailPong.desktop`
  (`git mv`): `Name=MailPong`, `Comment=Email, reimagined as chat.`,
  `Icon=io.github.caseylabs.MailPong`, `Exec=mailpong %U`,
  `TryExec=mailpong`, `StartupWMClass` set to the runtime value confirmed in
  M4, all `[xx]` translations removed, and **`X-Flatpak-RenamedFrom`
  removed** (MailPong is a new app, not a rename of Aerion).
- Metainfo → `build/flatpak/io.github.caseylabs.MailPong.metainfo.xml`: id,
  name, summary (the tagline), English description (chat view, alpha
  notice, "derived from Aerion by hkdb"), `<developer id="io.github.caseylabs">`
  with name "MailPong Contributors", URLs (homepage, bugtracker, vcs-browser,
  contribute → `caseylabs/mailpong`), `metadata_license` CC0-1.0,
  `project_license` Apache-2.0, a `<release version="0.4.0">` entry
  (date set at release), screenshot URLs on `raw.githubusercontent.com/caseylabs/mailpong/main/…`,
  `launchable`, `provides/binary mailpong`, `content_rating`, and no
  `xml:lang`.
- Flatpak manifests →
  `build/flatpak/io.github.caseylabs.MailPong-dev.yml`,
  `build/flatpak/flathub/io.github.caseylabs.MailPong.yml` and
  `…-source.yml`: `app-id`, `command: mailpong`, module name, install paths,
  source URL `https://github.com/caseylabs/mailpong.git`. Drop the creds-shim
  sources by default (D3) but keep the optional mechanism documented.
  Update `calculate-hashes.sh`, `release.sh`, `build-*.sh`, `test-build.sh`,
  `Dockerfile` and `build/flatpak/README.md`, and the
  `build/flatpak/flathub/README.md` notes (Flathub submission is deferred).
- `Makefile` `install-linux`/`uninstall-linux`, `build/linux/install.sh` and
  `uninstall.sh`: rename everything. **Delete every removal of
  `aerion.desktop`, `aerion.png`, `io.github.hkdb.Aerion*` and
  `io.github.clabs_public_forks.Aerion*`.** Also update `.gitignore` entries.

Acceptance criteria:
- `desktop-file-validate build/linux/io.github.caseylabs.MailPong.desktop`
  passes.
- `appstreamcli validate --pedantic build/flatpak/io.github.caseylabs.MailPong.metainfo.xml`
  passes, or with only documented, accepted warnings. Alternatively use
  `flatpak run --command=flatpak-builder-lint org.flatpak.Builder appstream …`.
- `flatpak-builder --user --install --force-clean build-dir build/flatpak/io.github.caseylabs.MailPong-dev.yml`
  builds, and `flatpak run io.github.caseylabs.MailPong` starts with data
  under `~/.var/app/io.github.caseylabs.MailPong/`.
- `make install-linux PREFIX=$tmp DESTDIR=` followed by
  `make uninstall-linux PREFIX=$tmp` leaves a seeded set of Aerion files in
  `$tmp` byte-identical (sha256 manifest before and after). Run the same
  check with `install.sh`/`uninstall.sh` against a sandbox `$HOME`.
- No non-English text remains in the desktop entry or metainfo.

Validation: the commands above. Flatpak and AppStream tools may be missing in
cloud sessions. Install them only with approval, or have the owner run them
on a Linux desktop and record the result.

### M7. macOS and Windows packaging identity

Goal: macOS and Windows builds install as MailPong next to any Aerion
install.

Changes:
- macOS: the M4 Info.plist work, plus `Makefile` `build`/`install-darwin`/
  `uninstall-darwin` (`MailPong.app`, ad-hoc codesign path). Confirm
  notifications register under the new bundle ID.
- Windows: `build/windows/installer/project.nsi` defines
  `UNINST_KEY_NAME "MailPong"` and the install directory
  `$PROGRAMFILES64\MailPong`. `info.json` takes its values from wails.json.
  Keep `wails.exe.manifest`. In `Makefile` `build-windows-installer`, rename
  the output to `mailpong-<arch>-installer.exe`.
- Leave code signing and notarization out. Document them as future work.

Acceptance criteria:
- macOS (CI `macos-latest` or the owner's Mac): `make build` produces
  `build/bin/MailPong.app`.
  `defaults read build/bin/MailPong.app/Contents/Info CFBundleIdentifier` prints
  `io.github.caseylabs.MailPong`, and the app launches next to an installed
  Aerion.app without sharing `~/Library/Application Support/Aerion`.
- Windows (CI `windows-latest` or the owner's PC): the installer and
  portable exe build. The installed uninstall key is
  `HKLM\…\Uninstall\MailPong`, the install directory is
  `Program Files\MailPong`, and installing or uninstalling MailPong leaves
  `%APPDATA%\Aerion` and Aerion's uninstall entry untouched.
- File properties show ProductName MailPong, CompanyName MailPong
  Contributors, and version 0.4.0.

Validation: the M8 `workflow_dispatch` dry-run builds, then manual checks by
the owner on real macOS/Windows. Record each as performed or not performed.

### M8. CI/CD and release workflow

Goal: workflows build MailPong artifacts from this repository and can't
publish by accident.

Changes to `.github/workflows/release.yml`:
- Apply the D14 artifact names. Use `MailPong-*`/`mailpong-*` upload
  globs.
- Drop the `GOOGLE_*`/`MICROSOFT_*` secret env (D3). Builds without
  credentials must still succeed; the Makefile already only warns.
- Remove the creds-shim build and its upload, unless D3 changes.
- Flatpak job: build from this checkout's manifest. **Remove the job that
  commits to and pushes `main`.** Manifest updates go through PRs.
- Releases are created as **draft prereleases**. Nothing is published
  automatically.
- Add `workflow_dispatch` with `dry_run: true` (the default). It builds every
  matrix entry and uploads workflow artifacts only, with no release.
- Set least-privilege `permissions:` (`contents: read` by default, `write`
  only for the release job).
- `ci.yml`: rename only where it names the app. Behavior is unchanged.
- `docs/RELEASE.md`: rewrite the release steps for MailPong (tag `v0.4.0`,
  review the draft, publish manually).

Acceptance criteria:
- `actionlint` passes on both workflows if available. Otherwise
  `python3 -c 'import yaml,sys;[yaml.safe_load(open(f)) for f in sys.argv[1:]]' .github/workflows/*.yml`
  passes and the owner reviews the diff.
- `git grep -n -i 'aerion\|hkdb\|clabs' .github/` returns only attribution
  in issue templates, if any.
- A `workflow_dispatch` dry run on `mailpong-migration` (private repo)
  finishes green and produces each artifact with its D14 name. This needs
  the owner to approve enabling the Release workflow for dispatch only.
- No workflow step pushes to a branch or creates a published release.

Validation: the dry-run link and artifact list go into Progress.

### M9. English-only UI with en-US, en-GB and en-CA spellcheck

Goal: the UI ships in English only and keeps the i18n architecture.
International email content is unaffected.

Changes:
- Delete the non-English locale JSONs: 10 core, 10 contacts, 9 calendar.
- `i18n/index.ts`: register only `en`, set `supportedLocales` to
  `[{ code: 'en', name: 'English' }]`, and make `detectSystemLocale()` return
  `'en'`. Keep `register`/`init`/`fallbackLocale` and the extension glob.
  A saved non-English locale (from an imported Aerion DB) falls back to
  `en` without error.
- Extension `i18n/index.ts`: register `en` only.
- `dateFnsLocale.ts`: remove the non-English cases and keep the function and
  its API (`en` returns `undefined` as before).
- `GeneralTab.svelte`: hide the language picker when
  `supportedLocales.length < 2`. Don't delete the component logic.
- Spellcheck: remove `dictionary-{cs,de,fr,it,nb,nl}` from
  `package.json`, add `dictionary-en-ca` (MIT AND BSD, v3.0.0 at planning
  time), and set `LOCALES`/`SPELLCHECK_DICTS` to `['en','en-gb','en-ca']`.
  `DICT_NAMES`: `en` "English (US)", `en-gb` "English (UK)", `en-ca` "English
  (Canada)". `appLocaleToDict` maps system `en-CA`/`en-GB` to their
  dictionaries when the composer uses the system language. Saved
  dictionary keys that no longer exist are dropped without error.
- Desktop and metainfo translations: done in M6.
- Remove `.github/ISSUE_TEMPLATE/translation.yml`. Reduce `docs/LANGUAGE.md`
  to a short "Localization architecture" note (English-only UI, how
  strings and keys work, how a locale could be added later), keeping the
  key-convention sections. Remove translation sections from CONTRIBUTING
  and the README.
- Leave the Go email parsing and encoding code untouched.

Acceptance criteria:
- `ls frontend/src/lib/i18n/locales extensions/*/frontend/i18n/locales` shows
  only `en.json` files.
- `cd frontend && npm ls dictionary-en dictionary-en-gb dictionary-en-ca`
  shows exactly those three, and no other `dictionary-*` remains.
- After `npm run build`, `frontend/public/spellcheck/` contains only
  `en`, `en-gb` and `en-ca` `.aff.gz`/`.dic.gz`.
- Standard checks pass. `svelte-check` reports no missing-module errors.
- **UI visual check:** in both Accounts-column states, Settings → General
  shows no language picker, Settings → Composer lists English (US), English
  (UK) and English (Canada), and the composer flags "color" with en-GB/en-CA
  and accepts "colour".
- **Unicode check:** Go tests covering MIME and charset parsing pass
  (`go test ./internal/sync/... ./internal/smtp/... ./internal/message/...`,
  or whichever packages hold the parsing tests). A fixture message with
  multilingual subject, sender and body (CJK, accents, RTL, emoji; RFC 2047
  encoded words; ISO-8859-1 and Shift_JIS parts) shows correctly in the chat
  view. Add a Go test if no existing test covers mixed charsets.

Validation: the commands above, `run-mailpong`, and the fixture import (local
test server, or a `.eml` file if the app supports import).

### M10. Licensing, attribution and legal documents

Goal: Apache-2.0 compliance and clear, honest attribution, with every legal
text flagged for human review.

Changes:
- `LICENSE`: **unchanged**.
- New `NOTICE` (plain text, short): "MailPong — Copyright 2026 MailPong
  Contributors. This product is derived from Aerion
  (https://github.com/hkdb/aerion), Copyright 2024-2025 Aerion Contributors,
  created by hkdb, licensed under the Apache License 2.0. MailPong modifies
  and extends Aerion. It is independently maintained and is not an official
  Aerion release." Plus a pointer to `THIRD_PARTY_NOTICES.md`.
- New `THIRD_PARTY_NOTICES.md`: the spellcheck dictionaries (SCOWL / Hunspell
  en_US, en_GB, en_CA, with their license texts from the packages), the
  Nunito font (confirm OFL-1.1, include its license), and a generated
  inventory of Go and npm runtime dependency licenses. Generate the
  inventory **ad hoc** with `go run github.com/google/go-licenses@<pinned> report ./...`
  and `npx license-checker-rseidelsohn@<pinned> --production --summary`, and
  add neither tool to the project. Include each upstream `NOTICE` file that
  Apache-2.0 §4(d) requires us to propagate.
- Modification notice, Apache-2.0 §4(b): "Modified by MailPong
  Contributors" statements in `NOTICE` and the README, plus the public Git
  history. **Legal review flag L1.**
- `DISCLAIMER.md` (linked near the top of the README): alpha status, no
  guaranteed support or response times, use at your own risk, back up your
  mail, OAuth and third-party provider terms are the user's responsibility,
  a reminder of the warranty disclaimer and limitation of liability with
  pointers to Apache-2.0 §7 and §8 (it restates them and doesn't replace
  them), and no affiliation with or endorsement by Aerion or hkdb.
- `docs/TERMS.md` and `docs/PRIVACY.md`: rewrite for MailPong. Same structure:
  local-only app, no telemetry, data stored locally under the D14 paths.
  Remove upstream's contacts (`aerion@3df.io`, `aerion.3df.io`, hkdb issue
  tracker) and use the MailPong GitHub issues and private vulnerability
  reporting instead. Reword the Google/Microsoft API sections for the
  bring-your-own-credentials model. **Legal review flags L2 and L3.**
- `SECURITY.md`: MailPong policy. Use GitHub private vulnerability reporting
  (enabled in M13), state no response-time guarantee during alpha, keep the
  user best practices, and point upstream issues to upstream's policy.
- Move `docs/CASAT2.md` to `docs/history/AERION_CASA_TIER2.md` with a
  one-line header saying it describes upstream's OAuth app and doesn't apply
  to MailPong.
- `CHANGELOG.md` → `docs/history/AERION_CHANGELOG.md` (`git mv`, contents
  unchanged). New `CHANGELOG.md`: a "MailPong 0.4.0 (unreleased)" section
  summarizing the fork's features and the migration, and a link to the Aerion
  history.

Acceptance criteria:
- `git diff <M1 base> -- LICENSE` is empty.
- `NOTICE`, `THIRD_PARTY_NOTICES.md` and `DISCLAIMER.md` exist. The README
  links all three, and the release tarball, `.app` and Windows installer
  ship `LICENSE` and `NOTICE` (checked in the M8 dry-run artifacts; add copy
  steps if they're missing).
- No document names upstream's email or website as MailPong's contact.
- Every legal flag (L1–L4) is listed under Outstanding Decisions with the
  exact text for the owner, or counsel, to review.

Validation: `git grep -n '3df.io'` returns only `docs/history/`, run the
brand check, and the owner reads the legal texts.

### M11. Documentation and GitHub templates

Goal: every doc describes MailPong correctly.

Changes:
- `README.md`: MailPong logo, name, tagline and alpha badge. A prominent
  **"Built on Aerion"** section near the top: "MailPong is derived from
  [Aerion](https://github.com/hkdb/aerion), the open-source email client
  created by [@hkdb](https://github.com/hkdb). MailPong is independently
  maintained and is not an official Aerion release; the Aerion project does
  not endorse or support it." Then features, installation (build from
  source until the first release), documentation, the disclaimer link,
  license and notices, and contributing.
- `CONTRIBUTING.md`: MailPong workflow, clone URL `caseylabs/mailpong`, new
  data paths, and a note on cherry-picking upstream fixes. Remove the fork
  language.
- `docs/BUILD.md`, `docs/RELEASE.md` (from M8), `docs/SQL_ROLLBACK.md`
  (paths), `docs/EXTENSIONS.md` (grep it section by section, since it's
  long), `docs/EXT_RULES.md`, `docs/KEYBOARD_SHORTCUTS.md`.
- User guide (`docs/user-guide/**`): rename the app. Rewrite installation
  pages to drop upstream release download links and describe
  build-from-source, Flatpak (local build) and the data paths. In the OAuth
  pages, describe bring-your-own credentials. Update FAQ and
  troubleshooting paths and IDs.
- **Manual Aerion data import (D5):** a new
  `docs/user-guide/getting-started/import-from-aerion.md`. It is opt-in and
  manual:
  1. Quit both apps.
  2. Back up the Aerion data directories as a dated archive.
  3. Copy (never move) the Aerion data directory contents into an empty
     MailPong data directory and rename `aerion.db` to `mailpong.db`.
  4. Start MailPong, then re-enter passwords or re-authorize OAuth accounts
     (keyring entries under service `aerion` aren't copied).
  5. Roll back by quitting MailPong and deleting the MailPong directories.
     Aerion is untouched.

  Include the per-OS paths and Flatpak paths. Warn against running this
  while Aerion is open, and against reusing Aerion's keyring secrets.
  **Test the procedure in M12 with fixture data only.**
- `.github/ISSUE_TEMPLATE/*`: MailPong names and links, an "is this an
  upstream Aerion issue?" pointer, and `config.yml` with a security contact
  link.

Acceptance criteria:
- The brand check over `*.md` and `.github/` shows only attribution, history
  (`docs/history/`) and the import guide's references to Aerion paths.
- Check internal Markdown links with a small `grep -o '](…)'` loop or the
  owner's preferred link checker, run ad hoc without adding a dependency.
- The README attribution section appears before "Features" and links both
  `https://github.com/hkdb/aerion` and `https://github.com/hkdb`.

Validation: brand check, link check, and the owner reads the README.

### M12. Full validation, data-safety proof and brand audit

Goal: prove the migration is complete and safe before going public.

Steps:
1. Commit `tools/brand-check.sh`, which runs the brand-check grep with a
   committed allowlist file (`tools/brand-check.allow`, one path or pattern
   per line, each with a reason). Add it to `make lint`.
2. Standard checks, `make build`, and a Flatpak dev build (M6).
3. **Data-safety test (Linux, sandboxed):** `HOME=$(mktemp -d)`, then seed
   fake Aerion data (`~/.local/share/aerion/aerion.db` with a dummy SQLite
   file, `~/.config/aerion/…`, `~/.config/autostart/io.github.hkdb.Aerion.desktop`,
   `…clabs_public_forks.Aerion.desktop`) and a dummy `aerion` keyring entry
   in a throwaway keyring (`dbus-run-session` + `gnome-keyring-daemon
   --unlock` with an empty password, or skip this part with a note). Take a
   sha256 manifest, then run MailPong: first start, add a test account,
   toggle autostart, quit, and run `make install`/`uninstall` into a prefix.
   Re-hash: the Aerion files and keyring entry must be identical.
4. **Manual import test (D5)** using the fixture Aerion data dir from a
   pre-migration build (`aerion-fork/main`) with a test account only: follow
   the guide, confirm MailPong opens the copied DB, and confirm the rollback
   leaves Aerion working.
5. **Basic email functionality** with a dedicated test mailbox (owner
   provided, never real mail) or an ad hoc local IMAP/SMTP test server
   (for example, a GreenMail container, started with `run_in_background` and
   not added to the repo): add an account (password auth), sync, open a
   thread in chat view, reply, send, archive, and confirm the sent message
   headers (`User-Agent: MailPong`, Message-ID domain).
6. **UI visual pass** with `run-mailpong`. Check the Mail, Contacts,
   Calendar, Settings and composer screens, each with the Accounts column
   expanded and collapsed, in light and dark themes and at narrow and wide
   widths. Record each check as performed or not performed.
7. Run the M8 `workflow_dispatch` dry run on the final branch head. The
   owner runs the macOS/Windows checks from M7.
8. Land `mailpong-migration` on `main` per AGENTS.md "Landing a Branch",
   with explicit owner approval because this is the new repo's first big
   change.

Acceptance criteria:
- `tools/brand-check.sh` exits 0, and the allowlist has a reason for every
  entry.
- All standard checks, builds and the dry run are green, or every failure
  that is unrelated and already in the M0 baseline is recorded.
- The data-safety and import tests pass, with hash manifests attached to
  Progress.
- The email functionality and visual checks are performed or explicitly
  listed as not performed, with the reason.

### M13. Go public and prepare the release — **Approval gate**

Goal: the repository is public, correctly configured and ready for a release
the owner approves.

Steps (each GitHub setting change approved by the owner):
1. Set the description to the exact GitHub description string, homepage to
   none or the README, and topics such as `email-client`, `wails`, `svelte`,
   `go`, `imap`, `desktop-app`, `chat`.
2. Set visibility to **public**.
3. Settings: Issues on; Wiki and Projects off; Discussions optional (owner);
   private vulnerability reporting, secret scanning, push protection and
   Dependabot alerts on; Actions default `GITHUB_TOKEN` permissions
   read-only; a ruleset for `main` that blocks force-push and deletion, with
   no required PRs (O6). Discussions off.
4. Re-enable the Release workflow.
5. Verify that the links in README, About, metainfo and docs resolve to
   `caseylabs/mailpong` (connector `get_file_contents` and spot-checks).
6. Prepare the `v0.4.0` tag and draft release notes. **Don't push the tag
   or publish** until the owner separately approves the initial release
   (out of scope here).

Acceptance criteria:
- The repo is public with the exact description, isn't a fork, and its
  settings match the list (connector `search_repositories` with
  `minimal_output=false` and the owner's screenshot of settings).
- No release or tag exists yet. The draft notes are ready for review.
- `clabs-public-forks/aerion` still has the same `main` SHA and settings as
  in M1.

Rollback: switch the repo back to private (stars and watchers are lost if
it was public for a while). Delete a draft release if one was created by
mistake.

### M14. Deferred follow-ups (do not execute without separate approval)

- Point `clabs-public-forks/aerion` to MailPong (README banner) and/or
  archive it.
- Stop tracking upstream (remove the `upstream` remote and record the
  decision).
- Register MailPong OAuth clients (Google verification and CASA, Azure
  publisher verification).
- Code signing and notarization for macOS and Windows.
- Flathub submission under `io.github.caseylabs.MailPong`.
- Nixpkgs packaging.
- Pin `wails@latest` in `release.yml` (supply-chain hygiene; already tracked
  as an unrelated dependency change).
- Consider applying the owner's `CaseyLabs/kc-secure-repo-template`
  hardening to the new repository.

## Risks, Assumptions and Rollback

### Risks

| # | Risk | Likelihood / Impact | Mitigation |
|---|---|---|---|
| R1 | MailPong reads or deletes Aerion data, autostart entries, icons or keyring secrets | Med / **High** | M3 removes the legacy cleanup, M3 tests, M6 install/uninstall hash test, M12 sandbox proof |
| R2 | Upstream tags pushed to the new repo trigger `release.yml` | Med / High | Push `main` only and disable Release before the push (M1) |
| R3 | Release workflow pushes to `main` or publishes accidentally | Med / Med | Remove the push job; draft-only releases; `dry_run` default (M8) |
| R4 | A blind rename breaks persisted values (contacts source, OAuth choices) or attribution | Med / High | D6/D11, per-milestone file lists, brand-check allowlist |
| R5 | Users expect Gmail/Outlook OAuth to work out of the box | High / Med | Clear BYO-credentials docs and the in-app OAuth-missing dialog (D3) |
| R6 | Version 0.4.0 suggests an official Aerion release | Med / Low | Always write "MailPong 0.4.0", plus the README note and NOTICE |
| R7 | Trademark or endorsement confusion with Aerion | Low / Med | New name and artwork, "not official / not endorsed" wording, nominative-only use of "Aerion" (L4) |
| R8 | Removing locales breaks a saved non-English setting from an imported DB | Low / Low | Fallback to `en` (M9 acceptance) |
| R9 | Platform checks (macOS, Windows, Flatpak) can't run in cloud sessions | High / Med | CI dry runs plus owner manual checks, recorded as performed or not performed |
| R10 | The module-path and filename renames make upstream cherry-picks harder | High / Low | Accepted (D13). Use `git cherry-pick -X rename-threshold` or port fixes by hand |
| R11 | Cloud sessions can't reach `caseylabs/mailpong` | High / Med | Owner adds the repo to the environment and GitHub App in M1 |
| R12 | Bundled GPL/MPL dictionaries in past fork builds | — / Low | None of those builds were released, and they are removed in M9. Legal note L3 |

### Assumptions

- The owner's GitHub account `CaseyLabs` can create public repositories and
  will hold admin on `caseylabs/mailpong`.
- No one else depends on the fork's app IDs or binaries. The fork has never
  published a release.
- Go module consumers don't exist outside this repo, so changing the module
  path breaks no one.
- Wails v2 keeps deriving the Windows WebView2 data path from the executable
  name and the Linux webview data from `ProgramName`. M4/M7 verify this at
  runtime.

### Rollback summary

- Repository: the original fork is never modified. Delete or re-privatize
  the new repo (with owner approval).
- Code: one commit per milestone on `mailpong-migration`. Revert that
  milestone's commit. Nothing lands on `main` before M12.
- Installations: MailPong uses separate IDs and paths. Uninstall MailPong
  (`make uninstall-linux`, `flatpak uninstall io.github.caseylabs.MailPong`,
  delete `MailPong.app`, or the Windows uninstaller) and delete the
  MailPong data directories. Aerion is unaffected.
- Manual import: Aerion data is only copied, and the user's backup archive
  stays as a second copy.

## Outstanding Decisions (need owner approval)

- **O1** Resolved: plan approved as written.
- **O2** Resolved: M1 approved. Claude creates the repo with the connector and
  pushes. The owner gives cloud sessions access to `caseylabs/mailpong` if a
  push is refused.
- **O3** Resolved: commits use
  `Casey Labs <4674433+CaseyLabs@users.noreply.github.com>`, set per repo in
  the new clone (or with `git -c` per commit), and carry no Claude trailers.
- **O4** Open (at M5): approve the master SVG concept.
- **O5** Resolved: `wails.json` `author.email` stays empty.
- **O6** Resolved: Discussions off. The `main` ruleset blocks force-push and
  deletion only, with no required PRs.
- **O7** Open (at M12/M13): landing the migration branch on `main`, and
  making the repo public.
- **Legal review flags** (not legal conclusions). Resolved process: M10
  drafts conservative texts, and the owner reviews and approves them before
  M13. The sign-off is recorded in the Decision Log.
  - **L1** Whether repository-level modification statements (NOTICE, README,
    Git history) meet Apache-2.0 §4(b) "prominent notices stating that You
    changed the files", or whether per-file notices are needed for
    substantially modified upstream files.
  - **L2** The wording of the rewritten Terms of Use (indemnity, limitation
    of liability, governing law). Upstream's Terms included an indemnity
    clause; decide whether to keep any equivalent.
  - **L3** The Privacy Policy under bring-your-own OAuth; any remaining Google
    API Services User Data Policy statements; and confirming that no earlier
    distributed build bundled GPL/MPL dictionaries.
  - **L4** Nominative use of the "Aerion" name in the description ("Built on
    the open-source Aerion client") and README. Apache-2.0 §6 grants no
    trademark rights. Confirm the wording implies no endorsement.

## Progress

- [x] M0. Pre-flight and baseline (2026-10-11)
  - [x] The owner confirmed nothing is unpushed. The source is `origin/main` `1c0445d`.
  - [x] Baseline recorded (see Surprises & Discoveries, M0 baseline).
  - [x] 2026-10-11 The owner recorded the Aerion installs (one Linux machine). See Surprises & Discoveries, "Owner's Aerion installs".
- [x] M1. Create `caseylabs/mailpong` and move remotes (approval gate) (2026-10-11): private, not a fork, full clone (369 commits), `main` = `aerion-fork/main` `1c0445d`, no tags or runs, Release workflow disabled, old fork unchanged.
  - [ ] Owner's local checkout: `git remote add mailpong https://github.com/caseylabs/mailpong.git` (optional).
- [x] M2. Agent instructions and dev tooling for MailPong (2026-10-11): the skill is renamed to `run-mailpong`, AGENTS.md has a Project Identity section, and the Accounts-column rule is in place. Docs-only change, so no standard checks or UI check were needed.
- [x] M3. Go module path and backend application identity (2026-10-11): module is `github.com/caseylabs/mailpong`, backend names/paths/keyring/IPC/protocol strings are MailPong, legacy Aerion autostart cleanup is removed, D6 identifiers carry comments, and new tests cover paths, autostart and service names. Standard checks pass (go vet, golangci-lint v2.12.2, `go test ./...`); no frontend change, so no UI check.
- [x] M4. Frontend, Wails metadata and UI branding (2026-10-11): wails.json, package.json/lockfile, boot splash, title bar, What's New (0.4.0), About (alpha, "Built on Aerion by @hkdb"), URLs, English strings, DnD MIME, localStorage keys, `CoreOAuthSection`, plist IDs and `app.Version` 0.4.0 done. lint, svelte-check, go vet, `go test ./app/...` and `make build` pass. **UI visual check not performed** (no browser tooling in this cloud session); the owner must check both Accounts-column states.
- [x] M5. MailPong artwork (approval gate) (2026-10-11): owner approved keeping their README icon; `brand/mailpong.svg` is its vector redraw, `tools/brand/render-icons.sh` generates every size, Aerion artwork removed; `identify`/`file` and `make build` pass. `docs/ss.png` retaken with fake data only (Xvfb + Playwright Chromium + `screenshot-fakes.js`), and the UI visual check done the same way: title-bar logo in dark (Nord) and light themes with the Accounts column expanded and collapsed. The window/dock icon itself is not visible under Xvfb. `Makefile`, `release.yml` and `build/flatpak/*-dev.yml` still name the removed `io.github.clabs_public_forks.Aerion` icon files (M6/M8).
- [x] M6. Linux desktop entry, AppStream and Flatpak (2026-10-11): desktop entry, metainfo, dev/flathub/source manifests, Flatpak scripts and READMEs, `install.sh`/`uninstall.sh`, Makefile `install-linux`/`uninstall-linux` and `.gitignore` are MailPong; all Aerion removal paths deleted; creds shim dropped (D3). `make` and script install/uninstall checks leave seeded Aerion files byte-identical in a sandbox. **Not run (tools missing in cloud):** `desktop-file-validate`, `appstreamcli`, `flatpak-builder` build and `flatpak run`; the owner must run them on a Linux desktop.
- [x] M7. macOS and Windows packaging identity (2026-10-11): `Makefile` darwin build/install/uninstall use `MailPong.app` and the installer output is `mailpong-<arch>-installer.exe`; `project.nsi` fixes `UNINST_KEY_NAME "MailPong"` (install dir and `info.json` already come from `wails.json`); `build/README.md` documents the identity and that signing/notarization is future work. **Not performed (no macOS/Windows/makensis in cloud):** the `MailPong.app` build, `defaults read`, NSIS build, registry and file-property checks; the M8 dry-run and the owner must do them.
- [x] M8. CI/CD and release workflow (2026-10-11): `release.yml` rewritten (D14 names, no OAuth secrets or creds shim, least-privilege `permissions`, `workflow_dispatch` `dry_run` default true, draft prerelease only, push-to-`main` job removed, Flatpak built from the checkout's dev manifest plus the binary built in the same run); `docs/RELEASE.md` rewritten; `ci.yml` needed no change. `actionlint` and `git grep` pass. **Deferred by the owner (2026-10-11):** the Release workflow stays disabled until the final milestone (M13), so the `workflow_dispatch` dry run (and its link and artifact list) runs with the M12/M13 validation. The dry run must also confirm the darwin amd64/arm64 and Windows arm64 builds target the right architecture (`make build` passes no `-platform`; inherited from upstream; review flagged it).
- [x] M9. English-only UI with en-US, en-GB and en-CA spellcheck (2026-10-11): 29 non-English locale JSONs deleted; `i18n/index.ts` registers `en` only, with `resolveLocale()` sending any saved non-English language to `en`; `dateFnsLocale.ts` keeps its API; language picker hidden when `supportedLocales.length < 2`; dictionaries are `dictionary-en`, `-en-gb` and `-en-ca` (build emits only those three), and saved dictionary keys that are no longer shipped are dropped; `translation.yml` removed, `docs/LANGUAGE.md` reduced to a localization-architecture note, CONTRIBUTING updated; new Go test `charset_mixed_test.go` (RFC 2047 words, ISO-8859-1/Shift_JIS/UTF-8 parts). lint, svelte-check, `npm run build`, go vet and the sync/smtp/message/settings tests pass. UI check done headless (Xvfb + Playwright), Accounts column expanded and collapsed: no language picker, Composer lists English (US)/(UK)/(Canada), and the spellcheck worker flags "color" with en-GB and en-CA and accepts "colour". **Not done:** the multilingual fixture was not shown in the chat view (no mail server or `.eml` import in cloud); the Go tests cover the decoding. `flathub/node-sources.json` still lists the removed dictionary tarballs (regenerating changed about 5,000 lines of unrelated entries) and is regenerated at M13.
- [x] M10. Licensing, attribution and legal documents (2026-10-11): `LICENSE` unchanged; new `NOTICE`, `THIRD_PARTY_NOTICES.md` (dictionary and Nunito license texts, Go and npm inventories), `DISCLAIMER.md`; `docs/TERMS.md`, `docs/PRIVACY.md` and `SECURITY.md` rewritten (no `3df.io` outside `docs/history/`); `docs/CASAT2.md` and the old `CHANGELOG.md` moved to `docs/history/`, new `CHANGELOG.md`; README links the three notices; tarball already shipped `LICENSE`/`NOTICE`, and copy steps (`LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`) were added for `MailPong.app` (Makefile) and the Windows installer (`project.nsi`). **Not performed (no macOS/Windows/makensis in cloud):** the `.app` and installer checks; the M8 dry-run must confirm `LICENSE` and `NOTICE` ship in each artifact. The owner must read the legal texts (L1-L4).
- [x] M11. Documentation and GitHub templates (2026-10-11): README (Built on Aerion before Features, alpha badge), CONTRIBUTING, BUILD, SQL_ROLLBACK, EXTENSIONS/EXT_RULES/KEYBOARD_SHORTCUTS, the whole user guide (build-from-source install pages, bring-your-own OAuth, no CASA claim) and issue templates/`config.yml` (security link, upstream-Aerion pointer) are MailPong; new `docs/user-guide/getting-started/import-from-aerion.md` (D5). Brand and relative-link checks pass. **Not done:** the import procedure is untested (M12 tests it with fixture data); the owner must read the README. `docs/EXTENSIONS.md` no longer links the missing `context/*.md` design notes.
- [x] M12. Full validation, data-safety proof and brand audit (2026-10-11): `tools/brand-check.sh` plus `tools/brand-check.allow` (path glob :: line regex :: reason; every entry has a reason; a planted stray "Aerion" makes it exit 1) run in `make lint` and in the CI lint job and exit 0 on the committed tree (review found the first version matched its own files once they were tracked; they are now excluded, a failing `git grep` fails the check, and a no-reason entry and a planted stray both exit 1); `.env.example` still named the picker "Aerion - Microsoft/Google" (fixed). Standard checks pass (frontend lint, svelte-check, `npm run build`, go vet, golangci-lint v2.12.2 0 issues, `go test ./...`, `make build`; Wails CLI had to be installed again).
  - Data-safety (Linux sandbox `HOME`, seeded fake `aerion.db`, `~/.config/aerion`, both Aerion autostart files, cache): MailPong first start created only `mailpong` paths; `make install-linux`/`uninstall-linux` into a prefix left the prefix empty again; sha256 of the five seeded Aerion files before and after are identical (`a3a5e715…` cache file, `bb157861…` config.json, `624b54a9…` both autostart files, `4fa1539f…` aerion.db). **Not done:** keyring entry (no `gnome-keyring-daemon` in cloud) and toggling autostart through the UI (covered by the M3 unit tests).
  - Import test (D5): built `1c0445d` (pre-migration), ran it in a sandbox to create `aerion.db` (schema v53), added a marker row, followed the guide (copy, rename `aerion.db`→`mailpong.db`); MailPong opened it (schema v53, marker row present) and left the Aerion files identical (aerion.db `7a29742b…`, device.key `daa30dad…`, hsts `c53c20f3…`, salts `e11d7be3…`/`d0da1a1f…`); after deleting the MailPong dirs the old Aerion build started again. **Not done:** a real account in the fixture (the old build has no scripted way to add one).
  - Email (GreenMail 2.1.3 jar, plain IMAP 3143/SMTP 3025, not added to the repo): account added through the UI (Other / Manual, password), synced, sent, replied, archived; server-side headers show `User-Agent: MailPong` and a Message-ID `<uuid@example.com>` (account's domain). The chat view of the thread was driven through the Wails bindings, not clicked.
  - UI pass (Xvfb + Playwright against `dev.sh`, wide 1400 and narrow 700): Mail, Contacts, Calendar, Settings and composer with the Accounts column expanded and collapsed, dark and light; no layout defects seen. Caveats: the light theme and the sidebar toggle were forced by script and did not hold on every shot, so light coverage is partial; the page logs `Cannot read properties of null (reading 'nodes')` in the dev server (not traced to baseline).
  - [ ] Flatpak dev build (`flatpak-builder` is missing in cloud; owner runs it, see M6).
  - [ ] `workflow_dispatch` dry run (owner deferred it to M13, since the Release workflow stays disabled until then).
  - [ ] Land `mailpong-migration` on `main` (needs the owner's explicit approval).
- [ ] M13. Go public and prepare the release (approval gate)
- [ ] M14. Deferred follow-ups (separate approval; not part of this plan's completion)

## Surprises & Discoveries

- 2026-10-11 M12: the brand check passed while its own files were untracked and failed once committed, because `git grep` only sees tracked files. Check scripts that scan the repo must exclude themselves and be run on a committed tree. The dev-server page error `Cannot read properties of null (reading 'nodes')` appears on every load and was not traced; the Playwright UI harness needs the composer's "Close this message?" dialog discarded between shots, or it contaminates later screenshots.
- 2026-10-11: the fork and upstream Aerion already share data directories and
  the `aerion` keyring service. Only the desktop/Flatpak app ID was changed
  earlier. The fork's autostart code and uninstall scripts delete upstream
  Aerion files.
- 2026-10-11: the cloud checkout is shallow (50 of 369 commits). Mirroring
  needs a full clone.
- 2026-10-11: `release.yml` pushes Flathub manifest commits to `main` and
  builds the Flatpak from `hkdb/aerion` sources and release assets.
- 2026-10-11: the bundled spellcheck dictionaries include GPL-2.0, GPL-3.0
  and MPL-2.0 packages (cs, nb, de, it, fr).
- 2026-10-11: the macOS bundle ID is the Wails default `com.wails.aerion`,
  and the Windows AUMID and macOS launch agent use upstream's
  `io.github.hkdb.Aerion`.
- 2026-10-11 M0 baseline on `1c0445d` (cloud, Linux, Go 1.26.0, Node 22):
  `npm ci`, `npm run build`, `npm run lint` and `npm run check` (0 errors,
  0 warnings) pass. `go vet ./...` is clean. `go test ./...` passes (38
  packages). `golangci-lint` v2.12.2 (CI's version, run ad hoc with `go run`)
  reports 0 issues. `make build` produces `build/bin/aerion` (no OAuth creds).
  Pre-existing issues: `gofmt -l` flags
  `extensions/calendar/backend/provider_microsoft_test.go` (comment
  alignment). The container's preinstalled golangci-lint v2.5.0 is too old
  for Go 1.26. GTK 3 and WebKitGTK 4.1 dev packages and Wails CLI v2.16.0
  had to be installed in the container. The brand check found 2,185 lines in
  425 files. Node here is v22 (CI uses 24).
- 2026-10-11 M1: the GitHub connector can't create repositories
  (`POST /user/repos` returns 403 "Resource not accessible by integration").
  The owner creates the empty repo, and Claude pushes once the GitHub App
  covers it.
- 2026-10-11 M1: GitHub reports the canonical name as `CaseyLabs/mailpong`.
  The lowercase URLs work and redirect. Both workflows (CI, Release) were
  registered as active on the first push, and the connector has no
  disable-workflow method.
- 2026-10-11: the fork carries 52 tags inherited from upstream, although it
  has no releases.
- 2026-10-11: this container's git identity is `Claude <noreply@anthropic.com>`
  (see O3).
- 2026-10-11: `docs.github.com` and `docs.flathub.org` are blocked by the
  sandbox proxy, so official docs were consulted through web search. Fork
  detachment: the "Leave fork network" setting is permanent and drops
  issues, PRs and stars. Flatpak conventions recommend
  `io.github.<user>.<App>`. Re-check the Flathub requirements page before
  any Flathub submission.

- 2026-10-11 (M2): `caseylabs/mailpong`'s Release workflow was disabled by the owner
  (connector state `disabled_manually`, 0 runs), which closed M1. `NOTICE`
  doesn't exist yet; M10 adds it and should list it in AGENTS.md's branding
  bullet.

- 2026-10-11 Owner's Aerion installs (one Linux machine, reported by the
  owner; the agent didn't inspect them). M12 must leave every path below
  unchanged:
  - **Active:** fork Flatpak `io.github.clabs_public_forks.Aerion` 0.3.5,
    user installation, from `make flatpak-install`. Its manifest has no
    `--filesystem` access, so its data lives only in
    `~/.var/app/io.github.clabs_public_forks.Aerion`.
  - **Native data (no binary on PATH):** `~/.local/share/aerion/aerion.db`,
    `~/.config/aerion` and `~/.cache/aerion`, from the AUR `aerion-bin`
    package (0.3.1–0.3.5, now removed; package caches are in
    `~/.cache/yay` and `~/.cache/Shelly`) and from `make dev` runs. The dev
    binary's directories are `~/.local/share/aerion-dev-linux-amd64` and
    `~/.cache/aerion-dev-linux-amd64`.
  - **Stale:** `~/.local/share/applications/io.github.hkdb.Aerion.desktop`
    (the upstream app ID; probably from the upstream bundle
    `~/downloads/Aerion-v0.3.3-x86_64.flatpak`). The `plasmail-*` XDG
    sandboxes in `~/.cache` are from the period when this fork was called
    "plasmail": `plasmail-dev`, `plasmail-dev-m2-fresh`,
    `plasmail-dev-m3-ctl`, `plasmail-evidence` and `plasmail-scratch`.
    Each holds its own `aerion` config, data and cache directories.
  - **Checkouts:** `~/sync/git/projects/{aerion,aerion-bak,aerion-plasma}`.
  - **Implications:** M3's `mailpong` paths and keyring service keep MailPong
    away from all of these. M12's data-safety proof should take checksums
    or mtimes of `~/.local/share/aerion`, `~/.config/aerion` and the
    Flatpak's `~/.var/app` directory before and after a MailPong run. M6
    gives MailPong its own Flatpak ID, so both Flatpaks can be installed
    side by side. Cleaning up the stale entries and the `plasmail-*` caches
    is the owner's choice and is out of scope.
- 2026-10-11 (M4): `wails build` re-runs `npm install` and prunes ~100 lines from `frontend/package-lock.json`; the lockfile was hand-edited (name and version only) and restored after the build. `wails` is not preinstalled: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`. The OAuth-missing dialog links to README `#installation` until M11 sets a docs path. `Makefile` still echoes "Aerion" (M7). `app.Version` was bumped to 0.4.0 with `wails.json`.
- 2026-10-11 (M6): `desktop-file-validate`, `appstreamcli` and `flatpak-builder` are not installed in this cloud session (only `xmllint`; metainfo is well-formed). `StartupWMClass` is set to `io.github.caseylabs.MailPong` (the `ProgramName` GTK app id) and was not confirmed at runtime. The flathub manifest pins `tag: v0.4.0` with an all-zero placeholder commit until `calculate-hashes.sh` runs at release; `go.mod.yml`/`node-sources.json` were not regenerated. The metainfo release date `2026-10-11` is a placeholder to set at M13. `.github/workflows/release.yml` still names the old files, so release tarballs do not match `install.sh` until M8; `flathub/*-source.yml` still uses `wails@latest` with `GOSUMDB=off` (inherited, unpinned); docs still name the old IDs (M9/M11).

- 2026-10-11 (M10): `go-licenses` (v1.6.0 and v2.0.1) fails on the Go 1.26 standard library ("does not have module info"), so the Go inventory comes from `go list -deps` plus each module's license text (39 modules: MIT, BSD-2/3-Clause; no copyleft, no upstream `NOTICE` files). npm production deps (127) are MIT, ISC, Apache-2.0, BSD-3-Clause, 0BSD; the Apache-2.0 ones ship no `NOTICE`. `@tiptap/*` and similar were not individually audited beyond `license-checker`'s metadata (`svelte-toolbelt` shows `MIT*`).
- 2026-10-11 (M10): `docs/user-guide/` still carries upstream's FAQ/intro wording; M10 only removed the `3df.io` contacts and the sponsorship/coffee link. M11 rebrands the rest.
- 2026-10-11 (M10): legal texts for the owner to review: L1 `NOTICE` and `README.md` (Legal line); L2 `docs/TERMS.md` (Indemnification, Limitation of Liability, no governing-law clause, kept upstream's indemnity unchanged), `DISCLAIMER.md`; L3 `docs/PRIVACY.md` ("Google API Services" and "Microsoft API Services" sections, plus the open point that no earlier distributed build bundled GPL/MPL dictionaries); L4 `NOTICE`, `DISCLAIMER.md` "Independence", `docs/TERMS.md` "Description of Service".

## Decision Log

- 2026-10-11 (M7): Wails names the macOS bundle `<wails.json name>.app` (`mailpong.app`); the Makefile keeps `MailPong.app`, which resolves on case-insensitive volumes (the Aerion Makefile relied on the same). The Windows uninstall key needed an explicit `UNINST_KEY_NAME`, because the Wails default would be `MailPong ContributorsMailPong`.

- 2026-10-11 (M3): the Go rebrand ran a protected-token script (D6 IDs kept) over all `*.go`/`*.m`, then hand fixes. The MDN `Reporting-UA` uses `smtp.ClientVersion`, set from `app.Version` in `app/state.go`. Two `migrations.go` comments keep "Older Aerion installs" because they describe history. The Makefile tag, `.gitignore`, and the `release.yml` build path were renamed in M3 so builds keep working; M8 still owns the rest of CI.

- 2026-10-11 (Owner): D2 app ID `io.github.caseylabs.MailPong`; D3
  bring-your-own OAuth credentials; D4 en-CA as a spellcheck dictionary only;
  D5 documented manual import only; D7 Claude creates an original master SVG;
  D8 "MailPong Contributors"; D9 continue at version 0.4.0; D10 move the
  Aerion changelog to `docs/history/`.
- 2026-10-11 (Recommended, pending O1): D1 new standalone repo with pushed
  history; D6 keep persisted identifiers; D11 branding scope; D12 work in the
  new repo on `mailpong-migration`; D13 keep upstream remote; D14 identifier
  table; D15 legal-document approach. Keep en-GB alongside en-US and en-CA
  (it already exists, and the requirement says to preserve English spelling
  support).

- 2026-10-11 (Owner): plan approved (O1); M1 approved (O2); commit identity
  Casey Labs noreply (O3); `author.email` empty (O5); Discussions off, and the
  `main` ruleset blocks only force-push and deletion (O6); the owner reviews
  the drafts for legal flags L1–L4. PLAN.md is committed to the old fork's
  working branch `fix/admiring-archimedes-0rwkyr` (not `main`) so it survives
  until M1 carries it over.

- 2026-10-11 (M2): AGENTS.md states the branding rule and the D6 identifiers
  itself instead of pointing to D11/D6. PLAN.md gets pruned, so a pointer
  would go stale. It also gives the `git remote add upstream` command,
  because fresh clones lack that remote. The skill's `pgrep` pattern matches
  `build/bin/*-dev-linux`, so it works before and after the binary rename.

- 2026-10-11 (M3 review): stale `io.github.clabs_public_forks.Aerion` IDs remain in `frontend/src/App.svelte`, `Composer.svelte`, the Makefile `install` target, `release.yml`, `build/flatpak/*` and `wails.json`. M4, M6, M7 and M8 own them. The upgrade-notice gap (no detection of an existing Aerion directory) follows D5 and is deliberate. The old fork's autostart entry is no longer cleaned up, which is also deliberate.

- 2026-10-11 M5 (D7 conflict, resolved): the owner answered "keep the icon currently used in my readme and build from that or improve it". I first took that icon to be Aerion's, which would conflict with D7. `brand/mailpong.svg` is redrawn from scratch in the same style (no Aerion pixels or paths reused). Resolved: the owner says the README icon (blue tile, white envelope) is their own work, not Aerion's, so it may be kept; `brand/mailpong.svg` is a vector redraw of it (the orange ball was dropped), and D7 stands for the remaining Aerion-origin artwork (the purple `brand/glyph-icon.*`, which is removed).
- 2026-10-11 (M9): `resolveLocale()` in `i18n/index.ts` maps any saved non-English language to `en` (used by `initI18n`, `setLocale` and the settings store). With no spellcheck languages saved, the default dictionary comes from `navigator.language` (en-CA and en-GB get their own; every other language gets en-US), because the UI locale is always `en`. Declined review findings: other `en-*` variants (AU/NZ/IE/ZA) are not mapped to en-GB (the plan names CA and GB only), and `store_test.go` keeps its `zh-TW` row (the backend stores any string).

## Outcomes & Retrospective

(To be filled in as milestones complete.)
- M1: `caseylabs/mailpong` is private and standalone; the old fork is unchanged.
- M2: agent guidance and the UI-run skill now say MailPong.
- M3: backend identity is MailPong and cannot touch Aerion files or keyring entries.
