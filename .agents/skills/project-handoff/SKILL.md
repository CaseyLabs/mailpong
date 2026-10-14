---
name: project-handoff
description: Prepare a repository-backed handoff for a fresh session in either host. Use when asked to hand off work or provide a next-session prompt.
---

Prepare a handoff using the user's requested focus, if any. Resolve the active checkout with `git rev-parse --show-toplevel`; resolve user-supplied relative plan paths from the invocation working directory.

1. **Questions.** Check recorded decisions, then batch material open questions using the host's question capability or plain text. Record unanswered blockers explicitly.
2. **Persist first.** Follow [the lifecycle rules](../../../docs/PROJECT_MANAGEMENT.md). Reconcile the selected milestone and validation, updating its linked ExecPlan (or inline checkpoint for small work) and brief roadmap summary. Include stable project/milestone/issue IDs, current step, changed files, remaining work, decisions, exact check results/unrun checks and authorization limits. Preserve context needed to resume. Record branch and pre-commit base revision as such, intended commit/push status and dirty work that must remain. With no active work, report candidates and gates without changing statuses. For work without a roadmap milestone, use an existing task document or concise handoff document.

3. **Commit.** If commits are prohibited, keep the checkpoint uncommitted and report its path and dirty state. Otherwise commit only verified task changes including the checkpoint, under project policy and current authorization. Preserve unrelated or unverified work and list it. Follow applicable push/landing policy. Capture the resulting revision and `git status --short --branch` for the prompt; do not make a second commit just to embed the first commit's hash.
4. **Prompt.** Emit one paste-ready fenced block that works in either host: checkout/branch/revision and dirty state; instruction and checkpoint paths to read; project/milestone IDs and next authorized concrete action; validation evidence and unrun checks; blockers and pitfalls; permitted scope and commit/push status. For planned work, ask the next session to use the project-next-milestone skill with the roadmap milestone ID and checkpoint path, but never substitute a host-specific command for this state.
5. Tell the user to start a fresh session and paste the prompt. Native resume or clear commands are optional host conveniences, not the continuation contract.
