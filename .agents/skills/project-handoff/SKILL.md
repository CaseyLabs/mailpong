---
name: project-handoff
description: Hand remaining work to a fresh Claude Code session - commit verified work, record open questions, and print a paste-ready prompt for the next session. Use when the user asks for a prompt for the next or a new session, a handoff, or to wrap up before /clear.
argument-hint: "[focus for the next session]"
---

Prepare a handoff so a fresh session can finish the remaining work without this conversation. $ARGUMENTS

1. **Open questions.** If there are decisions only the user can make, ask them now through AskUserQuestion (batched, recommended option first), so the answers go into the handoff instead of the next session.
2. **Commit.** Commit verified changes locally, unless the project's instructions say commits need approval, in which case list the uncommitted files instead. Never push. Note the branch, and whether it still needs merging.
3. **Persist state.** If the project has a plan or checklist file, update its progress and decision sections so they are the source of truth. If the work follows a plan with milestones, the next-session prompt can just be `/project-next-milestone [plan file]`; say so and skip step 4.
4. **Write the prompt** as one fenced markdown block the user can paste. It must stand alone:
   - **Read first:** the instruction files and docs the next session must read, and any privacy or safety rules to follow.
   - **State at handoff (date):** what's done and verified, with the exact commands or artifacts that prove it; branch and commit.
   - **Outstanding work:** numbered, concrete steps with file paths, commands, IDs, and counts. No references to "earlier" or "as discussed".
   - **Pitfalls:** mistakes or dead ends from this session, so they aren't repeated.
   - **Decisions for the user:** questions still open, to be asked through AskUserQuestion.
   - **Finish with:** what the next session should report and whether it may commit.
   Keep it general enough to reuse if the next session doesn't finish everything.
5. Tell the user to run `/clear` (or start a new session) and paste the prompt.
