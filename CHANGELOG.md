# CHANGELOG

## MailPong 0.4.0 (unreleased)

MailPong is an independent email client derived from [Aerion](https://github.com/hkdb/aerion) 0.3.5. This is its first release line.

**Features carried over from the fork**

- Chat-style mail: threads as chat bubbles with quoted history and signatures hidden, and a docked reply box at the bottom of each conversation
- Keyboard triage: archive, pin, snooze and mark unread
- A Low priority split for newsletters and other bulk mail
- Sender chats that combine all of one sender's threads into one conversation (pin, snooze, sender priority and sender chats are stored locally and are not synced)

**Visual refinement**

- Calmer chat list: roomy, airy density-aware rows (micro, compact, standard, large), sentence-case group labels, a quieter list header with an accent Compose button, and a soft Frost tint on the selected chat and on your own bubbles
- Easier-to-read text: one size step larger throughout the app
- Clearer hierarchy: contrast-checked primary, secondary and tertiary text, and a tonal step between the extension rail, Accounts column, chat list and conversation pane in Dark and Light
- Conversation pane polish: messages span the whole pane (theirs on the left, yours on the right), a Reading width setting (Narrow, Standard, Wide) for how wide bubbles grow, and larger message text
- Design tokens documented in `docs/UI_TOKENS.md`; `make lint` checks text contrast

**Migration to MailPong**

- New name, icon and app ID (`io.github.caseylabs.MailPong`); Go module `github.com/caseylabs/mailpong`
- Separate data directories, keyring service, database (`mailpong.db`) and single-instance lock: MailPong never reads or writes Aerion's data, so both can be installed side by side
- Bring-your-own OAuth credentials; no OAuth clients are bundled
- English-only UI; spellcheck dictionaries for English (US), English (UK) and English (Canada); email content in any language and charset still displays correctly
- Packaging for Linux (tarball, Flatpak), macOS and Windows uses the MailPong identity; the release workflow builds draft prereleases only
- Apache-2.0 attribution: `NOTICE`, `THIRD_PARTY_NOTICES.md`, `DISCLAIMER.md`; rewritten Terms of Use, Privacy Policy and Security Policy

For the Aerion history before this fork (0.3.5 and earlier), see [docs/history/AERION_CHANGELOG.md](docs/history/AERION_CHANGELOG.md).
