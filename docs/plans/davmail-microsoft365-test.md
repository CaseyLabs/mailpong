# DavMail + Mailven Microsoft 365 compatibility test

Created: 2026-10-14. Status: proposed test procedure; execution has not started.
Target: an authorized user's own Microsoft 365 mailbox at `cityuniversity.edu`.
Authorization for this task: write this plan only. No application installation, tenant change, sign-in, mailbox access or test message has been performed. The test user's credentials are saved in `.env` and you have permission to use it.

This is a standalone investigation proposal, not a selected roadmap milestone. If selected for substantial implementation work, reconcile it with [project management conventions](../PROJECT_MANAGEMENT.md) and link the selected milestone here before starting that work.

## Goal and decision

Determine whether DavMail can authenticate to the target tenant and expose working local IMAP/SMTP services to Mailven. Distinguish successful Microsoft sign-in, granted mailbox access, Mailven interoperability and actual mail delivery; none proves the others.

Expected topology:

```text
Mailven -- local IMAP/SMTP --> DavMail -- HTTPS + delegated OAuth --> Microsoft Graph
```

DavMail owns the Microsoft OAuth grant and token refresh. Mailven uses its manual IMAP/SMTP route and gateway credentials. Mailven's native Microsoft OAuth and Graph routes are separate alternatives and are outside this test.

## Context and references

Research on 2026-10-14 found that Mailven exposes manual server addresses, ports, password authentication and `none` connection security. Relevant code: [provider configuration](../../frontend/src/lib/config/providers.ts), [account form](../../frontend/src/lib/components/settings/AccountForm.svelte), [account model](../../internal/account/model.go), [IMAP client](../../internal/imap/client.go), [SMTP client](../../internal/smtp/client.go). This is static compatibility evidence, not a live interoperability result.

Read [provider requirements](../PROVIDERS.md), [sending and recovery](../SENDING.md) and the [manual provider guide](../user-guide/getting-started/first-account/other-providers.md) before execution. Use the [run-mailven skill](../../.agents/skills/run-mailven/SKILL.md) when driving the app; explicit live-test authorization governs the synthetic mailbox operations below. Fake UI checks cannot establish live compatibility.

Official upstream references, reviewed during the preceding research:

