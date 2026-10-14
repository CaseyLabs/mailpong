---
sidebar_position: 1
pagination_next: features/overview
---

# Gmail Setup

Mailven supports Gmail through two authentication methods: OAuth (recommended) and App Passwords.

## OAuth Method (Recommended)

Mailven bundles no Google OAuth credentials. Create your own first: see [Custom Google OAuth Credentials](./google-oauth-credentials.md), or use an app password below.

OAuth is the easiest and most secure way to connect your Gmail account.

### Steps

1. Click **Sign in with Google**
2. A browser window will open to Google's sign-in page
3. Sign in to your Google account
4. Review the permissions and click **Allow**
5. Return to Mailven and click **Add Account**

### Permissions Requested

Mailven requests access to Gmail and Google Contacts. Contact sync is configured separately in **Settings > Contacts**; you can manage or remove that contact source independently of the mail account.

### Security Note

You may see a warning that "This app isn't verified." Mailven has no bundled Google OAuth credentials, so you use an OAuth client you created. The warning depends on that project's publishing and verification status; creating a client does not itself make it verified. For an external project left in **Testing**, Google expires refresh tokens after seven days when the app requests Gmail access. Publish the project to **In production** to avoid this Testing-only seven-day limit; Gmail's sensitive/restricted scope verification requirements still apply. See [Google's OAuth token guidance](https://developers.google.com/identity/protocols/oauth2#expiration) and [OAuth verification guidance](https://support.google.com/cloud/answer/13463073).

You can review what Mailven does in the [source code](https://github.com/CaseyLabs/mailven).

To proceed:
1. Click **Advanced**
2. Click **Go to Mailven (unsafe)**
3. Review permissions and click **Allow**

## App Password Method (Alternative)

Mailven recommends OAuth. Use an app password only when you cannot use OAuth and your Google account offers this option. Google describes app passwords as less secure and may hide the option for some accounts, including some organization-managed accounts and accounts enrolled in Advanced Protection.

### Prerequisites

App passwords require 2-Step Verification, but enabling it does not guarantee that the option will be available. See Google's [app password eligibility and setup guidance](https://support.google.com/accounts/answer/185833?hl=en).

### Creating an App Password

1. Go to your [Google Account Security Settings](https://myaccount.google.com/security)
2. Under "How you sign in to Google," click **2-Step Verification**
3. Scroll down and click **App passwords**
4. Select **Mail** and **Other (Custom name)**
5. Enter "Mailven" as the name
6. Click **Generate**
7. Copy the 16-character password shown

### Adding to Mailven

1. Click **+ Add Account** in the sidebar (or go to **Settings > Accounts > Add Account**)
2. Select **Gmail** from the provider dropdown
3. Choose the **App Password** authentication method
4. Enter your Gmail address
5. Paste the app password you generated
6. Click **Add Account**

Mailven records the permissions returned by the provider separately from those requested. If a known grant lacks a required permission, only the affected feature is unavailable and its error explains the missing access. Mail authorization does not prove contacts access. Older accounts can have unknown permission evidence: their configured mail route remains available, while contacts may need explicit authorization. A new sign-in that omits permission evidence is also recorded as unknown. Mailven does not start reauthorization automatically.

## Troubleshooting

### "Access Denied" Error

- Ensure you've authorized the correct Google account
- Try generating a new app password
- Check if your organization has restrictions on third-party apps

### OAuth Sign-in Requested

Mailven refreshes OAuth access tokens automatically during normal use. An expired access token usually does not require action. If Google rejects or revokes the saved refresh grant, Mailven will ask you to sign in again. Then:

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

Google Contacts are managed as a separate contact source in **Settings > Contacts**. Connect or reauthorize that source there if contact sync needs access; mail authorization status and contact-source status are managed separately.

Autocomplete can also search Google's Other Contacts when the mail account's
grant includes `contacts.other.readonly`. This search does not require the
separate `contacts.readonly` permission used for saved-contact sync.

See [Contacts](/docs/features/contacts) for more information.
