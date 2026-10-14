# API-6 validation: shared mailbox API route

Local implementation only. No live Microsoft/Graph access, credentials or mailboxes were used.

## Proven with disposable fixtures

- Owner/child isolation: consent, refresh (one rotation under the owner's lock), rejection (owner pauses all dependents, partial consent pauses only the child), clear, sync, read, actions and send all resolve the owner grant and fail closed on a broken link (`app/mail_api_shared_test.go`).
- Requests address `/users/{mailbox}`; send requires `From` equal to the stored mailbox address.
- MA5: parent removal aborts and keeps the parent and credentials when listing or removing a child fails (`TestRemoveAccountKeepsParentWhenChildRemovalFails`).
- Migration 62 and route validation (`internal/account`, `internal/database` rewind tests).
- UI: linked shared mailbox offers shared-mode consent, no migration button, Accounts expanded and collapsed (`tests/ui/mail-api-migration.test.cjs`).

## Not verified (limits)

- Live Graph identity, delegation (Full Access, Send As/Send on Behalf), Sent copy and Bcc behavior. `sharedMailboxLiveVerified` stays `false`, so activation and migration return the gate error.
- Sender-review wording for Graph is unchanged; MA1 remains the sending privacy gate.
