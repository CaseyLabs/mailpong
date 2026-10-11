# Privacy Policy

**Last Updated:** October 11, 2026

<!-- Draft for legal review (PLAN.md flags L2/L3); remove this comment once signed off. -->

## Introduction

MailPong ("we," "our," or "the Application") is an open-source desktop email client derived from [Aerion](https://github.com/hkdb/aerion). It is an independent project, not an official Aerion release. This Privacy Policy explains how MailPong handles your information. Because MailPong is a desktop application that runs entirely on your computer, our approach to privacy is fundamentally different from cloud-based services.

**Key Point:** MailPong does not operate any servers that collect, store, or process your data. All your information stays on your device.

## Information We Do Not Collect

Unlike many applications, MailPong:

- **Does not collect** any personal information on external servers
- **Does not transmit** your emails, contacts, or credentials to us
- **Does not use** analytics, telemetry, or tracking of any kind
- **Does not display** advertisements
- **Does not sell or share** any user data (we don't have access to it)

## Information Stored Locally on Your Device

MailPong stores the following information locally on your computer:

### Email Data
- Email messages synchronized from your email accounts
- Folder structure and organization
- Attachments (cached locally)
- Draft messages

### Account Credentials
- Email account configurations (server addresses, ports)
- Authentication tokens (OAuth tokens for Gmail, Outlook)
- App-specific passwords (stored in your system's secure keyring)

### Contacts
- Contact information synchronized from CardDAV, Google, or Microsoft accounts
- Used solely for email address autocomplete

### Application Settings
- Your preferences and settings
- Window size and position
- Theme preferences

**All of this data is stored in your local application data directory and never leaves your device except when communicating directly with your email providers.**

## Google API Services

MailPong can use Google APIs for Gmail integration. MailPong ships no Google OAuth client: to use Gmail with OAuth you create your own Google Cloud OAuth app and enter its credentials in Settings, so you (not the MailPong project) are the operator of that app and are responsible for its consent screen and Google's terms. The project does not see your credentials, tokens or data. When you use your own Google OAuth app, MailPong handles information received from Google APIs in line with the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

### Google API Scopes Used

- **IMAP/SMTP Access** - To read, send, and manage your emails via standard IMAP/SMTP protocols (OAuth2 authenticated)
- **Google People API** - To access your contacts for email autocomplete

### Limited Use Disclosure

MailPong's use and transfer of information received from Google APIs to any other app will adhere to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

Specifically:
- MailPong only uses Google data to provide email functionality to you
- MailPong does not transfer Google data to third parties
- MailPong does not use Google data for advertising
- MailPong does not use Google data to train AI/ML models
- The project has no access to Google data: it stays on your device

## Microsoft API Services

MailPong can use Microsoft APIs for Outlook/Microsoft 365 integration. As with Google, MailPong ships no Microsoft OAuth client: you register your own app in Microsoft Entra and enter its credentials in Settings. You are responsible for that app registration and Microsoft's terms.

### Microsoft API Scopes Used

- **Mail.ReadWrite** - To read, send, and manage your emails
- **Contacts.Read** - To access your contacts for email autocomplete
- **User.Read** - To access your basic profile information
- **offline_access** - To maintain your session

All Microsoft data is stored locally and used solely for email functionality.

## Data Security

Your data security depends on:

1. **Your device security** - MailPong stores data locally, so your device's security measures (disk encryption, password protection, etc.) protect your email data
2. **Secure connections** - All connections to email servers use TLS/SSL encryption
3. **System keyring** - Sensitive credentials are stored in your operating system's secure credential storage
4. **No cloud exposure** - Your data is never transmitted to our servers because we don't operate any

## Third-Party Services

MailPong only connects to services you explicitly configure:

- Your email provider (via IMAP/SMTP)
- Google APIs (if you use Gmail)
- Microsoft APIs (if you use Outlook/Microsoft 365)
- CardDAV servers (if you configure contact sync)

We do not integrate with any advertising networks, analytics services, or other third-party data collectors.

## Data Retention and Deletion

Since all data is stored locally on your device:

- **You control retention** - Data remains until you delete it
- **Uninstalling MailPong** - Removes the application but may leave data in your local application data directory
- **Complete removal** - Delete the MailPong data directories (for example `~/.config/mailpong`, `~/.local/share/mailpong` and `~/.cache/mailpong` on Linux; `~/Library/Application Support/MailPong` and `~/Library/Caches/MailPong` on macOS; `%APPDATA%\MailPong` and `%LOCALAPPDATA%\MailPong` on Windows; `~/.var/app/io.github.caseylabs.MailPong/` for Flatpak) and the MailPong entries in your system keyring to remove all locally stored data

## Children's Privacy

MailPong is not directed at children under 13. We do not knowingly collect information from children. Since MailPong stores all data locally and does not transmit data to external servers, parental supervision of the device itself is the appropriate safeguard.

## Changes to This Privacy Policy

We may update this Privacy Policy from time to time. Changes will be reflected in the "Last Updated" date. Since MailPong is open source, you can review any changes in our public repository.

## Open Source Transparency

MailPong is open-source software. You can review exactly how we handle your data by examining our source code at [https://github.com/caseylabs/mailpong](https://github.com/caseylabs/mailpong).

## Contact Us

If you have questions about this Privacy Policy, please open an issue at:

**GitHub Issues:** [https://github.com/caseylabs/mailpong/issues](https://github.com/caseylabs/mailpong/issues)

**Security reports:** use GitHub private vulnerability reporting, as described in [SECURITY.md](../SECURITY.md).
