# Issues

## Open

Found by the full audit on 2026-10-08. Each entry gives a severity, the
location, and the failure. "Unverified" means a reviewer reported it and it
has not yet been confirmed against the code.

### UI

- **U2 low** `sidebar/AccountSection.svelte` account menu button (28 px, `right-2`, shown on hover/focus) covers the header's sync spinner and error icon, so the error `title` tooltip can't be hovered then. Existed at 24 px before M5.

Found by the Mail visual review on 2026-10-11 (fake data, 360/390/1280/1440/2560 px):

- **U3 medium** `chat/ChatList.svelte:54-63`: `selectedKey` starts `null` and is never matched to the conversation restored from UI state, so after launch the open conversation has no highlighted row, and arrow keys start from the top of the list instead of from it.
- **U4 medium** `sidebar/Sidebar.svelte:130` (`selectionSource = null` at launch): no folder in the Accounts column is highlighted until the user clicks one, so a new user can't see which inbox the list shows. The drawer at phone widths has the same problem.
- **U5 medium** `chat/ChatView.svelte:381`: `ChatComposer` sits outside the centred `--chat-column`, so on wide panes the reply bar spans the whole pane (1810 px at 2560 px wide) while the messages sit in a centred 960 px column. The `ChatViewHeader` title also stays at the pane's left edge.
- **U6 medium** `chat/ChatViewHeader.svelte:146-199`: at phone widths (360–390 px) the Compose button plus archive, delete, snooze and more leave about 60 px for the title, so it reads "Priya …" / "Weeke…". Compose has nothing to do with the open conversation, and the list header already offers it.
- **U7 low** `chat/ChatRow.svelte:70` (`rowPeople`) vs `chat/ChatViewHeader.svelte:79` (`threadPeople` from `ChatView.svelte:94`): the list row and the conversation header can order the same participants differently ("Sam Rivera, Priya Shah" in the row, "Priya Shah, Sam Rivera" in the header).
- **U8 low** `sidebar/Sidebar.svelte:514` (close button) and `chat/ChatListEmpty.svelte:30`: the Accounts drawer header is empty apart from the × button (no title such as "Accounts"), and the empty Snoozed tab ("No snoozed chats") doesn't say how to snooze a chat.

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

### UI tokens

- **U1 low** `frontend/src/lib/components/kit/ListPane.svelte`, `kit/ListHeader.svelte`, `kit/DetailPane.svelte` header, `extensions/calendar` `CalendarPane.svelte:181`: the Contacts and Calendar panes and headers don't use `bg-surface-list` or `min-h-header`, so M2/M5 token changes would only affect Mail. Decide in M2 whether the tokens should apply to the extension panes too.

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
