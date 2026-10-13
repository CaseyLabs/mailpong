---
name: project-ship
description: Commit current changes, push the branch, and create or update its pull request description. Use when the user asks to commit and push, update the PR or its description, open a draft PR, or merge a branch to main and push.
---

Ship the work in the user's requested mode (for example draft PR or merge), within existing authorization.

## Current state

Resolve the active checkout with `git rev-parse --show-toplevel` and run repository operations there. Inspect `git status --short --branch`, `git diff`, and `git log --oneline -5` explicitly. If GitHub CLI is available, query `gh pr view --json number,title,isDraft,baseRefName,url`. Distinguish no PR from authentication/network errors; report a missing capability rather than treating it as an absent PR.

## Steps

1. **Guardrails.** If the project's settings deny `git push`, or its CLAUDE.md/AGENTS.md says not to push, commit locally and stop. Push only the branch authorized by the user or shared project policy. Runtime constraints and explicit user instructions take precedence. Don't force-push unless the user asked.
2. **Commit.** Stage only files that belong to this change, and leave unrelated edits unstaged (mention them). Follow the repo's commit message style from `git log`. No assistant co-author trailer and no "Generated with" line.
3. **Push** the current branch, setting upstream if needed.
4. **PR.**
   - If a PR exists, rewrite its description so it describes the branch as it is now (summary, notable changes, testing), not as a changelog of review rounds.
   - If none exists and the user asked for one, create it (draft if they said draft or the target is an upstream repo). Use the repo's PR template and any PR skill in the project (such as `pr-draft-summary`) if present.
   - No generated-by line or other attribution.
5. **Land.** Follow AGENTS.md's Landing a Branch procedure when all feature milestones or the audit pass are complete, or when explicitly requested. Respect runtime constraints and explicit user instructions; run the relevant checks and push the resulting main branch when authorized by project policy.
6. Reply with the commit hash, branch, and PR URL.
