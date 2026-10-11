---
name: project-next-milestone
description: Implement the next unticked milestone of an execution plan (PLAN.md by default), one milestone per session, then stop. Use when the user asks to continue, resume, or implement the next milestone or step of a plan file.
argument-hint: "[plan file, default PLAN.md]"
model: sonnet
allowed-tools: Bash(sh ${CLAUDE_SKILL_DIR}/plan-outline.sh *)
---

Implement the next milestone in the plan file: the first one in Progress that isn't ticked. Do that one milestone only, then stop.

## Plan outline

!`sh ${CLAUDE_SKILL_DIR}/plan-outline.sh "$ARGUMENTS"`

## Start

- Don't read the whole plan. Using the outline's line numbers, read only: that milestone's section, any section the project's CLAUDE.md or AGENTS.md says to read every session (for example "Running builds safely"), and the Decision Log or Surprises entries the milestone depends on (grep for them).
- Prefer the current checkout over paths or line numbers quoted in the plan.
- If a previous session left the milestone partly done, resume from its sub-step checkboxes. Otherwise add indented sub-step checkboxes under the milestone's Progress line, and tick each with a timestamp as you finish it. Make the first sub-step map each acceptance criterion to the command or test that will prove it; write a missing test as part of the milestone.
- If the milestone's premises no longer match the code (it's already done, or what it builds on has changed), stop and say the plan needs revising rather than forcing it.

## While working

- Follow the project's CLAUDE.md and AGENTS.md rules (launch helpers, read-only directories, test servers, porting ledgers).
- Where the plan says to use plan mode, don't enter it. Write the design and its reasoning into the Decision Log, then implement it.
- Keep the plan's living sections (Progress, Surprises & Discoveries, Decision Log) current as you go. They are the hand-off if context gets compacted.
- Before writing new code for a sub-step, search for an existing implementation (Grep, or Explore for broad sweeps). Don't assume it's missing; extend what exists.
- Implement each sub-step fully: no stubs, TODOs, or fake returns. If something must be deferred, add it to Progress as an unticked item.
- Copy files with shell commands rather than reading them into context and retyping them.
- If a failure already exists in the baseline, record it and carry on.

## Questions

- Check the Decision Log first; most questions are already answered there.
- Ask only about real blockers: a missing secret, a system install not already approved, an unsettled identity or namespace choice, or a product decision that changes scope.
- Ask through AskUserQuestion, batching pending questions up to 4 per prompt, recommended option first.
- Put steps only a person can check into one numbered checklist, ask once, and record the answers in Surprises & Discoveries.

## Finish

Run `/project-finish-milestone`, then stop with its summary.
