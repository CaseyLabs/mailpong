---
sidebar_position: 4
---

# Contacts

MailPong can sync contacts from multiple sources to provide autocomplete when composing emails.

## Contact Sources

MailPong supports three types of contact sources:

### CardDAV

Connect to any CardDAV-compatible server:

**Self-Hosted / Third-Party:**
- Nextcloud
- Radicale
- Baikal
- ownCloud
- Synology Contacts

**Email Provider CardDAV:**
| Server | URL | Verified | By
|--------|------------|-------------|--------------|
| Nextcloud | `https://server/remote.php/dav/addressbooks/users/<USERNAME>/contacts/` | YES | [@hkdb](https://github.com/hkdb) | 
| Radicale | `https://server/user/` | NO | N/A |
| Baikal | `https://server/dav.php` | NO | N/A |
| Fastmail | `https://carddav.fastmail.com` | NO | N/A |
| iCloud | `https://contacts.icloud.com` | NO | N/A |
| Zoho | `https://contacts.zoho.com/carddav` | NO | N/A |
| AOL | `https://carddav.aol.com` | NO | N/A |
| GMX | `https://carddav.gmx.com/CardDavProxy/carddav/current-user-principal-uri/` | YES | [@Olivetti](https://github.com/Olivetti) |
| Mail.com | `https://carddav.mail.com` | NO | N/A |

Consult your server's documentation for the correct URL.

:::note
ProtonMail does not support CardDAV. Yahoo's CardDAV is deprecated and unreliable.
:::

### Google Contacts

Sync contacts from your Google account. This can be:

- The same Google account you use for Gmail
- A different Google account dedicated to contacts

### Microsoft Contacts

Sync contacts from your Microsoft/Outlook account. This can be:

- The same account you use for Outlook mail
- A different Microsoft account

## Adding a Contact Source

1. Go to **Settings > Contacts**
2. Click **Add Contact Source**
3. Select the source type:

### CardDAV Setup

1. Select **CardDAV**
2. Enter your CardDAV server URL
   - Example: `https://nextcloud.example.com/remote.php/dav`
3. Enter your username and password
4. Click **Add**

MailPong will discover available address books on the server. If the server presents a certificate MailPong doesn't trust (for example, a self-signed one), MailPong shows the certificate details; accept it to trust it for that host and continue.

### Google Contacts Setup

1. Select **Google Contacts**
2. If you have a Gmail account configured:
   - Select it from the dropdown to use the same authorization
3. Or sign in with a different Google account
4. Click **Add**

### Microsoft Contacts Setup

1. Select **Microsoft Contacts**
2. If you have an Outlook account configured:
   - Select it from the dropdown to use the same authorization
3. Or sign in with a different Microsoft account
4. Click **Add**

## Sync Settings

### Sync Interval

Configure how often contacts sync:

1. Go to **Settings > Contacts**
2. Click on a contact source
3. Set the sync interval:
   - Manual only
   - Every 15 minutes
   - Every hour
   - Every 6 hours
   - Daily

### Manual Sync

To sync contacts immediately:

1. Go to **Settings > Contacts**
2. Click the **Sync Now** button next to the contact source

### Sync Status

The last sync time is displayed for each contact source. If sync fails, an error indicator will appear.

## Using Contacts

### Autocomplete

When composing an email, type in the To, Cc, or Bcc fields:

- Matching contacts appear as suggestions
- Match is based on name and email address
- Select a suggestion with arrow keys and Enter, or click

### Contact Display

Autocomplete suggestions show:

- Contact name
- Email address
- Source indicator (which address book the contact is from)

## Multiple Address Books

If you have multiple address books (e.g., Personal and Work on the same CardDAV server), MailPong will:

- Discover all available address books
- Sync contacts from all of them
- Show contacts from all books in autocomplete

## Troubleshooting

### CardDAV Not Connecting

- Verify the server URL is correct
- Ensure the URL includes the full path to the DAV endpoint
- Check your username and password
- Verify the server supports CardDAV (not just CalDAV)

### No Address Books Found

Some CardDAV servers require a specific URL format:

| Server | URL | Verified | By
|--------|------------|-------------|--------------|
| Nextcloud | `https://server/remote.php/dav` | YES | [@hkdb](https://github.com/hkdb) | 
| Radicale | `https://server/user/` | NO | N/A |
| Baikal | `https://server/dav.php` | NO | N/A |
| Fastmail | `https://carddav.fastmail.com` | NO | N/A |
| iCloud | `https://contacts.icloud.com` | NO | N/A |
| Zoho | `https://contacts.zoho.com/carddav` | NO | N/A |
| AOL | `https://carddav.aol.com` | NO | N/A |
| GMX | `https://carddav.gmx.com/CardDavProxy/carddav/current-user-principal-uri/` | YES | [@Olivetti](https://github.com/Olivetti) |
| Mail.com | `https://carddav.mail.com` | NO | N/A |

Consult your server's documentation for the correct URL.

### Google/Microsoft Contacts Empty

If no contacts appear after syncing:

1. Ensure you've granted contacts permission during authorization
2. Try re-authorizing the account
3. Check that you have contacts in your Google/Microsoft account

### Contacts Not Appearing in Autocomplete

- Ensure the contact source has synced recently
- Check that the contact has an email address
- Try typing more characters to narrow the search
