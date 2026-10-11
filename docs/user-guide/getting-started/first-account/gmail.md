---
sidebar_position: 1
pagination_next: features/overview
---

# Gmail Setup

MailPong supports Gmail through two authentication methods: OAuth (recommended) and App Passwords.

## OAuth Method (Recommended)

MailPong bundles no Google OAuth credentials. Create your own first: see [Custom Google OAuth Credentials](./google-oauth-credentials.md), or use an app password below.

OAuth is the easiest and most secure way to connect your Gmail account.

### Steps

1. Click **Sign in with Google**
2. A browser window will open to Google's sign-in page
3. Sign in to your Google account
4. Review the permissions and click **Allow**
5. Return to MailPong and click **Add Account**

### Permissions Requested

MailPong requests the following permissions:
- **Read, send, and manage your email** - Required for basic email functionality
- **View your contacts** - Used for email address autocomplete

### Security Note

You may see a warning that "This app isn't verified." This is expected: MailPong has no bundled Google credentials, so you authorize against an OAuth client you created yourself, which Google shows as unverified until it is verified. You can verify what MailPong does by reviewing the [source code](https://github.com/caseylabs/mailpong).

To proceed:
1. Click **Advanced**
2. Click **Go to MailPong (unsafe)**
3. Review permissions and click **Allow**

## App Password Method (Alternative)

Use this method if:
- OAuth doesn't work in your environment
- Your organization restricts OAuth applications
- You prefer using app-specific passwords

### Prerequisites

You must have 2-Step Verification enabled on your Google account to use app passwords.

### Creating an App Password

1. Go to your [Google Account Security Settings](https://myaccount.google.com/security)
2. Under "How you sign in to Google," click **2-Step Verification**
3. Scroll down and click **App passwords**
4. Select **Mail** and **Other (Custom name)**
5. Enter "MailPong" as the name
6. Click **Generate**
7. Copy the 16-character password shown

### Adding to MailPong

1. Click **+ Add Account** in the sidebar (or go to **Settings > Accounts > Add Account**)
2. Select **Gmail** from the provider dropdown
3. Choose the **App Password** authentication method
4. Enter your Gmail address
5. Paste the app password you generated
6. Click **Add Account**

## Troubleshooting

### "Access Denied" Error

- Ensure you've authorized the correct Google account
- Try generating a new app password
- Check if your organization has restrictions on third-party apps

### OAuth Token Expired

OAuth tokens periodically expire. To re-authorize:

1. Go to **Settings > Accounts**
2. Click **Edit** on your Gmail account
3. Click **Re-authorize**
4. Complete the OAuth flow again

### Sync Issues

If emails aren't appearing:

1. Check your internet connection
2. Select the folder, click the sync icon, and choose **Force Re-sync**
3. Verify your account is still authorized in [Google Account Permissions](https://myaccount.google.com/permissions)

## Contact Sync

When you authorize Gmail with OAuth, MailPong can also sync your Google Contacts for email autocomplete. This is configured separately in **Settings > Contacts**.

See [Contacts](/docs/features/contacts) for more information.
