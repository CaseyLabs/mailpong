# Shared mailbox API activation design (API-1)

Design proposal only; shared API routes remain unsupported. [API-1](../PLAN.md#api-1), [MA4](../ISSUES.md#ma4) and [MA5](../ISSUES.md#ma5). No code, credentials, routes or live systems changed.

## Current model

A linked shared mailbox is an `accounts` row with `shared_mailbox_parent_id` set and `Email` = the shared address. It has no OAuth grant of its own: its IMAP/SMTP use copies of the parent's protocol tokens (`app/oauth.go:502`, `app/compose.go:232`), and `authOwnerID` resolves to the parent for sending. `account_mail_api_routes` is keyed by `account_id`, with `shared INTEGER` recording the requested shared route/access mode, not the scopes actually granted. Nothing stores which mailbox to address. Gmail delegation is unsupported by design (`BuildMailAPIProfile`, `RequireMailAPICapability`).

## Blockers mapped

| # | Site | Today | Needed |
| --- | --- | --- | --- |
| B1 | `app/mail_api.go:62` `prepare` | rejects linked shared accounts | allow a child; consent runs against the parent's identity, grant stored under the parent |
| B2 | `app/mail_api.go:144,315` | `shared` flag compared on reauth/status | compare mailbox target, not only the flag |
| B3 | `app/api_route.go:95` `runAPISync` | `route.Shared` rejected | build `graphapi.Reader{Shared: target}` (reader already supports it) |
| B4 | `app/api_route.go:226` `activateAPIRoute` | `route.Shared` rejected | allow once B1–B3, B5–B8 and the live gate hold |
| B5 | `app/api_actions.go:88` `apiRemote` | rejected | `graphapi.Writer{Shared: target}`; grant read from the owning account |
| B6 | `app/api_read.go:30` `fetchAPIRaw` | rejected | `Reader{Shared: target}`, `RequireMailAPICapability("read", true)` |
| B7 | `app/api_migration.go:64` | blocker "cannot be migrated yet" | replace with real blockers (below) |
| B8 | `app/mail_api_send.go:56,73-76`, `graphapi.Sender.Own` | `Own` = `acc.Email`, token slot read by `acc.ID` | for a child, `Own` is the parent's mailbox, From = shared address uses `/users/{from}/sendMail`; grant read from the owning account |
| B9 | every `GetOAuthTokensForClientConfig(accountID, …)` (`mail_api.go:222,235,253,281,302,346`, `mail_api_send.go:56`, `api_actions.go:92`) | slot keyed by the routed account | resolve the grant owner once (`route.GrantAccountID`), using the validated owner ID for storage, locking, snapshot refresh, rejection clearing and status |
| B12 | `app/mail_api.go:332` `clear`, `app/account.go:276` removal | clear deletes the routed account slot; parent removal can leave children | reference-aware authorization removal and fail-closed owner deletion |
| B13 | `internal/submission/api.go:68`, `app/sender_evidence.go:49`, `frontend/.../AccountDialog.svelte:524` | send requires shared route mode; linked child route UI hidden; sender evidence still describes SMTP | preserve preflight and sender review; describe Graph rights/route accurately in child UI |
| B10 | `internal/credentials/mail_api.go` (Google) | shared always refused | stays refused: Gmail delegation remains out of scope |
| B11 | token rotation: `SetOAuthTokensForClientConfig` | one row per account | a child must not hold a copy of the API grant (see decision D2) |

## Decisions

- **D1 Identity.** The route lives on the **linked child account**, which already has its own address, folders, messages, cursors and local data. The parent keeps its own route for its own mailbox. Add `mailbox_address TEXT NOT NULL DEFAULT ''` and `grant_account_id TEXT NOT NULL DEFAULT ''` to `account_mail_api_routes` (empty = today's behavior: own mailbox, grant under `account_id`). For newly configured routes, `shared` must agree with `mailbox_address != ''`; retain existing persisted flags and reject legacy `shared=1` rows with no target. `mailbox_address` is copied from `accounts.email` at consent, stored lower-cased, and validated against the child on every use; if the child's address changes the route is paused.
- **D2 Grant ownership.** The API grant stays in the parent's slot (`ClientConfigID`). The child route references it through `grant_account_id`; it is never copied, so refresh-token rotation has one writer and the existing per-slot refresh lock covers parent and child. The proposed owner resolver and revalidation fan-out make parent reauthorization available to children. The lock already resolves a child to its parent, but storage does not (`internal/credentials/refresh_lock.go:21`, `internal/credentials/oauth_clientconfig.go:28`); every token read/write must explicitly use the owner. Parent deletion is best-effort application recursion (`app/account.go:276`), not a database cascade; see lifecycle requirements below and [MA5](../ISSUES.md#ma5).
- **D3 Parent prerequisite.** Child consent is the parent's consent with the `.Shared` scopes (`BuildMailAPIProfile(base, true)`, work/school only). If the parent already has a non-shared API grant, the child flow reauthorizes the parent with the larger scope set; the parent's own route must remain valid (its `shared` flag stays false and its target stays `/me`; granted scopes are evidence on the token). Adding scopes must not silently change the parent's mailbox target.
- **D4 Reads/writes.** `Reader.Shared` / `Writer.Shared` = `mailbox_address`. Read/write requests keep `Prefer: IdType="ImmutableId"`; the MIME send endpoint uses its existing headers. Capability gate always `shared=true` for the child. `api_message_refs`, folder rows and delta cursors are already keyed by the child's account ID, so parent and child cannot collide.
- **D5 Send.** Rights are never inferred. Restrict child submissions to From equal to the validated child target (no other aliases in this scope); pass the parent address to `Preflight` so it requires the Shared send grant; the endpoint is `/users/{from}/sendMail`. This `/users/{from}` send requires Full Access plus Send As or Send on Behalf, not just OAuth scopes. Graph selects the sender semantics; the endpoint defaults to the shared mailbox Sent Items and tenant settings can add copies; the UI shows only configured evidence (`SenderEvidence`) and an unknown outcome stays unknown. Bcc privacy remains the MA1 gate.
- **D6 Migration.** Plan/Migrate/Rollback operate per account as today. New blockers replace B7: the grant lacks the Shared scopes; the owner's grant is missing or unusable (the parent's own mailbox may stay on IMAP); `mailbox_address` differs from the account email; the child has pending IMAP-era state the Plan cannot represent. Rollback returns the child to IMAP using the parent's protocol tokens (unchanged). Snapshot restore stays F5b-1.
- **D7 Gmail.** Excluded. Delegated Gmail would need its own authorization model.

## Invariants

1. A child route never reads or writes the parent's mailbox, and the parent route never addresses `/users/{shared}`: child paths and From are derived from the validated child target; own-route reads/writes use `/me`. Own-route alias sending remains governed by existing explicit shared-mode checks and is not widened here.
2. The API grant is stored once; no code path copies it to a child.
3. No automatic fallback or route switch; activation stays explicit opt-in, behind the Plan blockers.
4. Unknown capability, unknown scopes or a lost delegation pause the account (`errAPIPaused`), never fall back to IMAP, and keep local rows and queued actions.
5. Mailbox access granted by the tenant is never assumed from scopes: `RequireMailAPICapability` states this and checks recorded evidence only.
6. Existing own-mailbox accounts use empty new fields without switching routes. Legacy `shared=1` rows without a stored target remain blocked and require explicit configuration; never reinterpret them as `/me`. The schema migration is additive and rerunnable. Children must explicitly name their linked parent as `grant_account_id`; empty fields on a linked child fail closed.
7. Logs and events carry no tokens or message content.

## Bounded implementation proposal (not authorized)

Proposed milestone **[API-6 — Shared mailbox API route](../PLAN.md#api-6)**, depends on this design. API-5 is a rollback reliability follow-up, not an established prerequisite; it remains separately selected. Local implementation may be selected separately, but exposing activation/migration additionally requires the live gate:

1. Migration: the two columns (`ADD COLUMN`, rewind lines in `database_test.go`); `MailAPIRoute` fields; a validated `routeGrantOwner(route)` helper used by B9 and B12; owner lifecycle guards and tests for MA5.
2. Consent: B1–B2, scope upgrade (D3), identity check against the parent.
3. Sync/read/write/send wiring: B3, B5, B6, B8 with fixtures against fake Graph (`/users/{shared}` paths, wrong-address rejection, rotation through one slot).
4. Plan blockers (B7), activation (B4), UI: show the section for linked children with an explicit "needs Shared permissions and delegation" message.
5. Tests: table-driven route/grant resolution, token-rotation single-writer, child/parent isolation, migration rewind, backend-free UI regression suites via `make test-ui` (the older `tools/mail-api-migration-check.cjs` is supplemental).

## Validation gates

- Local, required before API-6 is done: items 1–5 above plus `go test ./internal/account ./internal/graphapi ./internal/sync ./internal/database ./app` and `make test`.
- **Live (separately authorized, not covered here):** a Microsoft 365 tenant with a shared mailbox: delegate read, move/flag, Send As and Send on Behalf, Sent-copy destination and count, Bcc privacy, tenant policy denial, reauthorization of the parent reaching the child. Fixtures do not close these ([live limits](MAIL_APIS.md#live-verification-limits)).
- Migration gate: no shared migration is offered in the UI until the live checks pass and are recorded in `docs/validation/`.

## Surprises & Discoveries

- `graphapi.Reader`, `Writer`, `Preflight` and the capability gate already model a shared target; app wiring and identity storage are missing; the submission preflight, owner lifecycle and UI also require integration.
- `route.Shared` records a requested access mode but cannot name a mailbox; grant evidence lives in the token. Scope expansion must not change the parent target.
- Linked children copy protocol tokens and share the owner refresh lock; API storage keys still need explicit owner resolution to avoid independent token copies (D2).


## Identity and grant lifecycle requirements

- Resolve a child to its existing, non-child Microsoft OAuth parent; reject missing owners, cycles, nested links, personal/unknown Microsoft account types, mismatched slot/provider/client and stale parent links. Consent uses the parent's provider, login hint and stable OAuth identity (`verifyIdentity`); the shared email is only the target. Reject malformed addresses before path construction; compare normalized addresses consistently with the existing envelope normalizer and case-insensitive comparison.
- This bounded design supports a shared address only where it is also a valid Graph `/users/{id-or-UPN}` target. Email aliases are not necessarily UPNs. Require separately authorized identity/path evidence before rollout; a target-resolution failure blocks migration. Arbitrary alternate targets, mailbox discovery scopes and folder-only delegation are excluded; they need a later identity design.
- The owner must have a staged or active own Graph route describing the authoritative API snapshot; authorizing a child may create a staged owner route without activating it. Children reference that snapshot/slot; duplicated child snapshots are not authoritative. Refresh and client-change checks use the owner's snapshot. Scope upgrades preserve every route state and target, update the owner snapshot under the owner slot lock, and revalidate dependent routes. Ordinary owner reauthorization must retain Shared scopes while children reference the slot, or explicitly refuse scope reduction.
- Serialize consent/clear/refresh on the owner slot; re-read owner/link/route metadata after acquiring it. Hold the per-account transition lock before the owner slot lock whenever both are needed, and never acquire a transition lock while holding the slot lock. Scope upgrade persists the owner grant/snapshot before staging the child; if child staging fails, keep the upgraded valid owner grant and report that no child was staged. Existing owner grant-save rollback remains required. No network request may use a route whose owner identity/client/target changed during transition. Serialize owner deletion with dependent operations and drain/cancel in-flight work before deleting credentials.
- Clearing a staged child removes only its route, never the parent's API tokens or route. Reject owner authorization removal while any child references it, including staged children; require explicit child rollback/removal first. Returning the parent to IMAP is permitted if its staged authorization remains available to children. Child rollback does not clear the owner grant.
- Owner deletion must stop dependent workers and abort on child enumeration/deletion failure before deleting the parent or its credentials. Validate owner existence and parent link at every child operation so legacy orphan rows fail closed; preserve their local data and never route them to `/me`. A new plain-text grant reference must not silently inherit account deletion semantics.
- A rejected owner refresh pauses all dependent routes and emits reauthorization events for their account IDs. A mailbox-specific delegation denial pauses only that child; it must not revoke or delete a valid owner grant. Reauthorization revalidates each dependent route before resuming it. Unknown writes/submissions retain their journal outcome and are not replayed.

## Acceptance-to-validation mapping for later implementation

| Invariant | Required disposable proof |
| --- | --- |
| Identity/target isolation | Parent `/me` and child `/users/{target}` read, delta, raw, move and send fixtures; malformed/changed address, wrong OAuth identity, missing/nested owner, legacy targetless shared route rejected before requests; parent/child folders, refs and cursors isolated |
| One grant and snapshot | Concurrent parent/child expiry and 401 refresh rotates one owner slot; no child token writes; omitted scopes retained; changed client rejected; owner reconsent preserves target/state and fans out resume/pause |
| Lifecycle | Child clear/rollback leaves owner tokens intact; owner clear with references refused; interrupted scope upgrade and failed child staging recover; failed child enumeration/deletion prevents parent deletion; orphan rows send no requests |
| Migration/data safety | Rerunnable schema and rewind tests; blockers and volatile rechecks retain queue/submission/sync guards; rollback keeps local rows and reports unfinished actions; no snapshot restoration claim |
| UI and provider limits | Frontend lint/check, full `make test-ui`, Mail Accounts expanded/collapsed and sender review; separately authorized live read/delta, move/flag, Full Access plus both sender modes, Sent copies, Bcc and tenant denial |

Local implementation checks also include `go test ./internal/credentials ./internal/submission`; frontend changes require `npm run lint` and `npm run check` in `frontend/`. No new tests or provider calls are needed to prove this documentation milestone.

## Provider-source basis

Reviewed 2026-10-14: Microsoft's [shared-folder access](https://learn.microsoft.com/en-us/graph/outlook-share-messages-folders) documents delegated read/write scopes and ID/UPN paths; [sending from another user](https://learn.microsoft.com/en-us/graph/outlook-send-mail-from-other-user) distinguishes mailbox rights from consent and describes endpoint-dependent Sent behavior. These are design requirements, not evidence that Mailven's MIME, delta or tenant scenarios have passed. Current behavior remains in [MAIL_APIS](MAIL_APIS.md); this proposal authorizes no activation.
