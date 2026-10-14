---
name: project-handoff
description: Prepare a repository-backed handoff for a fresh session in either host. Use when asked to hand off work or provide a next-session prompt.
---

Prepare a handoff using the user's requested focus, if any. Resolve the active checkout with `git rev-parse --show-toplevel`; resolve user-supplied relative plan paths from the invocation working directory.

1. **Questions.** Check recorded decisions, then batch material open questions using the host's question capability or plain text. Record unanswered blockers explicitly.
2. **Persist first.** Before committing, reconcile the plan with current implementation and validation, applying AGENTS.md's Planning Files preservation and pruning rules. Update Progress, Decision Log and relevant discoveries with current milestone/sub-step, changed files, completed work needed for continuation, remaining steps, blockers, exact validation commands/results, and any checks not run. Preserve outstanding verification and authorization limits; do not prune context needed to resume unfinished work. Record the branch and pre-commit base revision; label them as such so the checkpoint does not pretend to contain its own future commit hash. Record intended commit/push/merge status and any uncommitted work that must remain. For work without a plan, use an existing task document or create a concise handoff document.
3. **Commit.** Commit only verified task changes including the checkpoint, under project policy and current authorization. Preserve unrelated or unverified work and list it. Follow applicable push/landing policy. Capture the resulting revision and `git status --short --branch` for the prompt; do not make a second commit just to embed the first commit's hash.
4. **Prompt.** Emit one paste-ready fenced block that works in either host: checkout/branch/revision and dirty state; instruction and checkpoint paths to read; current milestone and next concrete action; validation evidence and unrun checks; blockers and pitfalls; permitted scope and commit/push status. For planned work, ask the next session to use the project-next-milestone skill with the plan path, but never substitute a host-specific command for this state.
5. Tell the user to start a fresh session and paste the prompt. Native resume or clear commands are optional host conveniences, not the continuation contract.
