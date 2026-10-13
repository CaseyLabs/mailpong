# Deferred AI verification and dependency work

## Scope
The shared AI guidance branch is closed for now at the user's request (2026-10-12). G1–G3 delivered consolidated policy, five canonical portable workflows, and Codex discovery/compliance, trusted-clone routing, repository-backed continuation and guarded UI verification. Retained evidence and limitations are in [the verification record](docs/validation/shared-skills-g3.md).

The remaining G4 verification and D2 dependency work are deferred to future branches. No milestone is active; do not resume these items automatically. Claude runtime compliance, bidirectional continuation and comparable context savings remain unverified; deferral does not close AI5 or imply full cross-host acceptance. Application behavior, permissions, installed dependencies and personal CLI configuration are unchanged by this closeout.

## Milestones

### G4 — Claude and cross-host verification (AI5; to do later)

Deferred by the user on 2026-10-12 to a future branch. Resume only when requested and Claude inference is available; do not probe quota or enable paid access merely to advance this work.

- Use fresh installed Claude Code root/subdirectory sessions in a symlink-preserving disposable clone, without custom discovery flags or personal skill installations. Retain prior discovery evidence unless host/config/guidance changes; prove shared policy loading and unique registrations where checks have become stale.
- Verify explicit selection, representative natural triggers/non-triggers, canonical body and linked reference reads, native acceptance/worker selection and actual applied Sonnet Low/Medium settings, plus the next-milestone Sonnet turn override. Use read-only smoke tasks or disposable fixtures; do not push or advance real milestones for tests.
- Test Claude-to-Codex and Codex-to-Claude continuation using only tracked guidance, PLAN.md checkpoints and dirty working-tree inspection. Each receiver must identify the next action, respect constraints and retain valid checks. Use plain-language handoff prompts; host-specific slash syntax is optional.
- Compare available Claude `/context` and Codex diagnostics on equivalent fresh tasks and a comparable baseline, recording accuracy and extra reads. Distinguish token/context measurements from byte/word proxies and full-turn usage; do not infer savings from quota errors or incompatible counters.

Acceptance: Claude discovers and follows shared skills without custom discovery configuration; both cross-host continuation directions work from repository state, including uncommitted work; comparable context evidence shows reduction without lost rules. Keep unavailable checks explicitly incomplete and AI5 open until continuation passes.

Validation: record installed versions, cwd, exact prompts/commands, source paths, covered revisions, selected native model/effort, context methodology/results and limitations. Inspect session/checkpoint evidence and run `git diff --check`; repeat prior Codex/UI checks only if relevant implementation changes invalidate them. Finish the independent review and milestone workflow before completing G4 on its future branch.

### D2 — Upgrade TypeScript when supported

Acceptance: both published peer ranges accept TypeScript 7, the dependency graph is supported, and frontend lint, check, and build pass.

Validation: recheck stable versions and peer ranges with the commands in CONTRIBUTING.md before changing dependencies; then run `npm run lint`, `npm run check`, and `npm run build` in frontend/.

## Progress
- [ ] G4 — deferred until explicitly requested: Claude workflow/policy/reference compliance, native routing and milestone override, bidirectional continuation (AI5), comparable context/accuracy evidence. Retain prior Codex/UI evidence unless relevant changes invalidate it.
- [ ] D2 — deferred; blocked by the peer ranges recorded on 2026-10-12 in ISSUES.md. Recheck compatibility before any upgrade.

## Surprises & Discoveries
- Skill discovery alone does not prove native-agent loading. Exact-checkout trust is required for Codex project roles; the trusted-clone proof and diagnosis are recorded in docs/validation/shared-skills-g3.md and CONTRIBUTING.md.
- Cross-host context savings cannot be inferred from file sizes, incompatible usage counters or quota errors. G4 must measure comparable tasks and accuracy.

## Decision Log
- Closeout (2026-10-12): the user's explicit merge/push request supersedes earlier local-checkpoint-only and hold-until-G4 decisions. Land the completed scope now; keep G4/AI5 and D2 open for later work.
- Canonical skills remain in .agents/skills/ with one relative .claude/skills directory symlink. Keep CLAUDE.md's AGENTS.md import; edit canonical resources and preserve symlink-capable clones.
- Host adapters own model/effort settings. The documented Claude-only next-milestone `model: sonnet` field is the one shared-frontmatter exception; current routing is documented in CONTRIBUTING.md.
- Keep TypeScript ~6.0.3 until both lint and Svelte check tools accept a newer major.

## Outcomes & Retrospective
G1–G3 are complete; completed milestone checklists and obsolete checkpoints have been pruned. The user authorized landing the completed scope by squash merge and push to main; closeout helper/static checks and independent read-only review passed (see the verification record). G4/AI5 remain incomplete and D2 remains blocked; follow-up work starts on a new branch only when requested. Historical validation records retain their original revision coverage and limitations.
