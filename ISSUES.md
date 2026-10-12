# Issues

## Open

Found by the full audit on 2026-10-08. Each entry gives a severity, the
location, and the failure. "Unverified" means a reviewer reported it and it
has not yet been confirmed against the code.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript 7 remains blocked upstream.
  Verified on 2026-10-12 with `npm view <package>@latest version peerDependencies --json`:
  TypeScript 7.0.2 is published, but latest `typescript-eslint` 8.71.1 requires
  `>=4.8.4 <6.1.0` and `svelte-check` 4.7.6 requires `^5.0.0 || ^6.0.0`.
  Both tools are already current here; retain TypeScript ~6.0.3 until both accept 7.
  Recheck using the commands in `CONTRIBUTING.md`.

### AI guidance portability (reviewed 2026-10-12)

- **AI1 medium** `.claude/skills/`: all five project skills exist only in Claude's
  discovery location; the repository has no `.agents/skills/` entries for Codex
  automatic discovery. Move implementations to `.agents/skills/` and keep thin
  Claude discovery wrappers, as selected in PLAN.md; no symlinks or setup.
- **AI2 medium** `.claude/skills/project-next-milestone/SKILL.md`,
  `project-ship/SKILL.md`, `project-finish-milestone/SKILL.md`,
  `project-handoff/SKILL.md`, and `run-mailven/SKILL.md`: workflows depend on
  Claude-specific interpolation, dynamic command injection, slash commands,
  named subagents, or browser tools. Merely exposing these files to Codex does
  not provide equivalent execution. Convert shared steps to ordinary instructions
  and explicit script calls, with small adapters for host-specific capabilities.
- **AI3 low** `.claude/rules/working-style.md`: shared preferences such as
  serializing builds/tests across agents and commit/PR attribution are stored
  only in Claude rules and are not automatically loaded as Codex project rules.
  Consolidate shared policy in `AGENTS.md`; retain only Claude-specific behavior
  in Claude adapters. The extension rule also duplicates `AGENTS.md` guidance.
- **AI4 medium** `.claude/skills/project-ship/SKILL.md:23` and
  `.claude/skills/project-finish-milestone/SKILL.md:15`: merge-only-on-request and
  the conditional simplification step conflict with `AGENTS.md`'s automatic
  branch landing and required milestone simplification. `project-handoff` also
  says never to push, conflicting with milestone push policy when applicable;
  `project-next-milestone` tells the agent to disregard requested plan mode.
  Make workflows respect runtime/user instructions and shared project policy.

- **AI5 medium** `.claude/skills/project-handoff/SKILL.md:11`: commits before
  persisting the plan checkpoint, so the resulting commit can omit the latest
  continuation state. Its planned-work handoff reduces to a Claude slash command,
  without requiring branch/revision, dirty state, validation provenance, or
  blockers in the shared checkpoint. Persist state before committing and verify
  bidirectional continuation from repository files, including uncommitted work.

## Recurring problems

None.
