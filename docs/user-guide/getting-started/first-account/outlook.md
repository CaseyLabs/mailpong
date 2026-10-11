---
sidebar_position: 2
pagination_next: features/overview
---

# Outlook Setup

MailPong supports Outlook.com, Hotmail, and Microsoft 365 accounts through Microsoft OAuth.

## Supported Account Types

- Outlook.com (personal)
- Hotmail.com
- Live.com
- Microsoft 365 (work/school)
- Office 365

## Before you start: bring your own credentials

MailPong bundles no Microsoft OAuth credentials. Register your own application in the Azure portal (Microsoft Entra ID → App registrations), choose the account types you need, add a **Mobile and desktop applications** platform with the redirect URI `http://localhost`, and add the IMAP, SMTP and `offline_access` delegated permissions. Then open **Settings → Accounts → OAuth Credentials**, choose **Custom** under Microsoft, and save your Application (client) ID. If you cannot register an app, Outlook accounts cannot sign in with OAuth, and Microsoft does not offer app passwords for this.

## oAuth Steps

1. Click **Sign in with Microsoft**
2. A browser window will open to Microsoft's sign-in page
3. Sign in with your Microsoft account
4. Review the permissions and click **Accept**
5. Return to MailPong and click **Add Account**

## Permissions Requested

MailPong requests the following permissions:
- **Read and write access to your mail** - Required for email functionality
- **Read your contacts** - Used for email address autocomplete
- **Read your basic profile** - To identify your account

## Microsoft 365 / Work Accounts

For work or school accounts, your organization's administrator may need to approve MailPong. If you see an "Admin approval required" message:

1. Contact your IT administrator
2. Request approval for the MailPong application
3. Once approved, retry the sign-in process

## Important Notes

### OAuth Only

Unlike Gmail, Outlook/Microsoft 365 does not support app passwords for third-party email clients. OAuth is the only authentication method available.

### Token Expiration

Microsoft OAuth tokens expire periodically. When this happens:

1. Go to **Settings > Accounts**
2. Click **Edit** on your Outlook account
3. Click **Re-authorize**
4. Complete the OAuth flow again

## Troubleshooting

### "Need Admin Approval" Error

Your organization's administrator has restricted third-party applications. Contact your IT department to request access.

### Sign-in Loop

If you keep getting redirected back to the sign-in page:

1. Clear your browser's cookies for `login.microsoftonline.com`
2. Try using a different browser
3. Ensure pop-ups are not being blocked

### Missing Folders

If some folders don't appear:

1. Check if the folder exists in Outlook web
2. Select the folder, click the sync icon, and choose **Force Re-sync**
3. Some shared mailboxes may have limited folder access

### Slow Sync

Microsoft 365 accounts with large mailboxes may take longer to sync initially. The sync progress is shown in the status bar.

## Contact Sync

When you authorize Outlook with OAuth, MailPong can also sync your Microsoft contacts for email autocomplete. This is configured separately in **Settings > Contacts**.

See [Contacts](/docs/features/contacts) for more information.
