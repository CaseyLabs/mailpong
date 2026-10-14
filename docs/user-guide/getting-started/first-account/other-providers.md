---
sidebar_position: 3
pagination_next: features/overview
---

# Other Email Providers

Mailven supports many email providers beyond Gmail and Outlook. This guide covers setup for pre-configured providers and manual configuration.

These accounts use IMAP/SMTP. Presets supply initial server settings; confirm current credentials, service eligibility and endpoints with your provider. For route support and the distinction between authentication and transport, see [Providers and account requirements](../../../PROVIDERS.md).

## Pre-Configured Providers

These providers have built-in server settings. You just need to provide your credentials.

:::note
Providers marked with 🧪 have not yet been tested. They should work based on their documented settings, but please [report any issues](https://github.com/CaseyLabs/mailven/issues) you encounter.
:::

### Yahoo Mail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **Yahoo** from the provider screen
3. Enter your Yahoo email address
4. Enter your [app password](#generating-app-passwords)
5. Click **Add Account**

### iCloud Mail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **iCloud** from the provider screen
3. Enter your iCloud email address
4. Enter your [app-specific password](https://support.apple.com/en-us/HT204397)
5. Click **Add Account**

### ProtonMail (via Proton Bridge)

ProtonMail requires Proton Bridge for third-party email clients.

1. Install and configure [Proton Bridge](https://proton.me/mail/bridge)
2. <a href="proton-cert" target="_blank">Install your Proton Bridge certificate</a> on your system (Skip this step if you only want Mailven to recognize this cert)
3. Click **+ Add Account** in the sidebar
4. Select **ProtonMail Bridge** from the provider screen
5. Enter your ProtonMail address
6. Enter the Bridge-generated password (not your ProtonMail password)
7. Click **Test Connection** and then click **Accept Always** if you skipped step 2
8. Click **Add Account**

### Fastmail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **Fastmail** from the provider screen
3. Enter your Fastmail email address
4. Enter your [app password](https://www.fastmail.help/hc/en-us/articles/360058752854)
5. Click **Add Account**

### Zoho Mail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **Zoho** from the provider screen
3. Enter your Zoho email address
4. Enter your [app-specific password](https://www.zoho.com/mail/help/adminconsole/two-factor-authentication.html#alink5)
5. Click **Add Account**

### AOL Mail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **AOL** from the provider screen
3. Enter your AOL email address
4. Enter your [app password](#generating-app-passwords)
5. Click **Add Account**

### GMX Mail 🧪

1. Click **+ Add Account** in the sidebar
2. Select **GMX** from the provider screen
3. Enter your GMX email address
4. Enable IMAP in GMX settings, then enter your password
5. Click **Add Account**

### Mail.com 🧪

1. Click **+ Add Account** in the sidebar
2. Select **Mail.com** from the provider screen
3. Enter your Mail.com email address
4. Enable IMAP in Mail.com settings, then enter your password
5. Click **Add Account**

## Generating App Passwords

Many providers require app-specific passwords when 2FA is enabled:

| Provider | How to Generate |
|----------|-----------------|
| Yahoo | [Account Security > Generate app password](https://login.yahoo.com/account/security) |
| iCloud | [Apple ID > App-Specific Passwords](https://appleid.apple.com/account/manage) |
| AOL | [Account Security > Generate app password](https://login.aol.com/account/security) |

## Manual Configuration

For providers not listed above, or for self-hosted email servers:

1. Click **+ Add Account** in the sidebar
2. Select **Generic IMAP/SMTP** from the provider screen
3. Enter your email address
4. Enter your password or app password
5. Configure server settings:

### IMAP Settings (Incoming Mail)

| Setting | Description |
|---------|-------------|
| Server | Your IMAP server hostname (e.g., `mail.example.com`) |
| Port | Usually 993 (SSL) or 143 (STARTTLS) |
| Security | SSL/TLS or STARTTLS |
| Username | Usually your full email address |

### SMTP Settings (Outgoing Mail)

| Setting | Description |
|---------|-------------|
| Server | Your SMTP server hostname (e.g., `smtp.example.com`) |
| Port | Usually 465 (SSL) or 587 (STARTTLS) |
| Security | SSL/TLS or STARTTLS |
| Username | Usually your full email address |

### Common Server Settings

| Provider | IMAP Server | SMTP Server |
|----------|-------------|-------------|
| Generic | imap.domain.com:993 | smtp.domain.com:465 |

Contact your email provider or system administrator for the correct server settings.

## Testing Your Connection

After entering your settings, Mailven will attempt to connect to the server. If the connection fails:

1. Double-check the server hostname and port
2. Verify your username and password
3. Ensure the security setting matches your server's requirements
4. Check if your firewall is blocking the connection

## Troubleshooting

### "Connection Refused" Error

- Verify the server hostname is correct
- Check if the port is correct (993 for IMAP SSL, 465 or 587 for SMTP)
- Ensure your firewall allows outbound connections on these ports

### "Authentication Failed" Error

- Confirm your username (usually your full email address)
- Ensure you're using an app password if 2FA is enabled
- Check if your provider requires enabling IMAP access in settings

### "Certificate Error"

- Ensure the security setting matches your server
- For self-signed certificates, you may need to add an exception
- An accepted certificate is trusted only for the incoming server it was accepted for and that account's outgoing server. Other servers presenting the same certificate are still rejected
- Contact your server administrator for the correct SSL/TLS settings
