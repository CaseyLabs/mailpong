# Shared AI guidance and remaining dependency work

## Scope
Make project guidance and all five project skills portable between Claude Code and Codex CLI. Keep shared policy in AGENTS.md and skill implementations in .agents/skills/. Expose the canonical tree through one relative directory symlink, `.claude/skills -> ../.agents/skills`: no wrappers, duplicate scripts, or duplicate workflow bodies; clones must support Git symlinks. Address remaining AI2–AI5 in ISSUES.md while reducing loaded context and preserving accuracy, UI safeguards, and the repository's commit/landing policy.

The directory migration and symlink trial are complete; shared-policy and workflow porting plus remaining G3 checks are pending. Application behavior, permissions, installed dependencies, and personal CLI configuration are out of scope. D2 remains separately blocked.

## Milestones

### G1 — Consolidate shared policy (AI3, AI4)

- Keep AGENTS.md authoritative and CLAUDE.md's `@AGENTS.md` import. Consolidate shared working preferences, attribution policy, and serialized builds/tests; remove redundant .claude/rules/ content. Preserve the extension/core/kit path triggers and the instruction to consult docs/EXT_RULES.md only for relevant changes.
- Keep root guidance short: mandatory constraints, non-obvious commands, routing, and checkpoint requirements. Put occasional procedures in skills/references. Ordinary links are pointers, not automatic imports; shared files must explicitly say when to read a reference. Do not use Claude `@` imports inside shared guidance or rely on matching nested-instruction loading across hosts.
- Small host-specific sections may remain in shared files, clearly conditional on the host and available capabilities. Keep lengthy browser/tool instructions in optional references. Permission settings, model IDs, custom-agent definitions, and tool schemas are host-specific configuration, not portable policy. Do not assume Markdown grants tools or permissions.
- Resolve all skill conflicts with project policy: ship's merge-only-on-request, finish's conditional simplification, and handoff's never-push rule. Preserve runtime/user precedence. Make plan-mode behavior respect the active host and explicit request; remove the next-milestone instruction to disregard plan mode.

Acceptance: both hosts receive one shared project policy; no mandatory rule depends on Claude-only discovery; conflicting workflow rules are removed; optional references load only when relevant.

Validation: inventory before/after rules, check CLAUDE.md import and remaining Claude rules, search for conflicting directives, record bytes/words of always-loaded project files, and run `git diff --check`. Count imported content once but include remaining unscoped rules. Aim below the current baseline without deleting safeguards; explain any increase.

### G2 — Share and port five skills (AI2)

- Preserve the canonical run-mailven, project-next-milestone, project-finish-milestone, project-handoff, and project-ship implementations in .agents/skills/<name>/ and the single .claude/skills directory symlink. Do not create per-skill links or wrappers. Keep all edits and resource references canonical.
- Keep shared frontmatter to name/description plus standard fields only when needed. Keep descriptions concise and specific. Move necessary Claude-only behavior into conditional instructions or host adapters; Codex-only invocation metadata belongs in agents/openai.yaml only if needed. Tool names in allowed-tools and host-specific invocation controls are not interchangeable. Verify both discovery paths resolve to the same files and expose each skill once.
- Replace dynamic command injection, `$ARGUMENTS`, `${CLAUDE_SKILL_DIR}`, mandatory slash commands, and named Claude subagents in shared workflows with explicit script calls, request-derived parameters/defaults, and capability-based steps. Keep reusable worker/reviewer briefs in shared references and thin .claude/agents adapters. Preserve independent review and its documented direct-review fallback; do not duplicate native agent configurations without a demonstrated need.
- Define path bases: scripts resolve resources from their own location; repository operations resolve the active checkout, including worktrees; user plan paths resolve from a documented working directory. Never bake in a user's checkout path. Shell-quote paths and arguments. Skill discovery does not change shell cwd.
- Split run-mailven's optional browser, screenshot, composer, and troubleshooting detail into references with clear read triggers. Keep prerequisites, real-account/write safeguards, state restoration, cleanup, and Accounts expanded/collapsed checks in the core workflow. Detect executable/module/display availability; keep explicit overrides and environment hints without treating Claude-specific variables as universal environment detection.
- Update paths in project guidance and developer documentation. Preserve the plan-outline helper and selective plan reads. Require Scope, the current milestone, relevant decisions, and Progress to be sufficient for correct continuation; select an actionable milestone, reporting blocked items rather than blindly implementing the first unchecked item.

