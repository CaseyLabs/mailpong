# Provider deliverability review and deferred work

## Scope
The shared AI guidance branch is closed for now at the user's request (2026-10-12). G1–G3 delivered consolidated policy, five canonical portable workflows, and Codex discovery/compliance, trusted-clone routing, repository-backed continuation and guarded UI verification. Retained evidence and limitations are in [the verification record](docs/validation/shared-skills-g3.md).

The remaining G4 verification and D2 dependency work are deferred to future branches; do not resume these items automatically. Claude runtime compliance, bidirectional continuation and comparable context savings remain unverified; deferral does not close AI5 or imply full cross-host acceptance. Application behavior, permissions, installed dependencies and personal CLI configuration are unchanged by this closeout.

The Gmail and Microsoft 365 workstream is research-only: produce a source-backed assessment of Mailven's current integrations, compare protocol and API approaches, and recommend prioritized follow-up work. Focus on ordinary personal and business correspondence, including aliases and shared mailboxes. Retain user-supplied OAuth credentials. Include a proposed in-app diagnostic, without implementing it in this phase.

The objective is to reduce avoidable rejection and spam classification. Inbox placement cannot be guaranteed ([Gmail sender guidelines](https://support.google.com/mail/answer/81126?hl=en)). No application code, public APIs, dependencies, provider registrations, tenant policies, or DNS records change in this phase. Private console state and actual deliverability remain unverified until separately authorized live checks are available.

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
- [ ] Next-session planning handoff (2026-10-13): update this existing plan for provider implementation, making F1–F3 actionable and F4/F5 gated on architecture selection. Planning only; no implementation milestone is active. Pre-commit base `c92d9de` on `main`; handoff branch `docs/provider-implementation-handoff`. Only PLAN.md changes for this handoff; commit/push and land this checkpoint under project policy. Initial working tree was clean.
- [x] P1/P2 — provider research complete (2026-10-13): [audit](docs/validation/provider-deliverability-review.md) and [recommendations](docs/validation/provider-transport-recommendations.md). P2 base `d579059bd5af1ca898c619b5abd02f4923139643`; independent acceptance passed. Implementation awaits the user's architecture choice; PD1–PD9 remain open.
- [ ] G4 — deferred until explicitly requested: Claude workflow/policy/reference compliance, native routing and milestone override, bidirectional continuation (AI5), comparable context/accuracy evidence. Retain prior Codex/UI evidence unless relevant changes invalidate it.
- [ ] D2 — deferred; blocked by the peer ranges recorded on 2026-10-12 in ISSUES.md. Recheck compatibility before any upgrade.

## Surprises & Discoveries
- Handoff: main advanced to `c92d9de` (README update) after research landed at `355f466`; the checkout was clean at handoff. Research acceptance is historical evidence, not executable verification for future fixes.
- P2: API sending requires recipient mapping beyond Mailven's Bcc-stripped SMTP MIME; full Graph removes both protocol dependencies while a send-only hybrid retains IMAP restrictions. Graph delegation-copy settings can create multiple intended Sent destinations. These are implementation gates, not live-tested compatibility.
- P1: Outlook.com publishes `smtp-mail.outlook.com`, while Mailven uses the Exchange Online SMTP endpoint for every Microsoft account. Google documents separate physical/header-value limits (header-limit page updated 2026-10-06). Go already adds SMTPUTF8 when advertised, and Microsoft permits old refresh-token reuse after rotation; the report limits findings to what is proven. Live account policy and received-mail behavior remain unverified.
- Preliminary provider review (2026-10-12): Mailven uses OAuth with IMAP/SMTP for both providers; its Gmail API integration reads send-as settings rather than submitting mail. Microsoft tenant security defaults can disable SMTP AUTH. The research must distinguish protocol compatibility from deliverability and account for any remaining IMAP restrictions.
- Skill discovery alone does not prove native-agent loading. Exact-checkout trust is required for Codex project roles; the trusted-clone proof and diagnosis are recorded in docs/validation/shared-skills-g3.md and CONTRIBUTING.md.
- Cross-host context savings cannot be inferred from file sizes, incompatible usage counters or quota errors. G4 must measure comparable tasks and accuracy.

## Decision Log
- Next-session handoff (2026-10-13): use the P2 report's ordered follow-ups to replace the completed research scope with an implementation plan. Preserve G4/D2 deferrals and PD1–PD9. Start with common setup/OAuth accuracy (F1), send safety (F2), then identity/Sent/diagnostics (F3); recommendations are not approval to migrate transports. Give each milestone acceptance, compatibility and proving commands. After planning, a later implementation invocation should use project-next-milestone with this PLAN.md and complete one milestone only.
- P2 (2026-10-13): recommend hardened Gmail IMAP/SMTP and full delegated Graph for Microsoft 365; preserve user-supplied OAuth credentials and explicit existing-account compatibility. F1–F5 are proposals requiring architecture selection and a new implementation plan. Research scope is complete; do not resume implementation, G4 or D2 automatically. The unrelated README.md edit cleared before landing; land the clean completed research branch under project policy.
- P1 closeout: include the previously authorized uncommitted plan addition with this documentation milestone; preserve user-selected research-only scope. Use direct diff/document simplification review alongside independent native acceptance; no application-code simplification is applicable. P2 remains the next milestone; retain this branch until the provider workstream is complete.
- Provider review (2026-10-12): use the original research-only P1/P2 plan, as explicitly selected by the user. Cover ordinary correspondence, retain user-supplied OAuth credentials, compare approaches before selecting an architecture, and specify an in-app diagnostic as a follow-up recommendation. Do not add the revised gated API implementation milestones or implement provider integrations in this phase.
- Closeout (2026-10-12): the user's explicit merge/push request supersedes earlier local-checkpoint-only and hold-until-G4 decisions. Land the completed scope now; keep G4/AI5 and D2 open for later work.
- Canonical skills remain in .agents/skills/ with one relative .claude/skills directory symlink. Keep CLAUDE.md's AGENTS.md import; edit canonical resources and preserve symlink-capable clones.
- Host adapters own model/effort settings. The documented Claude-only next-milestone `model: sonnet` field is the one shared-frontmatter exception; current routing is documented in CONTRIBUTING.md.
- Keep TypeScript ~6.0.3 until both lint and Svelte check tools accept a newer major.

## Outcomes & Retrospective
Handoff checkpoint: `git diff --check -- PLAN.md` passed. No application builds/tests, UI or live-provider checks ran for this planning-only handoff; no implementation is claimed. Architecture remains unanswered for F4/F5 and does not block planning common F1–F3.

P1/P2 delivered the linked source-backed audit and six-option transport comparison, provider recommendations, common/API-specific hardening, proposed user-invoked diagnostic and ordered follow-up acceptance/test/live-check requirements. P1 evidence retains its original revision coverage. PD1–PD9 remain unfixed under the research-only scope.

P2 changed only PLAN.md and docs/validation/provider-transport-recommendations.md. All eight independent native acceptance criteria passed; no review fixes were needed. Direct diff/document simplification found no cleanup warranted. `git diff --check d579059bd5af1ca898c619b5abd02f4923139643 -- PLAN.md docs/validation/provider-transport-recommendations.md` passed; reference/runtime diff was empty and added-stub scan had no hits. Targeted Python checks passed six acceptance sections, PD1–PD9, ten regression scenarios and source bounds. No application builds/tests, UI/live-provider checks or executable baseline result were claimed; live identity/MIME/Bcc/Sent/delivery checks remain future gates. P2 committed/pushed as `ad68d81`; the unrelated README.md edit cleared and the completed research scope was squash-landed on main under project policy. No new confirmed deferred code findings were discovered; no recurring tooling problem required guidance changes.

G1–G3 are complete; completed milestone checklists and obsolete checkpoints have been pruned. The user authorized landing the completed scope by squash merge and push to main; closeout helper/static checks and independent read-only review passed (see the verification record). G4/AI5 remain incomplete and D2 remains blocked; follow-up work starts on a new branch only when requested. Historical validation records retain their original revision coverage and limitations.
