---
sidebar_position: 5
---

# Keyboard Shortcuts

Mailven is designed for efficient keyboard navigation. Use these shortcuts to manage your email without touching the mouse.

The mail section's chat list, chat triage, and chat view keys are listed in [the keyboard reference](https://github.com/CaseyLabs/mailven/blob/main/docs/KEYBOARD_SHORTCUTS.md#chat-list). Where they differ from the tables below (for example `E` for Archive and `V` for Move in the chat list), the chat keys apply.

## Global Shortcuts

These work anywhere in the application (unless you're typing in a text field).

### Application

| Shortcut | Action |
|----------|--------|
| `Ctrl+Q` | Quit application |
| `Ctrl+N` | Compose new message |
| `Ctrl+S` | Focus search bar |
| `Ctrl+Shift+A` | Sync all accounts |
| `Ctrl+Shift+S` | Sync selected folder |
| `Ctrl+Tab` | Switch to next extension on the rail (Mail / Contacts / ...) |
| `` Ctrl+` `` | Switch to previous extension on the rail |
| `Ctrl+Shift+B` | Hide/show the left sidebar of the current view |

### Pane Navigation

| Shortcut | Action |
|----------|--------|
| `Alt+Left` / `Alt+H` | Focus previous pane (viewer -> list -> sidebar) |
| `Alt+Right` / `Alt+L` | Focus next pane (sidebar -> list -> viewer) |

A hidden sidebar is skipped when cycling panes.

### Folder Navigation

| Shortcut | Action |
|----------|--------|
| `Alt+Up` / `Alt+K` | Navigate to previous folder |
| `Alt+Down` / `Alt+J` | Navigate to next folder |
| `Alt+Enter` | Expand/collapse focused account folder tree |
| `Alt(L)+Alt(R)` | Brings up context menu for the focused folder |

### Message Actions (when message is selected/focused)

| Shortcut | Action |
|----------|--------|
| `Ctrl+R` | Reply to last message (requires viewed conversation) |
| `Ctrl+Shift+R` | Reply All to last message (requires viewed conversation) |
| `Ctrl+F` | Forward last message (requires viewed conversation) |
| `Ctrl+U` | Mark as read (keyboard-focused or checked messages) |
| `Ctrl+Shift+U` | Mark as unread (keyboard-focused or checked messages) |
| `Ctrl+K` | Archive (keyboard-focused or checked messages) |
| `Ctrl+J` | Mark as spam (keyboard-focused or checked messages) |
| `Ctrl+L` | Load remote images in viewed message |
| `Ctrl+Shift+L` | Open "Always Load Images" dropdown |
| `Shift+F` | Toggle focus mode for the message |

---

## Pane-Specific Shortcuts

These shortcuts depend on which pane is focused. They are disabled when typing in input fields.

### Sidebar (Folder List)

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Navigate to previous folder |
| `Down` / `J` | Navigate to next folder |
| `Alt+G` | Jump to All Inboxes |
| `Alt+Shift+G` | Jump to the last expanded folder or account |
| `Alt + Enter` / `Space` | Expand/collapse account (when account header is focused) |
| `Alt(L) + Alt(R)` | Brings up context menu for the focused folder |

### Message List

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Select previous conversation |
| `Down` / `J` | Select next conversation |
| `G` | Jump to first message on list |
| `Shift+G` | Jump to last loaded message on list |
| `Shift+Up` / `Shift+K` | Select previous + toggle checkbox |
| `Shift+Down` / `Shift+J` | Select next + toggle checkbox |
| `Space` | Toggle checkbox on current conversation |
| `Enter` / `V` | Open selected conversation in viewer |
| `D` | Delete selected/checked message(s) — move to Trash (same as `Delete`) |
| `CTRL+A` | Select all messages in folder |
| `Alt(R)` | Brings up context menu for the selected message(s) |
| `Alt+C` | Toggles copy to dialog |
| `Alt+M` | Toggle move to dialog |

### Conversation Viewer

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Scroll up |
| `Down` / `J` | Scroll down |
| `Tab` | Cycle through messages when focused on conversation viewer |
| `Delete` / `Backspace` | Delete focused message when focused on conversation viewer |
| `Ctrl + A` | Select all text of message in viewport |
| `Alt(R)` | Brings up context menu for the message focused |
| `F` | Toggles focus mode on the current thread (conversation) |

---

## Single-Key Shortcuts

These work when not in an input field. They apply to checked messages (bulk) or the keyboard-focused message in the list.

| Shortcut | Action |
|----------|--------|
| `S` | Toggle star |
| `Backspace` / `Delete` | Move to trash |
| `Shift+Backspace` / `Shift+Delete` | Permanently delete |
| `Escape` | Clear checkboxes (first press), close conversation (second press) |

---

## Composer Shortcuts

These only work when the composer is open.

| Shortcut | Action |
|----------|--------|
| `Ctrl+Enter` | Send message |
| `Alt+T` | Activate/Deactivate toolbar mode |
| `Alt+A` | Attach a file |
| `Alt+P` | Toggle PGP security mode (S = sign, E = encrypt) |
| `Alt+S` | Toggle S/MIME security mode (S = sign, E = encrypt) |
| `Ctrl+D` | Pop out/detach composer to separate window |
| `Escape` | Close composer (prompts to save draft if unsaved) |

---

### Text Formatting

| Shortcut | Action |
|----------|--------|
| `Ctrl+B` | Bold |
| `Ctrl+I` | Italic |
| `Ctrl+U` | Underline |
| `Alt+T`  | Toggle toolbar and follow hint to choose |

---

## Quick Reference Card

```
NAVIGATION
  Alt + Arrow Keys    Pane focus (Left/Right) or Folder nav (Up/Down)
  Alt + H/J/K/L       Vim-style: pane (H/L) or folder (J/K)
  Alt(L) + Alt(R)     Brings context menu up for the focused folder
  Alt + Enter         Expand/collapse account
  Ctrl + Tab          Switch to next extension on rail
  Ctrl + `            Switch to previous extension on rail
  Arrow Keys / HJKL   Navigate within focused pane
  Enter               Open conversation / Expand account
  Space               Toggle checkbox / Expand account

COMPOSE & REPLY
  Ctrl + N            New message
  Ctrl + R            Reply
  Ctrl + Shift + R    Reply All
  Ctrl + F            Forward
  Ctrl + Enter        Send (in composer)
  Ctrl + D            Detach composer
  Alt  + T            Toggle format toolbar mode
  Alt + P             PGP mode (S/E to toggle sign/encrypt)
  Alt + S             S/MIME mode (S/E to toggle sign/encrypt)

SELECTION
  Ctrl + A            Select all messages (list) / text (viewer)

MESSAGE ACTIONS
  S                   Star/Unstar
  Ctrl + U            Mark read
  Ctrl + Shift + U    Mark unread
  Ctrl + K            Archive
  Ctrl + J            Spam
  V                   View message
  Delete / D          Trash
  Shift + Delete      Permanent delete
  Alt(R)              Context Menu

OTHER
  Ctrl + Shift + A    Sync all accounts
  Ctrl + Shift + S    Sync selected folder
  Ctrl + S            Search
  Ctrl + L            Load images
  Ctrl + Q            Quit
  Escape              Clear/Close
```

## How Focus Works

Mailven has three main panes:

1. **Sidebar** - Account and folder list
2. **Message List** - Conversations in the current folder
3. **Conversation Viewer** - The selected conversation

A visual indicator shows which pane is focused. Use `Alt+Left/Right` or `Alt+H/L` to move focus between panes. Hide the sidebar with the toolbar's sidebar button or `Ctrl+Shift+B`; it stays hidden per view (Mail, Contacts, Calendar) until you show it again.

## Bulk Selection

To apply actions to multiple messages:

1. Navigate to a message with `J/K`
2. Press `Space` to check it
3. Navigate to more messages and check them
4. Apply an action (e.g., `Delete`, `Ctrl+K` for archive)

Or use `Shift+J/K` to select while navigating.

Press `Escape` to clear all selections.

## Action Priority

When you use an action shortcut:

1. If any messages are **checked**, the action applies to all checked messages
2. Otherwise, the action applies to the **keyboard-focused** message in the list

This lets you quickly act on single messages or perform bulk operations.


## Extensions

Extension shortcuts only fire when the corresponding extension is the active rail pane (selected via `Ctrl+Tab` / `` Ctrl+` ``). They never trigger while Mail is active — so shortcuts that overlap with Mail's (like `Ctrl+N`, `Ctrl+Shift+A`, `Ctrl+Shift+S`) are unambiguous: the active rail decides which handler runs. For example, `Ctrl+Shift+A` syncs accounts in Mail, syncs all calendar sources in Calendar, and syncs all contact sources in Contacts.

Pane-local navigation (Up/Down/J/K, Enter, Space, Alt+H/L for pane cycling, Alt+Up/Down for sidebar) uses the same kit-shared predicates Mail does — they're listed per-extension below for completeness, but they're not extension-specific re-implementations.

### Calendar

**Navigation (pane cycling)**

| Shortcut | Action |
|----------|--------|
| `Alt+Left` / `Alt+H` | Focus previous pane |
| `Alt+Right` / `Alt+L` | Focus next pane |

**Date / view navigation**

| Shortcut | Action |
|----------|--------|
| `T` | Jump the calendar view to today |
| `←` | Previous view-unit (prev month / week / day) |
| `→` | Next view-unit (next month / week / day) |
| `Alt+M` | Switch to month view |
| `Alt+W` | Switch to week view |
| `Alt+D` | Switch to day view |
| `Alt+A` | Switch to agenda view |

**Actions**

| Shortcut | Action |
|----------|--------|
| `Ctrl+N` | Open the new-event composer |
| `Ctrl+R` | Sync all calendar sources |
| `Ctrl+Shift+A` | Sync all calendar sources (same chord as Mail's "sync all accounts" — fires whichever handler matches the active rail) |
| `F` | Toggle focus mode for the selected event |

> The calendar sidebar uses checkbox toggles per calendar (not single-select navigation), so it doesn't follow the kit's `J`/`K` row-cycling pattern. Clicking a checkbox or row in the sidebar toggles visibility.

### Contacts

**Sidebar navigation (works from any pane)**

Mirrors mail's "Folder Navigation" shortcuts. These fire regardless of which contacts pane currently has keyboard focus — so you can scroll through addressbooks while the list or detail pane is focused.

| Shortcut | Action |
|----------|--------|
| `Alt+Up` / `Alt+K` | Move to previous source / addressbook in the sidebar |
| `Alt+Down` / `Alt+J` | Move to next source / addressbook in the sidebar |

**Pane cycling**

| Shortcut | Action |
|----------|--------|
| `Alt+Left` / `Alt+H` | Focus previous pane (detail → list → sidebar) |
| `Alt+Right` / `Alt+L` | Focus next pane (sidebar → list → detail) |

**Actions**

| Shortcut | Action |
|----------|--------|
| `E` | Edit the currently-focused contact |
| `Ctrl+N` | Open the new-contact dialog (pre-targeted to the sidebar-focused addressbook; falls back to local when the focused source isn't writable) |
| `Ctrl+Shift+A` | Sync all contact sources (same chord as Mail's "sync all accounts" — fires whichever handler matches the active rail) |
| `Ctrl+Shift+S` | Sync the contact source currently focused in the sidebar (no-op with warning toast when a built-in entry like "All" / "Local" is focused) |

> Within a focused pane, `Up`/`K` and `Down`/`J` cycle rows (contact list, sidebar sources) and `Enter` opens / activates — same kit predicates Mail's list and folder tree use.
