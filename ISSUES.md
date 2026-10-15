# Issue register

Authoritative unresolved findings and follow-ups, grouped by primary project. [Roadmap](PLAN.md) and [lifecycle rules](docs/PROJECT_MANAGEMENT.md) define selection, statuses and closure. Severity describes impact; priority describes recommended execution order and does not authorize work. All P1/P2/P3 assignments below are proposals from the 2026-10-14 reorganization; original severities are unchanged.

The 2026-10-08 audit used “unverified” for reported findings not confirmed against source. This reorganization preserves recorded evidence without independently verifying product defects. The P1 provider audit confirmed findings statically; F1–F3b fixed PD1/PD9, PD3–PD8 and U3 within their local scope, while PD2 remains a live gate. See [provider review](docs/validation/provider-deliverability-review.md), [implementation evidence](docs/validation/provider-implementation.md) and [P2 architecture](docs/validation/provider-transport-recommendations.md). Resolved findings are not reopened.

All Microsoft work, including Microsoft 365 and regular/personal Outlook.com, is deferred indefinitely by owner decision on 2026-10-14, until the owner explicitly chooses otherwise. PD2, DAV1–DAV3 and remaining MA4 work are retained unresolved for future development; Microsoft-specific MA1 work is also deferred. Explicit reopening is required before further investigation, implementation or live checks. Existing severity and rollout gates remain in force.

## Index

