---
sidebar_position: 6
---

# Settings

Configure Mailven to work the way you prefer.

Access settings by clicking the **gear icon** at the bottom of the far-left rail, below Mail and any enabled extensions (Contacts, Calendar).

## General Tab
---

### Title Bar

- **Mailven** (Default) - Built-in title bar matching Mailven's themes
- **Native** (Requires Restart) - Use the OS native title bar and window decorations
- **Disable** - Some users that are running tiling window managers or simply prefer a more minimalistic look and feel, the title bar can be completely disabled.

### Language

The Mailven interface is English only, so there is no language picker. Email content can be in any language, charset or script. For spellchecking in the composer, choose **English (US)**, **English (UK)** or **English (Canada)**.

### Theme

Choose your visual theme:

- **System** - Follows your operating system's light/dark preference
- **Light** - Always use the light theme (Nord Snow Storm colors)
- **Dark** (Default) - Always use the dark theme (Nord Polar Night colors)

### Dark mail content

Apply a dark filter to email message content when Mailven is in dark mode.

There are two schools of preferences for dark mode. One is to have dark UI but light e-mail content and the other is to have everything in dark mode. This setting provides a way for the user to choose which school they belong to.

When this toggle is turned on, each message will render in dark mode and have a ☀️ icon on the right side of the header which you can click on to temporarily render the email in light mode in case the dark mode filter breaks the e-mail rendering.

### Dark composer body

Use a dark background for the composer message body when Mailven is in dark mode.

By default, the e-mail content inside the conversation viewer will always have a white background. However, if you prefer having a dark background when using a dark theme, you can turn this setting on.

### Always show checkbox next to message

Reserve a fixed checkbox column in the message list instead of revealing it on hover or swipe.

By default, inside the message list column, the area where the checkbox on the left of the message is located is covered by the message and on mouseover, the message will shift to the right and show that area. In mobile layout, swiping to the right will auto select the message which solves the problem of not being able to do mouse over on a mobile device. If you prefer the original design where the checkbox area is always shown with the checkboxes always visiable in mobile layout, toggle this setting on.

### Hide message preview

Hide the line of the latest message below the subject in the chat list and the classic message list.

Enabled by default, so each chat in the list shows only the people and subject. Search results always show the matching preview text, regardless of this setting, so you can see why a message matched.

### Show colored circles in message list

Show colored circles with sender initials in the message list.

Enabled by default.

This is where you can disable the colored contact circles on the message list of Mailven to get a more minimalistic look especially if you don't feel like you get value from the color contact circles.

### Show colored circles in conversation viewer

Show colored circles with sender initials in the conversation viewer.

Enabled by default.

This is where you can disable the colored contact circles in the conversation viewer if you think the contact circles are unneccesary noise.

### Show contact photos in message list

Show the contact's profile picture (falling back to a colored circle) in the message list. Requires "Show colored circles in message list".

### Optional accent for unread messages

When enabled, this puts an additional accent marker based on your themes secondary color to each unread message for better visual grepping of what's read and unread if enabled. This feature is disabled by default. By default (when disabled), the only difference between read and unread is bold versus normal text.

There are currently 3 different accent markers that you can choose from:

- Dot
- Glow Dot
- Bar

### Message Density

Control how compact the message list appears:

- **Micro** - Minimal spacing, maximum messages visible
- **Compact** - Reduced spacing
- **Standard** - Default balanced spacing
- **Large** - More spacing for easier reading

### Auto-Mark as Read

Set how long a message must be viewed before marking it as read:

- **Immediately** - Mark read as soon as opened
- **After 1 second**
- **After 2 seconds**
- **After 5 seconds**
- **Never** - Only mark read manually

### Background

**Run in background** - Enable this if you want Mailven to not exit but run in the background when you close the Mailven window. This setting is also a pre-requisite for the next toggle to be enabled.
**Starts hidden on launch** - Enable this if you want Mailven to launch in the background on first launch without showing the window.

### Startup

**Autostart on login** - Enable this if you want Mailven to automatically launch when you login to your system.

### Tray Icon

Tray icon has not been implemented yet but is in the roadmap down the road.

## Composer Tab
---

### Default Compose Mode

