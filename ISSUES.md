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
- **A6 low** `internal/sync/fetch.go:259` and `:815`: cancelling a sync (found
  in the 2026-10-12 Flatpak debug log) logs four warnings: "Failed to read
  body literal" (`use of closed network connection`), "Incomplete message
  body, not storing", "Body literal ended early" and "Batch fetch failed with
  non-connection error" (`context canceled`). Nothing is wrong, but an
  intentional cancel should log at debug, not warn, so real warnings stand out.

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

### Packaging

- **P1 low** `internal/oauth2/config.go:62`: the Flatpak shim path
  `/app/lib/mailven/mailven-creds` is not installed by any Flatpak manifest, so
  Flatpak builds never load shim credentials. Pre-existing; install the shim there or drop the path.
- **P2 low** `build/README.md:10`, `Makefile`: macOS scripts hard-code `Mailven.app` while Wails
  names the bundle from `wails.json` `name` (`mailven`); works only on case-insensitive volumes.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript is held at 6.0.x.
  TypeScript 7 is blocked by peer ranges: `typescript-eslint` requires
  `<6.1.0` and `svelte-check` requires `^5 || ^6`. Upgrade once both accept 7.

## Recurring problems

Format: `- symptom: workaround (count)`. Promote to AGENTS.md or a skill and delete the line when a problem recurs or proves costly.

- Claude-in-Chrome tab stops answering (CDP timeout) during a long UI check: switch to the headless Playwright path (`pw.js`) (1)
- `rm` in the agent shell is a confirming wrapper that aborts when non-interactive: use `git rm` for tracked files, `command rm` otherwise (1)
