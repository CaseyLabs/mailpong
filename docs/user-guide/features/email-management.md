---
sidebar_position: 2
---

# Email Management

Aerion provides essential tools to manage your email efficiently.

## Chat Mail

The mail section shows email as chats. Each chat is one email thread, listed by the people in it with their avatar and the subject; a long subject wraps onto a second line. Turn off **Hide message preview** in Settings to also show the latest line.

- **List filters** - All, Unread, Low priority, and Snoozed. Under All, pinned chats come first, then Chats, then a collapsed **Low priority** group for newsletters and bulk mail. The group's header offers **Archive all** (undoable) and **Mark all read**.
- **Thread view** - Messages appear as chat bubbles, with your replies on the right. Quoted history and signatures are hidden behind a faint "⋯" marker at the end of the text. Newsletters and other rich mail show as cards that expand into the full message; encrypted or signed mail, and any message after **Show original**, renders in full.
- **Reply box** - A reply box stays docked at the bottom of the open chat. Typed text is saved as a draft. A **Reply** / **Reply All** toggle after the text field picks who gets the reply (highlighted while Reply All is on), and **Send** comes last, filling in once there is something to send. The empty field names the recipients, for example "Reply to Linda, Sean...". A quote mark at the end of a message shows or hides its quoted text.
- **Top bar** - **Compose**, **Archive**, **Delete**, and **Spam** sit on the left, followed by icon buttons for Reply, Reply All, Forward, and **Open in full composer** (moves the reply box's draft to the full composer); Snooze, Pin, Mark read/unread, and the ⋯ menu sit on the right. The chat's people and subject are centered on a bar below the buttons, separated from them by a line. On a narrow pane the labelled buttons show icons only, and Spam, Forward, Pin, and Mark read/unread move into the ⋯ menu. **Delete** always acts on the whole chat.
- **Triage** - **Archive** archives the chat and opens the next one. **Pin** keeps a chat at the top, **Snooze** hides it until a chosen time (or until new mail arrives), and **Mark unread** brings it back later. Each chat's ⋯ menu and row context menu can move its sender to Low priority or back to Priority.
- **Sender chats** - By default (see [Settings](settings.md#combine-messages-from-each-sender)), all the threads a sender starts show as a single chat; threads started from your own addresses stay separate. The row shows the sender and the latest subject. Inside the chat, a subject label marks where the subject changes from the message before it, and replies go to the latest message you received. Choose **Show as separate threads** in a chat's ⋯ menu or row context menu to split one sender's threads, and **Combine messages from this sender** to combine them again. Both actions can be undone. A thread you started stays separate even when its sender's messages are combined. Search across all accounts still lists the sender's threads separately.
- **Low priority** - Mail with mailing-list or bulk headers is low priority. A thread counts as low only while every message in it is, so a person's reply lifts it into Chats. By default, low-priority mail does not raise notifications.

Pin, snooze, and sender priority are stored locally on this computer and are not synced to the server or other devices. Pin is separate from the server star.

See [Keyboard Shortcuts](keyboard-shortcuts.md) for chat triage keys and [Settings](settings.md#chat-tab) for chat options.

## Unified Inbox

The Unified Inbox shows all incoming mail from all your accounts in one view.

- Each account is assigned a color for easy identification
- Messages are sorted by date across all accounts
- Click on an individual account's inbox to view only that account

### Account Colors

Set account colors in **Settings > Accounts > Edit Account**. The color appears as a subtle indicator on each message in the unified inbox.

## Folder Navigation

The sidebar shows all folders for each account:

- **Unified Inbox** - All inboxes combined
- **Individual Account Inboxes** - Nested under Unified Inbox
- **Account Folders** - Each account's folder tree

### Navigating Folders

- Click a folder to view its contents
- Click an account header to expand/collapse its folders
- Use `Alt+Up/Down` or `Alt+J/K` to navigate folders with keyboard

## Conversation View

Related messages are grouped into conversations (threads):

- Open a conversation to see all messages in the thread
- Messages are displayed chronologically
- Reply context is preserved for easier reading

Delete single message in a conversation:

- Right click on the header of a message and select delete from the context menu
- Focus (Alt+L or ALt+Right) on the conversation viewer, press tab to navigate to the message and press delete.

Focus mode:

Any thread or specific message can be toggled into focus mode which stretches the thread or message across the full size of the window. This is particularly useful if you would like to share your screen in an online meeting without exposing your folders and message list. It's of course also good for those who just want to focus on an e-mail and not be distracted by other elements of the app. While in focus mode, reply, reply all, and forward actions will launch a detached composer instead of the default composer. To reply, reply all, or forward with the default in window composer, exit focus mode before you toggle the action.

To toggle a thread into focus mode, click the focus mode icon in the top right corner of the conversation viewer next to the print icon.

To toggle a message into focus mode, click the focus mode icon in the top right corner of the message's header.

You can also toggle a thread into focus mode by just pressing **f** and toggle a message you are focused on into focus mode by pressing **Shift+f**.


## Message Actions

### Single Message Actions

When viewing a message or with a message selected:

| Action | Method |
|--------|--------|
| Reply | Click Reply button or `Ctrl+R` |
| Reply All | Click Reply All button or `Ctrl+Shift+R` |
| Forward | Click Forward button or `Ctrl+F` |
| Star/Unstar | Click star icon or `S` |
| Mark Read | Right-click menu or `Ctrl+U` |
| Mark Unread | Right-click menu or `Ctrl+Shift+U` |
| Archive | Click Archive button or `Ctrl+K` |
| Move to Trash | Click Trash button or `Delete` |
| Mark as Spam | Click Spam button or `Ctrl+J` |
| Copy to Folder | Right-click > Copy to or Right Alt > Copy to or `Alt+C` |
| Move to Folder | Right-click > Move to or Right Alt > Move to or `Alt+M` |
| Thread Focus Mode | `F` |
| Message Focus Mode | `Shift+F` |

### Copy to Folder and Move to Folder

Toggling either copy or move to folder will bring up a dialog with a scrollable list of folders to choose as destination. There's also a search bar on top of the folder list so that users can use it to find the target folder in the case that the account has a substantial amount of folders.

### Bulk Actions

The chat list has no checkboxes; use the Low priority group's bulk actions or triage chats one by one. In list views that offer checkboxes, select multiple messages to apply actions in bulk:

1. Check the checkbox on each message, or
2. Use `Space` to toggle the checkbox on the focused message, or
3. Use `Shift+Up/Down` or `Shift+J/K` to select while navigating

Then apply any action - it will apply to all selected messages.

**Tip:** Press `Escape` once to clear all checkboxes.

## Swipe Gestures

Aerion has some basic swipe gestures:

- Right: select message
- Left: delete message

For laptop trackpads, swipe with 2 fingers.
For mobile layout, swipe with 1 finger.

## Search

Find emails quickly with the search bar:

1. Click the search bar or press `Ctrl+S`
2. Type your search query
3. Press `Enter` to switch focus to results

Search looks through:
- Subject lines
- Sender and recipient addresses
- Message body content

Search results are highlighted to show matching terms.

### Search Scope

- Search in the currently selected folder, or
- Search across all folders (depending on your email server's capabilities)

### Server Side Search

If the search results don't yield what you are looking for, you can perform a more comprehensive server side IMAP search by clicking the **Search Server** link located on the bar above the first search result or at the center of the message list pane when basic search yields no results. This searches all messages on the server, including older messages that haven't been downloaded locally. It is slower but much more comprehensive. By default, server searches return a maximum of 200 results. If this still does not return what you are looking for, scroll down to the bar below the last search result and click the **Load More** link to return all results.

Alternatively, if you prefer to always use server side search or know a server side search is needed, simply press **Shift+Enter** after typing your search phrase in the search bar.

## Sorting

There are only 2 types of sort. **Newest on top** or **Oldest on top**. You can change between the two by clicking the farthest right button on top of the message list.

## Filtering

If you need to see only messages that are either unread, starred, or with attachments in any given folder, there's a **filter** icon on top of the message list between the **search** and **sort** icons. 

## Sync Options

### Automatic Sync

Aerion periodically checks for new mail. The sync interval can be adjusted in settings.

Aerion also holds IDLE connections for push e-mail. When new e-mail arrive, it triggers a sync of that account's core folders.

### Manual Sync

Right-click the folder in the sidebar and select **Sync Folder**, or select the folder and press `Ctrl + Shift + S`. Pressing `Ctrl + Shift + S` again will stop the sync.

### Sync All Accounts

Click the sync icon at the top of the message list, use the sync button in the bottom left corner of the app, or press `Ctrl + Shift + A` to sync all accounts at once. Clicking it or pressing `Ctrl + Shift + A` again while a sync is in progress will stop the sync.

### Force Re-sync

If messages appear to be missing or out of date:

1. Right-click the folder that seems to be out of sync in the sidebar
2. Click **Force Re-sync**

This will re-download messages from the server.

## Message Density

Adjust how much space each message takes in the list:

1. Go to **Settings > General**
2. Choose from:
   - **Micro** - Minimal spacing, more messages visible
   - **Compact** - Reduced spacing
   - **Standard** - Default spacing
   - **Large** - More spacing, easier to read

## Remote Images

For privacy, Aerion blocks remote images in emails by default. These images can be used to track when you open an email.

To load images in a specific message:
- Click the small **Load Images** button above the message body or press `Ctrl+L`

To always load images from a sender:
- Click the dropdown arrow next to Load Images or press `Ctrl+Shift+L`
- Select:
    - **For this domain** - Trusting all e-mails from this domain from now on
    - **For this e-mail address** - Trusting all e-mails from this e-mail address from now on

## Tracking Element Removal

Aerion automatically removes common tracking elements from emails:
- Tracking pixels (1x1 invisible images)
- Known tracking parameters in links

This helps protect your privacy without requiring any configuration.