- [SEND — Sending, privacy and compatibility](#send): [MA1](#ma1), [PD2](#pd2), [DAV1](#dav1), [DAV2](#dav2), [DAV3](#dav3) (resolved: [CD2](#cd2))
- [API — Provider API activation and migration](#api): [MA4](#ma4) (resolved: [F5b-1](#f5b-1), [F5b-3](#f5b-3))
- [SYNC — Mail synchronization and action reliability](#sync): none (resolved: [R2](#r2), [R3](#r3), [R4](#r4), [SY2](#sy2), [SY3](#sy3), [SY4](#sy4), [SY5](#sy5), [SY6](#sy6))
- [CHAT — Chat composer sender review](#chat): [CH1](#ch1) (resolved: [R1](#r1))
- [MAINT — Maintenance and documentation](#maint): [D2](#d2), [L2](#l2), [L3](#l3), [UI1](#ui1), [UI3](#ui3), [UI4](#ui4) (resolved: [UI2](#ui2), [DOC4](#doc4), [DOC5](#doc5), [MA2](#ma2), [MA6](#ma6), [SY1](#sy1))
- [AI — AI development workflows](#ai): [AI6](#ai6)
- [WAILS — Desktop framework migration assessment](#wails): no unresolved issue recorded

<a id="send"></a>

## SEND — Sending, privacy and compatibility

Project: [SEND](PLAN.md#send).

<a id="ma1"></a>

### MA1 — API Bcc privacy and local size ceiling

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-2](PLAN.md#send-2).
- **Severity / proposed priority / status:** medium / P1 / blocked.
- **Dependencies / gates:** Separate live authorization; provider stripping and size allowance unverified.
- **Resolution criteria:** Authorized controlled Bcc evidence establishes privacy, and the local ceiling is documented or justified; retain failed or unrun rollout gates.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **MA1 medium** `internal/gmailapi/send.go`, `internal/graphapi/send.go`: API clients add a `Bcc:` header for hidden envelope recipients because both providers read recipients from headers. Provider stripping of that header from delivered copies is undocumented and unverified; a Bcc recipient could be exposed. F4e implemented route activation, but the required separately authorized live Bcc check remains an unsatisfied rollout gate; activation does not establish privacy. Message size limit (25 MiB) is also a local, undocumented ceiling.

<a id="pd2"></a>

### PD2 — Existing Microsoft account live compatibility

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-3](PLAN.md#send-3).
- **Severity / proposed priority / status:** medium / P1 / deferred.
- **Dependencies / gates:** Explicit account-type evidence and separately authorized live checks; no endpoint migration approval.
- **Resolution criteria:** Recorded endpoint/account-type evidence and authorized compatibility results resolve the gate; any migration is separately scoped.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **PD2 medium — live compatibility gate** `app/oauth.go`, `internal/credentials/oauth_grants.go`: F1 corrects the setup guides and new personal/work defaults, verified by disposable fixtures and guarded UI checks. Existing Microsoft accounts retain their configured endpoints and unknown account type where no explicit evidence was recorded. Their prior `smtp.office365.com` endpoint can still differ from Outlook.com's published default; actual endpoint failure and migration compatibility remain unverified. Confirm account type and separately authorize live evidence before proposing any existing-account endpoint migration.

<a id="dav1"></a>

### DAV1 — DavMail 7.0.0 classic Graph refresh uses the wrong audience (upstream)

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-4](PLAN.md#send-4).
- **Severity / proposed priority / status:** unspecified / P2 / deferred; external gateway defect, not a confirmed Mailven defect.
- **Evidence / reproduction (2026-10-14):** With `O365Graph`, Outlook desktop client ID and `enableOidc=false`, first sign-in enumerates eight folders. Restart loads the stored refresh token but `O365Token.refreshToken()` requests `Settings.getOutlookUrl()` (default Outlook resource), while `O365Authenticator.buildAuthorizeUrl()` requests Graph. Subsequent LIST returns no folders and EXAMINE INBOX fails HTTP 401 `InvalidAuthenticationToken`, “Access token validation failure. Invalid audience.” Pinned upstream source: [7.0.0 O365Token](https://github.com/mguessan/davmail/blob/7.0.0/src/java/davmail/exchange/auth/O365Token.java). [Live evidence](docs/validation/davmail-microsoft365.md).
- **Impact / workaround:** Restart/token renewal can break a working Graph connection. Local experiment sets `davmail.outlookUrl=https://graph.microsoft.com`; restart with the stored refresh token then enumerated eight folders and logged successful Graph refresh. This override is specific to the tested Graph-only configuration; normal expiry-driven renewal subsequently passed; full folder-move convergence remains unverified. No upstream patch or report submitted.
- **Dependencies / closure criteria:** Upstream fix or supported release must request the Graph audience during classic Graph refresh and pass initial sign-in, restart and actual expiry/refresh tests without this override. A local workaround does not close the upstream issue.

<a id="dav2"></a>

### DAV2 — DavMail Graph search false negatives in synthetic messages

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-4](PLAN.md#send-4).
- **Severity / proposed priority / status:** unspecified / P3 / deferred; external gateway behavior, cause unverified.
- **Evidence / reproduction (2026-10-14):** A delivered Python-generated synthetic self-mail was visible in webmail. In one fresh gateway IMAP connection, `UID SEARCH HEADER SUBJECT <unique-subject>` returned zero and `UID SEARCH SUBJECT <same-subject>` returned one; BODY.PEEK verified its body/attachment. A later Mailven-generated synthetic message returned one with both searches. Final Sent inventory also omitted that original send with broad SUBJECT `MAILVEN-DAVMAIL-`, while its full unique subject returned the original plus reply. [Experiment evidence](docs/validation/davmail-microsoft365.md).
- **Source / uncertainty:** Pinned 7.0.0 `GraphExchangeSession.HeaderCondition` searches the MAPI `String 0x007D` transport-header property; SUBJECT searches the normal Graph subject field. This explains distinct filter paths, but not whether the first message's transport-header property is absent, formatted differently or indexed later. Fetching one message and successfully searching a different one does not prove a cache repair.
- **Impact / workaround:** Header-search false negatives can invalidate a delivery check. This experiment uses targeted SUBJECT search plus independent webmail/content evidence and never resends an uncertain message. Mailven interoperability is assessed separately.
- **Dependencies / closure criteria:** Controlled same-message comparisons and service-side property evidence identify the cause; a supported gateway release/configuration must return the expected messages for HEADER SUBJECT and broad SUBJECT queries, or its limitation must be explicitly documented upstream. No upstream report submitted.

<a id="dav3"></a>

### DAV3 — Mailven/DavMail move reconciliation stalls on pool acquisition

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-4](PLAN.md#send-4).
- **Severity / proposed priority / status:** unspecified / P2 / deferred; failing layer not isolated.
- **Evidence (2026-10-14):** Synthetic Mailven move appeared in the dedicated server destination. Subsequent folder reconciliation stalled, and Mailven logged repeated “Timed out waiting for connection from pool” with a 120000 ms timeout. The local synthetic seed retained a negative temporary UID in the move destination while the message was moved back through webmail. Concurrent UI/background synchronization was present; one probe also received `context canceled`. [Experiment evidence](docs/validation/davmail-microsoft365.md).
- **Relevant code:** `app/actions.go` (`MoveToFolder`), `internal/sync/folders.go`, `internal/sync/messages.go`, `internal/sync/fetch.go` and `internal/imap/pool.go`. Folder sync, body fetch and move operations share the per-account pool (three connections; two-minute waiter timeout). This matches the warning duration but does not identify which operation held a lease.
- **Impact / uncertainty:** Local folder state can remain unconverged during this tested workflow. Neither a Mailven pool leak nor a DavMail deadlock is established. A duplicated read-only verification probe was stopped; earlier timeout evidence predates that duplicate. After Mailven restart, manual-only polling and serialized refresh, the cache still showed the seed in both synthetic folders; a fresh server inventory showed one in the root and none in the destination. Gateway restart restored connectivity but did not remove the stale local copy. Clean reproduction and cause isolation are still needed.
- **Dependencies / closure criteria:** Identify the failing layer using bounded pool/gateway evidence and reproduce a serialized synthetic move in both directions, with no temporary UID left behind, no pool timeout and convergence within the configured observation window. Resolve only after a proved fix or supported configuration; do not weaken concurrency checks to hide the symptom.

<a id="cd2"></a>

### CD2 — Autosave encryption failure policy

- **Owner / milestone:** [SEND](PLAN.md#send) / [SEND-1](PLAN.md#send-1).
- **Severity / proposed priority / status:** medium / P1 / resolved.
- **Resolution / evidence:** SEND-1 verified 2026-10-14. Ordinary autosave rejects requested S/MIME/PGP encryption failures before persistence or new sync; bodies and attachments never fall back to plaintext. Missing/unusable-key fixtures cover both composers and failed create/update, with prior rows unchanged. Plaintext and successful encrypted round trips and recovery regressions pass; independent acceptance passed. [Validation and limits](docs/validation/send-1.md). No real drafts or live providers were touched.

<a id="api"></a>

## API — Provider API activation and migration

Project: [API](PLAN.md#api).

<a id="ma4"></a>

### MA4 — Shared mailbox API activation

- **Owner / milestone:** [API](PLAN.md#api) / [API-1](PLAN.md#api-1), proposed [API-6](PLAN.md#api-6).
- **Severity / proposed priority / status:** low / P2 / deferred (local implementation done in API-6; remaining Microsoft work deferred 2026-10-14; live evidence pending).
- **Dependencies / gates:** Recorded live Graph identity, delegation, sender/Sent/Bcc evidence under separate authorization.
- **Resolution criteria:** Shared identity storage and activation are designed and verified before shared migration is offered.
- **Evidence / impact / reproduction:** original recorded finding below; blockers rechecked read-only on 2026-10-14; [design](docs/SHARED_MAILBOX_API_DESIGN.md) documents identity/grant/activation requirements. API-6 implemented identity storage, parent-owned grants and consent locally ([evidence](docs/validation/api-6.md)); activation and migration stay closed by `sharedMailboxLiveVerified=false` until live checks are recorded.

- **MA4 low** `app/api_route.go` `ActivateMailAPIRoute`: shared (delegated) mailbox routes cannot be activated because the Graph shared-mailbox address is not stored and `apiRemote` rejects shared routes. Found in F4e-3; needs shared-mailbox identity storage before a later milestone. F5a left it open: storing the address is not cheap (route schema, Graph delegate flow, read/write clients); shared routes stay unmigratable with that reason.

<a id="f5b-1"></a>

### F5b-1 — Full migration snapshot restoration

- **Owner / milestone:** [API](PLAN.md#api) / [API-2](PLAN.md#api-2) (design); [API-3](PLAN.md#api-3) (implementation).
- **Severity / proposed priority / status:** low / P2 / resolved.
- **Resolution / evidence:** API-3 verified 2026-10-14. An explicit, acknowledged whole-database restore from the pre-migration snapshot, applied offline at next start with a never-overwritten pre-restore backup and undo. Preflight proves hash, integrity, forward migration and credential key on a copy. Crash fixtures at every phase boundary keep the live database whole and resume. Post-restore rules hold API actions as `unknown` (never replayed), deactivate the route, clear search and report sign-in and orphaned-credential follow-ups. The consent UI states that provider-side changes are not undone. Go fixtures, full Go suite and full UI suite (Accounts expanded and collapsed) pass. [Evidence and limits](docs/validation/api-3.md).
- **Preserved limits:** No native-window or real-account restore was run; Windows replace semantics and Flatpak paths are unverified. Live restore stays separately gated.

<a id="f5b-3"></a>

### F5b-3 — Rollback sync progress and scheduler coverage

- **Owner / milestone:** [API](PLAN.md#api) / [API-5](PLAN.md#api-5).
- **Severity / proposed priority / status:** low / P2 / resolved.
- **Resolution / evidence:** API-5 verified 2026-10-14. A narrow scheduler seam proves one request after restoring the IMAP route/rows, return before reconciliation completes, and no new request on failed/absent-scheduler/already-inactive paths. The report's `syncRequested` flag and UI describe invocation only, not start/completion/success; false directs users to manual Sync. Focused Go checks and full UI suite pass, including Accounts expanded/collapsed. [Evidence and limits](docs/validation/api-5.md).
- **Preserved limits:** Async sync can fail or be skipped by the scheduler; rollback does not restore a snapshot or replay provider actions. No live provider/migration check was performed.

<a id="sync"></a>

## SYNC — Mail synchronization and action reliability

Project: [SYNC](PLAN.md#sync).

<a id="r2"></a>

### R2 — Sent-copy reconciliation cost

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-4](PLAN.md#sync-4).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** SYNC-4 verified 2026-10-14. `saveSentCopies` calls `Store.ReconcileAttempt`, which scopes the observation UPDATE and the copy query to the new attempt; `ReconcileCopies` keeps the whole-account view. Scoped fixture passes. [Validation and limits](docs/validation/sync-4.md).

<a id="sy2"></a>

### SY2 — Oversized Graph message blocks cursor

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-3](PLAN.md#sync-3).
- **Severity / proposed priority / status:** low / P2 / resolved.
- **Resolution / evidence:** SYNC-3 verified 2026-10-14. A Graph message over the download cap is kept as a header-only row, logged and counted in `GraphResult.Oversize`; the folder cursor advances. Fixture passes. [Validation and limits](docs/validation/sync-3.md).

<a id="sy3"></a>

### SY3 — Graph delta reconciliation cost

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-4](PLAN.md#sync-4).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** SYNC-4 verified 2026-10-14. `deltaRun` collects listed IDs only for a reconcile and diffs them with the sorted-slice `localOnly` instead of a map; the per-ID verification GETs stay, since they protect moved messages, and are bounded by the missed IDs. [Validation and limits](docs/validation/sync-4.md).

<a id="sy4"></a>

### SY4 — Gmail reconciliation memory

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-4](PLAN.md#sync-4).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** SYNC-4 verified 2026-10-14. The duplicate ID map is replaced by `localOnly` (sort and binary search). Synthetic 1M-ID benchmark: 56.6 MB/op to about 0.7 MB/op extra, 162 ms to 75 ms. The enumerated ID slice remains, because nothing commits before enumeration completes. [Validation and limits](docs/validation/sync-4.md).

<a id="sy5"></a>

### SY5 — Gmail reset requests and 5xx classification

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-4](PLAN.md#sync-4).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** SYNC-4 verified 2026-10-14. Recorded as policy and pinned by tests: a reset costs one `minimal` GET per known message and no raw GET for unchanged ones; a 5xx shares the throttle path with at most `apiRetryLimit` retries and raises no authorization failure. [Validation and limits](docs/validation/sync-4.md).

<a id="sy6"></a>

### SY6 — Missing Date uses ingest time

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-3](PLAN.md#sync-3).
- **Severity / proposed priority / status:** low / P2 / resolved.
- **Resolution / evidence:** SYNC-3 verified 2026-10-14. `parseAPIRaw` uses the provider receive time (Gmail `internalDate`, Graph `receivedDateTime`) for `Date` and `ReceivedAt` when the header is missing. Fixtures pass. [Validation and limits](docs/validation/sync-3.md).

<a id="r3"></a>

### R3 — API move re-key failure

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-1](PLAN.md#sync-1).
- **Severity / proposed priority / status:** medium / P1 / resolved.
- **Resolution / evidence:** SYNC-1 verified 2026-10-14. A failed local re-key after a provider move keeps the action open and retries only the re-key; queued actions stay behind it and cannot use the stale ID. Fault-injection test passes. [Validation and limits](docs/validation/sync-1.md).

<a id="r4"></a>

### R4 — Partial batch move UI refresh

- **Owner / milestone:** [SYNC](PLAN.md#sync) / [SYNC-2](PLAN.md#sync-2).
- **Severity / proposed priority / status:** low / P2 / resolved.
- **Dependencies / gates:** Partial batch failure fixture.
- **Resolution criteria:** Already moved rows and counts refresh when a later batch item fails.
- **Resolution / evidence:** SYNC-2 verified 2026-10-14. `apiMoveLocal` emits `messages:moved` and refreshes counts for rows moved before the failure, then returns the error. Fixture passes. [Validation and limits](docs/validation/sync-2.md).
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **R4 low** `app/api_actions.go` `apiMove`: a mid-batch failure returns before
  `apiMoveLocal` emits `messages:moved`/counts, so rows already moved locally stay
  shown in the source list until the next refresh. (The queued moves are now
  kicked.)

<a id="chat"></a>

## CHAT — Chat composer sender review

Project: [CHAT](PLAN.md#chat).

<a id="ch1"></a>

### CH1 — Pending bubble reads "Sending…" during chat sender review

- **Owner / milestone:** [CHAT](PLAN.md#chat) / found during [CHAT-1](PLAN.md#chat-1); no milestone assigned.
- **Severity / proposed priority / status:** low / P3 / open.
- **Location:** `frontend/src/lib/components/chat/chatComposerState.svelte.ts` `#deliver`: the bubble is set to `queued` before `GetChatSenderEvidence` and the review dialog run.
- **Failure:** while the **Use this From and send** dialog is open, the reply bubble already shows "Sending…" although nothing is queued (seen in the CHAT-1 UI screenshots). Cancel or confirm resolves it correctly.
- **Closure criteria:** the bubble shows a review/waiting state (or none) until `QueueChatReply` returns, covered by a `chat-compose` UI assertion.

<a id="r1"></a>

### R1 — Chat sender review

- **Owner / milestone:** [CHAT](PLAN.md#chat) / [CHAT-1](PLAN.md#chat-1).
- **Severity / proposed priority / status:** medium / P2 / resolved.
- **Dependencies / gates:** Retain existing sender review policy.
- **Resolution criteria:** Chat can perform required sender review with keyboard-accessible UI and verified queue guards.
- **Resolution / evidence:** CHAT-1 verified 2026-10-14. Chat calls `GetChatSenderEvidence`, shows the review dialog and sends `senderReviewToken`; `QueueChatReply` and the send path still reject missing/stale tokens and blocked evidence. `tests/ui/chat-compose.test.cjs` covers confirm (Accounts expanded and collapsed), cancel, stale token and block. Live provider review was not exercised.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **R1 medium** `app/chat_reply.go` `QueueChatReply`: the chat composer has no
  sender review UI, so a reply from a sender needing review (Microsoft, shared
  mailboxes, non-Google OAuth, stale Gmail send-as) is refused at queue time and
  must be sent from the full composer. Add the review prompt to chat.

<a id="maint"></a>

## MAINT — Maintenance and documentation

Project: [MAINT](PLAN.md#maint).

<a id="ma2"></a>

### MA2 — Go formatting follow-up

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-2](PLAN.md#maint-2).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** MAINT-2 verified 2026-10-14. `gofmt -w internal/extensions/store.go`; `gofmt -l internal app extensions` is clean. See [evidence](docs/validation/maint-2.md).

<a id="ma6"></a>

### MA6 — Landing evidence branch coverage

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-5](PLAN.md#maint-5) review follow-up; user authorized both fixes on 2026-10-14.
- **Severity / proposed priority / status:** P2 (both review findings) / P2 / resolved.
- **Problem / reproduction:** `tools/project_land.py` accepted evidence based on a later feature commit, allowing an earlier broken Markdown link to land unchecked. Fresh `project-check.py --base main --run` evidence was rejected before rebase when main had diverged.
- **Resolution / evidence:** Check baselines normalize to a common ancestor; landing requires an ancestor of the landing merge base and complete feature-path coverage. Rebase still reruns checks. Both reproductions, restored-file coverage, older valid evidence and custom landing targets are covered by disposable fixtures; 35 helper tests and independent acceptance passed. See [validation](docs/validation/maint-5.md#review-follow-up-ma6).

<a id="d2"></a>

### D2 — TypeScript 7 tooling compatibility

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [D2](PLAN.md#d2).
- **Severity / proposed priority / status:** low / P3 / deferred.
- **Dependencies / gates:** Both tooling peer ranges block upgrade; recheck before proposing it.
- **Resolution criteria:** Both tools accept TypeScript 7 and a separately authorized upgrade passes required frontend checks.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **D2 low** `frontend/package.json`: TypeScript 7 remains blocked upstream.
  Verified on 2026-10-12 with `npm view <package>@latest version peerDependencies --json`:
  TypeScript 7.0.2 is published, but latest `typescript-eslint` 8.71.1 requires
  `>=4.8.4 <6.1.0` and `svelte-check` 4.7.6 requires `^5.0.0 || ^6.0.0`.
  Both tools are already current here; retain TypeScript ~6.0.3 until both accept 7.
  Recheck using the commands in `CONTRIBUTING.md`.

<a id="sy1"></a>

### SY1 — Duplicated API reader logic

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-2](PLAN.md#maint-2).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** MAINT-2 verified 2026-10-14. Read-client transport (401-once refresh, size cap, classification, `Retry-After`) now lives in `internal/apiread`; `gmailapi.ReadError` and `graphapi.ReadError` are aliases of `apiread.Error`. Existing provider tests pass unchanged. See [evidence](docs/validation/maint-2.md).

<a id="doc4"></a>

### DOC4 — All Inboxes shortcut guidance

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-1](PLAN.md#maint-1).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** MAINT-1 verified 2026-10-14. The user guide now describes `Alt+G` as jumping to the top of the sidebar, All Inboxes when present, matching `docs/KEYBOARD_SHORTCUTS.md` and `Sidebar.svelte` `selectFirstFolder`. Docs-only; checked by source comparison and anchor/link grep.

<a id="doc5"></a>

### DOC5 — Broken FAQ and log links

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-1](PLAN.md#maint-1).
- **Severity / proposed priority / status:** low / P3 / resolved.
- **Resolution / evidence:** MAINT-1 verified 2026-10-14. `faq.md` Svelte URL is `https://svelte.dev`; the `troubleshooting.md` log link points to the existing "Produce debug logs" section. Docs-only; checked by source comparison and anchor/link grep.

<a id="l2"></a>

### L2 — Legal review flag

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-3](PLAN.md#maint-3).
- **Severity / proposed priority / status:** unspecified (legal review flag, not a legal conclusion) / P2 / blocked.
- **Problem / evidence:** Terms wording: indemnity, limitation of liability and governing law. Current [draft](docs/TERMS.md) still carries a legal-review comment naming L2/L3. Original flags and review process are traceable with `git show 33c1851:PLAN.md` (Legal review flags). Current sign-off is not established by that comment or this task; do not assume the historical concern remains a verified defect.
- **Dependencies / closure:** owner supplies authoritative sign-off or selects review; record the decision and align draft markers only after verified approval. No legal text changes are authorized by backlog entry.

<a id="l3"></a>

### L3 — Legal review flag

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-3](PLAN.md#maint-3).
- **Severity / proposed priority / status:** unspecified (legal review flag, not a legal conclusion) / P2 / blocked.
- **Problem / evidence:** Privacy wording under bring-your-own OAuth, Google API policy statements and historical dictionary distribution. Current [draft](docs/PRIVACY.md) still carries a legal-review comment naming L2/L3. Original flags and review process are traceable with `git show 33c1851:PLAN.md` (Legal review flags). Current sign-off is not established by that comment or this task; do not assume the historical concern remains a verified defect.
- **Dependencies / closure:** owner supplies authoritative sign-off or selects review; record the decision and align draft markers only after verified approval. No legal text changes are authorized by backlog entry.

<a id="ui1"></a>

### UI1 — Light-theme primary text contrast

- **Owner / milestone:** [MAINT](PLAN.md#maint) / found by [MAINT-4](PLAN.md#maint-4); fix not selected.
- **Severity / proposed priority / status:** low / P3 / open.
- **Problem / evidence:** 2026-10-14, `make test-ui` UI audit (runtime contrast, backend-free fake data). In the light theme the header "Compose" label measures 3.57:1 (rgb(239,241,246) on rgb(93,129,172)) at 900 and 700 px, and the selected sidebar "Inbox" label measures 2.99:1 (rgb(93,129,172) on rgb(215,222,234)); WCAG AA normal text needs 4.5:1. Dark theme passes. `tools/contrast-check.mjs` checks text tiers on pane surfaces only, so it does not cover primary-on-primary/accent pairs.
- **Impact / limits:** reduced legibility for low-vision users in the light theme. Measured in Chromium against the dev build only; not checked on WebKitGTK.
- **Reproduction:** delete the UI1 entry in `tests/ui/ui-audit-allow.cjs`, then `node tests/ui/run.mjs ui-quality`.
- **Dependencies / closure:** theme token change reviewed for both themes; the UI1 allowlist entry is deleted and the audit passes.

<a id="ui2"></a>

### UI2 — Closed narrow sidebar stays in the Tab order

- **Owner / milestone:** [MAINT](PLAN.md#maint) / [MAINT-6](PLAN.md#maint-6); found by [MAINT-4](PLAN.md#maint-4).
- **Severity / proposed priority / status:** medium / P2 / resolved.
- **Problem / evidence:** 2026-10-14, `make test-ui` UI audit keyboard pass. Below 768 px the closed `aside.responsive-sidebar-overlay` (App.svelte, `SidebarFrame.svelte`, `app.css` `.responsive-sidebar-overlay`) is only translated off-screen, so about 16 of its controls (folders, account menus) still receive Tab focus while invisible.
- **Impact / limits:** keyboard and screen-reader users land on invisible controls in the narrow layout. The separate hidden-viewer finding is tracked as [UI4](#ui4).
- **Resolution / evidence:** [MAINT-6 evidence](docs/validation/maint-6.md): Mail and kit sidebar containers are inert only while closed in narrow mode. Regression tests cover closed/open/closed/resize for Mail and Contacts. The UI2 allowlist entry is removed; the full UI suite passes 58/58. Native WebKitGTK verification remains unrun.
- **Dependencies / closure:** closed overlays leave the Tab order (for example `inert`) without changing the open behavior; the UI2 allowlist entry is deleted and the audit passes.

<a id="ui3"></a>

### UI3 — Chat row menu lists Archive twice

- **Owner / milestone:** [MAINT](PLAN.md#maint) / found by [MAINT-4](PLAN.md#maint-4); fix not selected.
- **Severity / proposed priority / status:** low / P3 / open.
- **Problem / evidence:** 2026-10-14, `tests/ui/inbox-triage.test.cjs` (backend-free fakes). A chat row's context menu renders the chat's own Archive (`ChatRow.svelte` `extraItems`) and then the shared message Archive (`MessageContextMenu.svelte` Move/Delete actions). Both archive the same message IDs, but with different toasts ("Conversation archived" vs "Archived") and list follow-up.
- **Impact / limits:** redundant, slightly inconsistent menu; no data risk. Not checked in the open chat's header menu.
- **Reproduction:** right-click any chat row in the chat list.
- **Dependencies / closure:** the chat row menu shows one Archive with the chat behavior; `inbox-triage.test.cjs` `menu()` no longer needs `.first()` for Archive.

<a id="ui4"></a>

### UI4 — Closed responsive viewer retains focusable controls

- **Owner / milestone:** [MAINT](PLAN.md#maint) / found during [MAINT-6](PLAN.md#maint-6); fix not selected.
- **Severity / proposed priority / status:** medium / P2 / open.
- **Problem / evidence (2026-10-14):** Static review confirms Mail's `App.svelte` viewer and kit `DetailPane.svelte` remain mounted in closed responsive overlays. `app.css` `.responsive-viewer-overlay` only translates them off-screen; neither container disables focus on its descendant controls, including the Back button. This affects medium and narrow layouts. Runtime viewer-specific audit and native WebKitGTK verification remain unrun.
- **Impact / reproduction:** Return from a viewer to the list in a responsive layout; hidden viewer controls can still receive keyboard focus. The existing Mail UI audit does not establish coverage of every such control.
- **Dependencies / closure:** Separately select viewer scope; closed viewers must reject focus while opening, closing, selection and focus-mode behavior remain correct, with regression coverage for Mail and the shared detail pane.

<a id="ai"></a>

## AI — AI development workflows

Project: [AI](PLAN.md#ai).

<a id="ai6"></a>

### AI6 — Remaining workflow routing and context evidence

- **Owner / milestone:** [AI](PLAN.md#ai) / [G4](PLAN.md#g4).
- **Severity / proposed priority / status:** unspecified (verification follow-up, not a confirmed defect) / P3 / deferred.
- **Problem / evidence:** broader Claude workflow compliance/routing beyond handoff and comparable context savings remain unverified. [G1–G3 record](docs/validation/shared-skills-g3.md) distinguishes discovery, parsing and runtime evidence; [AI5](docs/validation/shared-handoff-ai5.md) completed bidirectional handoff verification on 2026-10-14. Those results do not establish the remaining G4 claims.
- **Dependencies / closure:** explicit selection on a new branch; configured-host availability, comparable measurements and representative routing trials. Close only with recorded evidence or an explicit decision to retire the claims; do not alter personal CLI settings.

<a id="wails"></a>

## WAILS — Desktop framework migration assessment

Project: [WAILS](PLAN.md#wails).

All future Wails work is deferred by owner decision on 2026-10-14 until the official stable Wails v3 release; owner selection is still required afterward. No verified issue recorded. Proposed research questions and risks remain in [the assessment brief](docs/PLAN-migration.md); do not turn them into confirmed defects.

## Recurring problems

None. Track environment/tooling symptoms, workaround and count here; on a second occurrence or a costly first occurrence, promote the solution into applicable guidance and remove the tally.
