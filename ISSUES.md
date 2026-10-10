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

- **M6 low** `app/idle_sent.go`: Sent activity from other clients is
  inferred from inbox IDLE events, so a new (non-reply) message sent
  elsewhere reaches Sent and its chat thread only on the next scheduled
  sync. Deeper fix: IDLE on the Sent folder too (a second connection, or a
  folder set in `internal/imap`'s IDLE) routed to the same incremental sync,
  which would also retire the 30 s throttle state; cheaper: a shorter Sent
  poll interval.
- **M10 low** `app/app.go` `Shutdown`: running folder syncs are never
  cancelled or awaited, so a background body fetch (`app/sync.go`
  `FetchBodiesInBackground` goroutine) outlives teardown. Seen in a Flatpak
  debug run 2026-10-10: `imapPool.CloseAll()` cut a 2.5 MB body literal
  mid-read, the "after broken stream" retry asked the pool for a new
  connection, and credential lookup failed with `sql: database is closed`
  just before `a.db.Close()` logged. Harmless today (the partial body is not
  stored), but the retry can race the DB close. Fix: call `CancelAllSyncs`
  early in `Shutdown` and wait (bounded) for folder sync slots to release
  before closing the pool and DB; optionally make `Pool.GetConnection`
  refuse new connections after `CloseAll`.

### Chat list

- **M7 low** `internal/message/chat_store.go` `StaleChatKeys`: turning
  **Combine messages from each sender** off drops a sender chat's pin,
  snooze and chat draft link instead of handing them back to one of its
  threads, so they are lost.
- **M8 low, unverified** `chatKeyRows`: while the combine default is on,
  every chat list, count, snooze wake and new-mail lookup takes the window
  pass over the whole scope (`hasSenderChats` skips it only when the
  default is off and no sender is combined). Not timed on a large mailbox;
  if slow, index or cache thread starters.

- **M9 low, unverified** chat UI (mail UI polish, merged 2026-10-10): checks
  not yet done in the running app: light themes (built-in and Adwaita
  hover and selection), measured row heights (target 72-80px with
  preview), selection switching solid/neutral on pane focus, signed,
  encrypted and bad-signature messages, the Settings > Chat combine
  switch, and the list giving way to the reading pane as the window
  narrows.

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
