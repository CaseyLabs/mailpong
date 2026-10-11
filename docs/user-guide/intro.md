---
sidebar_position: 1
slug: /intro
---

# Introduction

MailPong is an open source desktop email client that turns email threads into combined chats: like a Messenger app for your mailbox. It is alpha software, derived from [Aerion](https://github.com/hkdb/aerion) by [@hkdb](https://github.com/hkdb) and independently maintained. It is not an official Aerion release.

## Why MailPong?

The Linux desktop has long needed a proper email client. The existing options each have their drawbacks:

- **Thunderbird** - Clunky interface with too much legacy baggage
- **Geary** - Crippled by its dependency on Gnome Online Accounts, and search is unreliable
- **Mailspring** - Built on Electron, which means heavy resource consumption
- **Evolution** - Feels like it's still stuck in 1999

Aerion was created to fill this gap, and MailPong builds on it with a chat-style interface: a standalone, lightweight email client that just works.

## Design Goals

MailPong keeps Aerion's five core principles:

1. **Resource Efficiency** - Minimal CPU, RAM, and battery consumption
2. **Modern UX** - Clean, intuitive interface with dark mode support
3. **Keyboard & Mouse Friendly** - Full keyboard navigation with vim-style shortcuts
4. **Independence** - No dependency on Gnome Online Accounts or other system services
5. **Search That Works** - Basic search that actually finds your emails

## Technology

MailPong is built with [Wails](https://wails.io) and [Svelte](https://svelte.dev), not Electron. This means native performance with a modern web-based UI, without the memory overhead of running a full Chromium instance.

MailPong has not been through Google's CASA security assessment. The CASA Tier 2 certification belongs to the upstream Aerion project and does not apply to MailPong.

## Platform Support

While Linux is a first-class citizen, MailPong also runs on:

- **Linux** - Flatpak (local build) or binary
- **macOS** - Native .app bundle
- **Windows** - Executable or installer

MailPong has no published releases yet; build it from source (see [Installation](./getting-started/installation/index.md)).

## Features at a Glance

- Multiple email accounts with unified inbox
- Conversation threads
- OAuth support for Gmail and Outlook with your own credentials
- CardDAV, Google, and Microsoft contact sync
- WYSIWYG composer with detachable window
- WYSIWYG signature editor
- Basic tracking element removal
- Full-text search
- Comprehensive keyboard shortcuts
- Light and dark themes
- S/MIME and PGP support
- 1st party extension system:
    - Contacts - ALPHA
    - Calendar - ALPHA

## Open Source

MailPong is licensed under Apache 2.0. You can view the source code, report issues, and contribute at [github.com/caseylabs/mailpong](https://github.com/caseylabs/mailpong). Version numbering continues from the Aerion code base: "MailPong 0.4.0" is a MailPong version.

#### Sponsorship

MailPong has no sponsors. The upstream Aerion project is sponsored by its own team; that sponsorship doesn't extend to MailPong.

#### Aerion Contributors

MailPong exists because of the work of the Aerion project: its author [@hkdb](https://github.com/hkdb), the issue and translation contributors listed in the [Aerion README](https://github.com/hkdb/aerion), and everyone who contributed code. See [NOTICE](https://github.com/caseylabs/mailpong/blob/main/NOTICE).
