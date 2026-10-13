---
name: project-next-milestone
description: Implement one actionable milestone from an execution plan, then verify and commit it. Use when asked to continue or resume a plan.
# Claude-only turn override; Codex retains its selected session model.
model: sonnet
---

Implement one milestone only, then stop. Use the requested plan path, defaulting to `PLAN.md`. Resolve relative user paths from the shell working directory at invocation, before changing directories. Skill discovery does not change cwd.

The `model` frontmatter is a Claude-only override. In Codex, retain the selected session model and effort; do not try to select Sonnet. No skill-specific effort is set in Claude.

## Start

1. Resolve the active checkout with `git rev-parse --show-toplevel`. Use its canonical `.agents/skills/` resources, including in worktrees. Run `sh "$repo/.agents/skills/project-next-milestone/plan-outline.sh" "$plan"` with quoted absolute repo and plan paths, even when the requested plan appears absent. The helper owns the missing-plan check; do not substitute `test -f` or another preflight that skips it. A missing plan is a blocker; report the helper's result rather than inventing work.
2. Read only Scope, Progress, and the first unfinished milestone's section using the outline's line numbers, plus project-required sections and relevant Decision Log / Surprises entries. Skip explicitly blocked milestones only when a later milestone is independent and actionable; report skipped blockers. If none is actionable, report why and stop. If premises no longer match the code, report that the plan needs revising.
3. Resume existing sub-step checkboxes; otherwise add them beneath the milestone's Progress line. First map each acceptance criterion to its proving command or test, adding a missing test when needed. Record the base commit and tick sub-steps with timestamps.

## Work

Search for existing implementation before adding code. Complete each sub-step without stubs or fake returns. Keep Progress, relevant decisions, discoveries, changed files and exact validation results sufficient for continuation without chat history. Record baseline failures and confirmed deferred findings under project policy.

Ask only material unresolved questions after checking recorded decisions. Use the host's available question tool or plain text, batching questions; continue independent work. Respect runtime restrictions and execution authorization.

## Finish

Read the active checkout's `.agents/skills/project-finish-milestone/SKILL.md` and perform that workflow using available capabilities. A slash-command dispatcher is not required. Stop after its summary; do not start a second milestone.
