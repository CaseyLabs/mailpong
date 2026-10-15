# Privacy Policy

**Last Updated:** October 14, 2026

## Introduction

Mailven ("we," "our," or "the Application") is an open-source desktop email client derived from [Aerion](https://github.com/hkdb/aerion). It is an independent project, not an official Aerion release. This Privacy Policy explains how Mailven handles your information. Because Mailven is a desktop application that runs entirely on your computer, our approach to privacy is fundamentally different from cloud-based services.

**Key Point:** Mailven does not operate any servers that collect, store, or process your data. Mailven stores application data locally and communicates with providers and other services as described below.

## Information We Do Not Collect

Unlike many applications, Mailven:

- **Does not collect** personal information on project-operated servers through the application
- **Does not transmit** your emails, contacts, or credentials to us
- **Does not include** project-operated analytics, telemetry, or tracking. Remote email content can still contact third parties when you allow it, as described below
- **Does not display** advertisements
- **Does not sell** user data or send it to the Mailven project; configured services receive data needed for the features you use

## Information Stored Locally on Your Device

Mailven stores the following information locally on your computer:

### Email Data
- Email messages synchronized from your email accounts
- Folder structure and organization
- Attachments (cached locally)
- Draft messages

### Account Credentials
- Email account configurations (server addresses, ports)
- Authentication tokens (OAuth tokens for Gmail, Outlook)
- Passwords and tokens (stored in the system keyring when available, with encrypted local database storage as a fallback)

### Contacts
- Contact information synchronized from CardDAV, Google, or Microsoft accounts
- Used for address autocomplete, contact management, and optional contact photos in the message list

### Calendar Data
- Events and calendar information synchronized from configured calendar sources when the Calendar extension is enabled
- Local event changes awaiting synchronization

### Application Settings
- Your preferences and settings
- Window size and position
- Theme preferences

Mail, contacts, calendars, and settings are stored locally; credentials use the system keyring or encrypted database fallback. Sending mail and synchronizing contacts or calendars communicate relevant data with the configured services. Sent messages are delivered to their recipients through your mail provider.

## Google API Services

Mailven can use Google APIs for mail, contacts and calendars. OAuth uses a client configured in your build or supplied by you in Settings. If your build has no suitable client, create your own Google Cloud OAuth app and enter its credentials. When you provide your own app, you are responsible for that registration, its consent screen and compliance with Google's terms. A configured client does not give the Mailven project access to your local credentials, tokens or mailbox. Mailven's handling of Google data is subject to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

### Google API Access

Permissions depend on the feature you authorize:

- **IMAP/SMTP** - Gmail access for reading, sending, and managing mail, plus identity and read-only contact permissions in the default profile
- **Gmail API route** - A separate Gmail grant for reading, sending, and managing mail
- **Google People API** - Read-only contact access for synchronization, autocomplete, and contact display
- **Google Calendar** - Calendar access for viewing, creating, and editing events when separately configured

See [provider permissions](PROVIDERS.md#credentials-permissions-and-policy) for mail authorization profiles and the [Calendar guide](user-guide/features/extensions/calendar.md) for the optional extension. The consent screen identifies the permissions requested for your selected feature.

### Limited Use Disclosure

Mailven's use and transfer of information received from Google APIs to any other app will adhere to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

Specifically:
- Mailven uses Google data to provide the mail, contact, and calendar features you authorize
- Mailven communicates with Google to provide these features. When you send or forward mail, the selected message content and attachments are sent to the recipients you choose through your mail provider
- Mailven does not use Google data for advertising
- Mailven does not use Google data to train AI/ML models
- The Mailven project does not receive your Google data through the application

## Microsoft API Services

Mailven can use Microsoft APIs for Outlook/Microsoft 365 integration. OAuth uses a client configured in your build or supplied by you in Settings. If your build has no suitable client, register your own app in Microsoft Entra and enter its credentials. You are responsible for any app registration you supply and for complying with Microsoft's terms.

### Microsoft API Access

Permissions depend on the selected mail route and separately configured contact or calendar features. Protocol OAuth and Microsoft Graph use separate mail grants; see [provider permissions and current compatibility limits](PROVIDERS.md). Requested permissions do not guarantee that an account or route is supported. Microsoft data is cached locally and exchanged with configured services to provide the features you authorize.

## Data Security

Your data security depends on:

1. **Your device security** - Mailven stores data locally, so your device's security measures (disk encryption, password protection, etc.) protect your email data
2. **Connection security** - Mailven supports TLS and STARTTLS, but also allows unencrypted email connections. Protection depends on the server and connection settings you choose
3. **Credential storage** - Mailven uses your operating system's secure keyring when available and falls back to encrypted local database storage when it is unavailable
4. **No project-operated data servers** - Your data is never transmitted to our servers because we don't operate any

## Third-Party Services

Mailven communicates with services needed for the features you configure or enable, including:

- Your email provider (via IMAP/SMTP)
- Google APIs (if you use Gmail)
- Microsoft APIs (if you use Outlook/Microsoft 365)
- CardDAV servers (if you configure contact sync)
- CalDAV and provider calendar APIs (if you configure calendar sync)

Remote images in mail are blocked by default. If you load them or trust a sender, the image hosts may receive requests that reveal your IP address and when content is loaded. Optional online certificate revocation checks contact certificate-status services and may reveal when you view or download signed mail. See [remote image controls](user-guide/features/email-management.md#remote-images) and [security settings](user-guide/features/settings.md). These third-party requests are separate from project-operated telemetry; Mailven does not include advertising or analytics integrations.

## Data Retention and Deletion

Since all data is stored locally on your device:

- **You control retention** - Data remains until you delete it
- **Uninstalling Mailven** - Removes the application but may leave data in your local application data directory
- **Complete removal** - Delete the Mailven data directories (for example `~/.config/mailven`, `~/.local/share/mailven` and `~/.cache/mailven` on Linux; `~/Library/Application Support/Mailven` and `~/Library/Caches/Mailven` on macOS; `%APPDATA%\Mailven` and `%LOCALAPPDATA%\Mailven` on Windows; `~/.var/app/io.github.caseylabs.Mailven/` for Flatpak) and the Mailven entries in your system keyring to remove all locally stored data

## Children's Privacy

Mailven is not directed at children under 13. We do not knowingly collect information from children. Mailven communicates with configured providers and other services as described above; parental supervision should cover both the device and connected accounts.

## Changes to This Privacy Policy

We may update this Privacy Policy from time to time. Changes will be reflected in the "Last Updated" date. Since Mailven is open source, you can review any changes in our public repository.

## Open Source Transparency

Mailven is open-source software. You can review exactly how we handle your data by examining our source code at [https://github.com/CaseyLabs/mailven](https://github.com/CaseyLabs/mailven).

## Contact Us

If you have questions about this Privacy Policy, please open an issue at:

**GitHub Issues:** [https://github.com/CaseyLabs/mailven/issues](https://github.com/CaseyLabs/mailven/issues)

**Security reports:** use GitHub private vulnerability reporting, as described in [SECURITY.md](../SECURITY.md).
