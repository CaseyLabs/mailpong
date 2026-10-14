# Issue register

Authoritative unresolved findings and follow-ups, grouped by primary project. [Roadmap](PLAN.md) and [lifecycle rules](docs/PROJECT_MANAGEMENT.md) define selection, statuses and closure. Severity describes impact; priority describes recommended execution order and does not authorize work. All P1/P2/P3 assignments below are proposals from the 2026-10-14 reorganization; original severities are unchanged.

The 2026-10-08 audit used “unverified” for reported findings not confirmed against source. This reorganization preserves recorded evidence without independently verifying product defects. The P1 provider audit confirmed findings statically; F1–F3b fixed PD1/PD9, PD3–PD8 and U3 within their local scope, while PD2 remains a live gate. See [provider review](docs/validation/provider-deliverability-review.md), [implementation evidence](docs/validation/provider-implementation.md) and [P2 architecture](docs/validation/provider-transport-recommendations.md). Resolved findings are not reopened.

## Index

- [SEND — Sending, privacy and compatibility](#send): [MA1](#ma1), [PD2](#pd2) (resolved: [CD2](#cd2))
- [API — Provider API activation and migration](#api): [MA4](#ma4), [F5b-1](#f5b-1), [F5b-3](#f5b-3)
- [SYNC — Mail synchronization and action reliability](#sync): none (resolved: [R2](#r2), [R3](#r3), [R4](#r4), [SY2](#sy2), [SY3](#sy3), [SY4](#sy4), [SY5](#sy5), [SY6](#sy6))
- [CHAT — Chat composer sender review](#chat): [CH1](#ch1) (resolved: [R1](#r1))
- [MAINT — Maintenance and documentation](#maint): [MA2](#ma2), [D2](#d2), [SY1](#sy1), [L2](#l2), [L3](#l3), [UI1](#ui1), [UI2](#ui2), [UI3](#ui3) (resolved: [DOC4](#doc4), [DOC5](#doc5))
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
- **Severity / proposed priority / status:** medium / P1 / blocked.
- **Dependencies / gates:** Explicit account-type evidence and separately authorized live checks; no endpoint migration approval.
- **Resolution criteria:** Recorded endpoint/account-type evidence and authorized compatibility results resolve the gate; any migration is separately scoped.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **PD2 medium — live compatibility gate** `app/oauth.go`, `internal/credentials/oauth_grants.go`: F1 corrects the setup guides and new personal/work defaults, verified by disposable fixtures and guarded UI checks. Existing Microsoft accounts retain their configured endpoints and unknown account type where no explicit evidence was recorded. Their prior `smtp.office365.com` endpoint can still differ from Outlook.com's published default; actual endpoint failure and migration compatibility remain unverified. Confirm account type and separately authorize live evidence before proposing any existing-account endpoint migration.

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

- **Owner / milestone:** [API](PLAN.md#api) / [API-1](PLAN.md#api-1).
- **Severity / proposed priority / status:** low / P2 / deferred.
- **Dependencies / gates:** Shared identity/schema/delegate/read-write design first.
- **Resolution criteria:** Shared identity storage and activation are designed and verified before shared migration is offered.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **MA4 low** `app/api_route.go` `ActivateMailAPIRoute`: shared (delegated) mailbox routes cannot be activated because the Graph shared-mailbox address is not stored and `apiRemote` rejects shared routes. Found in F4e-3; needs shared-mailbox identity storage before a later milestone. F5a left it open: storing the address is not cheap (route schema, Graph delegate flow, read/write clients); shared routes stay unmigratable with that reason.

<a id="f5b-1"></a>

### F5b-1 — Full migration snapshot restoration

- **Owner / milestone:** [API](PLAN.md#api) / [API-2](PLAN.md#api-2); implementation: [API-3](PLAN.md#api-3).
- **Severity / proposed priority / status:** low / P2 / deferred.
- **Dependencies / gates:** Live-DB replacement design and data-loss UX before implementation API-3.
- **Resolution criteria:** Approved restore design and later isolated restore validation prove data/credential safety and explicit loss consent.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **F5b-1 (low)** — `app/api_migration.go` `RollbackMailAPIMigration`: full snapshot restore is not implemented; after a rollback only route state changes, so API-era local changes stay. F5c shows the snapshot path with a note but does not offer restore; it needs its own live-DB replacement design and data-loss UX.

<a id="f5b-3"></a>

### F5b-3 — Rollback sync progress and scheduler coverage

- **Owner / milestone:** [API](PLAN.md#api) / [API-5](PLAN.md#api-5).
- **Severity / proposed priority / status:** low / P2 / deferred.
- **Dependencies / gates:** Scheduler seam and rollback progress contract required.
- **Resolution criteria:** Fixture asserts the rollback sync trigger and any promised completion/progress behavior.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **F5b-3 (low)** — `app/api_migration.go` `RollbackMailAPIMigration`: the IMAP reconcile is only the async `syncScheduler.TriggerSync` inside `deactivateAPIRoute`; rollback does not wait for it and no test asserts the trigger (the scheduler is a concrete type). Add a scheduler seam and a test when F5c needs rollback progress.

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
- **Severity / proposed priority / status:** low / P3 / deferred.
- **Dependencies / gates:** Own the affected package before formatting.
- **Resolution criteria:** gofmt reports no formatting changes for the affected file.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **MA2 low** `internal/extensions/store.go`: `gofmt -l` lists the file as unformatted (noticed during F4e-1; untouched there). Run `gofmt -w` on it in a change that owns that package.

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
- **Severity / proposed priority / status:** low / P3 / deferred.
- **Dependencies / gates:** Behavior-preserving extraction only.
- **Resolution criteria:** Shared reader behavior preserves refresh, caps and error classification in both clients.
- **Evidence / impact / reproduction:** original recorded finding below; product behavior was not rechecked in this task. Use its named symbol and fixtures when selecting work.

- **Low** `internal/gmailapi/read.go`, `internal/graphapi/read.go`: `Reader`/`ReadError`/fetch (401-once refresh, size cap, classification) are duplicated across the two read clients. Found by F4d-3.

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

- **Owner / milestone:** [MAINT](PLAN.md#maint) / found by [MAINT-4](PLAN.md#maint-4); fix not selected.
- **Severity / proposed priority / status:** medium / P2 / open.
- **Problem / evidence:** 2026-10-14, `make test-ui` UI audit keyboard pass. Below 768 px the closed `aside.responsive-sidebar-overlay` (App.svelte, `SidebarFrame.svelte`, `app.css` `.responsive-sidebar-overlay`) is only translated off-screen, so about 16 of its controls (folders, account menus) still receive Tab focus while invisible.
- **Impact / limits:** keyboard and screen-reader users land on invisible controls in the narrow layout. The hidden medium-layout viewer overlay may behave the same; not yet audited separately.
- **Reproduction:** delete the UI2 entry in `tests/ui/ui-audit-allow.cjs`, then `node tests/ui/run.mjs ui-quality` (700 px configurations report `focus-invisible`).
- **Dependencies / closure:** closed overlays leave the Tab order (for example `inert`) without changing the open behavior; the UI2 allowlist entry is deleted and the audit passes.

<a id="ui3"></a>

### UI3 — Chat row menu lists Archive twice

- **Owner / milestone:** [MAINT](PLAN.md#maint) / found by [MAINT-4](PLAN.md#maint-4); fix not selected.
- **Severity / proposed priority / status:** low / P3 / open.
- **Problem / evidence:** 2026-10-14, `tests/ui/inbox-triage.test.cjs` (backend-free fakes). A chat row's context menu renders the chat's own Archive (`ChatRow.svelte` `extraItems`) and then the shared message Archive (`MessageContextMenu.svelte` Move/Delete actions). Both archive the same message IDs, but with different toasts ("Conversation archived" vs "Archived") and list follow-up.
- **Impact / limits:** redundant, slightly inconsistent menu; no data risk. Not checked in the open chat's header menu.
- **Reproduction:** right-click any chat row in the chat list.
- **Dependencies / closure:** the chat row menu shows one Archive with the chat behavior; `inbox-triage.test.cjs` `menu()` no longer needs `.first()` for Archive.

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

No verified issue recorded. Proposed research questions and risks remain in [the assessment brief](docs/PLAN-migration.md); do not turn them into confirmed defects.

## Recurring problems

None. Track environment/tooling symptoms, workaround and count here; on a second occurrence or a costly first occurrence, promote the solution into applicable guidance and remove the tally.
