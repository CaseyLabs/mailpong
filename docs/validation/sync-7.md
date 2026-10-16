# SYNC-7 validation — 2026-10-15

[SYNC-7](../../PLAN.md#sync-7) / [SY10](../../ISSUES.md#sy10) (narrowed to Graph, [SYNC-6](../../PLAN.md#sync-6)). Base `2bf3a28`.

## Decisions

- **Pin.** `messages.pinned` (migration 64, with rewind lines in the database tests and `tools/db/rollback-v61-to-v30.sql`). `DeleteOlderThan` skips pinned rows. The windowed UID reconcile leaves pinned UIDs out of its diff and asks the server for them with `UID SEARCH UID <set> NOT DELETED` (500 per command); the ones it no longer holds are deleted, the rest join the flag sync. A failed check keeps every pinned message, and a cancelled context aborts the sync. The CONDSTORE path needs no change: `CHANGEDSINCE` already covers every UID, and only `SyncMessages` deletes.
- **Search.** `Engine.SearchServer` runs `OR FROM/SUBJECT/TO/CC/BODY` in each subscribed folder except Trash and Spam, stores up to the limit (100 in the app) of matches not yet local, newest UID first, and pins them. The query and criteria are those of the removed SYNC-5 code, rebuilt without its result types.
- **Load older.** `Engine.LoadOlder` searches `BEFORE <window edge>` in the same folders, skips UIDs already local and stores the newest 100 per folder, pinned. It keeps no cursor, so a page that failed to fetch is retried. A short page in every folder means `done`; with no sync period the account is already complete.
- **UI.** One store serves both routes: Gmail-API accounts call the Gmail bindings, accounts without an active API route call `SearchIMAPServer` / `LoadOlderIMAP`, and Graph accounts get neither. The button reads "Search the whole mailbox" for both. `GmailOlderResult` became `OlderMailResult`.

## Evidence

- `internal/sync/imap_older_test.go` runs the engine against an in-memory IMAP server (`imapmemserver`) and a real database. `TestIMAPSearchServerPinsOlderMail`: the first sync keeps only the recent message; a search finds 2 and stores the 1 old match; two further syncs keep it; a repeat search adds 0; once the server expunges it, the next sync removes it and keeps the recent message. `TestIMAPSearchServerLimit`: 4 matches, limit 3, 3 stored. `TestIMAPLoadOlderPagesBackFromTheWindow`: pages of 2, 2 and 1 with a sync between steps; all messages stay; an expunged one is removed; no sync period reports `done`. Removing the `pinned = 0` clause makes the first and third fail.
- `tests/ui/chat-server-search.test.cjs`, `tests/ui/chat-load-older.test.cjs`: the actions appear for a Gmail-API and a protocol account (Accounts column expanded and collapsed for Gmail), call the matching binding, and are absent for Graph.
- `go vet ./...`, `go test ./app ./internal/...` — clean; `cd frontend && npm run lint`, `npm run check` — 0 errors.

## Limits

In-memory IMAP server and fake UI bindings only. The live IMAP check (search, Load older, pinned deletion on a real server) is unrun and needs an authorized account. `imapmemserver` ignores some real-server quirks (slow `BODY` search, `BEFORE` timezone handling). IMAP `BEFORE` and `SINCE` use whole dates, so the window's edge day is searched only by the sync. Load older is account-wide: one click loads a page of up to 100 per folder, so an account with many folders can add many messages per step. Search and Load older read every subscribed folder except Trash and Spam, not only the one open.
