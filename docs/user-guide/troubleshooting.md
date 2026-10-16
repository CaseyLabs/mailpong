---
sidebar_position: 8
---

# Troubleshooting

Solutions for common issues with Mailven.

## Connection Issues

### "Connection failed" errors

If you can't connect to your email server:

1. **Check your internet connection** - Ensure you can reach other websites
2. **Verify server settings** - Double-check the server hostname and port
3. **Check firewall/antivirus** - Ensure ports 993 (IMAP) and 465/587 (SMTP) aren't blocked
4. **Try a different network** - Some networks block email ports

### OAuth token expired

For Gmail and Outlook accounts:

1. Go to **Settings > Accounts**
2. Click **Edit** on the affected account
3. Click **Re-authorize**
4. Complete the OAuth flow in your browser

### "Certificate error" or SSL issues

- Ensure you're using the correct security setting (SSL/TLS vs STARTTLS)
- Check that the server hostname matches the certificate
- For self-hosted servers, ensure the certificate is valid. If the server uses a self-signed certificate, Mailven shows its details when you test a mail account or add a CardDAV or CalDAV source; accept it once or always to trust it for that exact host
- With STARTTLS selected, sending fails if the server doesn't offer STARTTLS; Mailven won't fall back to an unencrypted connection. Use SSL/TLS if the server supports it

## Sync Problems

### Messages not appearing

If emails you expect to see aren't showing up:

1. **Check sync period** - By default, Mailven syncs the last 30 days
   - Go to **Settings > Accounts > Edit** to change this
   - For a Gmail account that reads through the Gmail API, and for any IMAP account, **Load older mail** at the end of the list brings in the next page of mail from before that period without changing it, and **Search the whole mailbox** under a search finds older matches. Mail brought in this way stays, and is removed only when it is deleted on the server. Microsoft Graph accounts only offer the period setting.
2. **Check the folder** - Ensure you're looking in the correct folder
3. **Wait for sync** - Large mailboxes can take time to sync initially
4. **Sync Folder** - Right-click the folder in the sidebar and choose **Sync Folder**
5. **Force Re-sync** - If that doesn't help, right-click the folder and choose **Force Re-sync**

### "Sync error" under an account

When an account's sync fails, a red **Sync error** line appears under its name in the sidebar. Click it to open the sync status dialog, which shows the full error and, for sign-in failures, a short explanation. Choose **Edit account** there to re-authorize the account or change its OAuth client. Hover over the line to see the raw error as a tooltip. The error clears after the next successful sync and is not kept when Mailven restarts.

### Slow initial sync

First-time sync of large mailboxes can take several minutes. The sync progress is shown in the status bar. Let it complete before expecting all messages.

### Sync appears stuck

If a sync seems to be stuck or unresponsive, click the sync icon at the top of the message list or the sync button in the bottom left corner and choose **Cancel sync** in the sync status dialog, then click again to restart the sync.

### Sent messages not appearing in Sent folder

Some servers handle sent mail differently:

1. Check that the Sent folder is correctly mapped in account settings
2. Some providers automatically copy sent mail; others require the client to do it

### Folders not appearing

1. Right-click the folder in the sidebar and choose **Force Re-sync**
2. Check if the folder exists in your provider's web interface
3. Some folders may be hidden by your email provider

## Gmail Issues

### "Access denied" or authentication errors

- Your OAuth token may have expired - re-authorize the account
- If using app password, verify it's correct and still active
- Check if your organization has restrictions on third-party apps

### OAuth not working

Mailven has no bundled Google or Microsoft OAuth credentials. Check your own client registration, supported account audience and granted permissions. A Google project's publishing status and verification status are separate; publishing does not itself remove the unverified-app warning. See [Custom Google OAuth Credentials](/docs/getting-started/first-account/google-oauth-credentials) and the [provider requirements](../PROVIDERS.md). For an active API route, use **Reauthorize API access** in the account's Server tab. Google app passwords are an alternative only for eligible IMAP/SMTP accounts; they cannot repair an API grant or bypass Microsoft tenant policy.

The app password method:

1. Generate an app password in Google Account settings
2. Remove and re-add your account using app password authentication

### "This app isn't verified" warning

This should no longer happen as of April 26th, 2026. (See above section for more details)

## Outlook Issues

### Can't connect with regular password

Outlook/Microsoft 365 requires OAuth authentication. Password-based authentication is not supported.

Use **Sign in with Microsoft** when adding your account.

### "Need admin approval" error

Your organization's administrator has restricted third-party applications. Contact your IT department to request access, or check if your organization has specific email client requirements.

## Linux-Specific Issues

### App won't start

Ensure WebKit2GTK is installed:

**Debian/Ubuntu:**
```bash
sudo apt install libwebkit2gtk-4.1-0
```

**Fedora:**
```bash
sudo dnf install webkit2gtk4.1
```

