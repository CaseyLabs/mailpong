# Issues

## Deferred follow-ups

Found by the full audit on 2026-10-08. Each entry gives a severity, the
location, and the failure. "Unverified" means a reviewer reported it and it
has not yet been confirmed against the code.

### Mail APIs

- **MA1 medium** `internal/gmailapi/send.go`, `internal/graphapi/send.go`: API clients add a `Bcc:` header for hidden envelope recipients because both providers read recipients from headers. Provider stripping of that header from delivered copies is undocumented and unverified; a Bcc recipient could be exposed. F4e implemented route activation, but the required separately authorized live Bcc check remains an unsatisfied rollout gate; activation does not establish privacy. Message size limit (25 MiB) is also a local, undocumented ceiling.

- **MA4 low** `app/api_route.go` `ActivateMailAPIRoute`: shared (delegated) mailbox routes cannot be activated because the Graph shared-mailbox address is not stored and `apiRemote` rejects shared routes. Found in F4e-3; needs shared-mailbox identity storage before a later milestone. F5a left it open: storing the address is not cheap (route schema, Graph delegate flow, read/write clients); shared routes stay unmigratable with that reason.

- **MA2 low** `internal/extensions/store.go`: `gofmt -l` lists the file as unformatted (noticed during F4e-1; untouched there). Run `gofmt -w` on it in a change that owns that package.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript 7 remains blocked upstream.
  Verified on 2026-10-12 with `npm view <package>@latest version peerDependencies --json`:
  TypeScript 7.0.2 is published, but latest `typescript-eslint` 8.71.1 requires
  `>=4.8.4 <6.1.0` and `svelte-check` 4.7.6 requires `^5.0.0 || ^6.0.0`.
  Both tools are already current here; retain TypeScript ~6.0.3 until both accept 7.
  Recheck using the commands in `CONTRIBUTING.md`.

### AI guidance portability (reviewed 2026-10-12)

- **AI5 medium** `.agents/skills/project-handoff/SKILL.md`: G2 corrected checkpoint-before-commit ordering and now requires branch/revision, dirty state, validation provenance and blockers in the shared handoff. Bidirectional continuation, including uncommitted work, is still unverified; complete the deferred bidirectional runtime trials in PLAN.md G4 on a future branch before closing this item. The user closed the current branch on 2026-10-12; resume only when requested.

### Provider compatibility and deliverability (P1, 2026-10-12)

Confirmed by static inspection in P1; F1 verified the PD1/PD9 local fixes and new-account PD2 guidance/defaults; F2a verified the PD3 submission-outcome fix; F2b verified PD4/PD6/PD8 MIME/header/SIZE/international-envelope preflight, F3a verified PD5 sender evidence, review and blocking; and F3b verified the PD7 explicit route/destination policy and durable copy reconciliation plus U3 linked auth ownership/locks with disposable fixtures. Remaining findings and the PD2 live compatibility gate stay open. Details, provider sources, applicability and live-check limits are in [the provider review](docs/validation/provider-deliverability-review.md). Architecture and follow-up prioritization belong to P2.

- **PD2 medium — live compatibility gate** `app/oauth.go`, `internal/credentials/oauth_grants.go`: F1 corrects the setup guides and new personal/work defaults, verified by disposable fixtures and guarded UI checks. Existing Microsoft accounts retain their configured endpoints and unknown account type where no explicit evidence was recorded. Their prior `smtp.office365.com` endpoint can still differ from Outlook.com's published default; actual endpoint failure and migration compatibility remain unverified. Confirm account type and separately authorize live evidence before proposing any existing-account endpoint migration.

### Draft storage

- **CD2 medium** `app/draft.go:128–166`: existing autosave falls back to plaintext body/attachments when requested S/MIME or PGP encrypt-to-self fails. F2a rejects this downgrade for its new pre-submission recovery writes; ordinary autosave still needs a separately scoped encryption-failure policy. Confirmed by static inspection and disposable missing-key fixtures; no real drafts were changed.

### Branch review: provider implementation (2026-10-13)

