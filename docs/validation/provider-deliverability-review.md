# Gmail and Microsoft 365 provider requirements: P1 audit

Reviewed **2026-10-12, America/Vancouver** (retrieval continued into 2026-10-13 UTC). Code base: `cdf6b6437a8aa1c88d1cf057a5cc9de2518f78dd`; branch: `docs/provider-deliverability-review`. This is a documentation-only audit of ordinary desktop correspondence, aliases and shared mailboxes. It does not select a new transport or implement the P2 diagnostic.

Mailven already has OAuth authorization-code/PKCE, encrypted credential storage, TLS submission, MIME encoding and reconnect machinery. The main confirmed gaps concern account-specific setup, truthful send outcomes, sender authorization visibility, message limits and Sent-copy policy. Authentication to a mailbox does not establish that the resulting message will pass domain authentication or reach the inbox.

## Evidence and applicability

Official links below were checked on the review date. Provider requirements are distinguished from code observations and live-account unknowns. **Confirmed** means the behavior is established by the cited source or implementation, not that a production account was tested. **Unverified** means private settings or live/provider behavior are required. Severity describes the consequence of a gap: medium can disrupt mail or mislead recovery; low is a narrower edge case; informational records a capability or external prerequisite.

All code references are relative to the repository root and refer to the base revision above. No application source changed. The installed Go toolchain was `go1.27.1-X:nodwarf5`; the project documents Go 1.26. Only standard-library source inspection was used; no build or test result is inferred from that toolchain. Future executable verification must use the project's supported toolchain.

### Account settings and authentication