Acceptance: both native skill directories expose all five skills with one maintained implementation each, one directory symlink, no custom discovery flags, and no Claude preprocessing required by the shared body. Scripts and UI safeguards remain usable.

Validation: validate manifests, the directory symlink, and identical resource resolution through both paths; search for stale paths and host-only syntax in shared executable instructions; syntax-check changed scripts. Exercise plan-outline with default, explicit, missing, and space-containing paths. Check helper/resource lookup from repo root, a subdirectory, and a disposable worktree. Record core skill sizes and references loaded for representative tasks; target a materially smaller run-mailven core than its current 18,551 bytes. Run `git diff --check`.

### G3 — Verify discovery, context, and cross-host handoff (AI5)

- Use fresh installed Claude Code and Codex CLI sessions without special discovery flags or personal skill installations, in a clone with symlink support. Record versions, cwd, prompts/commands, source paths, results, and unavailable checks in Progress. Verify root and subdirectory starts, shared policy loading, all five skill registrations, and the absence of duplicate registrations. Distinguish documented behavior from observed behavior and user/global configuration effects.
- Check explicit selection (`/skill-name` in Claude; `$skill-name` or `/skills` in Codex) plus representative natural-language triggers and non-triggers. Verify the discovered skills load their bodies and needed references through the linked directory before acting. Use read-only smoke tasks or disposable Git fixtures; do not push, publish, or advance real milestones merely to test selection.
- Make PLAN.md the shared continuation record, not native session history or private auto-memory. Checkpoint before committing: active milestone/sub-step, branch/base revision, changed or uncommitted files, exact validation commands/results and the revision they cover, blockers, user decisions, and the next action. Avoid a self-referential commit hash in the commit that records it; the handoff can report the resulting hash separately. Persist enough state before compaction or a tool switch, including when work is unverified and cannot be committed.
- Test Claude-to-Codex and Codex-to-Claude continuation in a disposable fixture using only tracked guidance, the checkpoint, and working-tree inspection. Confirm the receiving host identifies the next action, honors constraints, and does not repeat valid checks. Use a plain-language handoff prompt; slash-command syntax may be an optional host-specific convenience.
- Exercise run-mailven start/wait/check/stop with fake-data Mail checks in both Accounts states; verify cleanup/state restoration and report native-window limitations. Follow existing safeguards for the real account-backed dev server.
- Document canonical edits, symlink requirements, path bases, invocation, and conditional host guidance in CONTRIBUTING.md. Compare available context diagnostics (Claude `/context`, Codex's available diagnostics) on equivalent fresh tasks. Bytes/words are reproducible proxies, not measured tokens or proof of savings; record accuracy and extra file reads as well as context size.

Acceptance: both installed hosts discover and follow shared skills with no custom discovery configuration; bidirectional continuation works from repository state; context is reduced without lost rules; UI smoke checks pass. Unavailable checks remain explicitly incomplete.

Validation: inspect session evidence, symlink/resource resolution, checkpoint continuation, context measurements, and helper/UI results; run `git diff --check`. Application builds/tests are required only for relevant implementation risks. Follow AGENTS.md's milestone review, simplification, commit, and landing policy. Remove remaining AI2–AI5 only with acceptance evidence; prune completed plan detail while preserving D2.

### D2 — Upgrade TypeScript when supported

Acceptance: both published peer ranges accept TypeScript 7, the dependency graph is supported, and frontend lint, check, and build pass.

Validation: recheck stable versions and peer ranges with the commands in CONTRIBUTING.md before changing dependencies; then run `npm run lint`, `npm run check`, and `npm run build` in frontend/.

## Progress
- [ ] G1 — shared policy and workflow conflicts; ready to implement.
- [ ] G2 — directory migration and single symlink complete; workflow portability still pending after G1.
- [ ] G3 — Claude 2.1.294 discovery passed on 2026-10-12: fresh project startup registered all five skills and slash commands exactly once through the single symlink. Disposable-fixture invocation was blocked by the account weekly usage limit; full execution, Codex session, context, UI, and handoff checks remain pending after G2.
  - Evidence: `claude -p 'Reply OK. Do not invoke skills or use tools.' --tools '' --permission-mode plan --no-session-persistence --output-format stream-json --verbose`; inspected the `system/init` skill/command lists and terminated after initialization. No plugin/discovery flag. Local summary: `/tmp/mailven-skill-directory-discovery.json` (ephemeral).
  - Checks passed: every canonical resource and its linked path are the same file; `bash -n .agents/skills/run-mailven/dev.sh`, `sh -n .agents/skills/project-next-milestone/plan-outline.sh`, `node --check` on all six JS helpers, Python `compile()` on serve-fakes.py; plan-outline and `git -C <skill-path> rev-parse --show-toplevel` through both paths; moved-file byte comparison allowing only canonical path substitutions; `git diff --check`. No app/UI check ran for this directory-only change.
- [ ] D2 — blocked upstream as of the earlier 2026-10-12 dependency review. `npm view` reported typescript-eslint 8.71.1 requires `<6.1.0`; svelte-check 4.7.6 accepts only 5/6. Not rechecked in this AI-guidance review.

## Surprises & Discoveries
- Verified locally on 2026-10-12: Codex CLI 0.162.1; Claude Code 2.1.294. Current guidance: Codex discovers repository .agents/skills/; Claude discovers .claude/skills/. Sharing the Agent Skills format does not imply shared discovery paths or preprocessing.
- Current Claude documentation supports native AGENTS.md with conditional fallback and user settings, but explicitly supports retaining CLAUDE.md's import. Codex's startup instruction chain runs from repository root to cwd; Claude also has on-demand subdirectory loading. Neither guarantees that arbitrary linked files are automatically read.
- Baseline: AGENTS.md 8,067 bytes, CLAUDE.md 274 bytes, run-mailven SKILL.md 18,551 bytes. Other unscoped rules and skill metadata also affect context. Token savings and full workflow behavior remain unverified; see G3 for the discovery trial.

## Decision Log
- User selected a trial of one directory symlink, superseding the wrapper preference. Canonical skills live in .agents/skills/; .claude/skills points to ../.agents/skills. Maintain no wrapper or copied skill trees. Require symlink-capable clones; use no custom discovery flags.
- Keep CLAUDE.md's import for compatibility. Required behavior stays in shared policy; skill discovery/implicit matching alone cannot enforce mandatory checks. Use conditional host-specific text in shared files where small and optional references where lengthy.
- A Claude plugin loaded via `claude --plugin-dir ./.agents` could share the same directory without wrappers, but requires special loading and namespaced commands. Rejected because normal CLI startup is preferred; generated full copies are also rejected.
- Treat PLAN.md as the portable handoff contract; native resume/compaction/auto-memory are host-specific conveniences. Keep checkpoint state concise and prune obsolete history.
- Official sources checked 2026-10-12: [Codex instructions](https://learn.chatgpt.com/docs/agent-configuration/agents-md), [Codex skills](https://learn.chatgpt.com/docs/build-skills), [Codex configuration](https://learn.chatgpt.com/docs/config-file/config-reference), [Claude instructions](https://code.claude.com/docs/en/memory), [Claude skills](https://code.claude.com/docs/en/skills), [Claude plugin paths](https://code.claude.com/docs/en/plugins-reference), [Claude best practices](https://code.claude.com/docs/en/best-practices), and [OpenAI skill/context guidance](https://developers.openai.com/blog/rethinking-skills-and-prompts-for-gpt-6-astra). Recheck relevant behavior when versions change; validate installed hosts in G3.
- Keep TypeScript ~6.0.3 until both tools support 7.

## Outcomes & Retrospective
All five skill directories now have one canonical tree and one Claude discovery symlink. Instructions/resources were preserved apart from canonical path references. Fresh Claude startup in the project confirms all five skills and slash commands register once; the disposable invocation hit account quota before model execution. Workflow porting, context savings, bidirectional continuation, and UI checks remain pending G1–G3.
