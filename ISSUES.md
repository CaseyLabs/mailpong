# Issues

## Deferred follow-ups

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

- **AI5 medium** `.agents/skills/project-handoff/SKILL.md`: G2 corrected checkpoint-before-commit ordering and now requires branch/revision, dirty state, validation provenance and blockers in the shared handoff. Bidirectional continuation, including uncommitted work, is still unverified; complete the deferred bidirectional runtime trials in PLAN.md G4 on a future branch before closing this item. The user closed the current branch on 2026-10-12; resume only when requested.

## Recurring problems

None.
