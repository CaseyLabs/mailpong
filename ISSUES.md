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

- **M12 low** chat list timing at 50k inbox messages with combining on
  (`BenchmarkChatQueries`): `ListChats` takes about 300 ms and `CountChats`
  230 ms. The sender-chat window pass in `chatKeyRows` (about 140 ms) and the
  grouping (about 90 ms) run over the whole inbox. Fix by caching thread
  starters in a table kept current by triggers on `messages`; the target is
  at most 188 ms (half the 376 ms baseline). Smaller levers, either
  combining setting: `chatGroups` joins `messages` and `folders` again
  although the key rows already read them, so the first pass and
  `CountChats` could carry the few columns they need instead; and each reload
  in `frontend/src/lib/stores/chat.svelte.ts` runs `GetChats`, `GetChatCount`
  and the Low count as separate full passes, which a `COUNT(*) OVER ()` in
  `chatPage` and one filtered count could merge.

- **M14 low** `app/chat.go` `moveStaleChatState`: splits from a changed
  combine default (`app/settings.go`) or the startup reconcile still move a
  sender chat's draft link to the newest thread, not to the thread holding its
  reply target as `splitSenderChat` does. Fix idea: give `ChatKeyMove` a
  `LinkTo` worked out in `StaleChatKeys`, and have `splitSenderChat` use the
  same planner.

- **M15 low** `frontend/src/lib/components/chat/chatFormat.ts` `replyAlias`:
  the reply box's "from" cannot see the alias that `selectReplyFromIdentity`
  picks from forwarding headers (Delivered-To, X-Original-To,
  X-Forwarded-For) or Bcc, which need an IMAP fetch. For such mail the line
  shows the account label (or nothing) while the reply goes out from the
  alias. Fix with a binding that returns the resolved identity.

### Upstream independence

This fork may eventually stop tracking upstream Aerion. Until then, some code
is kept or left untouched only to keep upstream merges clean. Record each such
cleanup here, so it can be done once the fork is independent.

None open.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript is held at 6.0.x.
  TypeScript 7 is blocked by peer ranges: `typescript-eslint` requires
  `<6.1.0` and `svelte-check` requires `^5 || ^6`. Upgrade once both accept 7.

## Recurring problems

Format: `- symptom: workaround (count)`. Promote to AGENTS.md or a skill and delete the line when a problem recurs or proves costly.

- None yet.
