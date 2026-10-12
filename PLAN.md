# Shared AI guidance and remaining dependency work

## Scope
Make project guidance and all five project skills usable by Claude Code and Codex CLI without maintaining separate copies. Address AI1–AI4 in ISSUES.md. Keep shared policy in AGENTS.md, portable workflows in .agents/skills/, and only host-specific adaptations in Claude files. Preserve existing UI verification safeguards and the repository's commit and landing workflow.

The migration does not change application behavior, account settings, permissions, or installed dependencies. D2 remains separate and blocked until stable frontend tools support TypeScript 7.

## Milestones

### G1 — Consolidate shared policy (AI3, AI4)

- Keep AGENTS.md authoritative and retain CLAUDE.md's `@AGENTS.md` import. Move shared working-style preferences, attribution policy, and the rule against concurrent builds/tests in one checkout into AGENTS.md where not already covered.
- Remove duplicated shared rules from .claude/rules/. Preserve the extension path scope in an explicit AGENTS.md instruction to consult docs/EXT_RULES.md; keep long references loaded only when relevant.
- Keep Claude-specific tool, model, and delegation choices in small adapters. Express shared questions, background processes, review, and handoff requirements without assuming a particular host's tool names.
- Reconcile project-ship's merge-only-on-request rule and project-finish-milestone's conditional simplification with AGENTS.md. Skills defer to project policy; runtime permissions and explicit user instructions still take precedence.

Acceptance: both tools receive the same shared project policy; no shared rule depends on Claude-only discovery; the merge and simplification conflicts are removed without weakening existing safeguards.

Validation: review the before/after rule inventory, inspect CLAUDE.md's import and all remaining .claude/rules/ files, search for duplicate or conflicting workflow instructions, and run `git diff --check`.

### G2 — Share and port the five skills (AI1, AI2)

- Move run-mailven, project-next-milestone, project-finish-milestone, project-handoff, and project-ship, including their scripts and resources, to .agents/skills/. Add a relative symlink for each skill under .claude/skills/ pointing to `../../.agents/skills/<name>`.
- Keep skill names and clear trigger descriptions. Replace Claude dynamic command injection and reliance on `$ARGUMENTS` or `${CLAUDE_SKILL_DIR}` with explicit script calls and instructions for taking parameters from the user's request, with documented defaults.
- Replace mandatory slash commands and named Claude subagents in shared steps with capability-based instructions. Keep reusable acceptance-checker and worker briefs in shared references; Claude agent definitions retain only host-specific metadata and instructions to read those briefs. Codex can give the same briefs to its available agents. Preserve the documented direct-review fallback when independent review is unavailable.
- Keep run-mailven's shell and Playwright path portable. Move Claude browser-tool details into an optional reference; do not assume those tools exist in Codex. Detect actual executable/module/display availability, retaining explicit path overrides and existing environment hints where useful.
- Preserve fake-data/write-blocking safeguards, UI-state restoration, process cleanup, and Mail checks with the Accounts column expanded and collapsed. Update executable paths in AGENTS.md, skills, scripts, and relevant developer documentation. Verify resource lookup works through canonical and symlink paths.

Acceptance: each skill has one maintained implementation and is discoverable from both hosts' standard project locations; shared steps need no Claude preprocessing; existing scripts and UI safeguards remain usable.

Validation: check all five symlink targets and SKILL.md manifests, search for stale paths and unadapted host-specific syntax, and syntax-check changed scripts with the appropriate interpreter. Run plan-outline.sh against a temporary fixture with and without an explicit plan path. Check portable helper/resource resolution from both paths. Run `git diff --check`.

### G3 — Verify both hosts and document maintenance