Choose whether composing opens in the main window or a separate window when you press the **Compose** button or **Ctrl+N**.

Options:

- **In window** (Default)
- **Detached window**

### External mailto Links

Choose how `mailto` links from other apps are handled.

Options:

- **In window** (Default)
- **Detached window**

### Default Message Format

New messages will use this format by default in the composer. You can still switch per message.

Options:

- **Rich text** (Default)
- **Plain text**

### Read Receipts

Configure how Mailven handles read receipt requests:

- **Never send** - Ignore all read receipt requests
- **Ask** - Prompt when a sender requests a read receipt
- **Always send** - Automatically send read receipts

### Spellcheck

This is the configuration for the spellcheck functionality. Currently, spellcheck only supports languages that use Latin alphabet. Spellcheck with the English language is turned on by default. You can turn this feature completely off by toggling it off. If you need spellcheck support for languages other than English, toggle the language you need on under the **Dictionary** sub-section. Here are the currently supported languages:

- English (US)
- English (UK)
- Čeština
- Deutsch
- Français
- Italiano
- Norsk Bokmål
- Nederlands

#### Added words

This is where you can view and edit the custom words that are added to your dictionary as legitimate words.

### Default Reply-To

This is where to set your default **reply-to** address for each account which will be used by every e-mail you send.

### Default CC

This is where to set your default **CC** address for each account which will be used by every e-mail you send.

### Default BCC

This is where to set your default **BCC** address for each account which will be used by every e-mail you send.

## Chat Tab
---

Options for the chat-style mail list, thread view, and reply box.

### Send Key

**Enter** (default) sends from the reply box, with Shift+Enter for a new line. **Ctrl+Enter** sends instead, and Enter adds a new line.

### Message Style

**Bubbles** (default) shows messages in colored speech bubbles. **Flat** shows a plain transcript: no bubbles or avatars, a name and time above each message, and a thin accent line beside your own messages, which stay on the left with everyone else's. Email cards, full bodies, the "N earlier messages" lines and the chat list work the same in both styles.

### Reading Width

How wide message bubbles can grow in an open chat: **Narrow**, **Standard** (default) or **Wide**. Messages always use the whole conversation pane, with other people's on the left and yours on the right. **Wide** also widens email cards and full email bodies, which stay wide enough for a 600 px HTML email at every setting.

### Include Quoted History

On by default. Chat replies quote the replied-to message below your text, so recipients using traditional email clients keep the context. The quote stays hidden in the chat view.

### After Archive, Snooze, Delete, or Move

Which chat opens when the current one leaves the list (dragging a chat onto a sidebar folder does not open another): the next (older) chat, which is the default, or the previous (newer) chat.

### Combine Messages from Each Sender

On by default. All the threads a sender starts show as one chat; threads started from your own addresses stay separate. When off, threads show separately unless you combined that sender. Choices made from a chat's menu (**Combine messages from this sender** or **Show as separate threads**) apply whichever way this is set. When a thread joins a sender's chat (after an upgrade or turning this on), its pin, snooze and unsent chat reply move to that chat; when a sender's chat goes back to separate threads (turning this off or splitting one sender), the chat's pin, snooze and reply link move to the newest inbox thread that sender started (or are dropped if there is none), and the draft stays in Drafts. A pin, snooze or reply link already on that thread is kept.

### Show Low Priority Group

On by default. The All filter lists newsletters and bulk mail in a Low priority group, collapsed until you expand it. When off, they appear only under the Low priority filter, and a pinned low-priority chat appears only under Low priority too.

### Notify Only for Priority Mail

On by default. New-mail notifications skip newsletters and bulk mail, and senders you moved to Low priority. Snoozed chats always notify when they return.

## Images Tab
---

### Always Load Remote Images

If toggled on, image blocking for all emails will be disabled. Remote images will load automatically instead of requiring the users to manually load images or setting always load rules based on domain or address.

### Addresses

This is a list of e-mail addresses that have been marked as **Always Load** images from. You can remove any address that you no longer want to auto-load images on emails for.

### Domains

This is a list of domains that have been marked as **Always Load** images from. You can remove any domain that you no longer want to auto-load images on emails for.


## Accounts Tab
---

Manage your email accounts.

### Add Account

