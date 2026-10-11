---
sidebar_position: 1
---

# Contacts

The Contacts extension is a frontend for MailPong's [contact sync](/docs/features/contacts) data. It surfaces the contacts MailPong already syncs (CardDAV, Google, and Microsoft) in a dedicated view, and adds the ability to create and edit them.

Like all [extensions](/docs/features/extensions), it's disabled by default — enable it under **Settings → Extensions**.

:::warning ALPHA
The Contacts extension is currently in ALPHA. Expect rough edges.
:::

## Enabling

1. Go to **Settings → Extensions**
2. Toggle **Contacts** on

A Contacts button is added to the [rail](/docs/features/extensions#the-rail) on the left of the window. The extension has its own settings dialog.

## Manual Sync

To sync contacts on demand, click the footer at the bottom of the Contacts sidebar.

## Known Issues

- Microsoft 365 organization contacts sync has known [issues](https://github.com/hkdb/aerion/issues/278) (reported upstream in Aerion).
