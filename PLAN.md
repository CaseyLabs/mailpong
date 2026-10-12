# ISSUES.md remediation

## Scope
Review and fix every finding in ISSUES.md on separate branches. Run `codex review --base main` (the CLI `/review` equivalent) and resolve findings before each squash merge and push. Preserve unrelated commits and files.

## Milestones
- A4/C12: enforce extension boundaries for contacts, using coreapi and generic kit only. Validate extension contracts and full Go tests.
- C17: structured DAV certificate errors, background prompts, and trusted-certificate management. Validate DAV tests, frontend checks, and UI.
- S6: validate certificate revocation with privacy preference. Validate generated certificate/CRL cases and settings UI.
- D2: verify published TypeScript tooling compatibility; upgrade only if supported. Validate lint, check, and build.
- Recurring tooling: incorporate reliable alternatives in current guidance and clear obsolete tallies.

## Progress
- [ ] A4/C12
- [ ] C17
- [ ] S6
- [ ] D2
- [ ] Recurring tooling

## Surprises & Discoveries
- Starting main contains the user's unpushed `889b417 Update AGENTS.md` commit.
- `codex review --base main` is available for the requested branch reviews.

## Decision Log
- Owner selected opt-in online revocation checking in Settings.

## Outcomes & Retrospective
A6: focused and full sync tests passed; `codex review --base main` caught a connection cleanup gap, fixed; repeat review passed. Review logs: `/tmp/mailven-issues-review/A6*.log`.

P1/P2: OAuth tests and Darwin dry runs passed; final `codex review --base main` passed after correcting remaining guide paths and removing the Node dependency from uninstall. Logs: `/tmp/mailven-issues-review/P*.log`.

M14: default/startup and existing split regression cases passed; `make test` passed; `codex review --base main` passed. Logs: `/tmp/mailven-issues-review/M14*.log`.
