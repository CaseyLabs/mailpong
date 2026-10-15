# DavMail Microsoft 365 experiment

Date: 2026-10-14, America/Vancouver (observations extend into 2026-10-15 UTC). Milestone: [SEND-4](../../PLAN.md#send-4). [Procedure and continuation checkpoint](../plans/davmail-microsoft365-test.md).

**Decision: partially verified; further work deferred by owner decision 2026-10-14.** DavMail's Outlook desktop client-ID flow authenticated the authorized university account and exposed Graph-backed local IMAP/SMTP to an isolated Mailven profile. Synthetic self-mail sending, receiving, attachments, replies and read flags worked. A configuration workaround was necessary for token refresh; normal expiry-driven renewal subsequently passed. Folder moves did not fully converge in Mailven, including after restart and serialized refresh. This configuration is not yet accepted for routine use.

## Scope and baseline

The user authorized the test account in `.env`, OAuth sign-in/consent/token storage and synthetic **self-mail only**, then explicitly selected the DavMail Outlook-client-ID route. No external recipient was addressed, tenant policy changed, administrator request submitted, application registered, existing Mailven account migrated or grant revoked. Webmail indicated existing forwarding; its configuration was untouched, and the destination mailbox was not accessed. Self-mail scope describes addressed recipients, not a guarantee against existing server forwarding.

- Branch: `experiment/davmail-microsoft365`; Mailven source baseline: `1e1368e2c70ac5ddd5c6ab75a531a22a18884d9b`.
- Linux `7.2.9-2-cachyos x86_64`; OpenJDK 27, build 27; Go `go1.27.1-X:nodwarf5`; Node `v26.11.1`. No application source or project dependency changes.
- Official SourceForge package `davmail-7.0.0-4403.zip`; locally measured SHA-256 `cd137f36929b296ae80f199a98c09d626b7e5adb73764568f8b458595154fdc6`. This identifies the tested archive; it is not an independently authenticated vendor checksum. [Download instructions](https://davmail.sourceforge.net/download.html), [7.0.0 release notes](https://github.com/mguessan/davmail/blob/7.0.0/RELEASE-NOTES.md).
- Pinned upstream source: tag 7.0.0, commit `494f025a4be0fcccbcad7239a321ebba8f0dc699`. SWT 4.40 was installed into the private gateway runtime from official Eclipse distribution using the upstream launcher. No project dependency was upgraded.
- The user completed embedded-browser sign-in/MFA and reported completion. The account identity matched the authorized mailbox. A distinct UPN and network/VPN requirements were not independently established.

## Authentication phases and actual grant

The initial DavMail application (`facd6cff-a294-4415-b59f-c5b01937d7bd`) with OIDC scopes `openid profile offline_access Mail.ReadWrite Mail.Send` reached an explicit **Approval required** page for DavMail. It listed basic profile, persistent access, mail read/write and sending. No approval request was submitted and that phase obtained no token or mailbox access.

The authorized follow-up used Outlook desktop client ID `d3590ed6-52b3-4102-aeff-aad2292ab01c`, `common`, redirect `urn:ietf:wg:oauth:2.0:oob`, Graph resource authorization and `enableOidc=false`, as described in [upstream issue 404](https://github.com/mguessan/davmail/issues/404). The normal Brave flow stalled after the Microsoft Office sign-in confirmation without returning an authorization code or visible authentication error. Repeating Continue did not resolve it. The supported embedded SWT interactive flow then succeeded after the user completed sign-in. No new consent/permission-expansion screen was accepted by the agent.

**The resulting Office grant is broad, not mail-only.** Token scope metadata reported the following values; disabling calendar/contact listeners does not narrow this grant. These values do not prove that every operation is permitted by tenant policy. Only authorized own-mail operations were exercised.

```text
_J.AB AuditLog.Create Calendar.ReadWrite Calendars.Read.Shared
Calendars.ReadWrite Channel.Create Channel.Delete.All Channel.ReadBasic.All
ChannelMember.ReadWrite.All ChannelMessage.Read.All ChannelMessage.ReadWrite ChannelMessage.Send
ChannelSettings.ReadWrite.All Chat.Create Chat.ReadWrite ChatMember.ReadWrite
Contacts.ReadWrite Content.Process.User DataLossPreventionPolicy.Evaluate Directory.AccessAsUser.All
Directory.Read.All Files.Read Files.Read.All Files.ReadWrite.All
FileStorageContainer.Selected Group.Read.All Group.ReadWrite.All InformationProtectionPolicy.Read
Mail.ReadWrite Mail.Send Notes.Create Organization.Read.All
People.Read People.Read.All Printer.Read.All PrinterShare.ReadBasic.All
PrintJob.Create PrintJob.ReadWriteBasic ProtectionScopes.Compute.User Reports.Read.All
ResourceSpecificPermissionGrant.ReadForTeam SensitiveInfoType.Detect SensitiveInfoType.Read.All SensitivityLabel.Evaluate
Tasks.ReadWrite Team.Create Team.ReadBasic.All TeamMember.ReadWrite.All
TeamsAppInstallation.ReadForTeam TeamSettings.ReadWrite.All TeamsTab.ReadWrite.All TeamsTab.ReadWriteForChat
User.Read.All User.ReadBasic.All User.ReadWrite Users.Read
```

## Effective working configuration

```properties
davmail.server=false
davmail.mode=O365Graph
davmail.authentication=O365Interactive
davmail.bindAddress=127.0.0.1
davmail.allowRemote=false
davmail.imapPort=1143
davmail.smtpPort=1025
davmail.popPort=0
davmail.caldavPort=0
davmail.ldapPort=0
davmail.ssl.nosecureimap=true
davmail.ssl.nosecuresmtp=true
davmail.imapAutoExpunge=false
davmail.smtpSaveInSent=true
davmail.disableUpdateCheck=true
davmail.enableOidc=false
davmail.oauth.clientId=d3590ed6-52b3-4102-aeff-aad2292ab01c
davmail.oauth.tenantId=common
davmail.oauth.redirectUri=urn:ietf:wg:oauth:2.0:oob
davmail.outlookUrl=https://graph.microsoft.com
davmail.oauth.tokenFilePath=<private-experiment>/tokens.properties
davmail.logFilePath=<private-experiment>/evidence/davmail.log
```

No mail-only `oauth.scope` override was used in the classic resource flow. All gateway listeners owned by its recorded PID were loopback-only, ports 1143 and 1025; unused services were disabled. Mailven used its manual/password route, IPv4 `127.0.0.1`, these ports, connection security `none`, authentication `auto`, and the same private independent gateway secret for IMAP/SMTP. The Microsoft password was not the protocol secret. Upstream [token implementation](https://github.com/mguessan/davmail/blob/7.0.0/src/java/davmail/exchange/auth/O365Token.java) encrypts stored refresh tokens using that secret.

Initial authorization enumerated eight mailbox folders and SMTP AUTH returned 235. Without the `outlookUrl` override, restart loaded a refresh token but returned empty LIST and HTTP 401 `InvalidAuthenticationToken`: “Access token validation failure. Invalid audience.” Classic refresh requests the default Outlook resource although initial authorization requests Graph. Setting `outlookUrl` to Graph restored folder access. This is a tested workaround for this Graph-only configuration, not a general EWS recommendation; [DAV1](../../ISSUES.md#dav1) remains open upstream.

Normal logs remained WARN. A private external log4j configuration temporarily enabled only `davmail.exchange.auth.O365Token=DEBUG` to establish scopes/expiry; raw logs include identity metadata and stay outside Git. WIRE logging was not enabled.

## Mailven interoperability and lifecycle evidence

Mailven's independent IMAP and SMTP connection tests passed, and eight original folders appeared. **Check account setup** skipped loopback transport/auth checks and returned unknown permission/sender evidence; it did not independently prove protocol authentication or delivery. Sent policy was explicitly set to **provider-managed** before sending.

Two uniquely identified original messages were submitted once: a gateway-generated receive seed and a Mailven-generated send. Each addressed only the test account and carried a small harmless text attachment. Independent webmail inspection and targeted IMAP BODY.PEEK verified content; Mailven verified the seed body, sender, attachment filename and SHA-256 match. Mailven's actual SendMessage binding succeeded with reviewed own-sender evidence. Recipient body/attachment hashes matched, and targeted Sent inspection showed exactly one original Mailven-sent copy, including in the final inventory. No uncertain send was retried. Receive/send end-to-end latency was not reliably measured.

A webmail self-reply arrived with the expected body and In-Reply-To; Mailven's cached message showed both. On the seed, Mailven read/unread changes matched server flags (about 1.06 seconds of verification observation, not total action latency). The reverse webmail flag change converged through background polling in **111.98 seconds** with a one-minute configured interval and no manual refresh during observation.

Mailven moved the seed into the dedicated `-MOVED` folder, confirmed over IMAP and in webmail. Webmail moved that single seed back to the test root. Mailven then logged repeated **120000 ms pool-acquisition timeouts** and retained a negative temporary UID in the destination. UI/background synchronization was present; one probe was canceled. A duplicated read-only probe was stopped; earlier timeouts predate that duplicate. After restarting Mailven, disabling polling, avoiding UI-mounted probes and refreshing the two synthetic folders serially, Mailven still displayed the seed in both folders. A fresh read-only gateway inventory showed exactly one seed in the root and zero messages in `-MOVED`. This establishes a stale local move copy; it does not establish a server duplicate, pool leak or gateway deadlock. [DAV3](../../ISSUES.md#dav3) records the unresolved failure and closure criteria.

Search checks also exposed false negatives: HEADER SUBJECT missed the seed while SUBJECT found it; a final broad SUBJECT search omitted the original send from Sent while a full-subject search found it. Content and exact-subject inventories prevented false delivery conclusions or resends. The cause remains unisolated; [DAV2](../../ISSUES.md#dav2) records these observations.

With the audience workaround, stored-token restart recovered mailbox access without a fresh sign-in. A **real expiry-driven renewal** was observed at `2026-10-15T01:19:41Z`: the prior expiry was 18:20:29 PDT, the gateway logged “Access token expires soon, trying to refresh it”, and the new expiry was 19:25:27 PDT. A subsequent Graph-backed IMAP operation returned OK. DavMail was not restarted during this observation, and neither the clock nor token expiry was edited. Renewal occurred within its normal 60-second pre-expiry window.

For temporary gateway loss, Mailven's connection test returned connection refused. Restarting DavMail from stored credentials restored the connection test and folder refresh without sign-in; the stale local move copy remained. Receipt of a newly submitted message after the final restart was not tested. Thus transport recovery and renewal passed, but full lifecycle mailbox convergence did not.

## Acceptance matrix

| ID | Status | Evidence / limit |
| --- | --- | --- |
| D1 | PASS | Exactly two loopback gateway listeners; unused services off |
| D2 | PASS | Authorized Outlook-ID embedded flow completed; broad existing Office grant documented; original DavMail-ID gate remains historical |
| D3 | PASS | Intended Graph mailbox folders enumerated; synthetic content accessed |
| D4 | PASS | Separate Mailven IMAP/SMTP tests succeeded; folders appeared; diagnostic skips disclosed |
| D5 | PASS | Seed content and attachment matched in webmail, IMAP and Mailven |
| D6 | PASS | Mailven self-send received with matching content/attachment; exactly one original Sent copy |
| D7 | FAIL | Reply and flags passed; reverse move left a stale local destination copy after restart/serialized refresh |
| D8 | FAIL | Stored-token restart, actual renewal and connection-loss detection/reconnect passed; mailbox convergence failed and new-message receipt after final restart unrun |
| D9 | PASS | Real-backend browser Mail view checked with Accounts expanded/collapsed, exact UI state restored; isolated account removed and task processes stopped |

D9 covers the running app's browser surface; native desktop-window inspection was unavailable and remains unrun. No conclusion extends to external delivery, aliases, shared mailboxes, migration, revocation or broader Office services.

## Isolation, cleanup and continuation

Runtime logs confirmed private XDG config/data/cache directories and encrypted database credential storage. A short private XDG_RUNTIME_DIR avoided the Unix socket path limit. An intentionally unavailable private session D-Bus address prevented use of the existing OS keyring. Existing Mailven profile/account state was untouched. Browser automation probes used real Wails bindings; later state-only probes prevented additional frontend auto-sync during serialized recovery.

UI state was restored exactly. The isolated test account was removed after a private SQLite backup and configuration snapshot; zero test-profile accounts remain. Task-created browser tabs, Mailven dev processes and DavMail were stopped. No display server was started. Final listener/process evidence is retained privately. The private installation, encrypted gateway token, independent secret, rollback snapshot and raw evidence remain outside Git for authorized resumption. No shared Microsoft Office grant was revoked.

Server evidence remains: two dedicated test folders, three synthetic received messages in the root (seed, send and reply), an empty move folder, and one Sent copy of each submitted original/reply. No unrelated message was changed and nothing was permanently deleted. Exact targeted searches are required because broader searches were inconsistent.

Possible future work after explicit owner reopening: isolate and reproduce DAV3 with bounded pool/gateway evidence, then verify clean bidirectional moves and lifecycle convergence. SEND-4 is deferred; do not migrate an existing account or call D1–D9 accepted. DAV1's workaround does not close its upstream defect, and DAV2 remains unexplained. Automated application suites were unrun because the repository change contains documentation only; the real dev app built/launched and the live checks above ran. Documentation validation and independent review are recorded in the checkpoint.