- In fresh Claude Code and Codex CLI sessions, verify shared policy loading and discovery of all five skills. Record installed CLI versions, exact prompts/commands, results, and any unavailable checks in Progress.
- Check explicit selection (`/skill-name` in Claude; `$skill-name` or the skill selector in Codex) and representative natural-language selection. Use read-only smoke tasks or a disposable Git fixture for milestone, finish, handoff, and ship workflows; do not push, publish, or alter real milestones just to test selection.
- Exercise run-mailven's portable start/wait/check/stop workflow, including fake-data UI checks with the Accounts column expanded and collapsed. Verify cleanup and report any native-window checks that cannot run. Follow the current run-mailven safeguards for the real account-backed dev server.
- Document canonical edit locations, relative links, skill invocation, and host-specific adapters in CONTRIBUTING.md. Document the symlink requirement for clones, including Windows Git configuration; do not introduce manually maintained fallback copies.
- Remove AI1–AI4 from ISSUES.md only when their acceptance evidence is complete. Prune completed plan detail when it no longer needs to be referenced, preserving D2 until resolved.

Acceptance: both installed hosts load the shared policy and discover the five skills; representative invocations follow portable workflows; run-mailven passes the affected smoke checks; maintenance instructions identify one source for each shared rule and skill. Any unavailable host check remains explicitly incomplete.

Validation: inspect fresh-session evidence and helper/UI results; confirm no duplicate skill registrations or broken links; review documentation and run `git diff --check`. Application builds and tests are required only if implementation changes create a relevant risk. Follow AGENTS.md's independent acceptance review, simplification, commit, and landing requirements for each migration milestone.

### D2 — Upgrade TypeScript when supported

Acceptance: both published peer ranges accept TypeScript 7, the dependency graph is supported, and frontend lint, check, and build pass.

Validation: recheck stable versions and peer ranges with the commands in CONTRIBUTING.md before changing dependencies; then run `npm run lint`, `npm run check`, and `npm run build` in frontend/.

## Progress
- [ ] G1 — shared policy and workflow conflicts; ready to implement.
- [ ] G2 — portable skills and shared discovery; follows G1.
- [ ] G3 — fresh-session verification and maintenance documentation; follows G2.
- [ ] D2 — blocked upstream as of 2026-10-12. `npm view` reports typescript-eslint 8.71.1 requires `<6.1.0`; svelte-check 4.7.6 accepts only 5/6. No supported upgrade is available.

## Surprises & Discoveries
- The 2026-10-12 review found that CLAUDE.md already imports AGENTS.md, but all five project skills live only in .claude/skills/. The local audit found no repository skills in Codex's .agents/skills/ discovery location.
- Installed versions at review: Codex CLI 0.162.1 and Claude Code 2.1.294. Documentation supports symlinked skill folders in both hosts; actual discovery and execution still need fresh-session verification after migration.
- Shared skill files alone do not make Claude interpolation, slash commands, custom-agent metadata, or browser tools portable.
- The latest lint/check tools still exclude the published TypeScript 7.0.2 release.

## Decision Log
- Use .agents/skills/ as the canonical location and per-skill relative links in .claude/skills/. Keep CLAUDE.md's import for compatibility rather than relying on version-dependent direct AGENTS.md discovery.
- Keep required behavior in AGENTS.md and detailed procedures in skills. Discovery makes a skill available; its description guides optional selection, so mandatory checks must not rely solely on implicit skill activation.
- Share workflow content and review briefs; retain small host-specific adapters for capabilities and metadata that do not have a common format. Do not create parallel copies of skills or policies.
- This task writes the migration plan only. G1–G3 implementation and runtime verification remain pending.
- Official references checked on 2026-10-12: [Codex instructions](https://learn.chatgpt.com/docs/agent-configuration/agents-md), [Codex skills](https://learn.chatgpt.com/docs/build-skills), [Claude instructions](https://code.claude.com/docs/en/memory), and [Claude skills](https://code.claude.com/docs/en/skills). Recheck relevant behavior if installed versions or documentation change during implementation.
- Keep TypeScript ~6.0.3 until both tools support 7.

## Outcomes & Retrospective
The AI-guidance review is complete and findings AI1–AI4 are recorded in ISSUES.md. Migration has not started. Record outcomes and remaining limitations as G1–G3 finish; the dependency upgrade remains blocked separately.
