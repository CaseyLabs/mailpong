# Migration snapshot restoration design

Design for [API-2](../PLAN.md#api-2) / [F5b-1](../ISSUES.md#f5b-1). Implemented by [API-3](../PLAN.md#api-3); current behavior and the deviations from this design are in [MAIL_APIS](MAIL_APIS.md#snapshot-restoration) and the [validation record](validation/api-3.md).

## What a restore is, and is not

- **Rollback (today)** returns one account to IMAP/SMTP. API-era local changes stay; remote actions are not undone.
- **Snapshot restore (this design)** replaces the *whole local database* with the pre-migration copy. It is a local operation. It never undoes anything that happened on a mail server: sent mail, deletes, moves, flag changes and drafts saved remotely stay as they are, and the next IMAP sync reconciles them.

## Facts that shape the design

- `apimigrate.Store.Snapshot` writes `VACUUM INTO <data>/migration-snapshots/<account>-<time>.db` plus a manifest. It copies **the entire `mailven.db`**, so every account, rule, draft, setting and queue is in it. A per-account restore does not exist; restoring means rolling back all accounts to the snapshot time.
- The live database (`app.db`, opened once in `app/app.go`) is shared by dozens of stores, the scheduler, IDLE, the credential store and detached windows, in WAL mode (`mailven.db-wal`, `-shm`). It cannot be swapped under a running process.
- Detached composers are separate processes of the same executable (`OpenComposerWindow` in `app/ipc.go`) that open the same database; the single-instance lock in `main.go` covers only the main instance.
- `database.Migrate` refuses a database newer than the build (`ErrSchemaTooNew`); a snapshot may be older than the current schema.
- Credentials live outside the database snapshot: the OS keyring and the encryptor key file in the data directory, with encrypted database rows as keyring fallback. Refresh tokens rotate, so fallback rows inside a snapshot may be revoked.
- State outside `mailven.db` is not in the snapshot: `contacts.db`, `search/<account>` indexes, `attachments/`, `extensions/*/data.db`, `keys/`, `config.toml`.

## Decision: offline whole-database swap at next startup

Rejected: in-process swap (live handles, WAL, goroutines; any missed store keeps writing to the old file) and per-account row merge (large foreign-key graph, schema drift, false precision). Chosen: stage a restore intent, quit, and apply it before the database is opened.

### Flow

1. **Preflight (running app).** Require: snapshot and manifest present, manifest version known, manifest account exists in the snapshot. On a *copy* of the snapshot run `PRAGMA integrity_check`, `Migrate` forward (to prove it can reach the current schema) and a credential probe (the encryptor key decrypts a sample fallback row). Refuse on failure, naming the check. Refuse a snapshot newer than the build.
2. **Consent.** See the UX section. On confirm, write `<data>/pending-restore.json` (0600, atomic temp+fsync+rename) with `phase=staged`, snapshot path and SHA-256, restore ID, consent record (time, accounts affected, counts shown) and the pre-restore backup path. Staging is refused while any detached composer is connected over IPC. Then quit through the normal shutdown path (scheduler, IDLE stopped).
3. **Startup, before `database.Open`.** If an intent exists, run the phases below. Each phase is written to the intent before it starts and is idempotent.
   - `backing-up`: fail closed unless no other process holds the database (a composer started after staging, or one left running): the startup path must prove exclusive access before continuing, by a mechanism API-3 selects and verifies on each platform. Then open the live database, checkpoint (TRUNCATE), `VACUUM INTO <data>/restore-backups/pre-restore-<id>.db` (0600). Never overwritten, never auto-deleted.
   - `staging`: `VACUUM INTO mailven.db.restoring` from the verified snapshot (fresh file, no WAL), integrity check, fsync.
   - `swapping`: remove `mailven.db-wal` and `mailven.db-shm`, then atomically rename `.restoring` over `mailven.db` (Windows: replace-existing move), fsync the directory. Stale WAL frames replayed onto the restored file are the main corruption risk, so the removal precedes the rename and a leftover WAL with no matching database is refused.
   - `post`: open normally (migrations run), apply the post-restore rules below, write `restore-result.json`, delete the intent.
4. **Crash recovery.** The intent is the commit marker. Before `swapping` the live database is untouched: resume or abort. In `swapping`, `.restoring` present means the rename has not happened (redo); absent means it has (continue to `post`). A corrupt or unparsable intent is moved aside and startup proceeds on the untouched database with a visible notice. Failure at any phase leaves the pre-restore backup and reports the phase.
5. **Undo.** The pre-restore backup is itself a restorable database through the same flow ("Undo restore"), so the data-loss step is reversible until the user deletes the backup.

### Post-restore rules

- **Route.** Force the restored account's route inactive and record `api_migrations` as `rolled-back` with the restore ID; never come back active.
- **Queues.** Queued actions, outbox and submission attempts that existed at snapshot time may already have been applied remotely. Hold them: nothing is sent or replayed until the user reviews each one. Never auto-send.
- **Accounts and credentials.** Restore never writes or deletes keyring entries. Accounts created after the snapshot disappear from the database; list their now-orphaned credentials and offer explicit cleanup. Accounts removed after the snapshot reappear without credentials: mark them needs-reauthorization. Any account whose credential fails validation is marked needs-reauthorization, not silently retried.
- **Derived state.** Drop and rebuild search indexes for all accounts; treat `attachments/` as cache; leave contacts and extension databases untouched and say so.
- **Sync.** Cursors come from the snapshot, so the next sync re-reads server state and may re-download. That is expected.

### Data-loss consent UX

Placed in the account's Mail API section as a secondary, destructive action beside the default *Return to IMAP / SMTP*; it appears only when a snapshot exists. The dialog must state, before the confirm control enables:

- the snapshot time and that **all accounts** return to that time, and everything stored locally since is lost (counts per account of messages, drafts, rules; accounts added since);
- that server-side changes are *not* undone and will be re-synced;
- that held outbound items need review;
- the location of the pre-restore backup and how to undo;
- that Mailven restarts.

Confirmation needs an explicit checkbox plus a button named with the date (for example "Restore database to 12 Oct 2026"). The Mail view must still work with Accounts expanded and collapsed. Copy is English-only.

## Failure cases to cover

| Case | Required outcome |
| --- | --- |
| Snapshot missing, hash mismatch, corrupt | Refuse at preflight; nothing staged |
| Snapshot schema newer than build | Refuse with the existing `ErrSchemaTooNew` guidance |
| Snapshot older schema | Migrate on a copy during preflight; fails closed |
| Crash in any phase, including between backup and rename | Recover per phase table; live database is never half-replaced |
| Stale `-wal` present at swap | Removed before rename; test proves no replay |
| Disk full or permission denied | Abort before `swapping`; backup retained; notice |
| Second instance or detached composer open | Refuse to stage while a composer is connected; at startup the main-instance lock plus a verified exclusive-access check, otherwise abort before `backing-up` |
| Encryptor key missing or mismatched | Refuse at preflight (restored fallback rows would be unreadable) |
| Keyring unavailable | Proceed; accounts needing credentials are marked for reauthorization |
| Intent corrupt | Moved aside; start on untouched database |

## Validation required for API-3

All in a temporary data directory with fixture databases and fake credential stores; never the real database. Fault injection at each phase boundary and mid-copy; WAL leftover test; old/new/same schema snapshots; hash and integrity failures; credential-key mismatch; orphan and removed-account credential handling; held-queue assertion that nothing is sent; undo through the pre-restore backup; Windows replace semantics noted as unverified until run there; backend-free UI regression for the consent dialog with Accounts expanded and collapsed. Native and live restore stay separately authorized.

## Open points for API-3 selection

Retention policy for `restore-backups` (default: never auto-delete); whether `contacts.db` should join the snapshot (default: no, changing manifest format needs a version bump); Flatpak path and permission behavior.
