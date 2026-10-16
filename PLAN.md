# Project roadmap

Authoritative portfolio of remaining work. [Issue register](ISSUES.md) owns findings; [project-management rules](docs/PROJECT_MANAGEMENT.md) define IDs, statuses, authorization and execution plans. **No active milestone.** All Microsoft work is deferred indefinitely until the owner explicitly reopens it. All future Wails work, including assessment, is deferred until Wails v3 officially releases as a stable version; release availability alone does not authorize execution. Owner decision: 2026-10-14. The DavMail experiment is preserved for future work. Other proposed or deferred work remains unauthorized.

## Portfolio

Priorities are recommendations assigned in this reorganization: P1 privacy/data integrity/correctness, P2 functional follow-ups, P3 maintenance/investigation. Original severities remain in ISSUES. Dependency order takes precedence over priority; independently executable work is ordered by priority. Projects with mixed gated work remain proposed; milestone statuses carry individual gates.

| Project | Status | Proposed priority | Unresolved issues | Candidate / gate |
| --- | --- | --- | --- | --- |
| [SEND — Sending, privacy and compatibility](#send) | proposed | P1 | [4](ISSUES.md#send) | PD10 resolved (Gmail final-MIME validation); SEND-3/4 deferred; SEND-2 retains its live gate, with Microsoft work deferred |
| [API — Provider API activation and migration](#api) | proposed | P2 | [1](ISSUES.md#api) | API-7 done (MA7, MA8, SY9 resolved); API-3 done (snapshot restore); Microsoft shared rollout deferred and live-gated |
| [SYNC — Mail synchronization and action reliability](#sync) | proposed | P1 | [1](ISSUES.md#sync) | SYNC-1–4 done; SY9 resolved by API-7; SYNC-5 done (Gmail Load older, SY10 partly); SYNC-7 done (older IMAP mail, SY10 partly); SYNC-6 deferred (Graph window) |
| [GOAUTH — Google OAuth client setup](#goauth) | proposed | P2 | [0](ISSUES.md#goauth) | GOAUTH-1–4 done (GO1, GO2 resolved); GOAUTH-5 done (assessment; owner kept bring-your-own) |
| [CHAT — Chat composer sender review](#chat) | proposed | P3 | [1](ISSUES.md#chat) | CH1 unassigned (cosmetic) |
| [MAINT — Maintenance and documentation](#maint) | proposed | P2 | [1](ISSUES.md#maint) | MAINT-8 done (UI4 resolved); D2 deferred/upstream blocked; MAINT-9 done (TF1 resolved); MAINT-10 done (UI3 resolved); MAINT-11 done (DOC6 resolved); MAINT-12 done (UI5 resolved); MAINT-13 done (UI6 resolved); TF2 resolved |
| [UIX — UI refresh](#uix) | proposed | P3 | 0 | UIX-1 (UI1 resolved) to UIX-6 done |
| [AI — AI development workflows](#ai) | deferred | P3 | [1](ISSUES.md#ai) | G4 deferred; no eligible work |
| [WAILS — Desktop framework migration assessment](#wails) | deferred | P3 | [0](ISSUES.md#wails) | W3-M0–M7 deferred until official stable Wails v3 release and owner selection |

**Remaining nondeferred scope:** SEND-2 has non-Microsoft privacy/size-limit investigation and separately gated live verification; MAINT-3 is complete following owner disposition. Product follow-up CH1 remains open in ISSUES.md with no milestone; UI1 to UI6 are resolved. Completed milestones are not new implementation work. No automatic selection or status transition occurs here.

## Scope and preserved decisions

Provider F1–F5 is complete for its local scope; completion does not establish live provider compatibility. Retain full Gmail API and delegated Microsoft Graph direction, future shared-mailbox support and explicit opt-in migration. Shared API activation remains unsupported. Existing accounts retain configured routes until explicitly migrated; there is no automatic route switch or fallback. Snapshot rollback currently changes route only and does not restore data or undo remote actions.

No live provider access, test emails, credentials changes, mailbox migration, provider registrations, tenant/DNS changes, dependency upgrades or personal CLI configuration changes are authorized by this roadmap. Live checks need accounts, From identities, controlled recipients and cases specified separately; see [live limits](docs/MAIL_APIS.md#live-verification-limits) and [P2 parity scenarios](docs/validation/provider-transport-recommendations.md). Wails research additionally prohibits application/dependency/packaging changes, real data, releases and unapproved prototypes or installs under its existing brief. G4 and D2 remain deferred. All further Microsoft 365 and regular/personal Outlook.com work—including native OAuth, IMAP/SMTP, Graph, shared mailboxes, DavMail, live validation and migration—is deferred indefinitely until explicitly reopened by the owner. This also applies to Microsoft-specific work inside mixed-provider milestones; unrelated provider work retains its existing status and gates.

## Progress

No active milestone.

**Next recommended:** none eligible — SEND-2's live Gmail size case needs owner authorization; otherwise turn CH1 (cosmetic chat composer sender review) into a milestone. MAINT-13 is done (2026-10-16): all 15 oversized components are under 500 lines (largest now 493), split into state classes, logic modules and child components without behavior change, with one commit per component (UI6 resolved; [checkpoint](docs/plans/maint-13.md); backend-free UI checks only, native unrun). MAINT-12 is done (2026-10-16): `App.svelte` is 361 lines; selection and focus mode, composer state, the global and Mail key handlers, bulk message actions, startup events and the window overlays moved into modules without behavior change (UI5 resolved; backend-free UI checks only, native unrun). MAINT-11 is done (2026-10-16): the provider and settings guides describe the offline whole-database snapshot restore with consent, backup and undo, and say route rollback does not restore it (DOC6 resolved; documentation only, no live restore). MAINT-10 is done (2026-10-16): a chat row menu shows one Archive, with the chat behavior (UI3 resolved; backend-free UI checks only, native unrun). MAINT-9 is done (2026-10-16): the calendar Syncer tracks its background goroutines and the test waits for them, so `TestSyncerSubscriptionsPerInstance` passes at `-count=100` (TF1 resolved). The owner set this order on 2026-10-16: when a MAINT milestone from MAINT-10 to MAINT-13 is done, its close-out sets the next one in ID order (then 13) as Next recommended. Each still runs in its own session when selected. No other milestone is eligible (SEND-2 waits on live authorization; SYNC-6, Microsoft, Wails, G4 and D2 are deferred). PD10 was fixed on 2026-10-16 (Gmail sends run the SMTP final-MIME validation and fold a long hidden Bcc list; live behavior unrun); TF2 was fixed the same day.

API-7 is done (2026-10-15): Edit Account saves on a Gmail API route (MA8), an activation interrupted by quitting returns to IMAP at the next start (MA7), and removing an account no longer hangs on a busy IMAP connection (SY9) ([evidence](docs/validation/api-7.md)); live and native checks are unrun. SYNC-7 is done (2026-10-15): IMAP accounts get "Search the whole mailbox" and "Load older mail" like Gmail; the mail they bring in is pinned (`messages.pinned`), so the window's deletions skip it and a sync checks it by UID instead ([evidence](docs/validation/sync-7.md), [checkpoint](#sync-7)); the live IMAP check is unrun and Graph stays with SYNC-6. SYNC-5 is done (2026-10-15): Gmail API accounts get "Load older mail" at the end of the list, a reconcile notices pinned mail deleted on the server, and a search no longer blanks the history cursor; the unused IMAP search bindings are removed and IMAP follows as SYNC-7 ([evidence](docs/validation/sync-5.md), [checkpoint](#sync-5)); the live Gmail check is unrun. MAINT-8 is done and UI4 resolved (2026-10-15): closed responsive viewers in Mail and the kit detail pane reject keyboard focus, and closing returns focus to the list ([evidence](docs/validation/maint-8.md), [checkpoint](#maint-8)); native WebKitGTK unrun. GOAUTH-5 is done (2026-10-15): recommendation and owner decision recorded ([evidence](docs/validation/goauth-5.md), [checkpoint](#goauth-5)). GOAUTH-4 is done (2026-10-15): Setup checklist in OAuth Credentials and the Google sign-in form, scope list generated from `internal/oauth2` with drift tests, and all console links confirmed live ([evidence](docs/validation/goauth-4.md), [checkpoint](#goauth-4)); GOAUTH-5 stays deferred. GOAUTH-3 is done (2026-10-15): Settings → Accounts → OAuth Credentials has Import JSON…, which reads Google's Desktop client_secret file and saves it to the ticked Mail, Contacts and Calendar slots; web, malformed, oversized and non-UTF-8 files are rejected with a specific message and change nothing ([evidence](docs/validation/goauth-3.md), [checkpoint](#goauth-3)); the native dialog is unrun. GOAUTH-2 is done (2026-10-15): Settings → Identity → Check Google setup probes Gmail, People and Calendar read-only and offers Enable API or Re-authorize per result; it also runs once after the first Google sign-in ([evidence](docs/validation/goauth-2.md), [checkpoint](#goauth-2)); the owner-run live check is unrun; GOAUTH-3–5 remain unselected. GOAUTH-1 is done (2026-10-15): disabled Gmail, People and Calendar APIs are reported with the API, project and an Enable URL, and the docs say Gmail API is required for every Google OAuth account ([evidence](docs/validation/goauth-1.md), [checkpoint](#goauth-1)); GOAUTH-2–5 remain unselected. GOAUTH was proposed on 2026-10-15 (bring-your-own Google client: diagnosis, health check, JSON import, guided setup); none of it is selected. SEND-2 static review and authorized Gmail Bcc cases 1–4 are recorded (2026-10-15): neither provider documents Bcc-header stripping or an API size limit; the alias cases cannot show a separate recipient's copy and size case 5 is unauthorized, so SEND-2 and MA1 stay blocked. SY7/SY8 (Gmail quota backoff, sync-period window, oversize messages) were fixed and verified live on the way ([evidence](docs/validation/send-2.md)). UIX-6 is done: Settings → Chat → Message style switches the thread between bubbles (default) and a flat transcript with no bubbles or avatars, a name line on every message and an accent rule on mine ([evidence](docs/validation/uix-6.md), [checkpoint](#uix-6)); the UIX project is complete. UIX-5 is done: runs of three or more older read messages fold into one "N earlier messages" row (latest five, unread, drafts and subject headings never fold); rows are `aria-expanded` buttons and hold the scroll position; the low-priority-sender "Notice from X" variant is deferred because messages carry no sender-priority data ([evidence](docs/validation/uix-5.md), [checkpoint](#uix-5)); UIX-6 remains unselected. UIX-4 is done: the chat composer is a rounded card with the sender, Attach and Send inside and the Reply-to/recipients/Topic chip row below; the From is a read-only label and the attachment summary card is deferred ([evidence](docs/validation/uix-4.md), [checkpoint](#uix-4)); UIX-5–6 remain unselected. UIX-3 is done: labeled New message and Search rows lead the expanded Accounts sidebar, section labels are quieter, and each chat row shows one status glyph (sending, snoozed, unread); the needs-reply glyph is deferred because `ChatItem` has no latest-sender data ([evidence](docs/validation/uix-3.md), [checkpoint](#uix-3)); UIX-4–6 remain unselected. UIX-2 is done: the chat header is title, `⋯`, bordered Archive and Delete, with Snooze and Pin in the menu ([evidence](docs/validation/uix-2.md), [checkpoint](#uix-2)); UIX-3–6 remain unselected. UIX-1 is done and UI1 resolved: Nord surfaces retuned to near-neutral greys, light primary darkened to 4.5:1, `--radius` restored, audit allowlist empty ([evidence](docs/validation/uix-1.md), [checkpoint](#uix-1)); UIX-3–6 remain unselected. MAINT-3 is done following owner approval and factual privacy/Terms revisions; L2/L3 are resolved by recorded owner disposition, with legal enforceability limits retained ([evidence](docs/validation/maint-3.md), [checkpoint](#maint-3)). MAINT-6 is done and UI2 resolved: closed narrow Mail and shared extension sidebars reject focus, with open/close/resize regressions and the full UI audit passing ([evidence](docs/validation/maint-6.md), [checkpoint](#maint-6)). UI4 records the separately scoped hidden-viewer follow-up. On 2026-10-14 the owner additionally deferred all future Wails work, including W3-M0–W3-M7 assessment, until the official stable Wails v3 release, and confirmed indefinite deferral of all Microsoft work until explicit owner reopening. Wails work still requires selection after the release gate is met. On 2026-10-14 the owner requested merging the `experiment/davmail-microsoft365` evidence into main and deferring all further Microsoft 365 and regular/personal Outlook.com development. SEND-3, SEND-4 and the remaining API-6/MA4 work are deferred, as are Microsoft-specific portions of SEND-2/MA1. Resumption requires explicit owner selection; prior experiment authorization does not authorize continued live work while deferred. The [experiment checkpoint](docs/plans/davmail-microsoft365-test.md) preserves results, private artifact locations and possible future investigation steps. D7/D8 remain failed; no compatibility acceptance is implied by landing.

API-3 is done and F5b-1 resolved: offline snapshot restore with consent, backup and undo ([evidence](docs/validation/api-3.md), [checkpoint](docs/plans/api-3.md)); native and real-account restore remain unrun. API-6 local implementation is done ([evidence](docs/validation/api-6.md), [checkpoint](docs/plans/api-6.md)); shared activation stays closed pending live checks. MAINT-5 is done: all four compact workflow helpers, logged check evidence and guarded landing are implemented and verified ([evidence](docs/validation/maint-5.md), [checkpoint](docs/plans/maint-5.md)). API-5 is done and F5b-3 resolved: rollback reports only a sync request, with scheduler-ordering and UI coverage ([evidence](docs/validation/api-5.md), [checkpoint](docs/plans/api-5.md)). API-1 design is done ([evidence](docs/validation/api-1.md), [checkpoint](docs/plans/api-1.md)); MA5 was resolved by API-6 (verified by fixture) and MA4 stays open for live evidence. No next milestone is activated.

CHAT-1 is done and R1 resolved. MAINT-1 is done and DOC4/DOC5 resolved. MAINT-2 is done and MA2/SY1 resolved ([evidence](docs/validation/maint-2.md)). MAINT-4 is done ([validation evidence](docs/validation/maint-4.md)); its findings UI1, UI2 and UI3 are resolved by UIX-1, MAINT-6 and MAINT-10. SEND-1 is done and CD2 resolved; SYNC-1 is done and R3 resolved ([evidence](docs/validation/sync-1.md)); SYNC-2 is done and R4 resolved ([evidence](docs/validation/sync-2.md)); SYNC-3 is done and SY2/SY6 resolved ([evidence](docs/validation/sync-3.md)); API-4 is done and F5b-2 resolved ([evidence](docs/validation/api-4.md)); SYNC-4 is done and R2/SY3/SY4/SY5 resolved ([evidence](docs/validation/sync-4.md)); see [validation evidence](docs/validation/send-1.md). Other proposed milestones remain unselected; SEND-2/3 retain their gates, and G4/D2 remain deferred.

<a id="send"></a>

## SEND — Sending, privacy and compatibility

- **Objective / outcome:** Protect encrypted drafts and hidden recipients, and establish provider compatibility with explicit evidence.
- **Status / proposed priority:** proposed / P1.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#send); each linked milestone lists its relevant issues.
- **Technical references:** [SENDING](docs/SENDING.md), [provider review](docs/validation/provider-deliverability-review.md), [providers](docs/PROVIDERS.md).

<a id="send-1"></a>

### SEND-1 — Autosave encryption failure policy

- **Status / proposed priority:** done / P1. **Dependencies / gates:** none.
- **Scope / deliverable:** Define and implement the separately scoped ordinary-autosave failure policy; preserve pre-submission recovery protections.
- **Acceptance:** Missing-key and encryption-failure fixtures prove the selected policy never silently downgrades requested encryption; retain existing draft behavior outside that policy.
- **Validation:** Focused draft Go tests with disposable data; frontend lint/check and guarded UI if prompts change.
- **Issues:** [CD2](ISSUES.md#cd2). **Progress:** Verified fail-closed saves in both composers, with plaintext and encrypted success regressions. [Evidence](docs/validation/send-1.md); [completed checkpoint](docs/plans/send-1.md).

<a id="send-2"></a>

### SEND-2 — API privacy and limits verification

- **Status / proposed priority:** blocked / P1. **Dependencies / gates:** separate live authorization.
- **Scope / deliverable:** Investigate provider header stripping and the local 25 MiB ceiling; specify controlled Bcc and limit cases before requesting live access.
- **Acceptance:** Recorded authorized delivered-copy evidence establishes Bcc handling; document the ceiling and provider uncertainty. An unrun or failed privacy check keeps the rollout gate open.
- **Validation:** Static/provider-source review first; live checks only for authorized accounts, From identities, controlled recipients and cases.
- **Issues:** [MA1](ISSUES.md#ma1). **Progress:** static documentation review done 2026-10-15 ([evidence](docs/validation/send-2.md); [Gmail source review and controlled cases](docs/validation/send-2-gmail.md); [branch checkpoint](docs/plans/send-2.md)); authorized Gmail Bcc cases 1–4 run the same day with sender plus-aliases: case 3 passed, cases 1–2 inconclusive (only the Sent copy exists); size case 5 unauthorized and unrun, so the milestone stays blocked on separate-recipient evidence. Activating the route found and fixed [SY7](ISSUES.md#sy7)/[SY8](ISSUES.md#sy8) and recorded [MA7](ISSUES.md#ma7)/[MA8](ISSUES.md#ma8); Microsoft-specific work deferred by owner decision 2026-10-14.

<a id="send-3"></a>

### SEND-3 — Existing Microsoft compatibility assessment

- **Status / proposed priority:** deferred / P1. **Dependencies / gates:** owner decision 2026-10-14; explicit reopening, account-type evidence and live authorization.
- **Scope / deliverable:** Assess existing account endpoints without automatically migrating them; preserve new-account defaults.
- **Acceptance:** Account-type/endpoint evidence and authorized results are recorded; local fixtures are distinguished from live compatibility. Any existing-account migration requires separate scope.
- **Validation:** Disposable endpoint fixtures and separately authorized live scenarios from provider references.
- **Issues:** [PD2](ISSUES.md#pd2). **Progress:** not started.

<a id="send-4"></a>

### SEND-4 — DavMail Microsoft 365 experiment

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** owner decision 2026-10-14; explicit reopening required. Outlook-client-ID sign-in and Graph self-mail verified with a broad Office grant and refresh-audience workaround. Full folder-move/lifecycle convergence remains failed; original DavMail-client administrator gate is historical.
- **Scope / deliverable:** Execute the selected [test plan](docs/plans/davmail-microsoft365-test.md), using an isolated local profile and the authorized account; synthetic recipients are self-mail only. No project dependency upgrades, account migration or tenant changes.
- **Acceptance:** Record D1–D9 as PASS, FAIL, BLOCKED or UNRUN with redacted evidence and a supported compatibility decision. A blocked investigation can conclude with an evidenced gate; it cannot claim live compatibility or passing unrun criteria.
- **Validation:** Actual listener inspection, protocol checks and live Mailven tests where authorization/policy permit; documentation link checks and independent acceptance review. Live results remain distinct from static evidence.
- **Issues:** [DAV1](ISSUES.md#dav1), upstream refresh-audience defect with tested workaround; [DAV2](ISSUES.md#dav2), observed search false negatives; [DAV3](ISSUES.md#dav3), move/pool reconciliation stall. **Progress:** D1–D6 and D9 passed; D7/D8 failed full convergence. Self-mail delivery, attachments, replies, flags and real expiry-driven token renewal verified; DAV3 stale move copy persists after restart/serialized refresh. Task processes stopped, isolated account removed; private tokens and synthetic server evidence retained. [Live evidence](docs/validation/davmail-microsoft365.md); [continuation checkpoint](docs/plans/davmail-microsoft365-test.md).

<a id="api"></a>

## API — Provider API activation and migration

- **Objective / outcome:** Extend opt-in API activation and migration recovery safely while preserving existing accounts and route semantics.
- **Status / proposed priority:** proposed / P2.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#api); each linked milestone lists its relevant issues.
- **Technical references:** [MAIL_APIS](docs/MAIL_APIS.md), [P2 recommendations](docs/validation/provider-transport-recommendations.md), [implementation record](docs/validation/provider-implementation.md).

<a id="api-1"></a>

### API-1 — Shared mailbox activation design

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none; implementation requires later selection.
- **Scope / deliverable:** Design shared identity storage, route schema, delegate flow and read/write identity semantics; produce a bounded implementation proposal.
- **Acceptance:** Proposal maps all shared route blockers and data/security invariants with validation and migration gates; do not claim shared support from design alone.
- **Validation:** Trace apiRemote/activation and Graph clients; read-only review against MAIL_APIS and provider architecture.
- **Issues:** [MA4](ISSUES.md#ma4). **Progress:** design verified ([evidence](docs/validation/api-1.md), [checkpoint](docs/plans/api-1.md)); implemented locally under API-6.

<a id="api-2"></a>

### API-2 — Snapshot restoration design

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none; implemented by [API-3](#api-3).
- **Scope / deliverable:** Design live-DB replacement, crash recovery, data-loss consent and rollback; leave route-only rollback truthful.
- **Acceptance:** Approved design distinguishes local snapshot restore from undoing remote actions and covers active database replacement, credentials and data-loss UX.
- **Validation:** Review isolated backup/restore strategy and failure cases; no real database replacement.
- **Issues:** [F5b-1](ISSUES.md#f5b-1). **Progress:** design recorded ([design](docs/API_SNAPSHOT_RESTORE_DESIGN.md), [evidence](docs/validation/api-2.md), [checkpoint](docs/plans/api-2.md)); owner approved the design 2026-10-14; implemented by [API-3](#api-3).

<a id="api-3"></a>

### API-3 — Snapshot restoration implementation

- **Status / proposed priority:** done / P2. **Dependencies / gates:** [API-2](#api-2) (design approved); implementation selected and authorized 2026-10-14. Live migration/restore remains separately gated.
- **Scope / deliverable:** Implement only the selected restore design and consent flow after its gate.
- **Acceptance:** Isolated restore/crash fixtures and UI verify approved recovery and loss semantics; provider-side changes remain accurately disclosed.
- **Validation:** Disposable database fixtures, relevant Go checks, frontend lint/check and guarded UI; live migration requires separate authorization.
- **Issues:** [F5b-1](ISSUES.md#f5b-1). **Progress:** done; F5b-1 resolved ([evidence](docs/validation/api-3.md), [checkpoint](docs/plans/api-3.md)). Native/real-account restore unrun.

<a id="api-4"></a>

### API-4 — Migration correlation follow-up

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none.
- **Scope / deliverable:** Investigate absent/ambiguous Message-ID handling and propose safe state-carrying behavior or explicit limitation guidance.
- **Acceptance:** No ambiguous identity is guessed; fixtures cover absent IDs and duplicates; selected behavior and limitations are documented.
- **Validation:** Focused internal/apimigrate fixtures; guarded migration UI if changed.
- **Issues:** F5b-2 (resolved). **Progress:** done ([evidence](docs/validation/api-4.md)).

<a id="api-5"></a>

### API-5 — Rollback sync progress and coverage

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none.
- **Scope / deliverable:** Define rollback progress promises; add scheduler seam and trigger coverage if selected.
- **Acceptance:** A test asserts rollback triggers IMAP sync; UI and API distinguish route transition from asynchronous reconcile completion.
- **Validation:** Focused rollback/scheduler fixtures; frontend checks and guarded UI for progress changes.
- **Issues:** [F5b-3](ISSUES.md#f5b-3). **Progress:** verified sync request after IMAP route/row restoration, failure/unavailable/no-new-request cases and UI copy under both Accounts states; full UI 50/50 ([evidence](docs/validation/api-5.md), [checkpoint](docs/plans/api-5.md)).

<a id="api-6"></a>

### API-6 — Shared mailbox API route

- **Status / proposed priority:** deferred / P2. **Dependencies / gates:** owner decision 2026-10-14 defers remaining Microsoft work; local implementation retained. API-1 design; explicit reopening required. Exposing shared activation/migration additionally requires separately authorized live validation.
- **Scope / deliverable:** Implement the bounded [shared route design](docs/SHARED_MAILBOX_API_DESIGN.md), including parent-owned grants, child identity isolation, consent/refresh/removal safety, migration blockers and route-aware sender review.
- **Acceptance:** Disposable fixtures prove owner/child isolation and failure handling; no shared migration is exposed before recorded live identity/delegation/sender/Sent/Bcc evidence. Design alone does not establish support.
- **Validation:** [Acceptance mapping and gates](docs/SHARED_MAILBOX_API_DESIGN.md#acceptance-to-validation-mapping-for-later-implementation); Go checks, frontend lint/check and full UI suite if implemented; live checks only with separate authorization.
- **Issues:** [MA4](ISSUES.md#ma4); MA5 resolved; [MA1](ISSUES.md#ma1) remains the sending privacy gate. **Progress:** done locally 2026-10-14 ([evidence](docs/validation/api-6.md)); shared activation stays closed until live checks are separately authorized and recorded ([checkpoint](docs/plans/api-6.md)).

<a id="api-7"></a>

### API-7 — Gmail API route fixes from live testing

- **Status / proposed priority:** done (2026-10-15) / P2. **Dependencies / gates:** none; fixtures only, live checks unrun.
- **Scope / deliverable:** Edit Account saves on an API route ([MA8](ISSUES.md#ma8)); an activation interrupted by quitting returns to IMAP at the next start ([MA7](ISSUES.md#ma7)); removing an account no longer hangs on a busy IMAP connection ([SY9](ISSUES.md#sy9), owned by SYNC).
- **Validation:** [evidence](docs/validation/api-7.md). Go tests per fix that fail without it; the live Gmail and native checks are unrun. Found [TF2](ISSUES.md#tf2).

<a id="sync"></a>

## SYNC — Mail synchronization and action reliability

- **Objective / outcome:** Prevent action/cursor correctness failures and bound synchronization work using measured evidence.
- **Status / proposed priority:** proposed / P1.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#sync); each linked milestone lists its relevant issues.
- **Technical references:** [MAIL_APIS](docs/MAIL_APIS.md), [implementation record](docs/validation/provider-implementation.md).

<a id="sync-1"></a>

### SYNC-1 — Durable API move re-key recovery

- **Status / proposed priority:** done / P1. **Dependencies / gates:** none.
- **Scope / deliverable:** Preserve an action or retry re-key after provider success and local failure.
- **Acceptance:** Fault injection proves stale queued IDs cannot cause local deletion following re-key failure.
- **Validation:** Focused apiactions/database tests; broaden Go tests if shared action semantics change.
- **Issues:** [R3](ISSUES.md#r3). **Progress:** Failed re-keys keep the move open and retry only the re-key; fault-injection test passes. [Evidence](docs/validation/sync-1.md).

<a id="sync-2"></a>

### SYNC-2 — Partial move batch refresh

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none.
- **Scope / deliverable:** Emit accurate move/count state for successful rows before reporting later batch failure.
- **Acceptance:** Partial batch fixture proves successful rows refresh and failures remain visible.
- **Validation:** Focused app fixtures and guarded Mail UI, including Accounts expanded/collapsed.
- **Issues:** [R4](ISSUES.md#r4). **Progress:** `apiMoveLocal` refreshes rows moved before a later batch failure, then returns the error; fixture passes. UI check not run. [Evidence](docs/validation/sync-2.md).

<a id="sync-3"></a>

### SYNC-3 — Ingest cursor and timestamp correctness

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none.
- **Scope / deliverable:** Scope oversize Graph handling and provider received-time preservation as separate substeps.
- **Acceptance:** Oversize messages do not silently block an entire folder; missing Date uses stable provider time without falsely sorting as newly received.
- **Validation:** Disposable Graph/Gmail raw-message and cursor fixtures; relevant sync tests.
- **Issues:** [SY2](ISSUES.md#sy2), [SY6](ISSUES.md#sy6). **Progress:** Oversize Graph messages become header-only rows and the cursor advances; missing Date uses the provider receive time. Fixtures pass; no live check. [Evidence](docs/validation/sync-3.md).

<a id="sync-4"></a>

### SYNC-4 — Bounded reconciliation investigation

- **Status / proposed priority:** done / P3. **Dependencies / gates:** none; [SYNC-1](#sync-1) takes priority among independent work.
- **Scope / deliverable:** Measure journal/folder growth, reset request cost and 5xx policy; select bounded optimizations from evidence.
- **Acceptance:** Reproducible synthetic measurements and correctness fixtures justify changes; enumeration/copy correctness and retry bounds remain intact.
- **Validation:** Focused synthetic large-folder/journal tests; no live mailbox benchmarking.
- **Issues:** [R2](ISSUES.md#r2), [SY3](ISSUES.md#sy3), [SY4](ISSUES.md#sy4), [SY5](ISSUES.md#sy5). **Progress:** R2 scoped to the new attempt; SY3/SY4 drop the per-ID map for a sorted-slice diff (1M IDs: 56.6 MB to ~0.7 MB extra); SY5 pinned as policy by request-count and bounded-5xx fixtures. No live check. [Evidence](docs/validation/sync-4.md).

<a id="sync-5"></a>

### SYNC-5 — Older mail beyond the sync window

- **Status / proposed priority:** done (2026-10-15) / P2. **Dependencies / gates:** none. The live Gmail check (search, Load older, pinned deletion) is unrun and gated on the owner moving an account to the Gmail API route.
- **Scope / deliverable:** (1) Owner chose Gmail only: the unused IMAP `IMAPSearchFolder` / `FetchServerMessage` bindings and their engine code are removed; IMAP moves to [SYNC-7](#sync-7). (2) "Load older mail" at the end of a list stores and pins mail from before the window on the Gmail API route. (3) A reconcile fetches pinned refs outside the window and removes the ones Gmail deleted. A search no longer blanks the history cursor. Graph is out of scope.
- **Acceptance:** Older mail is reachable offline-first by search and by load-older without widening the period; a reconcile still removes unpinned out-of-window mail and notices deleted pinned mail; the Accounts column checks pass expanded and collapsed.
- **Validation:** [evidence](docs/validation/sync-5.md). Fake-server Go fixtures and `make test-ui` regressions; the live Gmail check is separately authorized and unrun.
- **Issues:** [SY10](ISSUES.md#sy10).

<a id="sync-7"></a>

### SYNC-7 — Older IMAP mail beyond the sync window

- **Status / proposed priority:** done (2026-10-15) / P3. **Dependencies / gates:** none left; owner selected it on 2026-10-15. The live IMAP check is unrun and needs an authorized account.
- **Scope / deliverable:** Add a pin concept for IMAP messages (`messages.pinned`, migration with rewind lines) that `DeleteOlderThan` and the windowed UID reconcile skip, an existence check for pinned UIDs (including the CONDSTORE path), "Search the server" in the chat search across the account's folders, and per-folder "Load older" paging.
- **Acceptance:** Older IMAP mail is reachable by search and Load older without widening the period; a sync removes unpinned out-of-window mail and notices pinned mail deleted on the server; the Accounts column checks pass expanded and collapsed.
- **Validation:** [evidence](docs/validation/sync-7.md). In-memory IMAP server fixtures (`internal/sync/imap_older_test.go`) and `make test-ui` regressions; the live IMAP check is separately authorized and unrun.
- **Issues:** [SY10](ISSUES.md#sy10) (Graph remains, [SYNC-6](#sync-6)).

<a id="sync-6"></a>

### SYNC-6 — Microsoft Graph reconcile window

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** only if Microsoft accounts matter to the owner; no live Microsoft access is authorized.
- **Scope / deliverable:** Apply the account sync period to the Graph reconcile, as Gmail and IMAP already do, and offer server search beyond it.
- **Acceptance:** Graph reconcile stays within the window and server search is bounded and tested with fixtures.
- **Issues:** [SY10](ISSUES.md#sy10).

<a id="goauth"></a>

## GOAUTH — Google OAuth client setup

- **Objective / outcome:** Help users create and verify their own Google OAuth Desktop client with as little manual console work as Google allows, and say exactly what is wrong when a client is misconfigured.
- **Status / proposed priority:** proposed / P2.
- **Dependencies / blockers:** none between projects. Order: GOAUTH-1 → GOAUTH-2 → GOAUTH-3 → GOAUTH-4. GOAUTH-5 is deferred until the owner decides.
- **Issues:** [owning register section](ISSUES.md#goauth); each milestone links its issues.
- **Technical references:** [custom Google credentials guide](docs/user-guide/getting-started/first-account/google-oauth-credentials.md), [Gmail setup](docs/user-guide/getting-started/first-account/gmail.md), [providers](docs/PROVIDERS.md), `frontend/src/lib/components/kit/OAuthCredsSlotEditor.svelte` (read [EXT_RULES](docs/EXT_RULES.md) before changing it), `internal/gmailapi/sendas.go`.
- **Decisions (2026-10-15, recommendation; not yet confirmed by the owner):**
  - Mailven keeps bundling no Google client; users bring their own.
  - The setup is guided by links and does not need gcloud. Google offers no API that creates a standard Desktop OAuth client or consent screen: the IAP OAuth Admin API shut down on 2026-03-19, and `gcloud iam oauth-clients` covers Workforce Identity Federation only. The console flow `console.cloud.google.com/flows/enableapi?apiid=…` enables several APIs in one step.
  - Rejected: a Mailven "bootstrap" client using the `cloud-platform` scope. It would automate only project creation and API enabling, which links already do. It would ask for broad access and need its own Google verification.
  - Facts the user-facing text must state:
    - In Testing status, sign-ins expire after 7 days.
    - In production status, an unverified app shows Google's warning screen. Personal use by a few users known to the owner is exempt from verification.
    - A client secret is shown only when the client is created.

<a id="goauth-1"></a>

### GOAUTH-1 — Disabled Google API diagnosis

- **Status / proposed priority:** done / P2 (selected by the owner 2026-10-15). **Dependencies / gates:** none. Local fixtures only.
- **Result:** a 403 `SERVICE_DISABLED` from the Gmail send-as list, People (search, sync, writes) and Calendar becomes a typed `googleerr.DisabledError` ("*Gmail API is disabled in Google Cloud project N*") with an Enable URL Mailven builds from the validated service and project. Other 403s keep their handling. PROVIDERS.md, gmail.md and the custom-credentials guide agree that every Google OAuth account needs the Gmail API. Evidence and limits (local fixtures only; the URL is text in a toast, not a link): [validation](docs/validation/goauth-1.md).
- **Issues:** [GO1](ISSUES.md#go1), [GO2](ISSUES.md#go2) (both resolved).

<a id="goauth-2"></a>

### GOAUTH-2 — Google client health check

- **Status / proposed priority:** done / P2 (selected by the owner 2026-10-15). **Dependencies / gates:** GOAUTH-1. The owner-run live check against a client with one API disabled is unrun.
- **Result:** Settings → Accounts → Edit → Identity has a "Check Google setup" section for Google OAuth accounts. It makes read-only GETs (Gmail `users.getProfile` and the send-as list; People `people/me/connections`; Calendar list) and reports each API as working, disabled (with an Enable API button that opens the console link), missing permission (with Re-authorize), not set up or an error. People and Calendar are probed only when the account has a grant for them. The check runs on demand and once after the first Google sign-in, which shows a message only on a disabled or permission problem. No polling. Evidence and limits: [validation](docs/validation/goauth-2.md).
- **Issues:** [GO1](ISSUES.md#go1) (resolved by GOAUTH-1).

<a id="goauth-3"></a>

### GOAUTH-3 — Import a client_secret JSON file

- **Status / proposed priority:** done / P2 (selected by the owner 2026-10-15). **Dependencies / gates:** none technically. The native file dialog and a real Google download are unrun.
- **Result:** Settings → Accounts → OAuth Credentials has **Import JSON…** above the Google Mail editor. It picks the file natively; the Go parser accepts only a top-level `installed` client, rejects `web`, other shapes, malformed, non-UTF-8 and over-64 KB files with a specific message and changes nothing, then saves the client to the ticked Mail, Contacts and Calendar slots through the existing credential store. The secret never reaches the UI, results or logs. The button lives beside the kit editor, not in it, to keep the kit provider-agnostic ([evidence](docs/validation/goauth-3.md)).
- **Issues:** none.

<a id="goauth-4"></a>

### GOAUTH-4 — Guided Google Cloud setup

- **Status / proposed priority:** done / P2 (selected by the owner 2026-10-15; live links confirmed 2026-10-15). **Dependencies / gates:** GOAUTH-2 and GOAUTH-3 are done. Checking the console deep links live needs the owner's Google account in a browser and must change no console state without approval.
- **Scope / deliverable:**
  - Add an in-app checklist, started from the Google account form and the custom-credentials editor. It opens console links in the system browser:
    1. Create a project.
    2. Enable the Gmail, People and Calendar APIs in one step with `flows/enableapi?apiid=gmail.googleapis.com,people.googleapis.com,calendar-json.googleapis.com`.
    3. Google Auth Platform Branding.
    4. Audience: External, with the Testing and production trade-off explained.
    5. Data Access, with a copyable scope list generated from `internal/oauth2` so it cannot drift.
    6. Clients: create a Desktop app client and download its JSON now.
    7. Import the JSON (GOAUTH-3).
    8. Sign in, then run the health check (GOAUTH-2).
  - Rewrite the custom-credentials guide to follow the same steps.
- **Acceptance:**
  - Every deep link is confirmed against the live console, recorded with its date in `docs/validation/goauth-4.md`. A fallback is needed for any link that cannot be confirmed.
  - The scope list matches the scopes Mailven requests, enforced by a test.
  - The checklist is keyboard accessible, and the guide matches it.
- **Validation:** a Go test for the scope list; UI regression tests; `make test-ui`; a doc link check; an owner-authorized browser check of the links.
- **Result:** a Setup checklist in OAuth Credentials and on the Google sign-in form; links and the scope list come from `internal/oauth2`, with tests against provider scopes, Go scope literals and the guide's table; the guide follows the same eight steps. All six console links and the fallback passed an owner-authorized live check on 2026-10-15 without changing console state ([evidence](docs/validation/goauth-4.md)).
- **Issues:** none.

<a id="goauth-5"></a>

### GOAUTH-5 — Verified Mailven Google client assessment

- **Status / proposed priority:** done / P3 (reopened and selected by the owner 2026-10-15). **Dependencies / gates:** none remaining. Assessment only: no registration or submission to Google.
- **Result:** [recommendation](docs/validation/goauth-5.md) with sources. Mailven requests restricted Gmail scopes, so verification needs a verified domain, privacy policy, demo video and a yearly security assessment; third-party sources put CASA at roughly $500–$1,500 a year (unconfirmed). Whether CASA applies to a local-only client is unresolved: Google's text has no clause for it. The Aerion questionnaire is a starting draft only.
- **Owner decision (2026-10-15):** keep bring-your-own; no Mailven-owned verified client. The no-bundled-client decision stands.
- **Issues:** none.

<a id="chat"></a>

## CHAT — Chat composer sender review

- **Objective / outcome:** Allow reviewed sender identities in chat without weakening existing send safeguards.
- **Status / proposed priority:** proposed / P2.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#chat); each linked milestone lists its relevant issues.
- **Technical references:** [SENDING](docs/SENDING.md), [implementation record](docs/validation/provider-implementation.md).

<a id="chat-1"></a>

### CHAT-1 — Chat sender review

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none; existing sender policy is implemented.
- **Scope / deliverable:** Add sender review to chat while retaining queue-time safeguards.
- **Acceptance:** Review-required senders can complete an accessible chat review; stale/rejected evidence still blocks submission.
- **Validation:** Queue fixtures, frontend lint/check and guarded chat/Mail UI with both Accounts states; no test emails.
- **Issues:** [R1](ISSUES.md#r1). **Progress:** done (branch `chat-1-sender-review`, base 02a0b11). Chat runs the sender review (`GetChatSenderEvidence`, `ChatReply.senderReviewToken`) before `QueueChatReply`; queue-time and send-time checks are unchanged. Evidence: `node tests/ui/run.mjs chat-compose` (11 pass, review confirm under Accounts expanded and collapsed, cancel, stale token, block), `make test-ui` 46 pass, `go test ./app -run 'Sender|Chat'`, frontend lint/check. Live provider review not exercised (no test emails). Follow-up [CH1](ISSUES.md#ch1) (cosmetic) is open.

<a id="maint"></a>

## MAINT — Maintenance and documentation

- **Objective / outcome:** Keep developer tooling and user documentation accurate; resolve external review flags explicitly.
- **Status / proposed priority:** proposed / P2.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Sequence:** the owner ordered MAINT-9 to MAINT-13 on 2026-10-16. On completing one, set the next by ID as Next recommended in [Progress](#progress).
- **Issues:** [owning register section](ISSUES.md#maint); each linked milestone lists its relevant issues.
- **Technical references:** [CONTRIBUTING](CONTRIBUTING.md), [canonical shortcuts](docs/KEYBOARD_SHORTCUTS.md), [Terms draft](docs/TERMS.md), [Privacy draft](docs/PRIVACY.md).

<a id="maint-1"></a>

### MAINT-1 — User guide corrections

- **Status / proposed priority:** done / P3. **Dependencies / gates:** none.
- **Scope / deliverable:** Correct conditional shortcut guidance, malformed Svelte URL and log-location link.
- **Acceptance:** Guides match canonical shortcut behavior and all corrected destinations exist.
- **Validation:** Targeted source/doc comparison and Markdown/link checks; no product code needed.
- **Issues:** [DOC4](ISSUES.md#doc4), [DOC5](ISSUES.md#doc5). **Progress:** `Alt+G` guidance, Svelte URL and log link corrected (docs only, base `0a0ea88`); source comparison and link grep pass; no live check.

<a id="maint-2"></a>

### MAINT-2 — Owned formatting and reader reuse

- **Status / proposed priority:** done / P3. **Dependencies / gates:** none.
- **Scope / deliverable:** Format the owned Go file and assess behavior-preserving reader extraction in distinct substeps.
- **Acceptance:** Formatting is clean and any extraction preserves refresh, size caps and error classification across both providers.
- **Validation:** gofmt and focused affected Go tests; review diff for unrelated churn.
- **Issues:** [MA2](ISSUES.md#ma2), [SY1](ISSUES.md#sy1). **Progress:** base `db18d02`; `store.go` formatted; transport extracted to `internal/apiread` with provider tests unchanged and passing. [Evidence](docs/validation/maint-2.md).

<a id="maint-3"></a>

### MAINT-3 — Resolve legal draft review flags

- **Status / proposed priority:** done / P2. Owner disposition recorded on 2026-10-14; L2/L3 resolved. See [validation evidence](docs/validation/maint-3.md).
- **Acceptance:** Recorded owner disposition determines whether draft markers and texts may change; historical concerns are explicitly resolved or retained.
- **Validation:** Documentation and approval-evidence review; no assumed sign-off or autonomous legal text revision.
- **Issues:** [L2](ISSUES.md#l2), [L3](ISSUES.md#l3).
- **Checkpoint:** branch `docs/maint-3-review`, pre-milestone base `9d1542d11295908b7610a1913a51f16f8340d16e`. Owner approved proposed corrections, then requested explicit project-owner liability protection. Terms and Privacy revised and review markers removed; owner protection covers the software and accompanying documentation, with a mandatory-law exception; absolute immunity is not promised. Historical concerns and external-review limits are retained in the evidence. Changed files: `docs/TERMS.md`, `docs/PRIVACY.md`, `docs/PROVIDERS.md`, `docs/validation/maint-3.md`, `PLAN.md`, `ISSUES.md`. Final documentation checks and independent acceptance review recorded in validation evidence before landing. No next milestone selected; SEND-2 remains a candidate with its separate live gate, and deferred work requires reopening.

<a id="maint-4"></a>

### MAINT-4 — Deterministic UI audit, frontend regression tests and CI

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none; authorized 2026-10-14 (local commits only; merge and push need approval).
- **Scope / deliverable:** Backend-free headless UI test mode, UI quality audit over the theme × width × Accounts × previews matrix, chat presentation/triage/composition regression tests, `make test-ui` and a CI job with machine-readable diagnostics, workflow guidance.
- **Acceptance:** Repeatable PASS/FAIL results naming element, configuration and reason; no false failures on expected responsive behavior; synthetic data only and nothing persisted; critical chat behaviors covered; existing checks pass; CI config valid; failure detection demonstrated.
- **Validation:** [execution plan](docs/plans/maint-4.md) acceptance table; evidence in [docs/validation/maint-4.md](docs/validation/maint-4.md).
- **Issues:** found [UI1](ISSUES.md#ui1), [UI2](ISSUES.md#ui2), [UI3](ISSUES.md#ui3) (recorded, not fixed here). **Progress:** `make test-ui` passes 41/41 repeatably; mutations detected; CI job validated statically, not yet run on GitHub. [Evidence](docs/validation/maint-4.md); [completed checkpoint](docs/plans/maint-4.md).

<a id="maint-5"></a>

### MAINT-5 — Compact project workflow helpers

- **Status / proposed priority:** done / P3. **Dependencies / gates:** user authorized 2026-10-14 after the Mailven session audit; no reopening of deferred G4/D2.
- **Scope / deliverable:** Milestone context loader, serial check runner with compact persisted evidence, guarded branch landing helper, maintained local Markdown/site-link checker; integrate into existing project skills and developer docs.
- **Acceptance:** Helpers handle checkout/worktree paths, linked project context and site links; required checks remain intact and failures visible; landing defaults to preview and preserves clean-tree, conflict-abort, checks-after-rebase, configured identity/upstream and equal-tree-before-deletion rules. Disposable fixtures verify success and failure paths. Instructions avoid rereading unchanged skill files within one session while still loading required guidance for each new session.
- **Validation:** Standard-library unit/integration tests with temporary repositories and fake check commands, existing skill path tests, real docs/context/preview runs, whitespace/brand checks and independent acceptance. No production mail/backend changes.
- **Issues:** none initially; scriptable repetition is an optimization finding, not a verified product defect. **Progress:** 30 disposable fixture tests, existing skill/worktree path checks, local docs/whitespace/brand checks and independent acceptance pass; canonical next/finish/ship workflows integrated ([evidence](docs/validation/maint-5.md), [checkpoint](docs/plans/maint-5.md)). Review follow-up [MA6](ISSUES.md#ma6) resolved both P2 evidence defects with common-ancestor baselines and complete feature-path coverage; 35 helper tests and independent acceptance passed ([validation](docs/validation/maint-5.md#review-follow-up-ma6)).

<a id="maint-6"></a>

### MAINT-6 — Closed sidebar keyboard isolation

- **Status / proposed priority:** done / P2. **Authorization:** owner selected UI2 on 2026-10-14.
- **Scope:** Mail and shared kit narrow sidebars; preserve opening, closing and full/medium layouts. Viewer investigation is evidence only, not a separate implementation scope.
- **Acceptance:** Closed sidebars reject keyboard focus; open sidebars remain interactive; resizing restores normal sidebar focus; delete the UI2 audit allowlist entry and pass the full audit.
- **Validation:** Regression tests for Mail and kit sidebars; frontend lint/check/build; full UI suite including Mail Accounts expanded/collapsed. Native WebKitGTK checks remain unrun.
- **Issues:** [UI2](ISSUES.md#ui2); the viewer follow-up [UI4](ISSUES.md#ui4) was resolved by [MAINT-8](#maint-8).
- **Checkpoint:** branch `fix/ui2-sidebar-focus`, pre-commit base `2c25557`; implementation and verification complete. `node tests/ui/run.mjs sidebar-focus`: 2 pass. `python3 tools/project-check.py --base main --run`: whitespace, docs, frontend lint/check/build and 58 UI tests pass (0 failed/skipped). [Evidence and limitations](docs/validation/maint-6.md). No next milestone selected; future work requires selection. Existing Gmail investigation is preserved on `docs/send-2-gmail-verification`.

<a id="maint-7"></a>

### MAINT-7 — Unattended milestone runner

- **Status / proposed priority:** done / P3. **Authorization:** owner requested on 2026-10-15 migrating the external plan runner into `tools/` with automatic milestone selection.
- **Scope:** `tools/project-run.py`: one fresh Claude Code or Codex session per milestone, ranked selection from this roadmap, per-milestone local authorization, handoff continuation, blocked-milestone skipping and usage-limit resumption; `make milestone` runs it without `.env` credentials in the session environment. No push, landing or gate changes.
- **Acceptance:** Selection never picks deferred, blocked or done milestones, milestones in closed projects, or milestones with unfinished dependencies; success requires a commit, a clean tree and a `done` status; handoff and blocked outcomes behave as documented; [CONTRIBUTING](CONTRIBUTING.md#unattended-milestone-runs) and the [lifecycle rules](docs/PROJECT_MANAGEMENT.md#status-priority-and-eligibility) describe the authorization.
- **Validation:** `tools/tests/test_project_run.py` (fixture plans and a fake agent CLI in disposable repositories); helper test suite; doc links; brand check. A real agent session is unrun.
- **Checkpoint:** branch `feat/project-run`, base `main`; complete.

<a id="maint-8"></a>

### MAINT-8 — Closed responsive viewer keyboard isolation

- **Status / proposed priority:** done / P2 (selected by the owner 2026-10-15). **Dependencies / gates:** none.
- **Outcome:** closed responsive viewers in Mail and kit `DetailPane` are inert and hand the focused pane and focus to the list on close; open viewers, focus mode and full layout unchanged ([evidence](docs/validation/maint-8.md)). Native WebKitGTK check unrun.
- **Issues:** [UI4](ISSUES.md#ui4) resolved.

<a id="maint-9"></a>

### MAINT-9 — Calendar syncer test cleanup flake

- **Status / proposed priority:** done (2026-10-16) / P3. **Dependencies / gates:** none. Checkpoint: base fdf27f9; the fix is the milestone commit; `-count=100` and `make test` passed.
- **Scope:** diagnose and fix the `TestSyncerSubscriptionsPerInstance` TempDir cleanup race in `extensions/calendar/backend`; test-side stop/wait first, product code only if the syncer leaks a goroutine. Read `docs/EXT_RULES.md` before committing under `extensions/**`.
- **Acceptance:** `go test ./extensions/calendar/backend -count=100` passes; assertions are not loosened.
- **Validation:** that command, then the focused package tests and `make test` if shared behavior changed.
- **Issues:** [TF1](ISSUES.md#tf1).

<a id="maint-10"></a>

### MAINT-10 — Single Archive in the chat row menu

- **Status / proposed priority:** done (2026-10-16) / P3. **Dependencies / gates:** none. Checkpoint: base 5f59e2f; the fix is the milestone commit. `npm run lint`, `npm run check`, `inbox-triage` (7 passed) and the full `make test-ui` (107 passed, Accounts column expanded and collapsed included) passed; native WebKitGTK unrun.
- **Scope:** `ChatRow.svelte` and `MessageContextMenu.svelte`: the chat row menu shows one Archive with the chat behavior (the "Conversation archived" toast and list follow-up). The open chat's header menu is unchanged.
- **Acceptance:** one Archive item in a chat row menu; `inbox-triage.test.cjs` `menu()` drops `.first()` for Archive; the chat Archive still archives the same message IDs.
- **Validation:** `npm run lint`, `npm run check`, `node tests/ui/run.mjs inbox-triage`, then `make test-ui` including Mail with the Accounts column expanded and collapsed.
- **Issues:** [UI3](ISSUES.md#ui3).

<a id="maint-11"></a>

### MAINT-11 — Snapshot restore documentation

- **Status / proposed priority:** done (2026-10-16) / P3. **Dependencies / gates:** none ([API-3](#api-3) is implemented). Checkpoint: base ff5ff56; the docs change is the milestone commit. `python3 tools/check-doc-links.py --changed` passed (2 files, 0 errors); source review against `app/snapshot_restore.go` and `docs/MAIL_APIS.md`; no live restore.
- **Scope:** correct `docs/PROVIDERS.md` (Switching routes) and `docs/user-guide/features/settings.md` (Mail API route) to describe the offline whole-database restore with consent, backup and undo, linking `docs/MAIL_APIS.md`. State that ordinary route rollback does not restore the snapshot or undo provider changes. No live restore testing.
- **Acceptance:** neither guide says restoration is unavailable; the provider matrix is current; links resolve.
- **Validation:** `python3 tools/check-doc-links.py --changed`; source review against `app/snapshot_restore.go`.
- **Issues:** [DOC6](ISSUES.md#doc6).

<a id="maint-12"></a>

### MAINT-12 — Split App.svelte under the size limit

- **Status / proposed priority:** done (2026-10-16) / P3. **Dependencies / gates:** none; do not run in parallel with MAINT-13 (shared files). Checkpoint: base 04d80d9; the split is the milestone commit. `App.svelte` 1,712 → 361 lines; new modules `lib/stores/{mailSelection,appComposer,shutdown}.svelte.ts`, `lib/keyboard/{globalShortcuts,mailKeys,bulkActions,shortcutContext}.ts`, `lib/appStartup.ts` and `components/common/AppOverlays.svelte`. `npm run lint`, `npm run check`, the build and the full `make test-ui` (107 passed, tests unchanged, Accounts column expanded and collapsed included) passed; native WebKitGTK unrun.
- **Scope:** move keyboard shortcuts, composer state and selection logic out of `frontend/src/App.svelte` into modules or components without behavior change.
- **Acceptance:** `App.svelte` under 500 lines; `make test-ui` passes unchanged; keyboard accessibility preserved.
- **Validation:** `npm run lint`, `npm run check`, `make test-ui`, Mail with the Accounts column expanded and collapsed.
- **Issues:** [UI5](ISSUES.md#ui5).

<a id="maint-13"></a>

### MAINT-13 — Split oversized components, largest first

- **Status / proposed priority:** done (2026-10-16) / P3 ([checkpoint](docs/plans/maint-13.md)). **Dependencies / gates:** after MAINT-12; calendar components follow `docs/EXT_RULES.md`. Base 054800c; one commit per component, then a close-out commit. The UI6 listing shows no file over 500 lines; whitespace, docs, tooling, brand, frontend lint/check/build and the full `make test-ui` (107 passed, tests unchanged) passed (`tools/project-check.py --base 054800c --run`). The UI suite opens the composer and sidebar but no settings, signature, email-body or calendar component, so those splits rest on type checks and review; native WebKitGTK unrun.
- **Scope:** the 15 components listed in UI6, starting with `composer/Composer.svelte`, `settings/AccountForm.svelte` and `settings/account/AccountSecurityTab.svelte`.
- **Acceptance:** every tracked component under 500 lines (the milestone stays active until then); `make test-ui` passes.
- **Validation:** the UI6 listing command shows no file over 500 lines; frontend lint/check; full UI suite.
- **Issues:** [UI6](ISSUES.md#ui6).

<a id="d2"></a>

### D2 — TypeScript 7 compatibility

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** both tooling peer ranges; upgrade approval.
- **Scope / deliverable:** Recheck typescript-eslint and svelte-check ranges before proposing any upgrade.
- **Acceptance:** Both peer ranges support 7; only a separately selected upgrade can be implemented and verified.
- **Validation:** CONTRIBUTING dependency commands; lint/check/build after an authorized upgrade. Last 2026-10-12 evidence remains in ISSUES.
- **Issues:** [D2](ISSUES.md#d2). **Progress:** not started.

<a id="uix"></a>

## UIX — UI refresh

- **Objective / outcome:** Refine the Mail view toward a calmer, lower-noise look inspired by bb: quieter Nord surfaces, a minimal header, list status glyphs, conversation compaction, a carded composer and an optional flat transcript.
- **Status / proposed priority:** proposed / P3. UIX-1 to UIX-6 done. A 2026-10-15 review fix (branch `fix/uix-review`): Tab and Shift+Tab stop on folded-run rows between messages; scrolling or sending ends an expanded row's position hold; an expanded run's first message shows its sender, and the message after a folded run shows its date when the run hid a day change; pending replies follow flat mode; a snoozed or sending chat with unread mail shows a small unread dot on its glyph; the App.svelte size finding is [UI5](ISSUES.md#ui5).
- **Dependencies / blockers:** UIX-1 first (tokens); UIX-2–5 independent; UIX-6 last.
- **Issues:** [UI1](ISSUES.md#ui1), [UI5](ISSUES.md#ui5).
- **Technical references:** [design plan](docs/plans/ui-refresh.md), [UI_TOKENS](docs/UI_TOKENS.md), [UI tests](tests/ui/README.md).

Each milestone's scope, acceptance and validation are in the [design plan's milestone table](docs/plans/ui-refresh.md#milestones).

<a id="uix-1"></a>

### UIX-1 — Nord retune and type tokens

- **Status / proposed priority:** done / P3. **Authorization:** owner selected UIX-1 on 2026-10-14.
- **Scope / deliverable:** Lower-chroma Nord surfaces, light primary/selected contrast, base type/radius tokens; UI_TOKENS updated.
- **Acceptance:** UI1 audit allowlist entry removed; contrast check and full UI suite pass.
- **Validation:** `make lint`, full `make test-ui`, both themes with Accounts expanded/collapsed.
- **Issues:** [UI1](ISSUES.md#ui1). **Checkpoint:** branch `feat/uix-1-nord-retune`, base `d5d3bec`; done and verified ([evidence](docs/validation/uix-1.md)). Base type sizes deliberately unchanged; `--radius` was undefined since the theme reduction and is now defined. Native WebKitGTK and visual review unrun.

<a id="uix-2"></a>

### UIX-2 — Header simplification

- **Status / proposed priority:** done / P3. **Authorization:** owner selected UIX-2 on 2026-10-14; owner chose to keep a Delete button.
- **Scope / deliverable:** Conversation header with title, `⋯` menu and one Archive primary action.
- **Acceptance:** Every header action stays reachable by mouse, menu and keyboard.
- **Validation:** Header/triage regression tests, frontend lint/check, full UI suite.
- **Issues:** none. **Checkpoint:** branch `feat/uix-1-nord-retune`, base `ddd94f6`; done and verified ([evidence](docs/validation/uix-2.md)). Header is title, `⋯` beside it, bordered Archive and a Delete button; Snooze and Pin moved into the `⋯` menu at every width. Native WebKitGTK and visual review unrun.

<a id="uix-3"></a>

### UIX-3 — List and sidebar polish

- **Status / proposed priority:** done / P3. **Dependencies / gates:** selection; UIX-1.
- **Scope / deliverable:** Labeled sidebar actions, quieter section labels, row status glyphs; `ChatList.svelte` extraction first.
- **Acceptance:** Glyph priority regression test; components under 500 lines; audit passes in both Accounts states.
- **Validation:** New glyph test, frontend lint/check, full UI suite.
- **Issues:** none. **Progress:** done 2026-10-15; [evidence](docs/validation/uix-3.md). Owner selected UIX-3 on 2026-10-15 (branch `feat/uix-1-nord-retune`, base 23f529a). Decisions: the needs-reply glyph is deferred (no latest-sender data on `ChatItem`; Go changes out of scope), so the glyph slot shows sending/queued, then snoozed, then unread; labeled "New message" and "Search" rows go at the top of the expanded Accounts sidebar, and the list-header Compose stays. Steps: (1) extract `ChatListSection`; (2) status glyph + priority test; (3) quieter section labels; (4) sidebar actions; (5) lint/check/full UI suite, visual check in both Accounts states. All done except the screenshot review, which was not performed (see evidence). Follow-up when selected: a needs-reply glyph needs a latest-sender field from the backend summary.

<a id="uix-4"></a>

### UIX-4 — Composer card and chips

- **Status / proposed priority:** done / P3. **Dependencies / gates:** selection; UIX-1.
- **Scope / deliverable:** Carded chat composer with in-card sender, chip row; attachment summary card deferred.
- **Acceptance:** `chat-compose` suite passes; send key and sender review unchanged.
- **Validation:** `node tests/ui/run.mjs chat-compose`, full UI suite.
- **Issues:** none ([CH1](ISSUES.md#ch1) adjacent, out of scope). **Progress:** done 2026-10-15; [evidence](docs/validation/uix-4.md). Owner selected UIX-4 on 2026-10-15 (branch `feat/uix-1-nord-retune`, base 23d40a4). Decisions: the From stays a read-only label inside the card (no sender picker: the backend resolves the alias and the sender-review flow is unchanged); the optional conversation-attachment summary card is deferred (attachments load per message, no thread-level source). Native WebKitGTK and visual review unrun.

<a id="uix-5"></a>

### UIX-5 — Conversation compaction

- **Status / proposed priority:** done / P3. **Dependencies / gates:** selection; UIX-1.
- **Scope / deliverable:** Expandable one-line rows for earlier read messages (low-priority-sender variant deferred).
- **Acceptance:** Latest/unread never collapse; keyboard expand/collapse; scroll position holds.
- **Validation:** New compaction tests on fakes, full UI suite.
- **Issues:** none. **Progress:** done 2026-10-15; [evidence](docs/validation/uix-5.md). Owner selected UIX-5 on 2026-10-15 (branch `feat/uix-1-nord-retune`, base 8e4a14b). Decisions: the latest 5 messages never fold and a run needs at least 3 older read messages (so short chats are unchanged; tune with the owner); unread, draft and subject-heading messages never fold, and a message unread at open stays visible once read; the "Notice from X" variant for low-priority senders is deferred (the frontend has only a per-chat `isLowPriority`, no per-message sender priority; needs backend data). Native WebKitGTK and visual review unrun.

<a id="uix-6"></a>

### UIX-6 — Optional flat transcript mode

- **Status / proposed priority:** done / P3. **Dependencies / gates:** selection; UIX-1–5.
- **Scope / deliverable:** `chat_display_style` setting (bubbles default, flat) and a flat message variant.
- **Acceptance:** Both modes pass the audit; Go settings test; user guide updated.
- **Validation:** `go test ./internal/settings`, bindings regenerated per CONTRIBUTING, frontend checks, full UI suite in both modes.
- **Issues:** none. **Progress:** done 2026-10-15; [evidence](docs/validation/uix-6.md). Owner selected UIX-6 on 2026-10-15 (branch `feat/uix-1-nord-retune`, base 24e37b4). Decisions: flat mode uses the wide stream width, drops the avatar column and distinguishes my messages by name line plus a left accent rule; the audit matrix adds eight flat configurations (both themes, 1400/900 px, Accounts expanded/collapsed) rather than doubling. Native WebKitGTK unrun.

<a id="ai"></a>

## AI — AI development workflows

- **Objective / outcome:** Maintain portable, low-context, evidence-backed agent workflows across independent sessions.
- **Status / proposed priority:** deferred / P3.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#ai); each linked milestone lists its relevant issues.
- **Technical references:** [shared skills record](docs/validation/shared-skills-g3.md), [AI5 handoff record](docs/validation/shared-handoff-ai5.md), [CONTRIBUTING](CONTRIBUTING.md).

<a id="g4"></a>

### G4 — Remaining runtime routing and context verification

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** explicit request on new branch; available configured hosts.
- **Scope / deliverable:** Verify broader Claude workflow compliance/routing beyond handoff and comparable context savings; preserve AI5 completion scope.
- **Acceptance:** Representative runtime trials distinguish registration from execution; comparable measurements support or retire savings claims without personal CLI changes.
- **Validation:** Disposable host trials based on G1–G3 record; exact commands/results and unrun cases recorded; no product implementation.
- **Issues:** [AI6](ISSUES.md#ai6). **Progress:** not started.

<a id="wails"></a>

## WAILS — Desktop framework migration assessment

- **Objective / outcome:** Assess Wails v3 and GTK4/WebKitGTK 6.0 feasibility and produce a decision-ready design, without migration implementation.
- **Status / proposed priority:** deferred / P3.
- **Dependencies / blockers:** owner decision 2026-10-14 defers all future Wails work, including assessment, until Wails v3 officially releases as a stable version (not a prerelease). Verify the official release before proposing resumption; explicit owner selection and milestone prerequisites still apply.
- **Issues:** [owning register section](ISSUES.md#wails); each linked milestone lists its relevant issues.
- **Technical references:** [existing assessment brief](docs/PLAN-migration.md) (original M0–M7 scope and constraints; now W3-M0–W3-M7).

<a id="w3-m0"></a>

### W3-M0 — Repository preflight and safe v2 baseline

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection.
- **Scope / deliverable:** Baseline and synthetic-data strategy with checkout, tools, test commands and unrun checks.
- **Acceptance:** Meet the corresponding original M0 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M0 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m1"></a>

### W3-M1 — Wails v3 and GTK4/WebKitGTK readiness

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M0](#w3-m0).
- **Scope / deliverable:** Official version/API maturity assessment, dated evidence and explicit unknowns.
- **Acceptance:** Meet the corresponding original M1 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M1 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m2"></a>

### W3-M2 — Mailven v2 dependency inventory

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M0](#w3-m0).
- **Scope / deliverable:** File/symbol/API mapping covering lifecycle, bindings, extensions, platform helpers and both windows.
- **Acceptance:** Meet the corresponding original M2 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M2 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m3"></a>

### W3-M3 — Backend, frontend and windowing design

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M1](#w3-m1), [W3-M2](#w3-m2).
- **Scope / deliverable:** Compare compatibility-first and native multiwindow options; preserve mail/security invariants.
- **Acceptance:** Meet the corresponding original M3 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M3 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m4"></a>

### W3-M4 — Platform and packaging feasibility

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M1](#w3-m1), [W3-M2](#w3-m2), [W3-M3](#w3-m3).
- **Scope / deliverable:** Linux/GTK/Flatpak, macOS and Windows risks and explicit platform validation limits.
- **Acceptance:** Meet the corresponding original M4 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M4 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m5"></a>

### W3-M5 — Regression, performance and security test design

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M2](#w3-m2), [W3-M3](#w3-m3), [W3-M4](#w3-m4).
- **Scope / deliverable:** Behavior-to-fixture matrix with pass/fail evidence and owner/hardware requirements.
- **Acceptance:** Meet the corresponding original M5 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M5 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m6"></a>

### W3-M6 — Staged migration blueprint

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M3](#w3-m3), [W3-M4](#w3-m4), [W3-M5](#w3-m5).
- **Scope / deliverable:** Bounded future implementation steps, pinned target, rollback and separately approvable spikes.
- **Acceptance:** Meet the corresponding original M6 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M6 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m7"></a>

### W3-M7 — Independent review and owner decision gate

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** official stable Wails v3 release and explicit owner selection; [W3-M6](#w3-m6).
- **Scope / deliverable:** Evidence-backed proposed GO/DEFER/NO-GO, unresolved questions and separate implementation authorization.
- **Acceptance:** Meet the corresponding original M7 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M7 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

## Completed scope and historical evidence

Completed work is not a selectable backlog. Stable historical identifiers remain in these records and Git history; their reuse in older plans does not identify new work.

- Provider P1 audit and P2 architecture: [deliverability review](docs/validation/provider-deliverability-review.md), [transport recommendations](docs/validation/provider-transport-recommendations.md). F1–F5 local implementation: [implementation validation](docs/validation/provider-implementation.md); live limits remain explicit.
- G1–G3 shared-guidance implementation and partial runtime checks: [verification record](docs/validation/shared-skills-g3.md). Do not infer full G3/G4 runtime/context completion from parsing/discovery. AI5 bidirectional handoff completed under the 2026-10-14 request: [handoff record](docs/validation/shared-handoff-ai5.md); remaining G4 belongs to AI.
- Earlier rebrand/UI execution records remain in Git history (`git log -- PLAN.md`; original legal flags: `git show 33c1851:PLAN.md`). Upstream history and architecture: [Aerion history](docs/history/AERION_CHANGELOG.md), [upstream CASA record](docs/history/AERION_CASA_TIER2.md), [extension rules](docs/EXT_RULES.md) and [extension reference](docs/EXTENSIONS.md).
- Wails assessment remains unstarted and deferred until the official stable Wails v3 release and subsequent owner selection; existing M0–M7 proposal details are retained in [the brief](docs/PLAN-migration.md), mapped to W3-M0–W3-M7 here to avoid old M-number collisions. Intended `plans/wails-v3/` output paths were never created; selection should reconcile them with the new execution-plan convention.

- Documentation/workflow reorganization evidence: [mapping and validation](docs/validation/project-management.md). This was documentation-only work and did not select any product milestone.
