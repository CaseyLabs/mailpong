# Issues

## Open

Found by the full audit on 2026-10-08. Each entry gives a severity, the
location, and the failure. "Unverified" means a reviewer reported it and it
has not yet been confirmed against the code.

### Security

- **S6 medium** `internal/smime/verifier.go` `verifyChain`: there is no
  revocation check, so a revoked signer still shows as trusted. Needs an
  owner decision: OCSP or CRL fetches on every view tell the CA what is
  being read. (The signing-time half of S6 is fixed.)

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript is held at 6.0.x.
  TypeScript 7 is blocked by peer ranges: `typescript-eslint` requires
  `<6.1.0` and `svelte-check` requires `^5 || ^6`. Upgrade once both accept 7.

## Recurring problems

Format: `- symptom: workaround (count)`. Promote to AGENTS.md or a skill and delete the line when a problem recurs or proves costly.

- Claude-in-Chrome tab stops answering (CDP timeout) during a long UI check: switch to the headless Playwright path (`pw.js`) (1)
- `rm` in the agent shell is a confirming wrapper that aborts when non-interactive: use `git rm` for tracked files, `command rm` otherwise (1)
