---
name: project-next-milestone
description: Select or resume one authorized roadmap milestone, execute it, then verify and close it out. Use when asked to continue planned project work.
# Claude-only turn override; Codex retains its selected session model.
model: sonnet
---

Implement one milestone only, then stop. Use the requested roadmap or execution-plan path, defaulting to the invocation directory’s `PLAN.md`. A roadmap path alone does not authorize its proposed or deferred work. Resolve relative user paths from the shell working directory at invocation, before changing directories. Skill discovery does not change cwd.

The `model` frontmatter is a Claude-only override. In Codex, retain the selected session model and effort; do not try to select Sonnet. No skill-specific effort is set in Claude.

## Start

1. Resolve the active checkout with `git rev-parse --show-toplevel`. Use its canonical `.agents/skills/` resources, including in worktrees. Run `sh "$repo/.agents/skills/project-next-milestone/plan-outline.sh" "$plan"` with quoted absolute repo and plan paths, even when the requested plan appears absent. The helper owns the missing-plan check; do not substitute `test -f` or another preflight that skips it. A missing plan is a blocker; report the helper's result rather than inventing work.
2. Read [the project-management lifecycle](../../../docs/PROJECT_MANAGEMENT.md), then use the outline to read Portfolio/Progress and the user-selected or already active project/milestone. Read only linked issue entries and relevant reference sections. For a legacy execution plan, read Scope/Progress and the authorized milestone. Do not select the first unfinished entry merely by file order. If a continuation request has no active/previously authorized milestone, report eligible candidates and blockers and obtain selection; do not activate deferred work. If the request clearly selects one scope, proceed without redundant permission.
3. Reconcile the selected milestone with implementation and validation; confirm prerequisites and gates. Revise stale premises within authorized scope; ask about material changes. Preserve outstanding verification. Create/link `docs/plans/<milestone-id>.md` only when complexity warrants it; otherwise checkpoint inline. Record authorization, base commit and current step, map acceptance criteria to proving checks, then update the selected statuses and progress. Explicit live/upgrade/migration gates still apply.


## Work

Search for existing implementation before adding code. Complete each sub-step without stubs or fake returns. Update the selected execution checkpoint and roadmap summary, relevant decisions, discoveries, changed files and exact validation results at meaningful sub-step completions, blockers, scope changes and check results, keeping them sufficient for continuation without chat history. Record baseline failures and confirmed deferred findings under project policy. If the host reports context usage nearing its compaction point before the milestone finishes, you may hand off at a sub-step boundary under [session boundaries](../../../docs/PROJECT_MANAGEMENT.md#session-boundaries) instead of continuing into compaction. If this session has already been compacted once, hand off at the next sub-step boundary; do not continue into a second compaction.

Ask only material unresolved questions after checking recorded decisions. Use the host's available question tool or plain text, batching questions; continue independent work. Respect runtime restrictions and execution authorization.

## Finish

Read the active checkout's `.agents/skills/project-finish-milestone/SKILL.md` and perform that workflow using available capabilities. A slash-command dispatcher is not required. Stop after its summary; do not start a second milestone. Any next milestone starts in a fresh session.
