# Gmail API and Microsoft Graph integration

For the provider matrix and setup requirements, start with [Providers](PROVIDERS.md).

Gmail API routes support personal Gmail and Workspace accounts; Microsoft Graph
routes support Microsoft work/school accounts. Outlook.com personal accounts and
shared-mailbox routes are excluded. Users opt in per account by authorizing and
migrating from the configured IMAP/SMTP route. Provider policies, consent and
mailbox rights still govern access; local fixtures do not establish live provider
compatibility, privacy behavior or delivery.

## Authorization foundation

API authorization is isolated from protocol credentials. Accounts continue to use
IMAP/SMTP until an API route is activated through migration; the API route then
handles supported sync, reading, sending and actions. See the sections below for
current route behavior and unsupported operations.

`oauth2.BuildMailAPIProfile` accepts an already resolved `ProviderConfig`.
Resolve the user's selected credentials and registered audience first; the
builder preserves every field except scopes, including auth/token endpoints,
client ID/secret and login hint. It does not resolve another client, register a provider or start authorization.
The consent lifecycle persists API tokens with `SetOAuthTokensForClientConfig`
and reads them with `GetOAuthTokensForClientConfig`, using the returned
`ClientConfigID`:

| Profile | Separate token slot | Requested mail permissions |
| --- | --- | --- |
| Gmail API | `google-mail-api` | `gmail.modify` |
| Graph own mailbox | `microsoft-mail-api` | `Mail.ReadWrite`, `Mail.Send` |
| Graph with explicit shared-mailbox use | `microsoft-mail-api` | Own permissions plus `Mail.ReadWrite.Shared`, `Mail.Send.Shared` |

These token slots are not new OAuth app registrations. Profiles retain identity
scopes; Graph also requests `offline_access`. Gmail's existing authorization
flow uses offline access. Contacts and calendars keep their independent grants.
The lifecycle retains a profile/config snapshot and uses the existing
cross-process refresh lock discipline, rather than the legacy mail refresh or
grant-save entry points.

