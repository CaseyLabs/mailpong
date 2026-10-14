---
sidebar_position: 2
pagination_next: features/overview
---

# Outlook Setup

Mailven supports personal Outlook.com accounts (including Hotmail and Live addresses) and Microsoft 365 work or school accounts through Microsoft OAuth. If Microsoft asks you to choose an account type during sign-in, select **Personal account** for Outlook.com or **Work or school account** for an organization-managed mailbox. Otherwise, confirm the type in the account's Outlook/Microsoft settings. An email domain alone does not reliably identify whether an account is personal or managed by an organization.

## Supported Account Types

- Personal Microsoft accounts, including Outlook.com, Hotmail, and Live
- Microsoft 365 work or school accounts

## Before you start: bring your own credentials

Mailven bundles no Microsoft OAuth credentials. Register your own application in Microsoft Entra ID (**App registrations**), choose the account types you need, add a **Mobile and desktop applications** platform, and add the delegated IMAP, SMTP, and `offline_access` permissions. Then open **Settings → Accounts → OAuth Credentials**, choose **Custom** under Microsoft, and save your Application (client) ID. Use Microsoft's documented [OAuth permissions and setup](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth) for the registration. An administrator may need to approve the app or grant consent in a work or school tenant.

## oAuth Steps

1. Click **Sign in with Microsoft**
2. A browser window will open to Microsoft's sign-in page
3. Sign in with your Microsoft account
4. Review the permissions and click **Accept**
5. Return to Mailven and click **Add Account**

## Permissions Requested

Mailven's mail authorization requests Outlook IMAP access, SMTP send access, and basic sign-in identity. It does not grant Microsoft Contacts access. Add and authorize a Microsoft contact source separately in **Settings > Contacts** if you want contact sync.

## Microsoft 365 / Work Accounts

For work or school accounts, organizational policies can independently control IMAP availability, SMTP AUTH, and whether users may consent to an application. These controls can prevent one mail function even when another is allowed. If sign-in says "Admin approval required" or a mail protocol is disabled, contact your administrator to ask which application consent or protocol policy applies. Do not weaken organization-wide security defaults to make the connection work.

For personal Outlook.com accounts, IMAP is disabled by default. In Outlook.com, open **Settings > Mail > Forwarding and IMAP**, turn on the IMAP option, and save. Microsoft's published settings are IMAP `outlook.office365.com:993` with SSL/TLS and SMTP `smtp-mail.outlook.com:587` with STARTTLS, using OAuth/Modern Auth. For Microsoft 365 work or school accounts, the documented SMTP endpoint is `smtp.office365.com:587` with STARTTLS; the tenant must also permit IMAP and SMTP AUTH for the account. For new setup, choose **Personal** or **Work or school** in the Microsoft account type field. Mailven selects the matching SMTP default; review or customize Server settings before saving. Keep any existing or custom server settings unless you have confirmed the correct account type and are intentionally changing them. See Microsoft's [Outlook.com protocol settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com), [Exchange Online mailbox app controls](https://learn.microsoft.com/en-us/Exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes), and [SMTP AUTH policy guidance](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/authenticated-client-smtp-submission).

If you see an "Admin approval required" message:

1. Contact your IT administrator
2. Request approval for the Mailven application
3. Once approved, retry the sign-in process

## Important Notes

### OAuth Only

Mailven connects to Microsoft accounts with OAuth. Microsoft documents app passwords for some personal-account sign-in situations, but they are not an authentication option in Mailven. Do not switch to weaker/basic authentication to bypass an OAuth or tenant-policy failure.

### Token Expiration

Mailven refreshes Microsoft access tokens automatically during normal use. An expired access token usually needs no action. If Microsoft rejects or revokes the saved refresh grant, Mailven will ask you to sign in again. Then:

1. Go to **Settings > Accounts**
2. Click **Edit** on your Outlook account
3. Click **Re-authorize**
4. Complete the OAuth flow again

Mailven records the permissions returned by the provider separately from those requested. If a known grant lacks a required permission, only the affected feature is unavailable and its error explains the missing access. Mail authorization does not prove contacts access. Older accounts can have unknown permission evidence: their configured mail route remains available, while contacts may need explicit authorization. A new sign-in that omits permission evidence is also recorded as unknown. Mailven does not start reauthorization automatically.

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

Microsoft Contacts are authorized and managed as a separate contact source in **Settings > Contacts**. Connect or reauthorize that source there if contact sync needs access.

See [Contacts](/docs/features/contacts) for more information.
