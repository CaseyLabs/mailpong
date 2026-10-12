---
sidebar_position: 3
---

# Composing Emails

Mailven includes a full-featured email composer with rich text editing capabilities.

## Starting a New Message

- Click the **Compose** (pencil) button at the left of the conversation viewer toolbar (or at the top of the message list in narrow windows)
- Press `Ctrl+N` from anywhere in the app
- Click **Reply**, **Reply All**, or **Forward** on a message
- Press:
    - `Ctrl+R` - Reply
    - `Ctrl+Shift+R` - Reply All
    - `Ctrl+F` - Forward

## The Composer Interface

The composer includes:

- **From** - Select which account/identity to send from
    - On a reply, Mailven picks the identity the message was sent to (To/Cc). If none matches, it checks the forwarding headers (`X-Forwarded-For`, `X-Original-To`, `Delivered-To`), so replies to mail forwarded to your account come from the original address. Your account's own address in those headers does not count, so it never overrides your default identity. The identity's Reply-To is applied automatically.
    - On a Gmail account, if Gmail would rewrite the chosen From address (it is not a verified send-as address), the composer shows a warning naming the address Gmail will send from.
- **To** - Primary recipients
- **Cc/Bcc** - Click to show additional recipient fields
- **Subject** - Email subject line
- **Body** - Rich text editor for your message
- **Toolbar** - Formatting options and actions

## Rich Text Editor

The composer uses a WYSIWYG (What You See Is What You Get) editor powered by [TipTap](https://tiptap.dev/).

### Text Formatting

| Format | Toolbar | Keyboard |
|--------|---------|----------|
| Bold | **B** button | `Ctrl+B` |
| Italic | *I* button | `Ctrl+I` |
| Underline | U button | `Ctrl+U` |
| Strikethrough | ~~S~~ button | - |

### Structure

- **Headings** - H1, H2, H3 heading levels
- **Bullet Lists** - Unordered lists
- **Numbered Lists** - Ordered lists
- **Blockquotes** - Quoted text blocks
- **Code Blocks** - Monospace code formatting
- **Horizontal Rule** - Divider line

### Links

1. Select the text you want to link
2. Click the link button in the toolbar
3. Enter the URL
4. Click Apply

### Tables

1. Click the table button in the toolbar
2. Select the table size
3. The table will be inserted at the cursor position

Edit tables using the table tools that appear when a table is selected.

###  Toolbar

From the subject field, if you press tab, it will automatically focus on the body of the e-mail which effectively skips the toolbar. To focus on the toolbar, press `Alt+T` and single key shortcut key hints will displayacross the toolbar for you to select a button with keyboard. To exit toolbar mode, simply press `Alt+T` again.

###   Remote Images

The same remote image bar as the conversation viewer appears here to continuing blocking images when you reply or forward messages. If you load images prior to clicking reply, The bar won't be rendered and images will be automatically loaded. Image blocking is just to protect you as an Mailven user. They will all still be sent to your recipient(s) as originally composed.

### Spellcheck

English spellcheck is enabled by default. For other supported languages, you have to turn on the dictionary of the language that you need support for in the **Compose** tab of the settings dialog.

To get the context menu to get spelling suggestions or to add custom words to your dictionary, or to mark certain words to ignore for the e-mail you are currently composing, you can either right click words that are marked with a red squiggly line or press **F7** to toggle the same context menu for the closest word to the left that's marked with a red squiggly line.

## Plain Text Mode

Toggle to plain text mode when you need:

- Simpler formatting
- Better compatibility with older email clients
- Smaller message size

Click the **Plain Text** toggle in the toolbar to switch modes.

:::warning
Switching from rich text to plain text will remove all formatting.
:::

## Attachments

### Adding Attachments

Three ways to attach files:

1. **Click to Attach** - Click the attachment button and select files or press `Alt+A`
2. **Drag and Drop** - Drag files onto the composer window
3. **Paste Images** - Copy an image and paste directly into the editor

### Attachment Limits

Attachment size limits depend on your email provider:

| Provider | Typical Limit |
|----------|---------------|
| Gmail | 25 MB |
| Outlook | 20 MB |
| Most providers | 10-25 MB |

For larger files, consider using a file sharing service and including a link.

## Recipients

### Autocomplete

As you type in the To, Cc, or Bcc fields:

- Matching contacts from your synced address books appear
- Recent recipients are suggested
- Select a suggestion with arrow keys and Enter

### Multiple Recipients

- Separate multiple addresses with commas
- Each recipient appears as a "chip" that can be removed individually

### Cc and Bcc

Click **Cc** or **Bcc** to show these fields:

- **Cc (Carbon Copy)** - Visible to all recipients
- **Bcc (Blind Carbon Copy)** - Hidden from other recipients

## Drafts

### Auto-Save

Mailven automatically saves your draft as you compose. If you close the composer or the app unexpectedly, your draft will be preserved.

Drafts are saved to your email account's Drafts folder.

### Resuming Drafts

To continue working on a draft:

1. Navigate to the Drafts folder
2. Click on the draft to view it in the conversation viewer
3. Click  the **Pencil icon** on the top right corner of the draft inside the conversation viewer to open it in the composer

## Pop-Out Composer

Detach the composer to a separate window:

- Click the **Pop Out** button in the composer toolbar
- Press `Ctrl+D`

This allows you to:
- Reference other emails while composing
- Keep the composer open while navigating
- Position the composer on a different monitor

## Sending

When ready to send:

- Click the **Send** button
- Press `Ctrl+Enter`

The message will be sent through your email provider's SMTP server.

### Undo Send

Every send waits 10 seconds before it goes out, so you can take it back:

- **Main window** - The composer closes at once and a "Sending in 10 seconds" notice appears with **Undo**. Undo reopens the message as you left it (in its own window if another composer is open).
- **Pop-out composer** - The window stays open and counts down in its header, with **Undo** to return to editing and **Send now** to skip the wait.
- **Quitting** - Messages still waiting are sent before Mailven closes.

### Send Confirmation

Once the message goes out, a brief confirmation appears. If sending fails, you'll see an error message, and the message is kept in Drafts.

## Signatures

Set up signatures for each account identity:

1. Go to **Settings > Accounts**
2. Click **Edit** on an account
3. Navigate to the **Identities** section
4. Edit the signature in the rich text editor
5. Save changes

Signatures are automatically appended when composing new messages.

### Per-Identity Signatures

If you have multiple identities (email addresses) on one account, each can have its own signature.

## PGP & S/MIME Signing & Encryption

If you have configured PGP or S/MIME properly on Mailven, an extra bar will appear below the subject line of the composer. Each configured secret key or cert is identity aware so if you choose an alias that doesn't have a matching key or cert, the bar will disappear. Once you switch back to an identity that has a matching key or cert, the bar will automatically appear again. By design, Mailven will only sign and encrypt with keys or certs that match the from e-mail address of the identity chosen.

There are shortcut keys that are associated with bringing focus to each bar:

| Type | Keyboard |
|--------|----------|
| PGP | `Alt+P` |
| S/MIME | `Alt+S` |

You can then toggle Signing by pressing `S` or toggle Encryption by pressing `E`. After you are done with toggling what you need, you can exit focus by pressing `Esc`. This will bring focus back to where you were before.


