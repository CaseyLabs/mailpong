# PLAN: Assess and design Mailven's Wails v2 -> v3 migration

**Status:** Proposed. Analysis and design only; **migration implementation is not authorized**.
**Repository:** `https://github.com/CaseyLabs/mailven`
**Roadmap:** [WAILS](../PLAN.md#wails); original M0–M7 below map to W3-M0–W3-M7 there. Status/selection belongs to the root roadmap.
**Original intended path:** `plans/wails-v3/PLAN.md` (never created). This file retains the assessment scope; on explicit selection, use a linked `docs/plans/<milestone-id>.md` checkpoint and reconcile deliverable paths before writing them.
**Prepared:** 2026-10-11
**Decision required:** GO / DEFER / NO-GO, with evidence and an implementation-ready blueprint.

## Objective

Determine whether and how to migrate Mailven from Wails v2 to Wails v3, using GTK4 + WebKitGTK 6.0 on Linux, without compromising email correctness, local data, credentials, packaging, or supported platforms. Produce a reviewable **migration design and execution plan**, not a migrated application.

The output must answer:

1. What concrete Mailven problems would v3 solve? Which benefits are independent of GTK4 or the Wails upgrade?
2. How invasive is the migration, including extensions, generated bindings, Svelte, detached composers, desktop integration and distribution?
3. Is the target v3 release and Linux backend ready enough, or should Mailven remain on v2 temporarily?
4. What is the safest implementation sequence, with measurable acceptance criteria, rollback points and a realistic risk/effort assessment?

### Current starting point (revalidate in M0)

Repository `main` was inspected on 2026-10-11. It used Go 1.26, `github.com/wailsapp/wails/v2 v2.16.0`, Svelte 5 and Wails v2 generated frontend bindings. `main.go` creates separate Wails applications for the main mailbox and detached composer. `app/detached_composer.go` + `app/ipc.go` coordinate the composer with the main process. Linux builds use `webkit2_41` / GTK3 / WebKit2GTK 4.1, with WebKit pre-initialization (`webkit_preinit_linux.go`), GPU/DMABUF handling (`internal/platform/gbmcheck_linux.go`), and special Flatpak file-dialog handling (`app/attachment.go`). The Flathub manifest targets GNOME Platform 50. Treat these as **leads, not immutable facts**.

At preparation time, the latest checked v3 prerelease was `v3.0.0-beta.28` (2026-10-05), with a stable-API claim but beta warning. Do not assume it remains the latest when running this plan. **Do not target a speculative Wails v4** without an official release and documented migration path.

### Scope

- Source-backed Wails v2-to-v3 and GTK3/WebKit2GTK 4.1-to-GTK4/WebKitGTK 6.0 assessment.
- Repository-specific API/dependency inventory and effort/risk matrix.
- Proposed backend services, lifecycle, windowing, IPC, frontend binding and packaging architecture.
- Linux-first plus Windows/macOS cross-platform test and release strategy.
- A staged implementation blueprint with pass/fail gates and rollback.

### Out of scope for this plan

- Editing application logic, source dependencies, generated bindings, packaging files or release workflows.
- Switching the production branch to Wails v3, removing workarounds, or rewriting composer IPC.
- Dependency upgrades unrelated to migration; UI redesign; new chat/email features; framework rewrites.
- Running against real email accounts or modifying real databases, settings, mail, tokens or keyrings.
- Committing to `main`, merging migration work, publishing releases, running release automation, or changing GitHub settings.
- Installing system packages, globally replacing the v2 CLI, or using unbounded `@latest` upgrades without owner approval.

If a prototype or other implementation is needed to answer a question, describe the smallest **separately approvable** spike with a disposable worktree and synthetic fixtures. Do not perform the spike as part of this plan.

## Agent execution contract

- Read the shared `AGENTS.md` guidance, `CLAUDE.md`, `ISSUES.md` and relevant `.agents/skills/` instructions **selectively** before beginning; current repository instructions take precedence over assumptions in this file. The root `PLAN.md` is the authoritative portfolio; select WAILS work by its stable W3-M ID, never by the first incomplete milestone in file order.
- Work on one milestone at a time. Use the project-next-milestone skill with the selected W3-M ID and linked checkpoint (in either host). Finish the milestone, update **Progress**, **Surprises & Discoveries**, and **Decision Log**, then stop. Do not claim to have completed unexecuted checks.
- Read the current milestone and only the relevant code/docs; use `rg`, targeted `sed`, and small command outputs rather than reading huge files or printing exhaustive logs. Parallel read-only investigation is acceptable; use `project-worker` only for a bounded, independent research question. Avoid unnecessary subagent duplication.
- Use the latest **official** Claude Code and Wails documentation at execution time. Consult exact pinned v3 API references/source if docs and examples conflict. Record the URL, release/tag/date and verification method for conclusions; mark unknowns explicitly. Wails docs occasionally contain sample versions older than the latest release: examples are not version pin recommendations.
- The original proposal scoped planning deliverables to `plans/wails-v3/`; reconcile that location with `docs/plans/<milestone-id>.md` checkpoints when selecting work. This reorganization does not authorize assessment execution or deliverable creation. Inspect `git status --short`, do not overwrite uncommitted work, and follow repository commit conventions for documentation changes. Never silently perform implementation because the research identifies an obvious fix.
- When a check needs credentials, live accounts, destructive actions, package installs, CI minutes, platform hardware or a GUI unavailable locally: record it as **not run**, with a precise proposed validation method. Do not fabricate output or substitute an unrelated test.
- Security/data invariant: preserve Mailven's existing application IDs, data directories, database schema/migration semantics, stored credentials, OAuth flows, mailbox integrity, encryption compatibility and isolation from Aerion. Never print or embed secrets, `.env` contents or private messages in artifacts.
- The existing `/project-finish-milestone` skill may be used for checks and independent acceptance review; ensure that any automated commit/merge/push behavior is limited to the documentation-only branch according to repo policy. **No migration implementation is authorized by this plan.**

### Durable deliverables

1. **This file:** milestone definitions plus brief living Progress, Surprises & Discoveries, Decision Log and Outcomes & Retrospective entries.
2. `plans/wails-v3/ASSESSMENT.md`: evidence-backed current state, API mapping, risk register, alternatives, known unknowns and test matrix. Update it incrementally, without duplicating long content in this file.
3. `plans/wails-v3/MIGRATION_BLUEPRINT.md`: proposed phased implementation plan, exact file groups, sequencing, verification gates, platform matrix, deployment safeguards and rollback. Create only after completing the assessment milestones.

Keep documents compact and actionable. Use tables where useful. Every claim about Mailven should point to a `path:line`, symbol or reproducible command; every Wails API claim should link to documentation or a versioned source file. Do not treat untested projections as facts.

## Source priorities

1. Current local Mailven checkout and its build scripts/tests; `git log` and `git status` for state.
2. Official Wails: [v2 -> v3 migration](https://v3.wails.io/migration/v2-to-v3/), [v3 docs](https://v3.wails.io/), [Wails releases](https://github.com/wailsapp/wails/releases), [Wails issues/milestones](https://github.com/wailsapp/wails/milestones) and the matching tagged source.
3. Official [GTK4](https://docs.gtk.org/gtk4/), [WebKitGTK](https://webkitgtk.org/), [Flatpak](https://docs.flatpak.org/) and [xdg-desktop-portal](https://flatpak.github.io/xdg-desktop-portal/) documentation.
4. Official [Claude Code best practices](https://code.claude.com/docs/en/best-practices), particularly explore -> plan -> implement, bounded context, evidence-based verification, and subagents for isolated investigation.

## Milestones

### Phase A: Establish the facts

#### M0. Repository preflight and safe v2 baseline

**Tasks**
- Confirm checkout, clean/dirty state, current branch and HEAD; inspect the existing root `PLAN.md` only enough to identify outstanding obligations that could affect this migration. Never replace it.
- Check `go.mod`, `wails.json`, `Makefile`, frontend dependencies, CI workflows and Flatpak manifests. Record **actual** dependency versions, CLI/toolchain availability, packaging targets and current known issues.
- Capture existing test/build commands and distinguish runnable checks from those requiring GUI, platform-specific hardware or credentials. Check `ISSUES.md` for relevant known regressions.
- Identify the baseline's safe test data strategy: isolated `$HOME`/`XDG_*` dirs, separate keyring/session and fake or dedicated test mail accounts; do not launch Mailven against personal user data.

**Deliverable:** `ASSESSMENT.md` sections `Baseline` and `Assumptions / test environment`.

**Acceptance criteria**
- [ ] Baseline includes commit SHA, tool/dependency versions, exact paths to packaging workflows, existing tests and known baseline failures; none presented as verified unless checked.
- [ ] Existing user changes and root plan are untouched; synthetic/isolated test approach documented.
- [ ] Running vs deferred checks are individually identified, with reasons.

**Validation:** `git status --short`, `git rev-parse HEAD`, targeted file and CLI inspection; link to command output or relevant file lines.

#### M1. Verify Wails v3 and GTK4/WebKitGTK 6.0 readiness

**Tasks**
- Verify latest v2 stable and v3 stable/prerelease status at execution time; check v3 release notes, breaking changes, unresolved GA blockers, security or platform regressions and project support expectations. Verify v4 only if an official release or roadmap exists.
- Read current v2-to-v3 guide and tagged documentation/source for: app/window lifecycle, services, bindings, events, multi-window, single instance, dialogs, tray, notifications, Linux build tags, platform APIs and CLI/build customization.
- Distinguish `libwebkitgtk-6.0` **API/GTK ABI** from WebKitGTK's `2.x` browser-engine version. Determine which CSS/JS, compositor, GPU and sandboxing improvements also reach the older `4.1` API with the same engine version.
- Confirm GTK4/WebKitGTK 6.0 Linux prerequisites and Wails' supported GTK3 compatibility option, including any removal/deprecation timetable. Flag unverified benefits instead of presenting them as guaranteed performance gains.

**Deliverable:** `ASSESSMENT.md` section `Versions, APIs, platform maturity` with an explicit **v3 readiness assessment**.

**Acceptance criteria**
- [ ] Source links and check dates support each version/support claim; prerelease target is pinned for repeatability if subsequently prototyped.
- [ ] v3 readiness includes blockers that could affect Mailven, and a supported/unsupported/unknown classification for needed APIs.
- [ ] Engine improvements are separated from framework/API improvements; no unsourced performance promises.

**Validation:** Cross-check release/tag, documentation and issue status against official sources; record uncertainty where they disagree.

#### M2. Inventory every Mailven v2 dependency and behavior

**Tasks**
- Search Go files for `/wails/v2`, `wails.Run`, `wailsRuntime`, lifecycle hooks, dialogs, windows, clipboard, file drop, native integration, events, service bindings and shutdown behavior. Inspect relevant usages rather than merely counting imports.
- Search frontend for `wailsjs`, Wails event subscriptions, runtime calls, generated models, `frontend/src/main.ts`, `frontend/src/composerMain.ts`, `frontend/src/lib/composerApi.ts`, and extension frontend calls. Distinguish generated code from source code.
- Trace embedded contacts/calendar bridge APIs and any assumptions about reflection, promoted methods, service ownership, initialization order or event naming.
- Trace startup and preflight, hidden-to-visible transitions, ready signaling, background sync, window geometry, notification click routing, single-instance `mailto:` handling, dialogs, attachments and detached composer/process IPC.
- Classify each dependency as **straightforward adaptation / architectural redesign / platform-dependent / needs spike**, and document its intended v3 counterpart with evidence. Include unchanged functionality explicitly.

**Deliverable:** `ASSESSMENT.md` `Migration inventory` table: `existing file/symbol | behavior | v3 API/approach | impact | risk | validation`.

**Acceptance criteria**
- [ ] Inventory covers Go runtime, frontend bindings, both extensions, platform helpers, startup/shutdown, assets and secondary composer entry point.
- [ ] Each medium/high-risk item has a concrete file/symbol, proposed adaptation and verification method.
- [ ] Unsupported v3 capabilities or unidentified mappings are recorded as open questions, not silently dropped.

**Validation:** Reproduce search patterns (`rg -n`) and spot-check every high-risk mapping against current source and pinned Wails v3 APIs.

### Phase B: Choose an architecture

#### M3. Design backend, frontend and multi-window migration

**Tasks**
- Compare **Option A: compatibility-first v3 port preserving existing composer process and IPC temporarily** with **Option B: one v3 application with native composer windows and shared services**. Consider a staged A -> B route. Do not assume removal of IPC is automatically safer.
- Define lifecycle ordering for database preflight, encryption/keyring readiness, email sync, extensions, bridge registration, events, window visibility, close/hide, quit, OS shutdown and crash recovery.
- Specify how frontend binding imports and generated models change, including the `ComposerApi` abstraction, core App methods, calendar/contacts extension methods, event subscribers and composer-specific APIs.
- Specify event scoping and data access across windows: per-window UI state, shared singleton service concurrency, drafts/sends, attachment handling, cancellation and failure. Identify race tests and how to avoid duplicate sends or corrupt drafts.
- Map use of v3 native single instance, notifications, dialogs and menus against current bespoke helpers; separate **must-port** from **optional cleanup** to prevent scope explosion.
- Include security boundary analysis: remote HTML in email, URL/navigation policy, untrusted content, privileged Go methods, local file reads, attachment path validation and cross-window access. Preserve existing protections.

**Deliverable:** `ASSESSMENT.md` `Architecture options and decision` with a dependency diagram and a proposed source-level change map.

**Acceptance criteria**
- [ ] Both composer architectures have pros/cons, risks, migration prerequisites and rollback implications.
- [ ] Proposed lifecycle and bindings approach accounts for both windows and both extensions without losing existing methods/events.
- [ ] Clear recommendation identifies what to migrate first and what to defer.

**Validation:** Trace one representative `send/reply`, `draft save`, `notification click`, `mailto:` activation and `close/reopen` flow end-to-end against source symbols; confirm v3 API feasibility from official docs/source.

#### M4. Platform, GTK4, Flatpak and packaging feasibility

**Tasks**
- Linux: map GTK3/WebKit2GTK 4.1 -> GTK4/WebKitGTK 6.0 dependencies, build tags, GPU policies, app ID/window class, Wayland/X11 behavior, native title bar, fractional scaling, window bounds, WebKit initialization and DMABUF/black-window workarounds.
- Review GNOME 50 Flatpak runtime and SDK availability of required `.pc` dependencies and portals. Review file chooser/save, drag/drop, clipboard, notifications, URL handlers, sandbox access and display sockets on KDE Plasma Wayland and GNOME; distinguish documented support from proven runtime success.
- Map current `Makefile`, `wails.json`, `build/flatpak/**`, `.github/workflows/{ci,release}.yml`, OS bundles, Windows NSIS and macOS signing/build requirements to v3 CLI/config/build flow. Preserve release artifact IDs, source reproducibility and offline/vendor build constraints.
- Identify current workarounds that can be **tested for removal**, rather than removing them based on framework-version claims.

**Deliverable:** `ASSESSMENT.md` `Platform and distribution matrix` with `Linux native / Linux Flatpak / Windows / macOS` and `Wayland / X11 where applicable`.

**Acceptance criteria**
- [ ] Linux package names/ABI requirements and current Flatpak build compatibility are cited or explicitly unverified.
- [ ] Every current Linux-specific workaround is marked retain / test removal / replace, with a reason and test.
- [ ] Windows, macOS, Linux packaging and CI differences are identified; no platform is assumed supported merely because Linux compiles.

**Validation:** Inspect manifests and workflow build commands, compare to tagged v3 CLI docs; non-installed tools or unavailable platforms are reported as gaps.

### Phase C: Validation, decision and handoff

#### M5. Define a regression, performance and security test plan

**Tasks**
- Create a reproducible **v2 baseline vs proposed v3** test matrix with commands, fixtures, expected outcome, platform and pass/fail rule. Test with synthetic accounts/mail and isolated data directories.
- Minimum functional flows: initial launch, account creation/auth, IMAP sync and background resume, thread display, sent/replied mail, draft persistence/recovery, attachment save/open, clipboard, Tiptap typing/paste, notifications, `mailto:`, single instance, multi-window lifecycle, calendar/contacts extensions, settings, themes, restore/maximize, and shutdown.
- UI acceptance on both dark/light themes, both wide/narrow layouts, **Accounts column expanded and collapsed**, on KDE Plasma Wayland and, where possible, GNOME. Prefer existing `.agents/skills/run-mailven` tooling; document what cannot be automated.
- Baseline performance measurements using identical fixtures: cold/warm launch, time to usable UI, idle process-tree RSS/CPU, scroll smoothness/input responsiveness, multiple composer overhead, long-session stability. Repeat measurements (e.g., 5 runs); define measurement method and regression investigation thresholds, not unsupported guaranteed improvements.
- Include security/data integrity checks: byte-for-byte data preservation where applicable, no implicit migrations, keyring and secrets isolation, WebKit policy for untrusted HTML, no loss of drafts/sent messages, and no effects on Aerion installations.

**Deliverable:** `ASSESSMENT.md` `Verification matrix` with reproducible test cases and **critical acceptance gates**; note which are plan-only and which have run.

**Acceptance criteria**
- [ ] Critical email data/sending/auth checks are explicit and not reducible to a successful build.
- [ ] Each test defines pass/fail evidence and whether it can run automatically, requires hardware/OS or requires owner action.
- [ ] Performance comparisons distinguish actual measurements from targets and control for runtime/WebKit engine differences.

**Validation:** Verify fixture setup against existing test helpers and build commands; check every proposed test maps to a known behavior/risk from M2-M4.

#### M6. Write the staged migration implementation blueprint (design only)

**Tasks**
- Convert assessment into ordered future implementation milestones with bounded changes, dependencies, file groups, verification and rollback for each. Suggested sequence to *validate and adjust*:
  1. Establish repeatable v2 baseline, isolated fixtures and v3 target pin.
  2. Introduce a minimal compiling/running v3 desktop shell without changing mail-domain behavior.
  3. Port services, generated bindings, event bus and frontend integration incrementally; keep method-level contract checks.
  4. Port/verify startup and shutdown, notifications, activation, attachment/file-dialog and single-instance behaviors.
  5. Decide and execute separately whether to consolidate native composer windows and retire process IPC; include race/failure tests.
  6. Adopt/test GTK4 + WebKitGTK 6.0 and rebuild Linux/Flatpak integration; retain workarounds until regression tests justify removal.
  7. Update macOS/Windows/CI/distribution artifacts and complete security, performance and cross-platform regressions.
  8. Approve cutover, release readiness and rollback instructions only after critical gates pass.
- Separate **minimum v3 parity** from **optional v3-native architecture cleanup**; document whether they should ship together or in separate changes.
- For any blocker that cannot be answered without compiling v3, add a **separately approvable bounded spike** (exact objective, disposable worktree, no user data, exit criteria, cleanup) to the blueprint. **Do not run it now.**
- Include a fallback to staying on v2 with targeted updates and a revisit trigger, e.g., fixed upstream issue or stable release.

**Deliverable:** `MIGRATION_BLUEPRINT.md`, executable by a later coding-agent session only **after separate owner approval**.

**Acceptance criteria**
- [ ] Every implementation milestone has files/systems affected, done criteria, narrow tests, dependencies, risk and rollback.
- [ ] No single milestone combines an unverified ABI upgrade, backend service rewrite, frontend refactor and composer redesign without a tested boundary.
- [ ] Reverting a failed experiment cannot mutate production data, Aerion data, keyrings or released artifacts; distribution and schema rollback are addressed explicitly.
- [ ] A subsequent session could begin the first implementation milestone without rediscovering the whole repository.

**Validation:** Compare blueprint coverage against every high-risk row in M2-M5; all rows must map to a milestone, deferred decision or explicit no-change rationale.

#### M7. Independent review and owner decision gate

**Tasks**
- Review both deliverables against acceptance criteria using the project's read-only acceptance-checker (or direct equivalent). Check sources, unresolved questions, missing file/API mappings, test feasibility and risky assumptions. Fix planning gaps, not application code.
- Produce a decision summary with **GO / DEFER / NO-GO** recommendation, conditions, critical blockers, smallest first implementation step, and proposed stop/revisit triggers.
- Mark the recommendation as **proposed**, not owner-approved. Stop before any code migration; request separate authorization for implementation or a spike.

**Deliverable:** `ASSESSMENT.md` final executive decision section, finalized blueprint if GO/conditional GO.

**Acceptance criteria**
- [ ] Every previous milestone has evidence or a clearly documented limitation; independent review findings are resolved or tracked.
- [ ] GO/DEFER/NO-GO is supported by explicit, traceable criteria, not general enthusiasm for newer technology.
- [ ] Working application/source dependencies are unchanged; only approved planning documentation was created/edited.
- [ ] The final report states exactly what is ready, what is still unknown, and what requires owner authorization.

**Validation:** Review documentation diff and `git status`; verify links, relevant source references and checklist coverage. Do not run a release pipeline or merge migration code.

## Decision criteria

**Recommend GO only if** the necessary v3 desktop APIs exist and are usable, Mailven's core workflows can be preserved, GTK4/Flatpak/toolchain compatibility has a credible validation path, cross-platform distribution is feasible, and no known blocker threatens secrets, emails, drafts or database integrity. A GO may be conditional on a narrow spike.

**Recommend DEFER if** support gaps are likely transient (e.g., beta Linux regression, packaging breakage, missing GUI test infrastructure), or if parity would jeopardize an imminent release. Identify objective revisit triggers and the minimal v2 maintenance alternative.

**Recommend NO-GO if** required security, account, data, window or distribution behavior is unsupported without disproportionate invasive work, or a credible rollback cannot be designed. Clearly distinguish confirmed blockers from speculation.

Performance claims require comparable measurements. An upgrade need not improve a benchmark to be worthwhile, but regressions must be visible and assessed rather than concealed.

## Progress

Tick a milestone only after its acceptance criteria are evidenced. Keep entries short, with links to the relevant deliverable section. A missing test is **unverified**, not **passed**.

- [ ] M0. Repository preflight and safe v2 baseline
- [ ] M1. Verify Wails v3 and GTK4/WebKitGTK 6.0 readiness
- [ ] M2. Inventory every Mailven v2 dependency and behavior
- [ ] M3. Design backend, frontend and multi-window migration
- [ ] M4. Platform, GTK4, Flatpak and packaging feasibility
- [ ] M5. Define a regression, performance and security test plan
- [ ] M6. Write the staged migration implementation blueprint
- [ ] M7. Independent review and owner decision gate

## Surprises & Discoveries

- No findings yet. Record only material new facts, blockers and changed assumptions, with source and date.

## Decision Log

- 2026-10-11: This is **analysis/design only**. No Wails/GTK migration, no release and no change to existing app data is authorized.
- 2026-10-11 (original proposal): keep the root plan separate and use `plans/wails-v3/PLAN.md`. Superseded for routing only on 2026-10-14: root PLAN.md now owns the WAILS roadmap; select W3-M IDs and create linked checkpoints only when authorized.
- 2026-10-11: Treat native v3 composer windows as an architectural **option**, not an automatic requirement for minimum parity.

## Outcomes & Retrospective

- Pending M7. Summarize outcome and lessons in <=10 lines; do not copy the assessment.