| Account type | Required setup and communication | Responsible party and verification status |
| --- | --- | --- |
| Personal Gmail | OAuth is preferred; IMAP is always enabled in personal Gmail since January 2025. Use `imap.gmail.com:993` implicit TLS and `smtp.gmail.com:587` STARTTLS (465 implicit TLS is also documented). [Add Gmail to another email client](https://support.google.com/mail/answer/7126229?hl=en), [Gmail IMAP, POP and SMTP](https://developers.google.com/workspace/gmail/imap/imap-smtp) | User supplies a Desktop-app OAuth client; Mailven implements the flow. Requirements confirmed; actual registration/grant unverified. |
| Google Workspace | IMAP and permitted OAuth clients can be controlled by the administrator. Ordinary account-password authentication is not a supported fallback; app-password availability is conditional. The admin page still contains older less-secure-app instructions that conflict with its own 2025 notice: do not carry those instructions forward. [Workspace POP/IMAP controls](https://knowledge.workspace.google.com/admin/sync/turn-pop-and-imap-on-or-off-for-users), [Google app passwords](https://support.google.com/accounts/answer/185833?hl=en) | Workspace administrator owns service/client policies; user owns their registration. Effective policy unverified. |
| Outlook.com / Hotmail / Live | Modern Auth/OAuth2; documented IMAP is `outlook.office365.com:993` implicit TLS, SMTP is `smtp-mail.outlook.com:587` STARTTLS. IMAP must be enabled in Outlook.com settings; it is disabled by default. [Outlook.com protocol settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com) | User owns mailbox setting and registration; Mailven owns correct defaults. Published settings confirmed; existing endpoint interoperability unverified. |
| Microsoft 365 / Exchange Online | OAuth/XOAUTH2 with Outlook IMAP and SMTP scopes. SMTP submission uses `smtp.office365.com:587` STARTTLS and TLS 1.2 or newer. IMAP and SMTP AUTH availability are separate mailbox/tenant controls. [Microsoft OAuth for IMAP/POP/SMTP](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth), [Exchange Online SMTP AUTH](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/authenticated-client-smtp-submission), [Exchange client submission](https://learn.microsoft.com/en-us/exchange/mail-flow-best-practices/how-to-set-up-a-multifunction-device-or-application-to-send-email-using-microsoft-365-or-office-365), [Mailbox email-app controls](https://learn.microsoft.com/en-us/Exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes) | Tenant administrator owns protocol/consent policy and delegation; user owns registration; Mailven owns connection handling. Tenant settings unverified. |

For Google's installed client, use the system browser, PKCE S256 and a loopback redirect. A Desktop-app registration permits loopback HTTP; an installed client secret is not a confidential server secret. Mailven listens on IPv4 loopback and uses `http://localhost:<port>/callback`; Google permits localhost but notes firewall complications. This is not a confirmed redirect violation. [Google desktop OAuth](https://developers.google.com/identity/protocols/oauth2/native-app)

The broad `https://mail.google.com/` scope is the documented scope for IMAP/SMTP. Google classifies it as restricted; public distribution must follow the applicable verification/user-data rules, and server storage/transmission of restricted data can require a security assessment. Do not assume every private personal client needs the same review as a public service, or that switching only sending to Gmail API removes the broad scope while IMAP remains. [Gmail XOAUTH2](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol), [Gmail API scopes](https://developers.google.com/workspace/gmail/api/auth/scopes)

Google external clients left in Testing receive seven-day refresh tokens for mail scopes. Production publishing removes that particular testing lifetime, but does not itself verify the app, guarantee a grant, or prevent revocation. Refresh tokens can fail after revocation, password changes with Gmail scopes, inactivity or policy changes. [Google OAuth lifecycle](https://developers.google.com/identity/protocols/oauth2)

Google app passwords require two-step verification and may be unavailable with work/school restrictions, Advanced Protection or security-key-only setup. OAuth remains the recommended route. [Google app passwords](https://support.google.com/accounts/answer/185833?hl=en) Microsoft Support's Outlook.com page explicitly requires OAuth2 but also retains an app-password note for devices; Mailven's supported Microsoft mail flow is OAuth, and the guide should not generalize this into “Microsoft never offers app passwords.” Exchange Online Basic Authentication retirement is a separate issue. [Outlook.com protocol settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com), [Revised SMTP Basic Auth timeline](https://techcommunity.microsoft.com/blog/exchange/updated-exchange-online-smtp-auth-basic-authentication-deprecation-timeline/4489835)

Microsoft desktop apps should use authorization code with PKCE and the registered native redirect; consent depends on the permissions, publisher and tenant policy. Admin consent does not by itself enable IMAP or SMTP AUTH. The existing `common` endpoint supports the intended personal/work-school audience when the app registration permits it. [Microsoft authorization-code flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow), [Microsoft consent overview](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/user-admin-consent-overview)

Microsoft's revised January 2026 announcement leaves SMTP AUTH Basic Authentication behavior unchanged through December 2026; at the end of December it becomes disabled by default for existing tenants, with an administrator override, and unavailable by default for new tenants created afterward. Microsoft plans to announce the final removal date in the second half of 2027. Earlier March/April 2026 removal dates are superseded. This changes **Basic Authentication**, not a blanket retirement of OAuth SMTP AUTH. [Revised SMTP Basic Auth timeline](https://techcommunity.microsoft.com/blog/exchange/updated-exchange-online-smtp-auth-basic-authentication-deprecation-timeline/4489835)

Security defaults can disable SMTP AUTH even when an OAuth grant succeeds. Preserve the organization's security policy: diagnose the restriction and consider compatible approaches in P2, rather than treating disabling security defaults as the normal setup step. IMAP may independently be disabled. [Exchange Online SMTP AUTH](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/authenticated-client-smtp-submission), [Mailbox email-app controls](https://learn.microsoft.com/en-us/Exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes)

### Communication, limits and submission outcomes

Gmail documents OAuth-authenticated IMAP session lifetimes tied roughly to access-token validity, commonly one hour, and reconnection with a valid token after expiry. Mailven must recover from server closure rather than promise that a long-lived session never needs new credentials. [Gmail IMAP, POP and SMTP](https://developers.google.com/workspace/gmail/imap/imap-smtp)

Workspace quotas differ by account and transport: published paid-account examples include 2,000 messages per rolling 24 hours, 100 recipients per SMTP message and 500 per Gmail API message. Trial accounts have lower quotas; limits can change without notice. These are not personal-Gmail quotas or a per-Mailven budget: other clients and aliases consume the same mailbox allowance. [Workspace sending limits](https://knowledge.workspace.google.com/admin/gmail/gmail-sending-limits-in-google-workspace?hl=en)

Exchange Online publishes 10,000 recipients per day and 30 messages per minute, plus configurable recipient/message-size limits and tenant external-recipient limits. Default `onmicrosoft.com` domains have a separate 100-external-recipient/24-hour organization limit. Actual settings and license-dependent tenant limits must be checked rather than presented as guaranteed available capacity. Microsoft recommends specialist services for bulk commercial email. [Exchange Online limits](https://learn.microsoft.com/en-us/office365/servicedescriptions/exchange-online-service-description/exchange-online-limits)

Treat temporary rejection, permanent rejection and **unknown submission outcome** separately. A connection failure while awaiting the final DATA reply can occur after the server accepted the message; blindly retrying can duplicate mail. Conversely, successful submission transfers responsibility to the server and still allows a later bounce or junk placement. A read receipt is not delivery proof. These conclusions follow the SMTP transaction semantics, not a live Mailven delivery measurement. [RFC 5321](https://www.rfc-editor.org/rfc/rfc5321)

### Message format, sender identity and domain authentication

Generate structurally valid messages with Date, one From, a valid Message-ID, correct MIME and threading headers. RFC 5322 has a 998-character physical-line ceiling and a recommended 78-character length. Use RFC 2047 for appropriate non-ASCII header text and RFC 2231 for attachment parameters; SMTPUTF8 is a distinct requirement for internationalized mailbox addresses. Unicode body text does not require a Unicode envelope. [RFC 5322](https://www.rfc-editor.org/rfc/rfc5322), [RFC 2047](https://www.rfc-editor.org/rfc/rfc2047), [RFC 2231](https://www.rfc-editor.org/rfc/rfc2231), [RFC 6531](https://www.rfc-editor.org/rfc/rfc6531)

Gmail enforces additional header limits: 500 KB total, 5,000 fields, 32 KB for most individual values (exceptions include To/Cc/References), 500 characters for Message-ID and 998 for Subject. Folding does not remove a total-value limit. The current official header-limit page was updated October 6, 2026. [Gmail header limits](https://knowledge.workspace.google.com/admin/gmail/gmail-message-header-limits?hl=en)

Gmail send-as settings distinguish primary and custom From addresses, with verification status and optional external SMTP configuration. A verified alias is evidence of sender setup, not proof of aligned domain authentication through every route. [Gmail send-as resource](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs) For Exchange submission as a different mailbox, the authenticating user needs Send As permission; read access to a shared mailbox alone is insufficient. The documented shared-mailbox XOAUTH2 flow uses a delegated user's token and the shared-mailbox address as the SASL username. Mailven's parent-authenticated SMTP route must be evaluated as a Send As submission, separately from its shared-address IMAP route. [Microsoft OAuth for IMAP/POP/SMTP](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth), [Exchange client submission](https://learn.microsoft.com/en-us/exchange/mail-flow-best-practices/how-to-set-up-a-multifunction-device-or-application-to-send-email-using-microsoft-365-or-office-365)

SPF checks the envelope sender's authorization; DKIM verifies a domain signature; DMARC relates an authenticated domain to the visible From domain. Published records alone do not prove that a particular outgoing message passes these checks. Microsoft recommends configuring all three for owned domains; custom-domain DKIM also needs signing enabled at the provider, not just DNS records. S/MIME/PGP signatures are separate from DKIM. [Microsoft email authentication](https://learn.microsoft.com/en-us/defender-office-365/email-authentication-about), [Microsoft custom-domain DKIM](https://learn.microsoft.com/en-us/defender-office-365/email-authentication-dkim-configure)

Mailven should submit through the authorized provider and preserve accurate identities. It should not fabricate Received, Return-Path or Authentication-Results headers to claim delivery/authentication; Return-Path is inserted at final SMTP delivery. The provider owns relay IP reputation, reverse DNS and DKIM signing; a laptop's local EHLO address is not the provider's outbound relay identity. [RFC 5321](https://www.rfc-editor.org/rfc/rfc5321)

### Spam and bulk-mail context

For delivery to personal Gmail, baseline requirements include SPF or DKIM, valid relay DNS, TLS and compliant messages. Prefer SPF, DKIM and DMARC together. Gmail recommends reported spam below 0.1% and avoiding 0.3% or higher; accurate sender/content and wanted correspondence matter. Google explicitly cannot guarantee spam-filter passage. [Gmail sender guidelines](https://support.google.com/mail/answer/81126?hl=en)

Gmail's bulk classification counts the primary domain across senders/subdomains and can persist after volume falls; a normal desktop user can belong to an already-classified domain. Bulk marketing/subscription mail additionally needs aligned authentication and working one-click unsubscribe plus a visible body link. This is domain/list-operator context, not an instruction to add unsubscribe headers to ordinary correspondence. [Gmail sender guidelines](https://support.google.com/mail/answer/81126?hl=en), [Gmail sender FAQ](https://support.google.com/mail/answer/14229414?hl=en)

Microsoft's consumer Outlook high-volume announcement requires SPF, DKIM and DMARC for domains sending over 5,000 messages daily, and its April 29 update specifies rejection from May 5, 2025, rather than only initial junk routing. Do not apply that consumer-recipient threshold as an Exchange Online submission quota. [Outlook consumer high-volume requirements](https://techcommunity.microsoft.com/blog/microsoftdefenderforoffice365blog/strengthening-email-ecosystem-outlook%E2%80%99s-new-requirements-for-high%E2%80%90volume-senders/4399730) Exchange Online also scans outbound messages, can use a high-risk delivery pool and can block abusive sending; API submission would not bypass these controls. [Microsoft outbound spam controls](https://learn.microsoft.com/en-us/defender-office-365/outbound-spam-protection-about)

## Current implementation findings

Capabilities below are **confirmed by static inspection**, severity **informational**, owner **Mailven maintainers**, applicable to the relevant built-in provider flow. They are not a complete security assessment or a live interoperability pass.

| Capability | Code evidence and limit of the conclusion |
| --- | --- |
| OAuth desktop flow | `internal/oauth2/flow.go:107–138,327–354,448–507`; `internal/oauth2/server.go:58,151–177`: random PKCE/state, S256, loopback listener, state checked before accepting callback; HTTPS endpoint/redirect protections and a 30-second HTTP timeout. |
| Provider scopes and credential persistence | `internal/oauth2/providers.go:25–63`: Google mail/contacts/identity scopes; Microsoft Outlook IMAP/SMTP, offline_access/openid/email, no mail-grant Contacts.Read. `internal/credentials/oauth.go:299–327`: refresh-token storage uses keyring or encrypted database fallback. Private registration/grants remain unverified. |
| Automatic refresh | `app/compose.go:66–146`: refresh within five minutes of expiry, per-account/client-config lock, re-read after locking, retain/replace refresh token and store new tokens; invalid_grant emits reauthorization. Storage errors warn and continue with an in-memory token. |
| IMAP/IDLE recovery | `internal/imap/pool.go:43–65,78–99,135–155`; `internal/imap/idle.go:147–240,291–355,417–470`; `app/compose.go:173–205`: pool capped at three, NOOP health checks, IDLE retry/backoff and ten-minute cycling, reconnect credential callback obtains a valid token. Existing authenticated sessions are not refreshed merely because wall-clock token expiry passed. |
| TLS and certificate checks | `internal/smtp/client.go:119–195`; `app/compose.go:436`; `internal/certificate/verifier.go:16–49,53–75`: configured TLS/mandatory STARTTLS, no plaintext fallback for STARTTLS, system chain/hostname verification with explicit host/fingerprint trust exceptions. `InsecureSkipVerify` enables a custom verifier; it is not evidence that validation is simply disabled. SMTP defaults use 30-second connect/read/write deadlines (`client.go:19–43,80–89`). |
| MIME and Bcc | `internal/smtp/message.go:32–37,105–194,205–259,527–561`: subject/display-name encoding, UTF-8 quoted-printable bodies, multipart/base64 attachments, generated Date/Message-ID and threading, CRLF stripping/folding; Bcc excluded from RFC822 but included in `AllRecipients`. Legacy Content-Type name encoding is a compatibility accommodation, not a blanket standards-compliance claim. |
| Submission and encryption | `app/compose.go:367–413,464–509`; `internal/smtp/client.go:351–394`: construct MIME, apply configured S/MIME/PGP, authenticate with XOAUTH2, check final DATA close result; later Sent/contact/draft cleanup failures do not convert a successful submission into a failed send. |

The following **confirmed limitations/failures remain unfixed** in this research milestone. All are registered in `ISSUES.md`; severity is medium unless explicitly low. Ownership is Mailven maintainers. Risk descriptions are inferred consequences of the confirmed behavior, not claims that a particular account failed.

| ID / applicability | Finding and evidence | Consequence / next verification |
| --- | --- | --- |
| PD1 — Gmail and Microsoft setup, medium | `docs/user-guide/getting-started/first-account/gmail.md:77–84` and `outlook.md:51–58` prescribe reauthorization for routine expiry; `app/compose.go:74–146` already refreshes. `outlook.md:30–35` claims contact permission in the mail grant; `internal/oauth2/providers.go:51–58,122–138` separates that grant. | Misleading setup/recovery guidance. Correct the existing guides in a follow-up, distinguishing expiry from rejected refresh grants and separate contact authorization. |
| PD2 — Outlook.com defaults/settings, medium | `app/oauth.go:179–192` uses `smtp.office365.com` for all Microsoft accounts; `outlook.md:20–49` does not explain Outlook.com's IMAP opt-in and makes an absolute app-password statement. Published personal SMTP is `smtp-mail.outlook.com` [Outlook.com protocol settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com). | Confirmed mismatch with documented defaults, not a proven failure of the current endpoint. Distinguish personal versus tenant setup; verify endpoint compatibility before migration. |
| PD3 — all SMTP sends, medium | `internal/smtp/client.go:375–386` returns a generic error on write/final-reply failure; `app/send_queue.go:23–28,106–120` exposes an error string/draft ID, with no outcome state. | Users cannot distinguish definite failure from possibly accepted mail. Use a controlled disconnect-after-DATA fixture in future executable verification; no automatic resend was found. |
| PD4 — all composition, low | `internal/smtp/message.go:213–234,256–259` folds at spaces but leaves long tokens intact. A 1,100-character ASCII Subject with no spaces remains one overlong physical line. No Subject-value limit is checked before serialization. | Confirmed construction path can exceed RFC/Gmail limits [RFC 5322](https://www.rfc-editor.org/rfc/rfc5322), [Gmail header limits](https://knowledge.workspace.google.com/admin/gmail/gmail-message-header-limits?hl=en); reject or encode safely without silently changing content. Future fixture must cover long unbroken subjects and oversized folded values. |
| PD5 — providers with custom-domain identities, medium | `app/sendas.go:17,46–57,94–100` restricts send-as lookup to Google OAuth and caches it; `frontend/src/lib/components/composer/SendAsWarning.svelte:20–35` warns for pending/not-configured, hides failed lookups; `app/compose.go:475–481` submits the selected From without a send-as gate. | Known-unverified and unknown authorization need clear distinction. There is no corresponding Microsoft permission evidence in this path. Provider enforcement remains authoritative; do not mistake warning absence for authorization. |
| PD6 — all SMTP sends, medium | `internal/smtp/client.go:351–386` neither queries SIZE nor preflights final MIME size; `frontend/src/lib/components/composer/Composer.svelte:365–366` accepts individual attachments up to 100 MB. | Mail can be composed that the selected server rejects; per-file limits ignore encoding, total attachments and encryption overhead. Actual provider rejection threshold is unverified. |
| PD7 — Sent copies, medium | `app/compose.go:484–491,1242–1255` suppresses APPEND based on substring matches of IMAP hosts, independently of SMTP route or server Sent settings. Exchange Online exposes `MessageCopyForSMTPClientSubmissionEnabled`, including a false setting [Set-Mailbox Sent-copy settings](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-mailbox?view=exchange-ps#-messagecopyforsmtpclientsubmissionenabled). | Confirmed heuristic cannot account for disabled server copying or unrelated SMTP routes; missing/duplicate copies are possible. Verify actual personal/shared Sent destinations before changing policy. |
| PD8 — internationalized addresses, low | `internal/smtp/client.go:351–372` sends raw envelope addresses without a capability-required check or domain normalization. `net/smtp.Client.Mail` **does add SMTPUTF8 when advertised**, as verified in installed Go source and [Go net/smtp Client.Mail](https://pkg.go.dev/net/smtp#Client.Mail). | Missing preflight applies when SMTPUTF8 is absent, not when it is advertised. Determine IDN-domain and non-ASCII-local-part behavior separately; reject unsupported envelopes clearly while retaining international message text. |
| PD9 — new grants with partially granted scopes, medium | `internal/oauth2/flow.go:38` receives a token-response Scope; `app/oauth.go:214–216,482–486` stores requested provider scopes rather than the granted response. Google instructs clients to inspect granted scopes [Google OAuth lifecycle](https://developers.google.com/identity/protocols/oauth2). | Stored metadata can overstate consent. A controlled partial-consent response must prove capability handling before a fix; this audit confirms the metadata mismatch, not every downstream failure. |

### Unverified account-dependent findings

These are **informational unless stated otherwise** and are not confirmed defects for the issue register.

| ID / applicability / owner | Evidence and unanswered question | How to establish it |
| --- | --- | --- |
| U1 — all OAuth accounts; user/administrator | Repository credentials configuration cannot establish client type, publishing/verification status, effective consent or conditional-access policy. | Inspect the user's own registration and grant with redacted evidence; never copy tokens or private credentials into the report. |
| U2 — custom domains; domain administrator/provider | SPF/DKIM/DMARC, signing, relay reputation and receiver treatment are external. | Inspect authoritative DNS and a separately authorized received message's trusted Authentication-Results/DKIM headers. Presence alone is insufficient. |
| U3 — shared mailboxes, low; Mailven/tenant administrator | `app/account.go:101–115,152–159` copies parent tokens to a child; `app/compose.go:81,104–114` refreshes per account; `app/oauth.go:497–503` propagates only explicit reauthorization. Storage/locks can diverge, but Microsoft does not revoke an old refresh token merely because it was used [Microsoft refresh tokens](https://learn.microsoft.com/en-us/entra/identity-platform/refresh-tokens). | Test expiration, revocation and reauthorization across linked accounts. Do not claim rotation alone causes failure. Separately verify shared IMAP access, parent-authenticated SMTP Send As and Sent destination. |
| U4 — Gmail/Microsoft MIME/encryption; Mailven/provider | Existing encoding/signing paths establish construction, not what providers preserve or rewrite. | Authorized received-message fixtures for international content, inline/regular attachments, replies, Bcc-only and signed/encrypted mail. No content-based spam score is supported by this audit. |

## Responsibility and operating guidance

| Party | Controls | Evidence boundary |
| --- | --- | --- |
| Mailven | Secure auth/transport, valid messages, faithful sender identities, safe outcome reporting, capability/limit handling and usable diagnostics. | Static code evidence above; no ability to guarantee receiver classification. |
| Mailbox provider | Submission acceptance, relay TLS/DNS/IP reputation, signing when configured, abuse controls and server Sent behavior. | Provider documentation; actual account/route still needs verification. |
| Domain/tenant administrator | Allowed clients/protocols, consent, delegation, domain SPF/DKIM/DMARC and signing configuration. | Private console/DNS/received-message checks needed. |
| Sender and recipient | Wanted correspondence, accurate content, recipient complaints, filtering rules, quarantine and user spam decisions. | Message-specific behavior cannot be inferred from client branding or header cosmetics. |

For ordinary correspondence, use the authorized provider route and an authorized From identity; have the domain administrator enable and verify aligned authentication for custom domains. Check actual rejection/bounce details before retrying. Examine received headers and provider logs when available, and distinguish missing delivery, quarantine and junk placement. A recipient can correct an erroneous spam classification, but this is not a general bypass. API transports, transport selection and the in-app diagnostic remain P2 work.

## Official sources

All reviewed on the date above; section names identify the relevant material. These are primary sources, not third-party deliverability scores.

| ID | Source / reviewed material |
| --- | --- |
| G1 | [Add Gmail to another email client](https://support.google.com/mail/answer/7126229?hl=en): personal-account IMAP and OAuth changes. |
| G2 | [Gmail IMAP, POP and SMTP](https://developers.google.com/workspace/gmail/imap/imap-smtp): endpoints and session length. |
| G3 | [Workspace POP/IMAP controls](https://knowledge.workspace.google.com/admin/sync/turn-pop-and-imap-on-or-off-for-users): administrative/client controls; conflicting legacy instructions excluded. |
| G4 | [Google app passwords](https://support.google.com/accounts/answer/185833?hl=en): preference and eligibility. |
| G5 | [Google desktop OAuth](https://developers.google.com/identity/protocols/oauth2/native-app): PKCE, client type, loopback. |
| G6 | [Gmail XOAUTH2](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol): full-mail scope and SASL exchange. |
| G7 | [Gmail API scopes](https://developers.google.com/workspace/gmail/api/auth/scopes): restricted scopes and review requirements. |
| G8 | [Google OAuth lifecycle](https://developers.google.com/identity/protocols/oauth2): inspect granted scopes and refresh-token expiration. |
| G9 | [Workspace sending limits](https://knowledge.workspace.google.com/admin/gmail/gmail-sending-limits-in-google-workspace?hl=en): rolling quotas and SMTP/API differences. |
| G10 | [Gmail header limits](https://knowledge.workspace.google.com/admin/gmail/gmail-message-header-limits?hl=en): physical versus value/aggregate limits. |
| G11 | [Gmail send-as resource](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs): identity verification and SMTP configuration. |
| G12 | [Gmail sender guidelines](https://support.google.com/mail/answer/81126?hl=en): authentication, formatting, spam rates and delivery limits. |
| G13 | [Gmail sender FAQ](https://support.google.com/mail/answer/14229414?hl=en): domain aggregation and lasting bulk classification. |
| M1 | [Outlook.com protocol settings](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com): endpoints, OAuth and IMAP opt-in. |
| M2 | [Microsoft OAuth for IMAP/POP/SMTP](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth): Outlook scopes and shared-mailbox SASL. |
| M3 | [Exchange Online SMTP AUTH](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/authenticated-client-smtp-submission): organization/mailbox controls and security defaults. |
| M4 | [Exchange client submission](https://learn.microsoft.com/en-us/exchange/mail-flow-best-practices/how-to-set-up-a-multifunction-device-or-application-to-send-email-using-microsoft-365-or-office-365): TLS, endpoint, Send As and Sent copies. |
| M5 | [Mailbox email-app controls](https://learn.microsoft.com/en-us/Exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes): independent IMAP/SMTP availability. |
| M6 | [Revised SMTP Basic Auth timeline](https://techcommunity.microsoft.com/blog/exchange/updated-exchange-online-smtp-auth-basic-authentication-deprecation-timeline/4489835): January 2026 announcement, updated January 29. |
| M7 | [Microsoft authorization-code flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow): native clients, PKCE, redirect and audience. |
| M8 | [Microsoft consent overview](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/user-admin-consent-overview): policy-dependent user/admin consent. |
| M9 | [Exchange Online limits](https://learn.microsoft.com/en-us/office365/servicedescriptions/exchange-online-service-description/exchange-online-limits): mailbox, tenant and onmicrosoft.com limits. |
| M10 | [Microsoft email authentication](https://learn.microsoft.com/en-us/defender-office-365/email-authentication-about): SPF, DKIM, DMARC and alignment. |
| M11 | [Microsoft custom-domain DKIM](https://learn.microsoft.com/en-us/defender-office-365/email-authentication-dkim-configure): DNS and signing activation. |
| M12 | [Outlook consumer high-volume requirements](https://techcommunity.microsoft.com/blog/microsoftdefenderforoffice365blog/strengthening-email-ecosystem-outlook%E2%80%99s-new-requirements-for-high%E2%80%90volume-senders/4399730): April 29 update supersedes initial junk-only wording. |
| M13 | [Microsoft outbound spam controls](https://learn.microsoft.com/en-us/defender-office-365/outbound-spam-protection-about): high-risk pool and sending blocks. |
| M14 | [Set-Mailbox Sent-copy settings](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-mailbox?view=exchange-ps#-messagecopyforsmtpclientsubmissionenabled): SMTP/submission and delegation copy settings. |
| M15 | [Microsoft refresh tokens](https://learn.microsoft.com/en-us/entra/identity-platform/refresh-tokens): rotation without automatic old-token revocation, expiry and revocation. |
| R1 | [RFC 5321](https://www.rfc-editor.org/rfc/rfc5321): SMTP transaction completion, Return-Path and ambiguous final-reply failures. |
| R2 | [RFC 5322](https://www.rfc-editor.org/rfc/rfc5322): headers and line lengths. |
| R3 | [RFC 2047](https://www.rfc-editor.org/rfc/rfc2047): international header text. |
| R4 | [RFC 2231](https://www.rfc-editor.org/rfc/rfc2231): international MIME parameters. |
| R5 | [RFC 6531](https://www.rfc-editor.org/rfc/rfc6531): SMTPUTF8 capability requirements. |
| R6 | [Go net/smtp Client.Mail](https://pkg.go.dev/net/smtp#Client.Mail): extension behavior, corroborated with installed `/usr/lib/go/src/net/smtp/smtp.go:245–262`. |

## Validation record and remaining limitations

Acceptance mapping: authentication and settings are covered by the account matrix and OAuth paragraphs; communication by limits/outcomes and recovery findings; MIME/identity by the formatting paragraphs and capability/failure tables; deliverability by the domain/bulk paragraphs and responsibility table. PD1–PD9 have matching issue-register entries. P2 architecture comparison and diagnostics are explicitly pending.

Verification uses targeted `rg` and numbered reads of the referenced source and guides, primary-source browsing, and independent read-only audit/review. Microsoft Community Hub article bodies were unavailable to the web reader; the two announcement bodies were additionally read from their public HTML `__NEXT_DATA__` article data using Python `urllib.request`/`json`, with only relevant text inspected. No authenticated provider access was used.

No application builds, Go/frontend tests, manual UI checks, test emails, DNS mutations or tenant changes were performed. Existing automated tests were not rerun because this milestone changes documentation only. No baseline executable failure is claimed. Live console policy, received-message authentication, provider rewriting, Sent destinations and inbox placement remain unverified.

Independent native `project-acceptance-checker` review passed all eight P1 criteria, checked consequential source/code claims, and reported no corrections or placeholder implementations. The primary agent performed direct diff review in place of a separate code-review tool and reviewed documentation simplification; no application-code cleanup applied. `git diff --check cdf6b6437a8aa1c88d1cf057a5cc9de2518f78dd -- PLAN.md ISSUES.md docs/validation/provider-deliverability-review.md` passed. Read-only reference/runtime directories were unchanged.

Reproduce the narrow evidence checks from the repository root (bounds establish reference validity; semantic correctness was independently reviewed):

```sh
python3 - <<'PY_CHECK'
from pathlib import Path
import re
s = Path('docs/validation/provider-deliverability-review.md').read_text()
refs = re.findall(r'^\| ((?:G|M|R)\d+) \| \[([^\]]+)\]\((https://[^)]+)\)', s, re.M)
assert len(refs) == 34
count = 0
for path, nums in re.findall(r'`((?:app|internal|frontend|docs)/[^` :]+\.(?:go|svelte|md)):(\d+(?:[–,]\d+)*)`', s):
    lines = Path(path).read_text().splitlines()
    assert all(1 <= int(n) <= len(lines) for n in re.findall(r'\d+', nums))
    count += 1
assert set(re.findall(r'PD\d+', s)) == set(re.findall(r'PD\d+', Path('ISSUES.md').read_text())) == {f'PD{i}' for i in range(1, 10)}
assert '### P2' in Path('PLAN.md').read_text()
print(f'PASS: 34 sources; {count} reference bounds; PD1-PD9 coverage; P2 retained')
PY_CHECK
```

A source-backed audit establishes current requirements and code gaps, not runtime compliance or guaranteed deliverability.