- **R1 medium** `app/chat_reply.go` `QueueChatReply`: the chat composer has no
  sender review UI, so a reply from a sender needing review (Microsoft, shared
  mailboxes, non-Google OAuth, stale Gmail send-as) is refused at queue time and
  must be sent from the full composer. Add the review prompt to chat.
- **R2 low** `internal/submission/sent.go` `ReconcileCopies`: each send runs the
  observation UPDATE and returns every accepted copy for the account, so cost
  grows with journal size. Scope it to the new attempt or pending copies.

- **Low** `internal/gmailapi/read.go`, `internal/graphapi/read.go`: `Reader`/`ReadError`/fetch (401-once refresh, size cap, classification) are duplicated across the two read clients. Found by F4d-3.
- **Low** `internal/graphapi/read.go` `GetRaw`: a message larger than the 64 MiB cap fails the whole folder run with no skip, so one oversized message blocks that folder's cursor. Found by F4d-3.
- **Low** `internal/sync/graphapi.go` `deltaRun`: the reconcile keeps every seen ID in memory and per-message GETs run for each local ID the enumeration missed; large folders are slow but bounded. Found by F4d-3.
- **Low** `internal/sync/gmailapi.go` `reconcile`: collects every remote ID into one slice before committing, so memory grows with mailbox size (ID strings only; page requests stay bounded). Found by F4d-2 acceptance.
- **Low** `internal/sync/gmailapi.go` `reconcile`: costs one `minimal` GET per known message on the rare reset path; 5xx responses are treated like rate limits (bounded retries).
- **Low** `internal/sync/apicommon.go` `parseAPIRaw`: a message with no parseable `Date:` header gets `time.Now()`, not the provider's received time (Gmail `internalDate`, Graph `receivedDateTime`). Such messages sort as new on every full ingest. Found by the branch review.

- **F5b-1 (low)** — `app/api_migration.go` `RollbackMailAPIMigration`: full snapshot restore is not implemented; after a rollback only route state changes, so API-era local changes stay. F5c shows the snapshot path with a note but does not offer restore; it needs its own live-DB replacement design and data-loss UX.
- **F5b-2 (low)** — `internal/apimigrate` `Correlate`: messages with no Message-ID, or ambiguous duplicates, stay uncorrelated and appear once from the API with no carried local state.
- **F5b-3 (low)** — `app/api_migration.go` `RollbackMailAPIMigration`: the IMAP reconcile is only the async `syncScheduler.TriggerSync` inside `deactivateAPIRoute`; rollback does not wait for it and no test asserts the trigger (the scheduler is a concrete type). Add a scheduler seam and a test when F5c needs rollback progress.
- **R3 medium** `internal/apiactions/worker.go` `applied`: when a provider move
  returns a new remote ID and `ApplyAPIMove` fails, the error is only logged. Later
  queued actions keep the old remote ID, the provider answers 404, and `gone()`
  deletes the message locally until the next sync restores it. Retry the re-key
  or keep the action open instead of only logging.
- **R4 low** `app/api_actions.go` `apiMove`: a mid-batch failure returns before
  `apiMoveLocal` emits `messages:moved`/counts, so rows already moved locally stay
  shown in the source list until the next refresh. (The queued moves are now
  kicked.)

## Documentation audit (2026-10-14)

- **DOC3 low** `docs/PLAN-migration.md:47`: the execution contract requires `.claude/rules/working-style.md`, which no longer exists. Point to the current shared guidance in `AGENTS.md`.
- **DOC4 low** `docs/user-guide/features/keyboard-shortcuts.md:73`: Alt+G is described unconditionally as All Inboxes, but `Sidebar.svelte` selects the first visible item, with All Inboxes only when present. Align with `docs/KEYBOARD_SHORTCUTS.md:37`.
- **DOC5 low** `docs/user-guide/faq.md:25` has malformed `https:/svelte.dev`; `docs/user-guide/troubleshooting.md:269` links to the absent FAQ anchor `where-are-log-files-located`. Correct the URL and point log guidance to an existing section or supply the missing section.

## Recurring problems

None.
