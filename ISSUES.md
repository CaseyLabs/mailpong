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

### App layer

- **A4 medium** `app/extension_contacts.go:32-52` and contacts backend
  imports: EXT_RULES R1, R2 and R5. The bridge gets a token closure and the
  core DB, and the extension imports `internal/*`. This is architectural
  (see C12).

### Contacts

- **C12 medium** contacts backend: EXT_RULES R1, R2, R5 and R6 (see A4).
  Architectural.

### Calendar

- **C17 low** DAV certificate follow-ups: the setup dialogs now offer the
  certificate-accept prompt, but a certificate that changes on an existing
  CardDAV/CalDAV source's background sync only fails the sync (mail sync
  prompts), and certificates accepted permanently for DAV hosts don't show in
  the account Security tab's trusted-certificate list, which lists mail hosts
  only. Deeper fix: stamp the host on `certificate.Error` in
  `certificate.Transport` and return it structurally from the DAV bindings,
  replacing the frontend's error-string regex and the second `Probe` request
  (which also misses certificates only discovery redirects reach).

### Chat list

- **M14 low** `app/chat.go` `moveStaleChatState`: splits from a changed
  combine default (`app/settings.go`) or the startup reconcile still move a
  sender chat's draft link to the newest thread, not to the thread holding its
  reply target as `splitSenderChat` does. Fix idea: give `ChatKeyMove` a
  `LinkTo` worked out in `StaleChatKeys`, and have `splitSenderChat` use the
  same planner.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript is held at 6.0.x.
  TypeScript 7 is blocked by peer ranges: `typescript-eslint` requires
  `<6.1.0` and `svelte-check` requires `^5 || ^6`. Upgrade once both accept 7.

## Recurring problems

Format: `- symptom: workaround (count)`. Promote to AGENTS.md or a skill and delete the line when a problem recurs or proves costly.

- Claude-in-Chrome tab stops answering (CDP timeout) during a long UI check: switch to the headless Playwright path (`pw.js`) (1)
- `rm` in the agent shell is a confirming wrapper that aborts when non-interactive: use `git rm` for tracked files, `command rm` otherwise (1)