**Arch Linux:**
```bash
sudo pacman -S webkit2gtk-4.1
```

If libwebkit2gtk is already installed and Mailven still won't start, try one of the following commands:

**Flatpak**

```bash
flatpak run --env=WEBKIT_DISABLE_DMABUF_RENDERER=1 io.github.caseylabs.Mailven
```

```bash
flatpak run --env=WEBKIT_DISABLE_COMPOSITING_MODE=1 io.github.caseylabs.Mailven
```

```bash
flatpak run --env=LIBGL_ALWAYS_SOFTWARE=1 io.github.caseylabs.Mailven
```

After figuring out which works, to make the workaround permanent, for example, if DMABUF was the issue:

```bash
flatpak override --user --env=WEBKIT_DISABLE_DMABUF_RENDERER=1 io.github.caseylabs.Mailven
```

**Binary**

```bash
WEBKIT_DISABLE_DMABUF_RENDERER=1 mailven
```

```bash
WEBKIT_DISABLE_COMPOSITING_MODE=1 mailven
```

```bash
LIBGL_ALWAYS_SOFTWARE=1 mailven
```

After figuring out which works, to make the workaround permanent, for example, if DMABUF was the issue:

```bash
echo "export WEBKIT_DISABLE_DMABUF_RENDERER=1" >> ~/.profile  # Or whichever file that makes this environment variable stick
```
Then, logout and log back in.


### Weird rendering issues in Linux

Try the workarounds in the above section.

### Notifications not working

Ensure you have a notification daemon running. On GNOME, notifications work automatically. On other desktops, you may need to install and configure a notification daemon.

Also, if you don't have desktop portal installed on your system or launch Mailven from the terminal, you may want to launch Mailven with the `-dbus-notify` flag so that it sends notifications directly to dbus.

### File picker can't see my files (Flatpak)

In the Flatpak, Mailven opens and saves files through your desktop's file chooser portal, which can reach your home folder even though the sandbox has no direct access to it. If you get a plain GTK dialog that shows no home files instead, `xdg-desktop-portal` and a backend for your desktop (for example `xdg-desktop-portal-gnome`, `-kde` or `-gtk`) aren't running. Install and start them, then restart Mailven.

## macOS-Specific Issues

### "App is damaged" or "can't be opened" error

This happens because Mailven isn't signed with an Apple Developer certificate.

**Solution 1:** Right-click and Open
1. Right-click (or Control-click) on mailven.app
2. Select "Open"
3. Click "Open" in the dialog

**Solution 2:** Remove quarantine attribute
```bash
xattr -cr /Applications/mailven.app
```

## Windows-Specific Issues

### Blank window on startup

This usually indicates a WebView2 issue:

1. Download the latest [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) from Microsoft
2. Install it and restart Mailven

### Installation blocked by SmartScreen

Click "More info" and then "Run anyway" to proceed with installation.

### Antivirus blocking Mailven

Some antivirus software may flag Mailven as suspicious (common for new applications). Add an exception for Mailven in your antivirus settings.

## Contact Sync Issues

### CardDAV not discovering address books

- Verify the URL includes the full path to the DAV endpoint
- Common formats:
  - Nextcloud: `https://server/remote.php/dav`
  - Radicale: `https://server/user/`
  - Baikal: `https://server/dav.php`
- Check your username and password
- Ensure the server supports CardDAV (not just CalDAV)

### Google/Microsoft contacts empty

- Ensure you granted contacts permission during OAuth authorization
- Try removing and re-adding the contact source
- Check that you have contacts in your Google/Microsoft account

### Contacts not appearing in autocomplete

- Verify the contact source shows a recent sync time
- Try manually syncing: **Settings > Contacts > Sync Now**
- Ensure contacts have email addresses (contacts without email won't appear)

### Produce debug logs for further investigator with the maintainer

To produce logs for the maintainer, run one of the following command to launch Mailven:

Windows, Mac, Linux binary:

```bash
mailven --debug

```
Linux Flatpak:

```bash
flatpak run --env=MAILVEN_DEBUG=1 io.github.caseylabs.Mailven

```
Logs print to the terminal you launched from. To save them to a file instead, add `> mailven-debug.log 2>&1` to the end of the command, reproduce the issue, then quit Mailven.

Copy only the portion that relates to your issue and be sure to redact any potentially personally identifiable information as needed.

## Getting More Help

If your issue isn't covered here:

1. Check the [GitHub Issues](https://github.com/CaseyLabs/mailven/issues) for similar reports (if the problem also happens in upstream Aerion, say so)
2. Open a new issue with:
   - Your operating system and version
   - Mailven version
   - Steps to reproduce the problem
   - Any error messages you see
   - Relevant log output (see [Produce debug logs](#produce-debug-logs-for-further-investigator-with-the-maintainer) above)
