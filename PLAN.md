# Project roadmap

Authoritative portfolio of remaining work. [Issue register](ISSUES.md) owns findings; [project-management rules](docs/PROJECT_MANAGEMENT.md) define IDs, statuses, authorization and execution plans. **No milestone is active.** No proposed or deferred work below is authorized for implementation by this roadmap.

## Portfolio

Priorities are recommendations assigned in this reorganization: P1 privacy/data integrity/correctness, P2 functional follow-ups, P3 maintenance/investigation. Original severities remain in ISSUES. Dependency order takes precedence over priority; independently executable work is ordered by priority. Projects with mixed gated work remain proposed; milestone statuses carry individual gates.

| Project | Status | Proposed priority | Unresolved issues | Candidate / gate |
| --- | --- | --- | --- | --- |
| [SEND — Sending, privacy and compatibility](#send) | proposed | P1 | [2](ISSUES.md#send) | SEND-2/3 require live authorization |
| [API — Provider API activation and migration](#api) | proposed | P2 | [3](ISSUES.md#api) | API-2 design or API-6 local implementation; API-3 gated by API-2, shared rollout gated by live checks |
| [SYNC — Mail synchronization and action reliability](#sync) | proposed | P1 | [0](ISSUES.md#sync) | none; SYNC-1–4 done |
| [CHAT — Chat composer sender review](#chat) | proposed | P3 | [1](ISSUES.md#chat) | CH1 unassigned (cosmetic) |
| [MAINT — Maintenance and documentation](#maint) | proposed | P2 | [6](ISSUES.md#maint) | MAINT-3 external; D2 deferred/upstream blocked |
| [AI — AI development workflows](#ai) | deferred | P3 | [1](ISSUES.md#ai) | G4 deferred; no eligible work |
| [WAILS — Desktop framework migration assessment](#wails) | proposed | P3 | [0](ISSUES.md#wails) | W3-M0 assessment only |

**Recommended next selection:** none automatic; CHAT-1 is done; no independent P2 candidate is recommended automatically. External privacy gate SEND-2 is high priority but cannot run live without separately specified authorization. No automatic selection or status transition occurs here.

## Scope and preserved decisions

Provider F1–F5 is complete for its local scope; completion does not establish live provider compatibility. Retain full Gmail API and delegated Microsoft Graph direction, future shared-mailbox support and explicit opt-in migration. Shared API activation remains unsupported. Existing accounts retain configured routes until explicitly migrated; there is no automatic route switch or fallback. Snapshot rollback currently changes route only and does not restore data or undo remote actions.

No live provider access, test emails, credentials changes, mailbox migration, provider registrations, tenant/DNS changes, dependency upgrades or personal CLI configuration changes are authorized by this roadmap. Live checks need accounts, From identities, controlled recipients and cases specified separately; see [live limits](docs/MAIL_APIS.md#live-verification-limits) and [P2 parity scenarios](docs/validation/provider-transport-recommendations.md). Wails research additionally prohibits application/dependency/packaging changes, real data, releases and unapproved prototypes or installs under its existing brief. G4 and D2 remain deferred.

## Progress

No active milestone. MAINT-5 is done: all four compact workflow helpers, logged check evidence and guarded landing are implemented and verified ([evidence](docs/validation/maint-5.md), [checkpoint](docs/plans/maint-5.md)). API-5 is done and F5b-3 resolved: rollback reports only a sync request, with scheduler-ordering and UI coverage ([evidence](docs/validation/api-5.md), [checkpoint](docs/plans/api-5.md)). API-1 design is done ([evidence](docs/validation/api-1.md), [checkpoint](docs/plans/api-1.md)); shared support remains unsupported, MA4 stays deferred and MA5 is open. API-6 is proposed and requires selection; API-2 remains another API candidate. No next milestone is activated.

CHAT-1 is done and R1 resolved. MAINT-1 is done and DOC4/DOC5 resolved. MAINT-2 is done and MA2/SY1 resolved ([evidence](docs/validation/maint-2.md)). MAINT-4 is done ([validation evidence](docs/validation/maint-4.md)); its findings UI1–UI3 remain open. SEND-1 is done and CD2 resolved; SYNC-1 is done and R3 resolved ([evidence](docs/validation/sync-1.md)); SYNC-2 is done and R4 resolved ([evidence](docs/validation/sync-2.md)); SYNC-3 is done and SY2/SY6 resolved ([evidence](docs/validation/sync-3.md)); API-4 is done and F5b-2 resolved ([evidence](docs/validation/api-4.md)); SYNC-4 is done and R2/SY3/SY4/SY5 resolved ([evidence](docs/validation/sync-4.md)); see [validation evidence](docs/validation/send-1.md). Other proposed milestones remain unselected; SEND-2/3, API-3 and MAINT-3 retain their gates, and G4/D2 remain deferred.

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
- **Issues:** [MA1](ISSUES.md#ma1). **Progress:** not started.

<a id="send-3"></a>

### SEND-3 — Existing Microsoft compatibility assessment

- **Status / proposed priority:** blocked / P1. **Dependencies / gates:** explicit account-type evidence and live authorization.
- **Scope / deliverable:** Assess existing account endpoints without automatically migrating them; preserve new-account defaults.
- **Acceptance:** Account-type/endpoint evidence and authorized results are recorded; local fixtures are distinguished from live compatibility. Any existing-account migration requires separate scope.
- **Validation:** Disposable endpoint fixtures and separately authorized live scenarios from provider references.
- **Issues:** [PD2](ISSUES.md#pd2). **Progress:** not started.

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
- **Issues:** [MA4](ISSUES.md#ma4), [MA5](ISSUES.md#ma5). **Progress:** design verified by source review and independent acceptance ([evidence](docs/validation/api-1.md), [checkpoint](docs/plans/api-1.md)); implementation remains proposed under API-6.

<a id="api-2"></a>

### API-2 — Snapshot restoration design

- **Status / proposed priority:** proposed / P2. **Dependencies / gates:** none.
- **Scope / deliverable:** Design live-DB replacement, crash recovery, data-loss consent and rollback; leave route-only rollback truthful.
- **Acceptance:** Approved design distinguishes local snapshot restore from undoing remote actions and covers active database replacement, credentials and data-loss UX.
- **Validation:** Review isolated backup/restore strategy and failure cases; no real database replacement.
- **Issues:** [F5b-1](ISSUES.md#f5b-1). **Progress:** not started.

<a id="api-3"></a>

### API-3 — Snapshot restoration implementation

- **Status / proposed priority:** blocked / P2. **Dependencies / gates:** [API-2](#api-2) and explicit implementation selection.
- **Scope / deliverable:** Implement only the selected restore design and consent flow after its gate.
- **Acceptance:** Isolated restore/crash fixtures and UI verify approved recovery and loss semantics; provider-side changes remain accurately disclosed.
- **Validation:** Disposable database fixtures, relevant Go checks, frontend lint/check and guarded UI; live migration requires separate authorization.
- **Issues:** [F5b-1](ISSUES.md#f5b-1). **Progress:** not started.

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

- **Status / proposed priority:** proposed / P2. **Dependencies / gates:** API-1 design; separate implementation selection. Exposing shared activation/migration additionally requires separately authorized live validation.
- **Scope / deliverable:** Implement the bounded [shared route design](docs/SHARED_MAILBOX_API_DESIGN.md), including parent-owned grants, child identity isolation, consent/refresh/removal safety, migration blockers and route-aware sender review.
- **Acceptance:** Disposable fixtures prove owner/child isolation and failure handling; no shared migration is exposed before recorded live identity/delegation/sender/Sent/Bcc evidence. Design alone does not establish support.
- **Validation:** [Acceptance mapping and gates](docs/SHARED_MAILBOX_API_DESIGN.md#acceptance-to-validation-mapping-for-later-implementation); Go checks, frontend lint/check and full UI suite if implemented; live checks only with separate authorization.
- **Issues:** [MA4](ISSUES.md#ma4), [MA5](ISSUES.md#ma5); [MA1](ISSUES.md#ma1) remains the sending privacy gate. **Progress:** not selected.

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

- **Status / proposed priority:** blocked / P2. **Dependencies / gates:** authoritative owner sign-off or separately selected review.
- **Scope / deliverable:** Locate approval evidence or obtain review for current draft markers; legal conclusions are external decisions.
- **Acceptance:** Recorded owner disposition determines whether draft markers and texts may change; historical concerns are explicitly resolved or retained.
- **Validation:** Documentation and approval-evidence review; no assumed sign-off or autonomous legal text revision.
- **Issues:** [L2](ISSUES.md#l2), [L3](ISSUES.md#l3). **Progress:** not started.

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
- **Issues:** none initially; scriptable repetition is an optimization finding, not a verified product defect. **Progress:** 30 disposable fixture tests, existing skill/worktree path checks, local docs/whitespace/brand checks and independent acceptance pass; canonical next/finish/ship workflows integrated ([evidence](docs/validation/maint-5.md), [checkpoint](docs/plans/maint-5.md)).

<a id="d2"></a>

### D2 — TypeScript 7 compatibility

- **Status / proposed priority:** deferred / P3. **Dependencies / gates:** both tooling peer ranges; upgrade approval.
- **Scope / deliverable:** Recheck typescript-eslint and svelte-check ranges before proposing any upgrade.
- **Acceptance:** Both peer ranges support 7; only a separately selected upgrade can be implemented and verified.
- **Validation:** CONTRIBUTING dependency commands; lint/check/build after an authorized upgrade. Last 2026-10-12 evidence remains in ISSUES.
- **Issues:** [D2](ISSUES.md#d2). **Progress:** not started.

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
- **Status / proposed priority:** proposed / P3.
- **Dependencies / blockers:** no cross-project implementation prerequisite established; milestone gates below apply.
- **Issues:** [owning register section](ISSUES.md#wails); each linked milestone lists its relevant issues.
- **Technical references:** [existing assessment brief](docs/PLAN-migration.md) (original M0–M7 scope and constraints; now W3-M0–W3-M7).

<a id="w3-m0"></a>

### W3-M0 — Repository preflight and safe v2 baseline

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** none.
- **Scope / deliverable:** Baseline and synthetic-data strategy with checkout, tools, test commands and unrun checks.
- **Acceptance:** Meet the corresponding original M0 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M0 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m1"></a>

### W3-M1 — Wails v3 and GTK4/WebKitGTK readiness

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M0](#w3-m0).
- **Scope / deliverable:** Official version/API maturity assessment, dated evidence and explicit unknowns.
- **Acceptance:** Meet the corresponding original M1 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M1 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m2"></a>

### W3-M2 — Mailven v2 dependency inventory

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M0](#w3-m0).
- **Scope / deliverable:** File/symbol/API mapping covering lifecycle, bindings, extensions, platform helpers and both windows.
- **Acceptance:** Meet the corresponding original M2 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M2 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m3"></a>

### W3-M3 — Backend, frontend and windowing design

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M1](#w3-m1), [W3-M2](#w3-m2).
- **Scope / deliverable:** Compare compatibility-first and native multiwindow options; preserve mail/security invariants.
- **Acceptance:** Meet the corresponding original M3 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M3 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m4"></a>

### W3-M4 — Platform and packaging feasibility

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M1](#w3-m1), [W3-M2](#w3-m2), [W3-M3](#w3-m3).
- **Scope / deliverable:** Linux/GTK/Flatpak, macOS and Windows risks and explicit platform validation limits.
- **Acceptance:** Meet the corresponding original M4 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M4 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m5"></a>

### W3-M5 — Regression, performance and security test design

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M2](#w3-m2), [W3-M3](#w3-m3), [W3-M4](#w3-m4).
- **Scope / deliverable:** Behavior-to-fixture matrix with pass/fail evidence and owner/hardware requirements.
- **Acceptance:** Meet the corresponding original M5 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M5 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m6"></a>

### W3-M6 — Staged migration blueprint

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M3](#w3-m3), [W3-M4](#w3-m4), [W3-M5](#w3-m5).
- **Scope / deliverable:** Bounded future implementation steps, pinned target, rollback and separately approvable spikes.
- **Acceptance:** Meet the corresponding original M6 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M6 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

<a id="w3-m7"></a>

### W3-M7 — Independent review and owner decision gate

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [W3-M6](#w3-m6).
- **Scope / deliverable:** Evidence-backed proposed GO/DEFER/NO-GO, unresolved questions and separate implementation authorization.
- **Acceptance:** Meet the corresponding original M7 criteria in the assessment brief; do not implement or run a prototype.
- **Validation:** Corresponding M7 validation in docs/PLAN-migration.md; record source/date and unrun checks.
- **Issues:** none recorded; research risks are not verified findings. **Progress:** not started.

## Completed scope and historical evidence

Completed work is not a selectable backlog. Stable historical identifiers remain in these records and Git history; their reuse in older plans does not identify new work.

- Provider P1 audit and P2 architecture: [deliverability review](docs/validation/provider-deliverability-review.md), [transport recommendations](docs/validation/provider-transport-recommendations.md). F1–F5 local implementation: [implementation validation](docs/validation/provider-implementation.md); live limits remain explicit.
- G1–G3 shared-guidance implementation and partial runtime checks: [verification record](docs/validation/shared-skills-g3.md). Do not infer full G3/G4 runtime/context completion from parsing/discovery. AI5 bidirectional handoff completed under the 2026-10-14 request: [handoff record](docs/validation/shared-handoff-ai5.md); remaining G4 belongs to AI.
- Earlier rebrand/UI execution records remain in Git history (`git log -- PLAN.md`; original legal flags: `git show 33c1851:PLAN.md`). Upstream history and architecture: [Aerion history](docs/history/AERION_CHANGELOG.md), [upstream CASA record](docs/history/AERION_CASA_TIER2.md), [extension rules](docs/EXT_RULES.md) and [extension reference](docs/EXTENSIONS.md).
- Wails assessment remains unstarted; existing M0–M7 proposal details are retained in [the brief](docs/PLAN-migration.md), mapped to W3-M0–W3-M7 here to avoid old M-number collisions. Intended `plans/wails-v3/` output paths were never created; selection should reconcile them with the new execution-plan convention.

- Documentation/workflow reorganization evidence: [mapping and validation](docs/validation/project-management.md). This was documentation-only work and did not select any product milestone.
