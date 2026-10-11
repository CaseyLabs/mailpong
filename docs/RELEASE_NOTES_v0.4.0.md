# MailPong v0.4.0 (draft release notes)

Draft for owner review. Paste into the GitHub release body when the owner approves the first release; delete this file afterwards. The tag `v0.4.0` has not been created.

---

**MailPong v0.4.0 is the first alpha release.** MailPong turns your email threads into chats. It is an independent project derived from [Aerion](https://github.com/hkdb/aerion) 0.3.5 by [@hkdb](https://github.com/hkdb); it is not an official Aerion release and the Aerion project does not endorse or support it.

## Read before installing

- **Alpha software.** Expect rough edges. Keep your mail on the server; MailPong does not delete anything unless you ask it to.
- **Bring your own OAuth credentials.** No Google or Microsoft OAuth clients are bundled. Password and app-password sign-in works without any setup. See the user guide for OAuth.
- **Unsigned builds.** The macOS and Windows artifacts are not code-signed or notarized, so expect OS warnings.
- **Separate from Aerion.** MailPong uses its own data directories, database, keyring service and app ID (`io.github.caseylabs.MailPong`). It never reads or writes Aerion's data, so both can be installed side by side. To bring accounts over, follow [Import from Aerion](https://github.com/caseylabs/mailpong/blob/main/docs/user-guide/getting-started/import-from-aerion.md).

## Highlights

- Chat-style mail: threads as chat bubbles, quoted history and signatures hidden, a docked reply box under each conversation
- Keyboard triage: archive, pin, snooze, mark unread
- A Low priority split for newsletters and bulk mail
- Sender chats that combine all of one sender's threads (pin, snooze and sender priority are stored locally and are not synced)
- English UI with en-US, en-GB and en-CA spellcheck; email in any language and charset still displays correctly

## Licensing

Apache-2.0. See `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md` and `DISCLAIMER.md`.

Full list of changes: [CHANGELOG.md](https://github.com/caseylabs/mailpong/blob/main/CHANGELOG.md).