- [DavMail FAQ](https://davmail.sourceforge.net/faq.html): OAuth/MFA, custom application registration, token renewal and Sent-copy considerations. Some mode names and EWS instructions lag newer documentation.
- [DavMail setup](https://davmail.sourceforge.net/gettingstarted.html): separate `O365Graph` backend and interactive/manual/device-code authentication choices.
- [DavMail release notes](https://raw.githubusercontent.com/mguessan/davmail/master/RELEASE-NOTES.md): 7.0.0 dated 2026-09-19 includes Graph synchronization improvements. Recheck the installed version and current release before testing.
- [Microsoft EWS retirement](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/deprecation-of-ews-exchange-online): published phased disablement starts October 2026, with full disablement listed for April 2027. Reverify the schedule; prefer Graph for this investigation.
- [Microsoft consent experience](https://learn.microsoft.com/entra/identity-platform/application-consent-experience): tenant policy can require administrator approval.

No public evidence reviewed establishes the tenant's consent, Conditional Access or mailbox policy. A blocked Mailven OAuth application does not establish whether DavMail's application is allowed, or vice versa. Disabled Exchange Online IMAP/SMTP may still permit this topology because those protocols terminate locally; that hypothesis must be tested through Graph access.

## Scope and prerequisites

- Use one own-mailbox test account, preferably a university-approved disposable mailbox. Record the actual sign-in UPN separately from the mailbox address; do not assume the email domain identifies the tenant ID or login name.
- Before live execution, establish authorization for that account, local DavMail installation/configuration, OAuth consent/token storage, and the synthetic operations below. Identify controlled recipients for send/reply tests. Do not alter tenant policy or application registrations unless explicitly authorized for that step.
- Start with a fresh, supported DavMail release with `O365Graph`, on the same computer as Mailven. Record package source, version, Java/runtime, OS, Mailven revision and network/VPN requirements. Do not upgrade project dependencies to conduct the experiment.
- Keep a separate Mailven test profile/data directory if supported by the launch path. Verify isolation before launch. Otherwise back up application data with Mailven stopped, document restoration, and avoid changing existing account routes. A local backup cannot undo server-side changes.
- Bind DavMail only to loopback. Disable unused listeners and remote access; inspect the effective listeners. Plain local transport is acceptable only on loopback; any remote deployment requires a separately designed TLS configuration.
- Keep configuration backups and token files private. Record redacted configuration, never passwords, tokens, authorization URLs/codes, raw mail or unreviewed debug logs in Git. Use normal logs first; WIRE logging can expose sensitive material.
- Exclude shared mailboxes, aliases, contacts/calendar integration, migration of an existing Mailven account, bulk operations and tenant security changes. No permanent deletion or access to unrelated messages.

## Progress

- [x] Write the proposed procedure and acceptance matrix.
- [ ] Confirm live-test scope, account, controlled recipients and profile isolation.
- [ ] Inventory versions and configure a loopback-only Graph gateway.
- [ ] Authenticate DavMail and establish mailbox access.
- [ ] Connect Mailven and test receiving, sending and limited synchronization.
- [ ] Verify restart and token refresh behavior.
- [ ] Clean up, record evidence and make the compatibility decision.

## Test procedure

### 1. Capture the baseline

Record date/time/timezone, branch/revision, dirty files, OS, runtime versions, current mailbox address/UPN and any known direct Mailven failure, including its error code and route. Do not reproduce a direct-login failure unnecessarily. Confirm the test account can sign in to university webmail through its normal MFA flow; this is a baseline, not proof of third-party access.

Create a private evidence directory outside the repository. Record test identifiers in the form `MAILVEN-DAVMAIL-<UTC timestamp>-<case>`. Check the intended recipients and confirm that sending is authorized before opening a live composer.

### 2. Configure and authenticate DavMail

1. Select backend `O365Graph` and authentication `O365Interactive`. If the embedded browser cannot complete institutional SSO/MFA, try documented `O365Manual` with the system browser. Use device-code authentication only if tenant policy permits it; record each attempted flow and result.
2. Record the OAuth client ID, authority/tenant selection, redirect URI and requested scopes actually used by this release. Verify Graph delegated mail access is requested; do not copy the FAQ's EWS resource/scopes into the Graph configuration. Record any extra calendar/contact/shared scopes even if those services are disabled; disabling a listener does not prove reduced OAuth permissions.
3. Configure unused high local ports, for example IMAP `1143` and SMTP `1025`, and record their actual values. Verify listeners with `ss -ltnp` on Linux or the OS equivalent; all enabled gateway listeners must be loopback-only.
4. Follow the installed release's documented client-credential handling. Determine which local password/secret the gateway expects and how it protects stored tokens; do not assume it is the Microsoft password or that an arbitrary value works. Keep that value consistent across both client protocols.
5. Trigger a connection with an approved protocol client, or Mailven's account connection test in step 3, if DavMail authenticates lazily. Complete Microsoft sign-in/MFA and consent in the intended tenant. Confirm the displayed identity, application and requested permissions before accepting.
6. Establish that DavMail can enumerate the intended mailbox/folders over Graph. A browser success page or acquired token alone is insufficient. Retain redacted errors, correlation IDs, timestamps and granted-scope evidence where available.

If sign-in requires admin approval, assignment or a compliant device, record the exact gate. Do not cycle through unrelated client IDs or weaken MFA/Conditional Access. A custom app registration is a separately authorized remediation, followed by a fresh test of its actual grant.

### 3. Connect Mailven to the gateway

Add a new **Other / Manual** account; retain the IMAP/SMTP route. Use the mailbox email as the account identity and the gateway's required username (normally the sign-in UPN), confirmed against the installed DavMail release.

| Mailven setting | Initial test value |
| --- | --- |
| Authentication | Password to DavMail; OAuth occurs in DavMail |
| IMAP host / port | `localhost` / actual gateway IMAP port, e.g. `1143` |
| SMTP host / port | `localhost` / actual gateway SMTP port, e.g. `1025` |
| Connection security | None for loopback-only plaintext listeners; otherwise match configured TLS |
| Authentication mechanism | Auto initially; record negotiated mechanisms and any justified PLAIN/LOGIN retry |
| SMTP credentials | Same gateway identity/secret as incoming, unless the release requires otherwise |

Use Mailven's connection tests and **Check account setup** diagnostics. Confirm both protocols authenticate and folders are visible. Save a redacted diagnostic summary. These checks send no test mail and do not prove delivery. If `localhost` resolves to an address DavMail does not bind, record and correct the address/binding mismatch before interpreting it as authentication failure.

### 4. Validate mailbox behavior with synthetic messages

1. From a controlled sender, send a uniquely marked plain-text message and a small harmless attachment to the test mailbox. Confirm both in webmail, then in Mailven; verify sender, subject, body, attachment name/content and receive latency.
2. Establish the intended Sent-copy policy before sending. Select Mailven's provider-managed Sent behavior if DavMail/Graph is saving a copy; verify the installed gateway's actual behavior. Avoid adding an IMAP copy without evidence it is needed.
3. Send one uniquely marked message from Mailven to a controlled recipient. Confirm actual receipt, sender identity, body and attachment content at the recipient; independently inspect the test mailbox's Sent folder for exactly one copy. SMTP acceptance alone is not delivery. Do not automatically resend after an uncertain outcome; inspect recipient/Sent evidence first.
4. Reply from the controlled recipient and verify Mailven receives the reply. If external delivery is part of the intended use, include a separately controlled external recipient; otherwise limit the conclusion to the tested recipient scope.
5. On synthetic messages only, mark read/unread and move into a dedicated test folder; verify changes in webmail. Make the reverse change in webmail and verify Mailven catches up. Record automatic update/polling latency and distinguish manual-refresh success from background synchronization.
6. Review Mail view with Accounts expanded and collapsed. Record usability or connection-status problems, redacting screenshots. This does not replace live protocol evidence.

### 5. Validate persistence and refresh

Restart Mailven, then DavMail, and confirm folder access plus receipt of a newly marked message without an unexpected fresh sign-in. Observe one real access-token expiry/refresh cycle, using the token lifetime or normal redacted gateway logging to establish that refresh occurred. Elapsed time or restart alone does not prove renewal. Do not edit the system clock or commit token contents. If this cannot be observed in the available test window, mark refresh unverified.

Briefly stop only the test gateway and check Mailven reports the connection loss. Restart it and confirm recovery and mailbox convergence without duplicate sends or copies. Leave a deliberate grant-revocation test optional: it requires separate consent to revoke the test application's grant and must not affect shared or production grants.

## Acceptance and evidence matrix

Record each row as PASS, FAIL, BLOCKED or UNRUN, with timestamp and redacted evidence. Do not convert an external gate into a product defect without supporting evidence.

| ID | Criterion | Evidence / expected result |
| --- | --- | --- |
| D1 | Local boundary | Effective configuration and listener inspection: loopback only; unused services off |
| D2 | Tenant sign-in and consent | Correct account/tenant completes required SSO/MFA; approved application grant |
| D3 | Mailbox authorization | DavMail Graph access enumerates the intended mailbox/folders |
| D4 | Mailven connection | Both local IMAP and SMTP authentication succeed; folders appear |
| D5 | Receiving | Webmail and Mailven show the same synthetic message and attachment |
| D6 | Sending | Controlled recipient confirms receipt/content; exactly one Sent copy exists |
| D7 | Synchronization | Reply, flags and folder moves converge in both directions; latency recorded |
| D8 | Lifecycle | Restart and temporary gateway loss recover; real token refresh is observed |
| D9 | UI and cleanup | Both Accounts states checked; original state restored; task-started processes stopped |

Diagnosis categories: local configuration/runtime, OAuth flow, tenant consent/assignment/Conditional Access, Graph permission/mailbox authorization, gateway translation, Mailven protocol compatibility, or delivery. Preserve exact codes such as `AADSTS...`, HTTP status, and protocol responses when available; an IMAP authentication error alone does not identify the upstream cause.

Decision rules:

- **Viable for the tested configuration:** D1–D9 pass; report account type, application identity, versions, recipient scope and observed limitations. This does not certify all tenant users or long-term production readiness.
- **Partially verified:** sign-in/read/connectivity succeeds but sending, synchronization, refresh or other required checks remain unrun or fail. Name each missing capability; do not call the full route working.
- **Blocked by tenant policy:** evidence identifies an unsatisfied institutional gate; state the required administrator decision without changing policy.
- **Incompatible in the tested configuration:** a reproducible failure remains after isolating configuration and policy causes. Specify the failing layer and version; avoid claiming universal incompatibility.

## Cleanup and durable results

Record a sanitized result in `docs/validation/davmail-microsoft365.md` when executed, linked back to this plan. Include the complete matrix, redacted configuration, version/revision, actual scopes, observed refresh, delivery/Sent evidence, failures, unrun checks and recommendation. Keep raw evidence private. Record confirmed product defects left unfixed in [ISSUES.md](../../ISSUES.md) under the applicable project, with reproduction and closure criteria; do not label unknown tenant policy as a confirmed defect.

Stop task-started Mailven/DavMail/display processes by recorded handles. Restore UI state and configuration; remove the test account from Mailven if retention was not requested. Agree whether to retain or revoke the test grant and remove local tokens; revoke only the intended grant with appropriate authorization. Preserve synthetic mailbox messages as evidence or move them to a designated cleanup folder; avoid permanent deletion. Explain any remaining server-side changes.

If this investigation leads to code changes, validate them under the repository's relevant Go/frontend checks and add a regression test for each confirmed frontend bug. No application tests are required merely to save this proposal.

## Surprises & Discoveries

- The FAQ emphasizes EWS, while current setup docs and release notes describe Graph. Test the installed release's Graph configuration rather than treating the FAQ as a complete current setup recipe.
- Tenant compatibility and live Mailven/DavMail interoperability remain unverified.

## Decision Log

- Prefer Graph; keep EWS outside the primary test because of Exchange Online retirement.
- Test an additional manual account rather than migrating an existing Mailven route.
- Separate plan authoring from live execution; the present request is to save a test plan.

## Outcomes & Retrospective

Pending live execution. No tenant compatibility outcome has been established.

## Checkpoint

Planning checkout: branch `docs/outlook-redirect-uri`, base revision `64db5a1`. An unrelated modification to `frontend/wailsjs/go/models.ts` was present and is outside this task. Only this new plan is intended to change.

Current step: plan authored; live execution unrun. Next action when testing is selected: confirm the account/recipient scope and profile isolation, then inventory the installed runtime and gateway version. Remaining external gates: tenant authorization, test account access, consent and access policy. Record actual commands/results and update the progress/matrix during execution; no live test is implied by this document.

Authoring validation: `python3 tools/check-doc-links.py docs/plans/davmail-microsoft365-test.md` passed (one file, zero errors); plan whitespace inspection passed. Repository-wide `git diff --check` failed on trailing whitespace in the pre-existing `frontend/wailsjs/go/models.ts` modification; that file was left untouched. Independent read-only plan review passed with no material gaps. Application builds, automated UI checks and live provider tests were not run for this documentation-only task.
