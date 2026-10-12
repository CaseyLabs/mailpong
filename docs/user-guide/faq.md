---
sidebar_position: 7
---

# Frequently Asked Questions

Common questions about Mailven.

## General

### Is Mailven free?

Yes. Mailven is open source software released under the Apache License 2.0. You can use it for free, modify it, and distribute it.

### Where is my data stored?

All your data is stored locally on your computer. Mailven does not operate any cloud servers. Your emails, credentials, and settings stay on your device.

### Does Mailven sync to the cloud?

No. Mailven connects directly to your email provider's servers via IMAP/SMTP. There is no Mailven cloud service. Your emails are stored both on your email provider's servers and cached locally on your computer.

### What makes Mailven lightweight?

Mailven was designed from the ground up with being "lightweight" in mind. It is built with [Wails](https://wails.io), a lightweight and fast Electron alternative for Go. This means it gets the performance of Go and gives the flexibility to the developer to choose whichever JS/TS framework for frontend development. Wails uses native system components for rendering instead of bundling a full Chromium browser, resulting in significantly lower memory usage and faster startup. For the frontend, [Svelte](https:/svelte.dev) was chosen as the compiler plays a huge role in optimizing TS performance.

### Is Mailven safe to use?

Mailven is open source, so you can decide for yourself by reviewing the [source code](https://github.com/CaseyLabs/mailven). It doesn't collect telemetry, doesn't phone home, and stores all data locally.

Mailven is alpha software and has not been through Google's CASA security assessment. The CASA Tier 2 certification belongs to the upstream Aerion project and does not apply to Mailven.

## Accounts

### Can I use multiple email accounts?

Yes. You can reasonably add as many email accounts as you want from different providers. All accounts appear in the sidebar, and the Unified Inbox shows messages from all accounts combined. Mailven is mostly tested with around 10 accounts added to it.

### Why do I need to re-authorize my Gmail/Outlook account?

OAuth tokens expire periodically for security reasons. When they expire, you'll need to re-authorize to get a new token. This is normal behavior for OAuth-based authentication.

### What's the difference between OAuth and App Password?

- **OAuth** - You sign in through your provider's website (Google, Microsoft) and grant Mailven permission. More secure, easier to set up, but tokens expire.
- **App Password** - You generate a special password in your provider's settings and enter it in Mailven. Works when OAuth isn't available or in restricted environments.

### Can I use work/corporate email accounts?

Yes, if your organization allows third-party email clients:

- **Microsoft 365/Office 365** - Works with OAuth if your admin has approved third-party apps
- **Google Workspace** - Will work seamlessly after Mailven is verified by Google which is hopefully soon.
- **Generic IMAP/SMTP** - Works with any corporate email server that supports IMAP

## Gmail & Outlook

### Why does Google show a security warning?

Mailven has no bundled Google credentials. You sign in through an OAuth client you create yourself, which Google shows as unverified until you publish it; see [Custom Google OAuth Credentials](/docs/getting-started/first-account/google-oauth-credentials). You can also use an app password.

### How do I create a Google App Password?

1. Go to your [Google Account Security](https://myaccount.google.com/security)
2. Enable 2-Step Verification if not already enabled
3. Under 2-Step Verification, click "App passwords"
4. Select "Mail" and give it a name like "Mailven"
5. Copy the 16-character password and use it in Mailven

### Does Mailven read my emails on a server?

No. Mailven runs entirely on your computer. When it accesses your email, it connects directly to your email provider (Gmail, Outlook, etc.). Mailven's developers never see your emails.

### Can I use Mailven with Gmail labels?

Gmail labels appear as folders in Mailven. You can view mail by label and move messages to labels. However, Gmail's label-specific features (like applying multiple labels to one email) are not fully supported in the standard IMAP interface.

## Features

### Can I use Mailven offline?

Partially. You can read emails that have been cached locally, but you cannot send emails or sync new messages without an internet connection. However, you can save drafts locally which will sync to the provider draft folder after once it's back online.

### How do I set Mailven as my default email app?

This depends on your operating system:

**Linux (GNOME):**
```bash
xdg-settings set default-url-scheme-handler mailto io.github.caseylabs.Mailven.desktop
```

**Linux (KDE):**
Set in System Settings → Applications → Default Applications

**macOS:**
Open Mail.app → Preferences → Default email reader → Select Mailven

**Windows:**
Settings → Apps → Default apps → Email → Select Mailven

### Can I import emails from Thunderbird?

Not currently. Mailven syncs emails from your email servers via IMAP. If your emails are on the server, they'll appear in Mailven once you add your account.

### Can I set up email filters/rules?

No. Mailven doesn't currently support client-side email filtering. Set up rules through your email provider's web interface instead.

### Does Mailven support calendar integration?

As of v0.3.0, Mailven is shipped with a calendar extension disabled by default. Users can enable it to integrate Calendar workflow.

### Where can I see my synced contacts?

Mailven by default only syncs and read contacts for auto-completion. However, it's shipped with a disabled by default contacts management extension which can be enabled by users to fully manage their contacts.

## Technical

### What platforms does Mailven support?

- Linux (AppImage, amd64, arm64)
- macOS (Intel and Apple Silicon)
- Windows (amd64, arm64)

### What are the system requirements?

Mailven is lightweight and runs on most modern systems:

- 2 GB RAM (4 GB recommended)
- 100 MB disk space (plus space for cached emails)
- Internet connection for email sync

**Linux:** Requires WebKit2GTK 4.1

### How do I completely uninstall Mailven?

1. Remove the application
2. Delete the configuration directory:
   - Linux: `~/.config/mailven/`
   - macOS: `~/Library/Application Support/Mailven/`
   - Windows: `%APPDATA%\Mailven\`
3. Remove the data directory:
   - Linux: `~/.local/share/mailven/`
   - macOS: `~/Library/Application Support/Mailven/`
   - Windows: `%APPDATA%\Mailven`
4. Remove the cache directory:
   - Linux: `~/.cache/mailven/`
   - macOS: `~/Library/Caches/Mailven/`
   - Windows: `%LOCALAPPDATA%\Mailven\Cache`

## Support

### Where do I report bugs?

Report bugs on [GitHub Issues](https://github.com/CaseyLabs/mailven/issues).

### How can I contribute?

Mailven welcomes contributions! Check the [GitHub repository](https://github.com/CaseyLabs/mailven) for:

- Bug reports and feature requests
- The UI is English-only; see [LANGUAGE.md](https://github.com/CaseyLabs/mailven/blob/main/docs/LANGUAGE.md)
- Pull requests: read [CONTRIBUTING.md](https://github.com/CaseyLabs/mailven/blob/main/CONTRIBUTING.md)

### Is there commercial support?

There's no commercial support program for Mailven, and no guaranteed support (see the [Disclaimer](../../DISCLAIMER.md)).

For community help, use [GitHub Issues](https://github.com/CaseyLabs/mailven/issues).
