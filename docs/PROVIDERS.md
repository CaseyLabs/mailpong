# Providers and account requirements

This is the starting point for understanding Mailven's provider support. It describes the current implementation; an available route is not a guarantee of live compatibility or delivery. Provider requirements were checked against the linked official sources on 2026-10-14. Tenant policies and account eligibility can differ.

## Choose an account type and route

A **provider** hosts the mailbox; an **account type** determines its policies and permissions; a **route** determines how Mailven reads and sends mail. OAuth authenticates a connection: signing in with Google or Microsoft does not itself switch IMAP/SMTP to an API.

New accounts use IMAP for mail access and SMTP for sending. Existing accounts retain their configured route unless the user explicitly migrates them.

| Account | IMAP/SMTP authentication in Mailven | Optional mail API route | Setup and requirements |
| --- | --- | --- | --- |
| Personal Gmail | Google OAuth, or an eligible app password | Gmail API for OAuth accounts | [Gmail setup](user-guide/getting-started/first-account/gmail.md); bring your own desktop OAuth client |
| Google Workspace, including custom domains | Google OAuth, or an app password if the organization permits it | Gmail API for the authorized user's own mailbox | Same Google setup; organization policy can restrict client access and consent; Gmail delegated-mailbox API access is unsupported |
| Personal Outlook.com, Hotmail or Live | Microsoft OAuth | Not supported by Mailven's API route | [Outlook setup](user-guide/getting-started/first-account/outlook.md); select Personal and enable IMAP in Outlook.com |
| Microsoft 365 work or school | Microsoft OAuth | Delegated Microsoft Graph for the user's own mailbox | Select Work or school; app consent, IMAP and SMTP AUTH are separate organization controls |
| Linked Microsoft shared mailbox | Parent account's OAuth credentials; shared mailbox identity for IMAP | Cannot be activated or migrated yet | Mailbox access and permission to send as/on behalf of it must be granted separately; a successful login is insufficient |
| Yahoo, iCloud, Fastmail, Zoho, AOL, GMX, Mail.com | Password or app password appropriate to the service | None | [Other providers](user-guide/getting-started/first-account/other-providers.md); presets are configuration defaults, not live compatibility evidence |
| Proton Mail through Bridge | Credentials supplied by the locally running Bridge | None | [Bridge setup](user-guide/getting-started/first-account/other-providers.md#protonmail-via-proton-bridge); use Bridge's actual local endpoints and credentials |
| Manual/custom provider | Server credentials or explicitly configured custom OAuth | None | Configure IMAP/SMTP endpoints, TLS and the provider's OAuth metadata when applicable |

Do not infer Microsoft account type from the email domain. Personal Outlook.com and Microsoft 365 use different SMTP defaults; preserve existing/custom settings unless intentionally changing them. Microsoft's [Outlook.com settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com) document the personal account endpoints and IMAP enablement.

## Credentials, permissions and policy

Mailven bundles no Google or Microsoft OAuth credentials. Create a desktop client/application registration and configure it under **Settings → Accounts → OAuth Credentials**. Use [Google's setup guide for Mailven](user-guide/getting-started/first-account/google-oauth-credentials.md) or the [Microsoft setup instructions](user-guide/getting-started/first-account/outlook.md#before-you-start-bring-your-own-credentials). The registration must allow the account audience and desktop redirect flow being used.

| Route | Mail authorization requested by Mailven | What it does not establish |
| --- | --- | --- |
| Google IMAP/SMTP | `https://mail.google.com/`, identity scopes; the default profile also requests read-only Contacts and Other Contacts | Access to another mailbox, successful sending from every alias, or independently authorized contact/calendar sources |
| Microsoft IMAP/SMTP | Outlook resource `IMAP.AccessAsUser.All`, `SMTP.Send`, identity and `offline_access` | Graph access, enabled protocols, or shared-mailbox/Send As rights |
| Gmail API | `gmail.modify` plus identity scopes, in a separate API grant | Immediate permanent deletion or delegated-mailbox support |
| Microsoft Graph | Delegated `Mail.ReadWrite`, `Mail.Send`, identity and `offline_access`, in a separate API grant | Application-only access or shared-mailbox activation; shared permission profiles exist internally but shared routes remain unavailable |

Permission names above abbreviate Google API and Microsoft resource URLs; the exact profiles are in [MAIL_APIS.md](MAIL_APIS.md) and `internal/oauth2/providers.go`. Google's [scope reference](https://developers.google.com/workspace/gmail/api/auth/scopes) defines the restricted Gmail scopes. Microsoft's [protocol OAuth instructions](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth) and [Graph permissions reference](https://learn.microsoft.com/en-us/graph/permissions-reference) describe the separate resources.

Requested permissions are not proof of granted permissions. Mailven stores returned grant evidence and checks the relevant capability. Legacy unknown grants do not prove API access. API tokens are stored separately from protocol tokens; contacts and calendars have their own authorization lifecycles. Normal token expiry is handled by refresh; a rejected/revoked grant requires explicit reauthorization. Reauthorizing an active API route is done in its **Mail API route** section.

Google app passwords require 2-Step Verification and may be unavailable for particular accounts or organization policies ([Google eligibility](https://support.google.com/accounts/answer/185833?hl=en)). Google external OAuth projects left in Testing can have seven-day refresh tokens for Gmail access; moving to production does not itself satisfy verification requirements ([Google token lifecycle](https://developers.google.com/identity/protocols/oauth2#expiration)).

Mailbox access, Send As and Send on Behalf permissions are distinct from OAuth consent. For Graph, Microsoft requires both the appropriate delegated permission and mailbox rights to [send from another user](https://learn.microsoft.com/en-us/graph/outlook-send-mail-from-other-user). Mailven's shared API route remains unsupported even when those rights exist. Do not weaken tenant security policy to bypass an authorization failure.

## Switching routes and current limits

For an eligible OAuth account, open **Settings → Accounts → Edit → Server → Mail API route**. Authorizing stages a separate grant; it does not change the active route. Review migration and its blockers before confirming the switch. See the [Settings guide](user-guide/features/settings.md#server-tab) for the workflow and [API migration reference](MAIL_APIS.md#migration-and-rollback) for storage and recovery details.

Migration saves a local database snapshot, preserves the account and credentials, and hides the old IMAP folders while correlating API messages. Ambiguous message matches remain uncorrelated; migration does not promise to carry every local flag across those matches. **Return to IMAP / SMTP** switches routes back and reports unfinished API actions. It does not restore the snapshot or undo provider changes made while the API route was active. Full snapshot restoration is not implemented.

Current API routes support sending, receiving and queued flag/folder actions, with these limits:

- Shared mailboxes cannot be migrated; Gmail delegated-mailbox and personal Outlook.com API routes are unsupported.
- Drafts are saved locally; remote API draft synchronization is unavailable.
- Permanent delete, empty trash/spam, copy, cross-account moves and undoing API moves are unavailable.
- There is no automatic SMTP/IMAP fallback after migration and no automatic resend of an uncertain submission.
- Bcc privacy, provider message-size limits and real-account migration/rollback remain unverified live. Local fixtures and UI checks do not establish provider compatibility. See [open findings](../ISSUES.md#ma1).

## Sending, limits and troubleshooting

An authenticated mailbox is not proof that an alias can send. Mailven checks route-specific sender evidence and may require review in the full composer; chat replies cannot perform that review. Review **Sent copy policy** for SMTP routes to avoid unintended copies. API routes rely on the provider's Sent behavior. See [Sending and recovery](SENDING.md) for sender evidence, uncertain outcomes and Sent-copy handling.

Message limits apply to the encoded MIME, including attachments and encryption overhead. SMTP SIZE is a server advertisement, not a delivery guarantee. The API submission ceiling is a local 25 MiB bound, not a verified provider allowance; other local attachment/read limits do not establish what a mailbox can send or receive. Acceptance is not delivery or inbox placement.

Use the account's **Check account setup** diagnostic for endpoint, TLS, protocol authentication and cached permission/sender evidence. It runs only when started and sends no test mail. It does not prove API access, mailbox delegation or delivery. Optional domain DNS checks do not prove SPF/DKIM/DMARC alignment on a delivered message; review exported reports before sharing. See [diagnostic details](SENDING.md#check-account-setup) and [Troubleshooting](user-guide/troubleshooting.md).

## Implementation map and maintenance

| Area | Source of truth |
| --- | --- |
| Provider presets and OAuth profiles | `frontend/src/lib/config/providers.ts`, `internal/oauth2/providers.go`, `internal/oauth2/mail_api.go` |
| Actual grant evidence and credential ownership | `internal/credentials/`, [Sending and recovery](SENDING.md#sent-copies-and-linked-authentication) |
| API consent, route activation and migration | `app/mail_api.go`, `app/api_route.go`, `app/api_migration.go`, [MAIL_APIS.md](MAIL_APIS.md) |
| API clients, sync and queued actions | `internal/gmailapi/`, `internal/graphapi/`, `internal/sync/`, `internal/apiactions/` |
| Provider setup diagnostics | `app/setup_diagnostic.go`, `internal/setupdiagnostic/`, [diagnostic contract](SENDING.md#check-account-setup) |

When provider behavior changes, update this matrix and the affected setup/technical sections in the same change. Keep unsupported behavior and live evidence separate from implementation status. Preserve stored OAuth identifiers and independent token slots. Verify external requirements against official sources before changing them. Never use research notes as proof that a route has passed live testing.

The [deliverability review](validation/provider-deliverability-review.md) and [transport recommendations](validation/provider-transport-recommendations.md) are dated design evidence. Current behavior belongs here and in the linked technical/user guides; unfinished work belongs in `PLAN.md` and `ISSUES.md`.
