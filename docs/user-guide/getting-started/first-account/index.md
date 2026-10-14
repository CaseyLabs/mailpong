---
sidebar_position: 1
pagination_next: features/overview
---

# First Account Setup

This guide walks you through adding your first email account to Mailven.

For account types, authentication requirements and optional mail API routes, see [Providers and account requirements](../../../PROVIDERS.md). OAuth sign-in alone does not switch the account from IMAP/SMTP to an API.

## Adding an Account

The quickest way to add an account is to click the **+ Add Account** button in the sidebar.

Alternatively, you can go to **Settings > Accounts > Add Account**.

## Quick Setup Steps

1. **Select your provider** from the dropdown menu
2. **Select account color** - Used to distinguish emails in the unified inbox
3. **Configure account name** - A friendly name to identify this account
4. **Configure display name** - The name you want people receiving your e-mails to see
5. **Enter your e-mail address** - The e-mail address of the account
6. **Enter your user name** - The username is usually also your e-mail address for most providers
7. **Follow the authentication flow**:
   - For Gmail/Outlook: Click the OAuth button and authorize in your browser
   - For other providers: Enter app password
   - For manual setup: Enter your server details
8. **Notable Advance Settings** - Some settings that you might care about:
    - **Sync Period** - How much of the e-mail to sync from provider to local. Default: `6 Months`
    - **Check for New Mail** - Aside from push mail, how often to sync. Default: `1 hour`
    - **Request Read Receipts** - Read receipt settings for send. Default: `Never`
9. **Click Add Account**

Mailven will connect to your email server and begin synchronizing your messages. If it doesn't start syncing automatically, click on the 3 dot menu next to the account and click "Sync Now".

## Further Details

Choose your provider below for detailed on each provider's setup:

import DocCardList from '@theme/DocCardList';

<DocCardList />

## Initial Sync

After adding your account, Mailven will sync your emails. This may take a long time depending on your mailbox size.

By default, Mailven syncs emails from the last 180 days. You can adjust this in the account settings.

## Adding More Accounts

You can reasonably add as many email accounts as you want to Mailven. Each account can have its own color for easy identification in the unified inbox.

To add another account, click the **+ Add Account** button in the sidebar, or go to **Settings > Accounts > Add Account**.