Click **Add Account** to add a new email account. See [First Account Setup](/docs/getting-started/first-account) for details.

### Edit Account

Click **Edit** on any account to modify:

#### General Tab

- **Account Color** - Color used in unified inbox
- **Account Name** - Display name for this account
- **Default Display Name** - The name people see the e-mail is from when you send e-mails
- **Email Address** - Your email address (read-only for OAuth accounts)
- **Username** - Username of your account
- **Credentials**:
    - **Password** - Password of your account
    - **Re-authorize** - Re-authorize oauth account 
- **Sync Period** - How far back to sync (30, 90, 180 days, or all)

#### Identity Tab

Manage sending identities for this account:

- **Name** - Display name for outgoing mail
- **Email** - Sending address
- **Signature** - Rich text signature appended to messages

You can have multiple identities per account (e.g., for aliases).

Each identity can also have a **Reply-To** address. Replies to your messages go there instead of the account default. Leave it empty to use the default.

For Gmail accounts, a **Gmail send-as addresses** section lists the addresses from Gmail's *Send mail as* list. Gmail only sends as an address it has verified; for any other it rewrites the sender to the account address. To set one up:

1. In Gmail, go to **Settings → Accounts → Send mail as → Add another email address** and verify it.
2. In Mailven, press **Refresh** in the send-as section, then **Add** next to the address (optionally importing its Gmail signature).

Identities show a **Verified**, **Pending**, or **Not in Gmail** badge. Mailven refreshes the list once a day in the background.

If you are using an Microsoft 365 account, there will be an additional section in this tab for you to add **shared mailboxes** here. After completely the flow, you will see the shared mailbox as a separate account on the side bar.

#### Server Tab

For non-OAuth accounts:

- **Incoming Mail** - IMAP server, port, and security
- **Outgoing Mail** - SMTP server, port, and security
- **Check for New Mail** - How often to check for new messages outside of IDLE
- **Request Read Receipts** - When to request read receipts for outgoing messages
- **Server Mapping** - Map special folders if not auto-detected:
    - Sent folder
    - Drafts folder
    - Trash folder
    - Archive folder (if none is mapped or detected, as on Gmail, Archive moves messages out of the Inbox to All Mail)
    - Spam folder
- **Folder sync subscriptions** - Configure additional folders to be synced automatically
    - Manage folder sync - This is disabled by default. Enable it to see the UI to configure which folders to auto-sync
        - Sync all folders - Disabled by default. Enable it to sync all imap folders
        - Or just check only the folders you want to auto-sync
