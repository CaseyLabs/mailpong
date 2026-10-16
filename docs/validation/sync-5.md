# SYNC-5 validation — 2026-10-15

[SYNC-5](../../PLAN.md#sync-5) / [SY10](../../ISSUES.md#sy10) (narrowed; IMAP and Graph remain). Base `db95b9d`.

## Decisions

- **Scope.** The owner chose Gmail only. The unused `IMAPSearchFolder` / `FetchServerMessage` bindings, `internal/sync/search.go` and `buildMessageFromStreamedData` are removed; IMAP search and Load older move to [SYNC-7](../../PLAN.md#sync-7), because IMAP sync deletes everything older than the window and so needs a pin concept first.
- **Pinned deletion.** A windowed reconcile now fetches each pinned ref outside the window (`commitBatch`: one `minimal` GET each) and removes the ones Gmail no longer has.
- **Load older.** `GmailSync.LoadOlder` lists `before:<epoch>` (100 per step, newest first), stores and pins the messages not yet local, and keeps its position in the `older` sync scope, separate from the history cursor. The position starts at the window's edge and moves to the oldest received time among the page's rows (`Store.OldestAPIReceived`). A short page means the end; with no sync period the account is already complete.
- **Cursor fix found on the way.** `SearchServer` committed its pages with an empty cursor, which overwrote the stored history cursor and forced a full reconcile on the next sync. Search and Load older now commit with the existing cursor and kind (`storePinned`).

## Evidence

- `TestGmailAPIServerSearchPinsOutsideWindow` (extended): a pinned message deleted on the server is removed by the next expired-history reconcile (`Removed == 1`).
- `TestGmailAPILoadOlderPagesBackFromTheWindow`: pages of 2, 2, 1 and 0 messages; rows 3, 5, 6, 6; `done` on the short page; the history cursor is unchanged; a later reconcile keeps the loaded mail and removes one deleted on the server; an account with no sync period reports `done`.
- `tests/ui/chat-load-older.test.cjs`: Load older mail appears for a Gmail-API account with the Accounts column expanded and collapsed, stays after a page that added mail, reports "All older mail is loaded" after the last, and is absent for a protocol account.
- `go test ./app ./internal/sync ./internal/message ./internal/database` — PASS; `go vet`, `make lint` (golangci-lint 0 issues, ESLint, brand, contrast) — clean; `npm run check` — 0 errors; `make test-ui` — 105 passed, 0 failed, 0 skipped.

## Limits

Fake Gmail server and fake UI bindings only. The live Gmail check (search, Load older, pinned deletion) is unrun and gated on the owner moving an account to the Gmail API route. The native window was not run. Gmail's `before:` works on whole seconds, so messages sharing the oldest second of a page can be skipped. Load older also loads every label, whichever folder is open. Existing English-only `search.serverSearch*` locale strings from the removed IMAP flow were already unused and are left in place.
