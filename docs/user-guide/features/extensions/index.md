---
sidebar_position: 7
---

# Extensions

Mailven includes a first-party extension system. Extensions are built into the app but **disabled by default** — if you never enable one, you won't see it.

## Disabled by Default — by Design

Disabling by default is a deliberate, permanent choice, not a temporary state:

- Mailven stays a simple email client for those who want email and nothing else.
- A disabled extension loads none of its components, so there's no wasted resource consumption for features you aren't using.
- Resource consumption scales proportionately with what you choose to enable.

Each extension also stores its data in its own local database, separate from the rest of the app.

:::info Requires Mailven 0.3.0+
The extension system and its extensions are available starting in Mailven 0.3.0.
:::

## Shipped Extensions

| Extension | Status | Description |
|-----------|--------|-------------|
| [Contacts](/docs/features/extensions/contacts) | ALPHA | Browse and edit contacts from your accounts |
| [Calendar](/docs/features/extensions/calendar) | ALPHA | View calendar events from your accounts |

:::warning ALPHA
The Contacts and Calendar extensions are currently in **ALPHA**. Expect rough edges.
:::

## Enabling an Extension

1. Go to **Settings → Extensions**
2. Toggle the extension on

## The Rail

With no extensions enabled, Mailven looks exactly like a plain email client. Enable at least one extension and a navigation **rail** appears on the left edge of the window. The rail has a button for Mail plus one button for each enabled extension — click a button to switch between them. Disable every extension and the rail disappears again.

## Google & Microsoft Credentials

Extensions that connect to Google or Microsoft use their own OAuth client credentials, chosen per provider in the extension's settings dialog.

**Google**

- **Custom** — your own client ID and secret. Mailven bundles no Google credentials. See [Custom Google OAuth Credentials](/docs/getting-started/first-account/google-oauth-credentials).
- **Mailven - Google** — reuse the credentials you saved for Gmail in Settings → Accounts.

**Microsoft**

- **Custom** — your own client ID and secret from an Azure AD (Microsoft Entra) app registration that has a `http://localhost/callback` redirect URI for a public client.
- **Mailven - Microsoft** — reuse the credentials you saved for Outlook in Settings → Accounts.

Options other than Custom appear only when the matching credentials exist.
