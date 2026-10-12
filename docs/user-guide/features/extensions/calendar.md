---
sidebar_position: 2
---

# Calendar

The Calendar extension lets you view, create, and edit calendar events from CalDAV, Google Calendar, and Microsoft 365 in a dedicated view.

Like all [extensions](/docs/features/extensions), it's disabled by default — enable it under **Settings → Extensions**.

:::warning ALPHA
The Calendar extension is currently in ALPHA. Expect rough edges.
:::

## Enabling

1. Go to **Settings → Extensions**
2. Toggle **Calendar** on

A Calendar button is added to the [rail](/docs/features/extensions#the-rail) on the left of the window. The extension has its own settings dialog.

## Manual Sync

To sync calendar events on demand, click the footer at the bottom of the Calendar sidebar.

## Changes That Fail to Sync

Edits made while offline are saved locally and sent when you're back online. If the server keeps rejecting a change (for example, a read-only calendar), a red cloud icon with a count appears in the sidebar footer. Click it to see each change and its server error, then:

- **Retry** sends the change once more.
- **Discard** drops the change and resyncs the calendar so it shows the server's version again. Discarding a new event removes it, since it never reached the server.

## Known Issues

- All providers (CalDAV, Google, and Microsoft) rely on the server to send and parse invites. If your provider isn't set up to handle invites server-side, invitations may appear somewhat broken.

Untrusted CalDAV certificates prompt during setup and background sync, showing the exact failing host. Accept for the session or permanently. Permanent pins are listed under an account's Server → Trusted Certificates settings.