Gmail `gmail.modify` covers ordinary message operations and sending, but excludes
immediate permanent deletion. It also permits listing send-as aliases, so no
additional settings-write permission is requested. Permanent deletion needs a
separate permission/feature decision before support is claimed. Ordinary user
consent does not establish access to another Gmail mailbox; this builder rejects
Gmail delegated-mailbox profiles. See [Google's scopes](https://developers.google.com/workspace/gmail/api/auth/scopes)
and [send-as listing permissions](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs/list).

Graph Shared permissions are delegated organizational permissions and require
actual mailbox rights; they also cover the user's own mailbox. Own-mailbox
permissions alone do not establish shared access. Consent validates account
type and requested audience before requesting shared access. Personal Microsoft
accounts are excluded from Graph API routing. See [Microsoft's permission
reference](https://learn.microsoft.com/en-us/graph/permissions-reference).

`credentials.RequireMailAPICapability` checks recorded consent for `read`,
`write`, `send`, or Gmail `aliases` (send-as listing), plus whether another
mailbox is requested. It rejects unknown grants, incorrect/missing resources,
unsupported providers/operations and insufficient consent. This is an operation
category check: concrete API methods still need their documented permissions
and policy/rights checks. It does not validate a token's signature, expiry,
audience claim or mailbox rights. Resource metadata records the requested OAuth
resource, not independently verified token contents. API clients must still
handle provider authentication and authorization failures.

An Outlook protocol grant cannot satisfy Graph access. Requested scopes and old
scope-array metadata are not evidence of actual consent. Local fixtures verify
isolated encrypted token storage, rotation, omitted refresh-scope retention,
explicit narrowing, PKCE and recorded resource context. They do not prove live
provider compatibility.

## Authorization lifecycle and route model

`StartMailAPIConsent(accountID, shared)` starts user-invoked PKCE consent for
the account's own provider (Google: Gmail API; Microsoft work/school: Graph).
Personal Outlook.com accounts, custom-provider accounts, Gmail delegated
access are rejected. A linked shared-mailbox child is staged under its parent's grant (below) but cannot be activated or migrated until live Graph checks are recorded. The signed-in
identity must match the account (stable Microsoft ID, else email). Consent
must grant read, write and send (plus the shared scopes when requested);
anything less saves nothing and reports `partial-consent`.

Success stores the grant in the isolated `*-mail-api` slot and a **staged**
row in `account_mail_api_routes` (migration 58) with a secret-free snapshot of
the consented provider (endpoints, client ID, scopes, resource). Staged means
sync and sending still use IMAP/SMTP; no route switch happens yet. Events:
`mail-api:started`, `mail-api:success`, `mail-api:error`,
`mail-api:cancelled` (`CancelMailAPIConsent`) and `mail-api:reauth-required`.

API refresh runs under the slot's cross-process lock, rereads tokens after
locking, refreshes with the snapshot (never current settings) and refuses if
the registered client ID changed. Only `invalid_grant` clears the API refresh
token and asks for reauthorization; transient errors keep the grant. The
protocol slot is never read or written by this path. Failure kinds:
`reauthorize`, `policy` (tenant or admin policy), `resource` (API resource
unavailable), `partial-consent`, `other`.

`GetMailAPIRouteStatus` reads local state only. `ClearMailAPIRoute` removes
the route row and the API grant, leaving protocol credentials, drafts and
queued mail untouched. Fixtures prove the local lifecycle, not live providers.

## Live verification limits

Local fixtures cover API sync, actions, submission handling and migration
recovery. Live behavior still needs separately authorized checks for Bcc privacy,
signed and encrypted MIME, international content, attachments, provider size
limits, Sent behavior, throttling, alias permissions, and real-account migration
and rollback. Fixtures do not establish provider behavior or delivery. Shared
mailbox API routes remain unsupported. No provider registration, tenant policy,
DNS change or test email is implied by this documentation.

Relevant cases cover personal Gmail, Workspace/custom domains, Microsoft 365
own/shared mailboxes and Outlook.com as a separate compatibility target; unsupported
routes need implementation before they can claim parity. Check policy/grants,
alias/delegation, Bcc privacy, international content, attachments, PGP/S/MIME
verification/decryption, Sent counts/destinations and migration/rollback. Record
route/time, provider acceptance, later bounce/delay, trusted received
SPF/DKIM/DMARC evidence and observed inbox/junk/quarantine placement without
promising placement. Never send automatically or change external configuration.
Historical local checks and outstanding platform/UI checks are recorded in
[provider implementation validation](validation/provider-implementation.md).

## API submission

`internal/gmailapi` (`users.messages.send`, JSON `{"raw": base64url}`) and
`internal/graphapi` (`sendMail` MIME form: base64 body, `text/plain`) implement
`submission.Sender`. They take the final MIME, envelope sender and recipients,
and never alter the content other than adding a `Bcc:` header naming envelope
recipients that no To/Cc/Bcc header shows. Both providers read recipients from
headers; whether they strip that header from delivered copies is undocumented,
so this behavior remains **unverified** by live provider checks. Active API routes
call these clients from the compose send path; staged routes continue to use SMTP.

Preflight rejects, with no truncation or SMTP fallback: a route that is not
staged/active for the provider, a grant without `send`, a Gmail From that is not
a verified send-as identity (Graph: a From other than the mailbox without the
Shared permission), and raw MIME over 25 MiB. This is a local submission ceiling,
not a verified provider allowance or a guarantee that a smaller message is accepted.
For Gmail it measures `req.Raw` before the added Bcc header and base64url JSON
encoding, so the request itself is larger ([SEND-2 evidence](validation/send-2.md)).
Gmail preflight also runs the same exact-final-MIME validation as SMTP
(`smtp.ValidateMessage`), so an invalid message fails locally with no HTTP
request; the added Bcc header is folded at address boundaries so a long hidden
list keeps every recipient within the 998-byte line limit.

Outcome mapping: 2xx is accepted (not delivered); 4xx is a definitive
rejection classified with `oauth2.ClassifyMailAPIError` (403 rate-limit bodies
are temporary); 429, 5xx and any failure after request bytes were written are
unknown, never replayed; a failure before any byte is written is not-submitted.
`Retry-After` is a backoff hint only. A 401 triggers one locked snapshot refresh
and one re-attempt; a rejected grant (`invalid_grant`) emits
`mail-api:reauth-required` and leaves protocol credentials untouched.
Redirects are never followed, so a POST cannot be replayed.

## API receive and sync

Active routes use Gmail or Graph sync in place of IMAP sync. Staged routes continue
to use IMAP. Gmail and Graph receive behavior is described below.

**Caller invariant.** Serialize API sync runs per account. Neither `GmailSync.Run`
nor `GraphSync.Run` has an internal mutex. Production callers use
`app/api_route.go` `runAPISync`, which holds the per-account lock;
future callers must preserve that serialization.

**Identity (migration 59).** `api_message_refs` is one remote message per `(account, provider, remote_id)` with the provider thread ID, the full container (label/folder) ID set, the unmapped subset and an opaque version. `api_message_rows` links each local `messages` row to that identity, so a multi-label Gmail message is one identity with one row per mapped folder. API rows get a local surrogate `uid` from `folders.uid_next` (never reused after a delete, never sent to a provider); the local `thread_id` still derives from headers.

**Mapping (`internal/message/api_mapping.go`).**

- Gmail: INBOX/SENT/DRAFT/TRASH/SPAM map to folder types; user labels (named by `labels.list`) become folders; STARRED and UNREAD become flags (DRAFT also sets the draft flag); IMPORTANT and `CATEGORY_*` are kept as metadata, not folders; a message with no folder label lives in All Mail. Any other ID (CHAT, labels absent from the list) is kept unmapped.
- Graph: folders map by well-known name, with paths built from `parentFolderId`; a folder whose parent chain never reaches the mailbox root (or loops) is unmapped, and so is a message in it. One row per message.

**Cursors (`api_sync_state`).** One row per `(account, scope)` holding an opaque cursor, its kind, a generation and an unreconciled marker. `CommitAPIPage` stores a page's messages, removals and its new cursor in one transaction, so a crash or error leaves the old cursor and no partial page; retrying a page is idempotent. `InvalidateAPICursor` clears the cursor, bumps the generation and sets the marker without touching local mail; pages built against an older generation are refused, and the marker stays until a page with `ClearInvalid` commits.

**Gmail read client (`internal/gmailapi/read.go`).** `Reader` calls `labels.list`, paged `messages.list` (500 per page, spam and trash included), `messages.get` with `format=raw` (base64url decodes to the byte-identical MIME) or `format=minimal` (labels and historyId only), `history.list` (messageAdded, messageDeleted, labelAdded, labelRemoved) and `users.getProfile`. It reuses the API token source: one refresh and retry on 401, no redirects, no new dependencies. Failures are a `ReadError`: 429, rate-limit 403 reasons and 5xx are rate limits (`RateLimited`, `RetryAfter`); any other 403 is a policy failure; a history 404 is `ErrHistoryExpired`; a message 404 is `ErrMessageGone`.

**Gmail sync (`internal/sync/gmailapi.go`, `GmailSync.Run`).** The `read` capability gate runs first (`RequireMailAPICapability`); a refused grant reports `partial-consent` and sends no request.
- *Initial sync / reconcile.* The profile historyId is captured before listing. With a sync period (`sync_period_days` > 0, as for IMAP) the list asks only for messages received after the window start (`q=after:<epoch seconds>`), so local messages outside the window are removed like any vanished ID; 0 mirrors the whole mailbox. Incremental runs are not windowed: history replays any touched message, and messages that age out stay until the next reconcile. Remote IDs are diffed against local refs (sorted-slice search, no second per-ID map; a 5xx shares the bounded rate-limit retry path): unknown IDs are fetched raw and ingested, known IDs are re-fetched as `minimal` (one GET each, since the list returns IDs only) and skipped when their historyId equals the stored version, and vanished IDs are removed. Messages commit in pages of 50 with the cursor still empty and the unreconciled marker set; the last page removes vanished refs, stores the captured historyId as the `gmail-history` cursor and clears the marker. Messages that change while listing are replayed by the first incremental run.
- *Server search beyond the window.* Search the whole mailbox (shown under a chat search for accounts on the Gmail API route; in the unified inbox it covers every such account) lists up to 100 matches with `messages.list?q=<query>` (spam and trash included), ingests the ones not yet local through the normal raw-message path, and marks their refs `pinned` (`api_message_refs.pinned`, migration 63). A windowed reconcile skips pinned refs when removing vanished IDs, so searched-in mail outside the window stays; a pinned message deleted on the server is removed by the incremental history replay, and a reconcile after history expiry fetches each pinned ref outside the window (one `minimal` GET each) and removes the ones Gmail no longer has. Search and sync share one per-account lock. Graph and IMAP accounts have no equivalent yet.
- *Load older.* "Load older mail" at the end of a chat list (one account, or every Gmail API account in the unified inbox) lists the next 100 messages with `messages.list?q=before:<epoch>` (spam and trash included), stores and pins the ones not yet local, and moves its position to the oldest received time among them. The position is kept in the `older` sync scope (`api_sync_state`), apart from the history cursor, and starts at the window's edge (never newer than it, so a shorter period restarts there). A page shorter than 100 means nothing older remains; an account with no sync period is already complete. Search and Load older store through the same path and leave the history cursor untouched. IMAP has no equivalent: the sync deletes anything older than the window, so older IMAP mail needs a pin concept first (SYNC-7).
- *IMAP.* Accounts on the protocol route get the same two actions (`SearchIMAPServer`, `LoadOlderIMAP`; `internal/sync/older.go`). Search runs `UID SEARCH OR FROM/SUBJECT/TO/CC/BODY` in each subscribed folder except Trash and Spam and stores up to 100 matches not yet local; Load older searches `BEFORE <window edge>` in each of those folders and stores the newest 100 per folder not yet local. Both fetch headers through the normal path and mark the rows `messages.pinned` (migration 64). `DeleteOlderThan` skips pinned rows, and the windowed UID reconcile excludes them from its diff and instead asks the server for them by UID (`UID SEARCH UID <set>`, not `\Deleted`); a pinned message the server no longer holds is deleted, and a failed check keeps every pinned message. Flag changes reach pinned rows through the same flag sync (CONDSTORE `CHANGEDSINCE` covers them). Load older keeps no cursor: it skips what is local, so a page of UIDs that failed to fetch is retried.
- *Incremental sync.* History pages are aggregated into a net state per message (a delete wins), then applied in batches. Batches re-commit the old cursor, so an interrupted run repeats idempotent work; only the last batch advances the cursor to the history response's historyId. A label change that adds no folder updates the flags of the existing rows from `minimal` data (`APIRow.FlagsOnly`); a new folder row, or an unknown message, fetches the raw message.
- *Oversized messages.* A raw response over the cap (`Reader.MaxRawBytes`, default the 40 MiB JSON cap) is kept as a header-only row built from `format=metadata` (Subject, From, Message-ID, labels, `internalDate`), logged and counted in `GmailResult.Oversize`, so the run completes. Gmail accepts incoming mail up to 50 MB, whose raw form inside the JSON response exceeds the cap.
- *Reset recovery.* A history 404 invalidates the cursor (generation bump, unreconciled marker) and runs the reconcile. Local data is never wiped on failure: a failed reconcile leaves the old rows and the marker, and the next run restarts it. Messages without an API ref (local-only state) are never touched.
- *Ingestion.* A message without a parseable `Date:` header takes the provider's receive time (Gmail `internalDate`, Graph `receivedDateTime`) as both `Date` and `ReceivedAt`, so it keeps a stable position across full ingests. Raw MIME goes through the IMAP parser (`parseHeadersIntoMessage`, `parseMessageBodyFull`, references, read-receipt and bulk headers, header-derived `thread_id`), so S/MIME, PGP, attachments, charsets and international addresses behave as for IMAP rows. The raw S/MIME/PGP bodies and attachments are written in the same transaction as the row (`APIRowExtras`). Gmail's `threadId` is stored only as `remote_thread_id`. Folders for mapped labels are created from the container listing, including empty folders; Graph does the same for its mapped folders.
- *Failure policy.* Reauthorize, policy and partial-consent failures go to the `OnFailure` hook (the reauthorization event) and never clear credentials. Rate limits retry up to 3 times, waiting `Retry-After` (capped at 60 s; when absent 15 s doubling, so the waits outlast Gmail's per-minute quota window, whose 403 carries no `Retry-After`), then stop with the `ReadError` as a hint and an unchanged cursor. Cancellation returns the context error and changes nothing past the last committed page.

**Local limits (`internal/gmailapi/read.go`, `internal/sync/gmailapi.go`).** 500 IDs per list/history request; history expiry is detected by a 404 rather than predicted. Responses have a 40 MiB cap including base64 overhead, with 4 concurrent raw fetches, 50 messages per commit batch and 3 rate-limit retries. These are implementation bounds, not verified mailbox limits.

**Graph read client (`internal/graphapi/read.go`).** `Reader` reads `/me` or, for a shared mailbox, `/users/{shared}` (which needs the `.Shared` grant). It calls `mailFolders` (root, well-known names, paged folders and `childFolders`), per-folder `messages/delta`, `GET messages/{id}` for metadata and `messages/{id}/$value` for byte-identical MIME. Every request sends `Prefer: IdType="ImmutableId"`; delta also sends `odata.maxpagesize`. Paging and delta links must stay on the API origin, so the bearer token is never sent elsewhere. Redirects are not followed. 429 and 5xx are rate limits (`RateLimited`, `RetryAfter`); 403 is a policy failure and 401 (after one refresh) a reauthorization failure; a delta 410 is `ErrDeltaExpired`, a folder 404 `ErrFolderGone`, a message 404 `ErrMessageGone`.

**Graph sync (`internal/sync/graphapi.go`, `GraphSync.Run`).** The `read` gate runs first and passes the shared flag; without the grant the run fails with `partial-consent` before any request.
- *Per folder.* Each mapped folder has its own cursor (scope `folder:<id>`, kind `graph-delta`). With a stored delta link the run resumes it; without one, or after 410, the cursor is invalidated and the folder reconciles. Other folders are unaffected. Folders that map to nothing are skipped; their messages are not read.
- *Messages.* A message whose change key and folder are unchanged is skipped; a change key bump on a non-draft refreshes flags from metadata (`IsRead`, flagged, `IsDraft`) without refetching the body; a new message, a move, or a changed draft fetches the raw MIME and goes through the same parser as IMAP and Gmail rows. A message in an unmapped folder keeps its ref with no rows. `conversationId` is stored as `remote_thread_id` only.
- *Removals and moves.* `@removed` entries are verified with `GET messages/{id}`: a 404 removes the ref and rows; a message still present elsewhere is upserted under its new folder, so a move keeps its identity. A reconcile also verifies local IDs the fresh enumeration did not list.
- *Folder lifecycle.* The folder ID to path map of the last complete run is stored under scope `folders`. A renamed folder reconciles into its new local folder, and the old, now empty, local folder is deleted; a deleted folder's messages are re-checked, then its empty local folder is removed. A local folder still holding messages is kept.
- *Cursors and failures.* Pages commit with the old cursor; only the last page stores the delta link. Rate limits retry up to 3 times honoring `Retry-After` (capped at 60 s); failures report through `OnFailure` and never clear credentials.

**Local Graph limits (`internal/graphapi/read.go`, `internal/sync/graphapi.go`).** Delta page size 50, folder page size 100, 4 concurrent requests, 64 MiB raw message cap and 3 rate-limit retries. A message over the cap is kept as a header-only row built from Graph metadata (subject, sender, receive time; snippet "This message is too large to download."), logged as a warning and counted in `GraphResult.Oversize`; the folder cursor advances and opening the message reports that it is too large. These bounds do not establish the mailbox's send/receive allowance.

### Provider limit source rationale

The following rationale was recorded during F4d/F4e on 2026-10-13. These are
historical source references, not newly checked requirements or live probes;
verify the official references before changing provider-dependent behavior.

- Gmail list/history requests use the documented 500-result ceiling:
  [messages.list](https://developers.google.com/gmail/api/reference/rest/v1/users.messages/list),
  [history.list](https://developers.google.com/gmail/api/reference/rest/v1/users.history/list).
  [Sync guidance](https://developers.google.com/gmail/api/guides/sync) describes
  history as typically available for at least a week, with expiry detected by 404;
  this is not a guaranteed retention period.
- The Gmail four-worker choice followed the recorded 250 quota units/s per user
  and five units per `messages.get` in the [quota reference](https://developers.google.com/gmail/api/reference/quota).
  Four workers are a local choice, not a provider concurrency guarantee. Rate-limit
  403/429 classification follows [error guidance](https://developers.google.com/gmail/api/guides/handle-errors).
  The 1000-ID write batch ceiling comes from [batchModify](https://developers.google.com/gmail/api/reference/rest/v1/users.messages/batchModify).
- Graph folder delta, paging and per-folder cursors follow
  [message delta](https://learn.microsoft.com/en-us/graph/api/message-delta) and
  [delta query guidance](https://learn.microsoft.com/en-us/graph/delta-query-messages).
  The four-request concurrency bound and 429/Retry-After handling follow
  [Outlook service limits](https://learn.microsoft.com/en-us/graph/throttling-limits#outlook-service-limits).
  ID handling follows [immutable IDs](https://learn.microsoft.com/en-us/graph/outlook-immutable-id)
  and the returned ID from [message move](https://learn.microsoft.com/en-us/graph/api/message-move).
- Commit batch size 50, Graph delta/folder page sizes 50/100, response-size caps,
  three retries and the 60-second Retry-After cap are local implementation bounds.
  Treating a Graph 400 containing `syncState` as an expired token is an unverified
  compatibility heuristic; the recorded documented expiry response is 410.

## API actions and activation

The local-first action queue is used only when `apiRouted` sees a route in state
`active`. Accounts without an active route continue using IMAP.

**Shared transport (`internal/apiwrite`).** `Do` sends one write and returns an `Error` carrying an outcome: *applied* (2xx), *rejected* (other 4xx, including 404 gone), *unknown*, *not-sent* and *reauthorize* (a 401 after one refresh, or a 403 that is not a rate limit; the `Failure` field keeps the `ClassifyMailAPIError` category). Unknown covers 5xx, redirects (never followed), a transport failure after the request headers were written, and an unreadable echo after a 2xx. A failure before any byte is written is not-sent. Only a 429, or a 403 with a rate-limit body, is retried: the provider answers it before running the operation, so the retry is safe. Up to 3 retries wait `Retry-After` (capped at 60 s) or a doubling 1 s backoff, then the outcome is unknown. A 5xx is never replayed. Tokens come from `submission.TokenSource`, with one refresh per call.

**Gmail (`internal/gmailapi/write.go`, `Writer`).** `Modify` (`messages.modify`, add/remove label IDs: read/unread via `UNREAD`, star via `STARRED`, archive by removing `INBOX`), `Trash`, `Untrash` and `BatchModify` (`messages.batchModify`, split into chunks of at most 1000 IDs, stopping at the first failing chunk and returning how many IDs were in applied chunks). There is no batch trash (Gmail has no such endpoint, and `batchModify` with `TRASH` is unverified), so the queue loops single `Trash` calls. Permanent delete is not offered. A 404 wraps `ErrWriteGone`.

**Graph (`internal/graphapi/write.go`, `Writer`).** `Update` (`PATCH messages/{id}` with `isRead` and `flag.flagStatus`; an empty patch sends nothing), `Move` (`POST messages/{id}/move` with `destinationId`; the response carries the **new immutable ID**) and `Trash` (a move to `deleteditems`). Requests send `Prefer: IdType="ImmutableId"`. A shared mailbox uses `/users/{shared}` and needs the `.Shared` grant. A move response without an ID is unknown, not applied. Permanent delete is not offered.

**Capability gate.** Every call first runs `RequireMailAPICapability("write", shared)`. A refused grant (nil, read-only, send-only, unknown scopes, own-mailbox grant used for a shared mailbox) returns *not-sent* and sends no request.

**Retry rules for the queue.** Move is not idempotent: after *unknown* it must be verified (Graph `GET`, Gmail `messages.get` minimal) before any re-send. Label and flag sets are idempotent and may be re-sent after verification.

**Local re-key (`message.Store.ApplyAPIMove`).** One transaction moves the local row to the destination folder (new UID from `uid_next`) and updates the ref's containers. If the provider returned a new remote ID, the new ref is inserted, `api_message_rows` is repointed and the old ref deleted (the foreign key has no `ON UPDATE CASCADE`). A missing source row, an occupied destination or an existing target ref fails and changes nothing.

**Limits (constants, never probed).** 1000 IDs per `batchModify`; `Retry-After` cap 60 s; 3 rate-limit retries.

**Action queue (`api_pending_actions`, migration 60; `message.Store`).** `EnqueueAPIFlag` and `EnqueueAPIMove` change the local rows and insert the queue row in one transaction, so a crash loses or duplicates nothing and the UI never waits on the network. One row covers one *remote* message: sibling local rows (a Gmail message under several labels) follow flag changes, and a flag action coalesces with the newest pending action of the same op (it is dropped when the value returns to the prior one). A move relocates the row with a new surrogate UID, or deletes the source row when the destination folder already holds one (a label the message carries), and stores the prior containers for a revert. Statuses: pending, in-flight, unknown, applied, rejected. `NextAPIAction` returns the oldest due pending or unknown action with no earlier open action on the same remote ID, so actions on one message stay in order.

**Worker (`internal/apiactions`).** `Drain(account)` sends due actions one at a time and stops when the route is no longer active or the context ends. Gmail: read removes or adds `UNREAD`, star toggles `STARRED`, a move is `Modify(add Dest, remove Source)`, trash is `Trash`, and leaving the trash is `Untrash` then `Modify`. Flags are sent as single `Modify` calls, not `BatchModify`. Graph: flags use `Update`; a move is `Move(Dest)` and re-keys the local row and any queued action on the old remote ID (`ApplyAPIMove`). Settling: applied; gone (404) removes the message, rejects the action and emits `sync:error`; rejected reverts the local row and emits; not-sent and reauthorize retry later (reauthorize also emits); throttling and missing token or remote use persisted exponential backoff (5 s doubling to 5 min). A request that may have been applied is *unknown*: on restart in-flight becomes unknown, and the next attempt first reads the remote state: matching settles it as applied, differing re-sends, anything else stays unknown and is surfaced once. Unknown is never replayed or reverted on a guess.

**Reconcile guard.** `applyAPIUpsert` skips a ref that has an open action, so receive sync cannot overwrite pending local flags or folders; removals still apply.

**Routing (`app/api_actions.go`, `app/actions.go`).** For an active route, `MarkAsRead/Unread`, `Star/Unstar`, `MoveToFolder`, `Archive`, `Trash`, `MarkAsSpam` and `MarkAsNotSpam` queue instead of using IMAP; others in the same selection keep IMAP. Local folders map to containers by path: Gmail system labels are their ID, `[Gmail]/All Mail` is no label, a user label is resolved by name through `labels.list` (needs the network); Graph folders are found by path in the folder map the Graph driver stores. Not available for API accounts, failing with an explicit error and no IMAP fallback: permanent delete, empty trash/spam, copy, and moves between an API account and another account. API moves are not undoable. The worker starts at launch (`Recover`, then drains accounts with open actions) and is kicked after each enqueue.

**Activation (`app/api_route.go`).** `ActivateMailAPIRoute(accountID)` works only from a staged, non-shared route whose grant passes the read, write and send gates. Order: take the per-account transition lock, mark the route `activating`, set the route `active` (new IMAP syncs now refuse to start), refuse if an IMAP sync is running or a queue action is in flight or unknown (pending actions are allowed), stop IDLE, swap folder rows, then run one blocking API reconcile (`GmailSync`/`GraphSync`). Any failure reverts: route back to staged, rows swapped back, IDLE restarted, account stays on IMAP. Success or failure clears the mark; a mark still set at startup means the app quit during activation, and `recoverInterruptedAPIActivations` makes the same revert before IDLE and the sync scheduler start, so the half-done reconcile never resumes. Credentials, drafts, queued mail and IMAP data are never deleted. Shared (delegated) routes cannot be activated: Graph's shared-mailbox address is not stored and `apiRemote` rejects them.

**Folder swap (`folder.Store.SwapHidden`).** IMAP and API folder rows share one table. The inactive set is hidden in one transaction: `folder_type` becomes `hidden:<tag>:<type>` and `path` gets an `\x1f<tag>\x1f` prefix (tags `imap`, `api`). `List`, `GetByPath` and `ListSubscribed` skip hidden rows and type-equality queries never match them, so API rows are kept, never shown beside IMAP rows, and never receive IMAP calls. Deactivation swaps back. Special-folder mappings (Edit Account → folder mapping) always name IMAP paths: while the route is active, `UpdateAccount`, `GetAccountFoldersForMapping` and `GetAutoDetectedFolders` read the hidden IMAP rows through `folder.Store.ListHidden`.

**While active.** The scheduler hook (`Scheduler.SetAPISync`) runs the API sync instead of IMAP folder sync and reports through the existing sync status/error events. `SyncFolder`, `SyncFolders`, `SyncAccountComplete` and IDLE start-up bypass IMAP for routed accounts. Submission goes through `submitViaAPI`: no SMTP fallback on any error, and no local Sent copy (the provider files it). Errors are surfaced; nothing falls back.

**Pause and resume.** A reauthorization failure (`mail-api:reauth-required`, or any grant that no longer passes the gates) pauses the account: API sync returns `errAPIPaused` (handled, so no IMAP fallback) and the action worker is inactive. Local rows and queued actions are kept. A successful re-consent resumes sync and the queue and keeps the route active; a route-type or shared change while active is rejected before any credential write.

**Deactivation.** `DeactivateMailAPIRoute` and `ClearMailAPIRoute` cancel running API work, swap rows back and return to IMAP. Queued API actions are kept, not sent, until the route is active again.

**Tests.** `go test ./app -run 'MailAPIRoute|Activation|APISync|UpdateAccountOnActiveAPIRoute'`; `go test ./internal/folder -run SwapHidden`; `go test ./internal/sync -run APISync`. Queue fixtures (atomic local change plus enqueue, restart with in-flight actions, 404/410, throttling, cancellation) live in `internal/message` and `internal/apiactions`.

## API read paths

**Raw source and bodies (`app/api_read.go`).** `fetchRawMessage` and `fetchBody` replace direct IMAP fetches for body loading, attachment listing/download, raw source, crypto handling and forward attachments. For an active API route they read the message through the provider API (`read` capability gate, 60 s timeout), never over IMAP with the surrogate UID. `fetchBody` parses with `Engine.ParseAPIBody` and caches via `message.Store.StoreAPIBody` (body, attachments, crypto extras). A 404 reports "this message no longer exists on the server"; paused accounts and shared routes return explicit errors. Cached bodies keep working offline. `syncSentFolder` is skipped for routed accounts because API sync picks up the sent copy.

**Cross-account moves.** Moving between an API-routed account and any other account keeps an explicit error ("Moving messages between accounts is not available for accounts using the mail API"); API undo stays unsupported.

**Drafts on API routes.** Autosave retains drafts locally and reports that remote draft sync is unavailable. Cleanup of previously IMAP-synced drafts uses the recorded folder ID and original IMAP path, including after migration hides that folder; API surrogate UIDs are never used for this cleanup. Gmail sender evidence refresh uses the active API grant and checks its aliases capability.

**Draft ownership.** `saveDraftToDB` moves a draft to the selected account: the previous account's remote copy is deleted, `account_id` is rewritten and the IMAP link reset, so encryption, sync and deletion use one account.

The [API-1 shared-mailbox design](SHARED_MAILBOX_API_DESIGN.md) maps the identity, grant-owner and lifecycle changes required before shared activation; it does not implement them or establish live compatibility.

**Deferred.** Gmail permanent delete stays excluded from the ordinary API profile (needs a separate scope request and UX). Shared-mailbox routes remain unmigratable (MA4).

**Tests.** `go test ./app -run 'APIBody|APIAttachment|APIMove|Draft'`.

## Migration and rollback

Bindings: `PlanMailAPIMigration`, `MigrateMailAPIAccount`, `RollbackMailAPIMigration` (`app/api_migration.go`, `internal/apimigrate`). Progress lives in `api_migrations` (one row per account, state `snapshotted`, `activated`, `completed`, `failed`, `rolled-back`).

- **Plan** is read-only. It reports route, row counts, drafts, queued submissions, pending actions and blockers: unusable grant, IMAP sync running, in-flight or unknown actions, unknown or queued submissions, shared routes. Migrate refuses while any blocker exists, including when resuming saved progress; volatile blockers are checked again before activation.
- **Migrate** takes the route transition lock and runs snapshot, activate, correlate, complete. The snapshot is a `VACUUM INTO` copy in `<data>/migration-snapshots/` (owner-only, 0600/0700) with a `.json` manifest (version, account, counts, route, time). Tokens are not copied anywhere else. Account ID, credentials, identities, drafts, outbox, rules and contacts are untouched; IMAP rows stay hidden, not deleted.
- **Correlation** matches each API row to a hidden IMAP row by Message-ID (angle brackets ignored) plus date to the second, preferring the same folder type, and carries local-only state (answered/forwarded flags, cached body where the API row has none, thread, attachment content). Messages with no Message-ID, several matching IMAP rows, or an IMAP row also claimed by another API row (duplicate API messages) are left uncorrelated, never guessed; they appear once from the API without carried local state (cached body, answered/forwarded flags, verification results, attachments), and the correlation result counts them as ambiguous or without Message-ID. Each step is idempotent; re-running after an interruption resumes without duplicates.
- **Failure** at any step deactivates the route, records `failed` with the error, and leaves the account on IMAP; retry is explicit. Runtime API errors do not fall back to IMAP/SMTP; the user can explicitly return the account to IMAP through deactivation or migration rollback.
- **Rollback** synchronously deactivates the route and makes IMAP folders visible again. The report's `syncRequested` flag means only that the scheduler's non-blocking `TriggerSync` was invoked; it does not establish sync start, completion or success. No scheduler or an already inactive route yields `syncRequested=false`; request a manual sync if needed. Reconciliation uses the normal account sync status and can finish or fail after rollback returns. Rollback replays nothing, and reports queued/unknown actions and actions applied since migration. Rollback does not restore the snapshot. It returns the account to IMAP while retaining the snapshot for reference; snapshot restoration is not part of this rollback operation. Restoring the snapshot is a separate, explicit action (next section).

**UI.** Account settings → Server tab → "Mail API route" (`MailAPISection.svelte`), shown for OAuth accounts and for linked shared mailboxes (opened from the parent's Identity tab), where it adds a note that shared mailboxes need the Shared mail permissions and an administrator-granted delegation and offers consent only (shared mode); migration is not offered. Flow: *Authorize API access* (`StartMailAPIConsent`, cancellable while the browser step waits; `mail-api:success|error|cancelled` events) stages the route; the plan view lists row counts, pending actions and blockers (review is disabled while any blocker exists); *Review migration* shows the snapshot note and a consent checkbox that gates *Migrate this account*, with *Cancel* available until then (the backend call itself is not interruptible; a failure returns the account to IMAP and is shown). An active account can *Reauthorize API access* in place, including after a `mail-api:reauth-required` event pauses sync and queued actions; this keeps the active route and offers cancellation while the browser step waits. It also shows *Return to IMAP / SMTP* behind an explicit confirmation, then lists actions that did not finish and the count applied since migration. *Remove API authorization* clears a staged route. The Settings account list and the Mail sidebar account header show the route name while it is active (`mailApiRoutes` store). Reauthorization and policy failures reuse the consent error text (`reauthorize`, `partial-consent`). The UI shows the snapshot path for reference; restoring the snapshot is not offered. Guarded UI check: `node tools/mail-api-migration-check.cjs <dir>`.

### Snapshot restoration

Account Mail API section → "Restore database to snapshot…" (`PreviewMailAPISnapshotRestore`, `RestoreMailAPISnapshot`, `UndoSnapshotRestore`; `app/snapshot_restore.go`, `internal/snapshotrestore`). Design: [Migration snapshot restoration design](API_SNAPSHOT_RESTORE_DESIGN.md). It is an offline whole-database swap, not a per-account rollback.

- **Consent:** the dialog names the snapshot date, says every account returns to it and local changes after it are discarded, lists per-account counts and accounts added since, says provider-side changes are not undone and re-sync, that queued work is held, where the pre-restore backup lives, and that Mailven closes and must be reopened by the user. A checkbox is required. Open composer windows block staging.
- **Preflight** (on a scratch copy; nothing live changes): files and manifest (version 1, matching account), SHA-256, integrity check, forward migration (a snapshot from a newer schema is refused), and a credential probe (`device.key` must exist and decrypt sampled stored secrets).
- **Apply** runs at next start, after the single-instance lock and before any database read, driven by `pending-restore.json`: back up the live database (`VACUUM INTO restore-backups/pre-restore-<id>.db`, never overwritten), stage `mailven.db.restoring`, remove WAL/SHM, atomically rename. Every phase is resumable after a crash; a failure before the rename leaves the live database untouched and the backup kept. Exclusive access is required.
- **After restore:** the account's Mail API route is `staged` (inactive) and `api_migrations` records `rolled-back`; API actions still pending/in-flight become `unknown` and are never replayed; queued outgoing submissions are counted, not auto-sent; the full-text index is cleared and rebuilt; accounts whose credentials fail the probe are listed under "sign in again"; accounts that exist only in the pre-restore database are listed so their credentials can be cleaned up explicitly. The banner offers undo (restores the pre-restore backup through the same flow) and dismiss.
- **Not verified:** native-window restore, Windows replace semantics and Flatpak paths, and any real account. Fixtures prove the file and database behavior only ([validation](validation/api-3.md)).

**Live checks still outstanding:** Bcc privacy, signed/encrypted MIME behavior, provider message-size limits, and real-account migration or rollback. Shared-mailbox API routes remain unsupported. Fixtures do not close them; they need separately authorized accounts and cases.
