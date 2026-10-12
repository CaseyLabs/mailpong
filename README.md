# Mailven | _Email, reimagined as chat_

## Overview

Mailven is a Messenger-style email client, that simplifies your inbox by turning emails into combined chats with the clutter removed.

![Mailven chat mail view](docs/ss.png)

This is a personal project that is alpha software under active development. User-caution is heavily emphasized, no guarantees or warranties provided. 

### History

Mailven started with the desire for a simple chat app interface for email, similar to the way that [Beeper](https://www.beeper.com/) has done for messenger apps. Unfortunately, no such solutuion existed.

However, the the open-source [Aerion](https://github.com/hkdb/aerion) project created by [@hkdb](https://github.com/hkdb) has made great strides in creating a modern looking cross-platform email client. Mailven first started out as a simple custom fork of Aerion v0.35, but has since diverged into its own project.

## Features

Mailven strives to present your emails as chats, similar to how instant messaging apps like Messages, WhatsApp, etc. The goal is to reduce the information overload that contemporary email has become:

- Emails are grouped by the sender, not subjects, so that senders always show up as a single chat.
- Unneccessary message clutter (such as signatures, quoted text, HTML) are hidden by default.
- Newsletters and notifications are automatically marked as "Low priority".
- Chats can be pinned and snoozed.

See [Email Management](docs/user-guide/features/email-management.md#chat-mail) for details.

## Installation

There are no published releases yet. Build from source with the [Build Guide](docs/BUILD.md), or follow the [Installation Guide](docs/user-guide/getting-started/installation/index.md) for build-from-source and local Flatpak steps on each platform.

## Documentation

- [User Guide](docs/user-guide/intro.md)
- [Build Guide](docs/BUILD.md)
- [Release process](docs/RELEASE.md)
- [Changelog](CHANGELOG.md)

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities according to [SECURITY.md](SECURITY.md).

## License

Mailven is licensed under the Apache License 2.0. 

**Legal:** [Disclaimer](DISCLAIMER.md) · [NOTICE](NOTICE) · [Third-Party Notices](THIRD_PARTY_NOTICES.md) · [License](LICENSE) (Apache-2.0)

