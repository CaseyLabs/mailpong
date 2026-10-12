---
name: project-ship
description: Commit current changes, push the branch, and create or update its pull request description. Use when the user asks to commit and push, update the PR or its description, open a draft PR, or merge a branch to main and push.
argument-hint: "[draft | merge | extra instructions]"
allowed-tools: Bash(git status *) Bash(git diff *) Bash(git log *) Bash(git add *) Bash(git commit *) Bash(git push *) Bash(gh pr *)
---

Ship the current work. $ARGUMENTS

## Current state

!`git status --short --branch; echo; git log --oneline -5; echo; gh pr view --json number,title,isDraft,baseRefName,url 2>/dev/null || echo "No PR for this branch."`

## Steps

1. **Guardrails.** If the project's settings deny `git push`, or its CLAUDE.md/AGENTS.md says not to push, commit locally and stop. Never push to a branch the user didn't ask for. Don't force-push unless the user asked.
2. **Commit.** Stage only files that belong to this change, and leave unrelated edits unstaged (mention them). Follow the repo's commit message style from `git log`. No Claude co-author trailer and no "Generated with" line.
3. **Push** the current branch, setting upstream if needed.
4. **PR.**
   - If a PR exists, rewrite its description so it describes the branch as it is now (summary, notable changes, testing), not as a changelog of review rounds.
   - If none exists and the user asked for one, create it (draft if they said draft or the target is an upstream repo). Use the repo's PR template and any PR skill in the project (such as `pr-draft-summary`) if present.
   - No "Generated with Claude Code" line or other attribution.
5. **Merge** only when the user asked ("merge to main"): merge locally, run the project's tests, then push main.
6. Reply with the commit hash, branch, and PR URL.
