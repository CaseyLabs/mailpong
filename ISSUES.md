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

### Mail sync

- **M11 low** `app/sync.go` `shutdownSyncs`: it waits only for slots still in
  `syncContexts`. `CancelAllSyncs` replaces that map, so a sync cancelled just
  before quit (network change, wake) is not awaited and can still touch the DB
  as it closes. The closed pool stops its connections. Fix: keep cancelled
  slots' `done` channels until released. IDLE handlers and `SyncFolders` are
  awaited through `trackSync`; draft, undo and action goroutines are not,
  only cut off by `Pool.Close`.

### Chat list

- **M12 low** `app/chat.go` `splitSenderChat`: the chat draft link moves to
  the newest thread the sender started, but its `MessageID` (the reply
  target) can sit in another thread of the old sender chat, so the reply
  threads under that message. Undoing a split restores the sender chat's
  pin and snooze but leaves the pin and link on the thread. A failing
  `MoveChatLink` after `MoveChatState` rolls the choice back with the pin
  already moved.
- **M12 low** chat list timing at 50k inbox messages (benchmark
  `BenchmarkChatQueries`): `ListChats` takes about 320 ms with combining off
  and 380 ms on, `CountChats` 110 ms off and 250 ms on. The grouped query
  with JSON aggregates over the whole inbox dominates; the sender-chat window
  pass adds about 60 and 140 ms. Fix by paging before aggregating, or by
  caching thread starters.

- **M9 low, unverified** chat UI: checked on fake data only (2026-10-10), all
  passing. Still unchecked with real mail: hover on real rows, and opening a
  chat in the narrow layout (the list-only layout was seen at 700px, but the
  reading pane opening was not, to avoid marking real mail read).

### Upstream independence

This fork may eventually stop tracking upstream Aerion. Until then, some code
is kept or left untouched only to keep upstream merges clean. Record each such
cleanup here, so it can be done once the fork is independent.

- **U1** frontend: delete the unmounted classic `MessageList`,
  `ConversationRow` and `ConversationViewer`, kept only to avoid merge
  conflicts. They hold unshared copies of chat-component logic (the inbox
  lookup, the sync toggle, trash-with-undo) that go with them; the chat
  side uses `accountStore.getFolder`/`getInbox` and
  `components/chat/chatTriage.ts`.
- **U3** non-English locale files: this fork updates English only, so the
  other locales drift. Remove them, or decide to maintain them.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript is held at 6.0.x.
  TypeScript 7 is blocked by peer ranges: `typescript-eslint` requires
  `<6.1.0` and `svelte-check` requires `^5 || ^6`. Upgrade once both accept 7.
