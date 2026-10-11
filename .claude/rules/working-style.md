# Working style

Owner preferences carried into the repo so cloud sessions follow them too.

- When a step doesn't need the owner's input, keep going. Put status notes in the same message as your next action. Stop and ask only when you can't continue without the owner, or before anything destructive.
- For long multi-step tasks (migrations, audits, big refactors), keep a task checklist in a file and update it as you go, so progress survives context summarization.
- When working through a list (audit findings, issues, milestones), commit each item when it's verified and move straight to the next. Don't ask "should I continue?" between items.
- Ask questions through AskUserQuestion prompts, batching up to 4 per prompt with the recommended option first, rather than as a list in a message.

## Shell and context

- Start dev servers, watchers, and test mail servers with `run_in_background`, never in the foreground. Stop the ones you started before finishing.
- Keep command output small: pipe tests, builds, and logs through `tail`, `grep`, or `head`. When output is saved to a tool-results file, grep or `sed -n` the part you need instead of reading the whole file.
- For questions about Claude Code's own docs or features, use the claude-code-guide subagent so long doc pages stay out of the main context.

## Delegation

- Do trivial and single-file work directly; use local tools (grep, tests, linters, compilers) over extra model calls or model-based reviews.
- Delegate only when context isolation or specialization clearly pays off. One agent per question: don't send several agents over the same ground, and don't spawn planning, review, or summary agents unless asked.
- Use `project-worker` (Sonnet) instead of `general-purpose` or `claude` for delegated tasks. Agents started by `/code-review` and `/simplify` keep their own agent choice.
- Only one agent at a time runs builds or tests in a checkout; parallel agents may read and run light static checks (lint, vet, format) but not build or test.
- Run agents sequentially unless the tasks are independent. Don't use agent teams unless asked.
- If a delegated task fails or returns partial, change the approach before retrying. Keep architecture, integration, and security decisions in the main session, and verify agent claims that matter before acting on them.

## Attribution

- Never add "Generated with Claude Code" or any similar line to pull request descriptions.
- Never list Claude as a co-author or add any Claude/Anthropic trailer to commit messages.

## Compaction

When compacting, also preserve the checklist file's path and state and any user answers not yet written to a file.