- **Trusted Certificates** - Manually trusted TLS certificates for all mail, contacts, and calendar servers, including hosts reached during discovery. Each entry shows the host; removing an entry revokes trust for that host only.
- **Mail API route** - Gmail and Microsoft 365 (work/school) accounts can optionally switch from IMAP/SMTP to the provider's mail API. *Authorize API access* grants a separate permission, then *Review migration* shows what will happen and any blockers. Before anything changes a snapshot of the local database is saved; existing messages are matched and kept, and drafts and queued mail are preserved. If an active API grant is rejected, sync and queued actions pause; use *Reauthorize API access* in the same settings section to restore access without switching routes. *Return to IMAP / SMTP* switches the account back to its protocol route; nothing is replayed to the provider and unfinished API actions are listed. The result says whether an IMAP sync request was made; that request does not confirm that sync started or finished. If no request was made, use Sync manually. There is no automatic fallback between routes, and no route promises delivery or inbox placement. Hidden (Bcc) recipient privacy and provider message-size limits on the API routes are not yet verified against live providers. A linked shared mailbox can authorize API access under its parent account's sign-in, but it cannot be activated or migrated until its live Graph checks are recorded. Returning to IMAP/SMTP does not undo provider changes or restore the snapshot. To go back to the saved snapshot, use *Restore database to snapshot…* in the same section: it is an offline, whole-database restore (all accounts, not just this one) that discards local changes made since the snapshot and leaves provider-side changes in place. You must acknowledge a consent dialog, Mailven saves a backup of the current database and closes, and the restore happens the next time you open it, after which a banner offers *Undo restore* from that backup. See [Snapshot restoration](../../MAIL_APIS.md#snapshot-restoration). See [provider requirements and route limits](../../PROVIDERS.md#switching-routes-and-current-limits).

#### Security Tab

- **PGP** - PGP/GPG signing and encryption settings of your account
    - **Import Secret Keys** - Import your PGP secret key
    - **Key Servers** - A list of key servers to use. There are 3 defaults already. You can also add your own here
    - **Recipient Keys**
        - **Import** - Import public keys of your contacts 
        - **Search** - Search for public keys. Starts with WKD and then searches key servers 
        - **List** - There's a list below the above 2 buttons that shows the current stored public keys
- **S/MIME** - S/MIME cert based signing and encryption settings for your account
    - **Online certificate revocation checks** - Off by default and applies to all accounts. When enabled, verifies current OCSP responses or complete CRLs for the signer and intermediate certificates. Requests may reveal when you view or download signed mail. Good/revoked results are cached for at most one hour and never beyond their next update. Unchecked, unavailable, stale, or unsupported revocation status is shown as a warning; a revoked certificate is shown as an error. The preference applies on the next verification when viewing or downloading a message; background sync extracts message content without online checks. Certificates already reported revoked remain blocked for automatic encryption until a successful online verification clears that result. Checks use direct HTTP(S) connections to public endpoints; proxy-only networks can report unavailable status.
    - **Import .p12** - Import your .p12 certificate. Imported certificate will appear below
    - **Import** - Import recipient certificates here. Imported or auto-collected public certs will appear below. Auto-collected certs are used for encryption only when they chain to a trusted certificate authority; import a self-signed or otherwise untrusted cert to accept it for encryption

### Remove Account

Click **Trash Icon** to delete an account. This removes:

- The account configuration
- Locally cached emails from this account

Your emails on the server are not affected.

## Contacts Tab
---

Manage contact sources for autocomplete.

### Add Contact Source

Add a new source for contact sync:

- **CardDAV** - Connect to a CardDAV server
- **Google Contacts** - Sync from Google
- **Microsoft Contacts** - Sync from Microsoft

See [Contacts](/docs/features/contacts) for detailed setup.

### Edit Contact Source

Click on a contact source to:

- Change sync interval
- Update credentials
- View sync status

### Sync

Click **Sync** to immediately fetch contacts from a source.

### Force Re-Sync

Click **Force re-sync** to immediately re-fetch all contacts from a source.

### Remove Contact Source

Click **Delete** to stop syncing from a contact source. Previously synced contacts remain available until the next restart.

### oAuth Credentials

Right below all the accounts, you will see a section labled **oAuth Credentials**. This is where you can change the Client ID and Secret of what Microsoft and Google uses for Mailven in runtime.

## Extensions
---

This is a 1st party extension system. It doesn't allow for community or 3rd party development of extensions yet. Whether or not the extension system will be opened up to the community or 3rd parties will depend on future demands. Currently, the extension system is meant to make it possible for Mailven to stay a lightweight e-mail only client but at the same time be able to provide features that only specific power users want such as contacts management, calendar management and more in the future.

In this tab, you can enable **Contacts** and/or **Calendar** extensions. Each extension has its own settings dialog. To learn more, see the [Extensions](/docs/features/extensions) section.

## About Tab
---

View information about Mailven:

- **Version** - Current Mailven version
- **GitHub** - Link to source code and issues
- **Privacy Policy** - How Mailven handles your data
- **Terms of Use** - Usage terms

## Applying Changes
---

Most settings take effect immediately. Some changes (like adding accounts or contact sources) may require a sync before you see results.

## Configuration Location
---

Mailven stores configuration in your system's standard application data location:

| Platform | Location |
|----------|----------|
| Linux | `~/.config/mailven/` or `$XDG_CONFIG_HOME/mailven/` |
| macOS | `~/Library/Application Support/Mailven/` |
| Windows | `%APPDATA%\Mailven\` |

:::warning
Modifying configuration files directly is not recommended. Use the Settings UI instead.
:::

## Notifications
---

By default on Linux, Mailven uses `org.freedesktop.portal.Desktop` but if you launch Mailven from the terminal, notifications will not work. In those cases, you will want to use the `--dbus-notify` flag to launch Mailven.
