# MailPong | _Email, reimagined as chat_

## Overview

MailPong is like a Messenger app for your mailbox: it simplifies your inbox by turning email threads into combined chats with the clutter removed. 

![MailPong chat mail view](docs/ss.png)

This is a personal project that is alpha software under active development, and user-caution is heavily emphasized. No guarantees or warranties provided. 

### History

MailPong began with the desire for an email client that provides a simple chat-like interface for email, in a similar way that the [Beeper app](https://www.beeper.com/) does for messenger apps. Unfortunately, no such solutuion existed.

However, the the open-source [Aerion](https://github.com/hkdb/aerion) project created by [@hkdb](https://github.com/hkdb) has made great strides in creating a modern cross-platform email client. MailPong first started out as a simple custom fork of Aerion v0.35, but has since greatly diverged into its own project.

## Features

- Email threads show as chat bubbles, with quoted history and signatures hidden
- A docked reply box at the bottom of each conversation
- A Low priority split that keeps newsletters and other bulk mail out of the main list
- Sender chats that combine all of one sender's threads into a single conversation (on by default, split per sender)
- Pin, snooze, sender priority, and sender chats are stored on this computer and are not synced to the server

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

MailPong is licensed under the Apache License 2.0. 

**Legal:** [Disclaimer](DISCLAIMER.md) · [NOTICE](NOTICE) · [Third-Party Notices](THIRD_PARTY_NOTICES.md) · [License](LICENSE) (Apache-2.0)
