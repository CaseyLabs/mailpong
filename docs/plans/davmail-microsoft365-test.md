# DavMail + Mailven Microsoft 365 compatibility test

Created: 2026-10-14. Status: deferred by owner decision 2026-10-14; Outlook-client-ID evidence preserved under [SEND-4](../../PLAN.md#send-4).
Target: an authorized user's own Microsoft 365 mailbox at `cityuniversity.edu`.
Authorization: on 2026-10-14 the user requested implementation on a new branch and explicitly authorized the test account in `.env` for DavMail sign-in, OAuth consent/token storage and synthetic **self-mail only**. Local installation/configuration and an isolated Mailven profile are within scope. External recipients, tenant changes, custom application registration, existing-account migration and grant revocation remain excluded.

Authorization update: the user selected the DavMail Outlook-client-ID route after comparison with email-oauth2-proxy. This authorizes one documented Outlook desktop client-ID test, Graph resource authorization and inspection of actual permissions; the own account/self-mail scope persists. Do not accept an unexpected permission-expansion prompt without presenting its concrete permissions. Other client IDs, policy changes and admin requests remain excluded.

## Scope

Selected investigation: [SEND-4](../../PLAN.md#send-4), governed by [project management conventions](../PROJECT_MANAGEMENT.md). Execute this one milestone and record an evidence-based result; do not infer tenant compatibility from local readiness. Stop live work at an unsatisfied institutional or user-interaction gate and preserve exact redacted evidence.

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
- [x] Confirm live-test scope and account; recipients are self-mail only.
- [x] Verify private XDG paths and forced encrypted credential fallback; shorten runtime path to satisfy Unix socket limits.
- [x] Inventory versions and configure a loopback-only Graph gateway.
- [x] Authenticate the Outlook client-ID flow through embedded SWT; Graph folder enumeration succeeds. Initial refresh-audience defect recorded as [DAV1](../../ISSUES.md#dav1), workaround restores folders.
- [x] Connect Mailven; verify self-mail/attachments/reply/flags; record failed move convergence as DAV3.
- [x] Observe actual expiry-driven renewal and reconnect; full lifecycle convergence fails and fresh-message receipt after final restart is unrun.
- [x] Restore UI, remove isolated account with rollback, stop task processes and record partial verification; SEND-4 is deferred; acceptance remains incomplete.

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
- Live sign-in reached an administrator-approval gate for DavMail, with basic profile, persistent access, mail read/write and sending shown. No request was submitted.
- Standalone/system Java lacks an embedded SWT/OpenJFX browser; pinned manual-mode source supports console authentication in server mode despite the setup documentation caveat.
- Outlook-ID SWT authentication and self-mail interoperability succeeded, with a broad Office grant and refresh-audience workaround. Folder moves remain unconverged; see DAV1–DAV3.

## Decision Log

- On 2026-10-14 the owner requested landing this experiment into main for preservation and deferring all further Microsoft 365 and regular/personal Outlook.com development. This supersedes the earlier keep-branch/no-landing instruction; it does not accept failed D7/D8 or resolve DAV1–DAV3. Explicit reopening is required before any further live work.

- Prefer Graph; keep EWS outside the primary test because of Exchange Online retirement.
- Test an additional manual account rather than migrating an existing Mailven route.
- The 2026-10-14 implementation request selects SEND-4; the subsequent scope answer authorizes the test account and self-mail only.

## Outcomes & Retrospective

Initial DavMail-ID execution was **blocked by tenant policy**. The authorized Outlook-ID follow-up is **partially verified**: D1–D6 and D9 pass; D7/D8 fail full convergence. [Sanitized results](../validation/davmail-microsoft365.md) contain configuration, complete scopes, live evidence and limits.

## Checkpoint

Branch: `experiment/davmail-microsoft365`; milestone base: `1e1368e2c70ac5ddd5c6ab75a531a22a18884d9b`. Initial checkpoint commit `7904395` is superseded by the amended checkpoint for this same milestone. SEND-4 is deferred because the owner chose to save the experiment for future development. Landing is explicitly authorized despite incomplete compatibility acceptance; preserve failed D7/D8 and unresolved DAV1–DAV3.

Current step: experiment saved for landing, live evidence recorded and cleanup completed. Next authorized action after landing: wait for explicit owner reopening. Possible future investigation after reopening: isolate DAV3 using bounded IMAP pool/gateway evidence, reproduce a serialized synthetic move without a stale local destination copy, then rerun D7/D8 including new-message receipt after restart. Do not resend existing cases. Existing DAV1 workaround remains necessary; DAV2 search false negatives require targeted queries. Prior own-account/self-mail authorization is historical; deferral requires explicit reopening before resumption. Other client IDs, admin requests, tenant changes, migration and grant revocation remain excluded.

Changed files: `PLAN.md`, `ISSUES.md`, `docs/PROVIDERS.md`, this plan and `docs/validation/davmail-microsoft365.md`. No application source change; no code simplification warranted. Unfixed findings DAV1, DAV2 and DAV3 are in ISSUES.md. Application suites and native-window inspection are unrun; real dev launch/build, real-backend Accounts expanded/collapsed UI checks and live protocol tests ran. D7/D8 failure must not be reported as full compatibility.

Private artifacts: `/home/user/.local/share/mailven-experiments/davmail-20261014T233015Z/outlook-followup` (private parent). Includes gateway/configuration, encrypted tokens/independent secret, raw logs, profiles, SQLite/config rollback before account removal and evidence. Keep contents out of Git. Existing profile/keyring isolation was verified through runtime paths and unavailable private session bus; credential fallback was encrypted database storage. Short runtime path is recorded in `runtime-path`.

Key evidence: `grant-metadata.json`, `natural-refresh.json`, `mailven-receive.json`, `reply-result.json`, `background-flag.json`, `mailven-synthetic-state.json`, `final-server-inventory.json`, `final-sent-exact.json`, `lifecycle-outage.json`, `lifecycle-recovery.json`, `live-ui-check.json`, `account-removal.json`, and final cleanup/listener records, all under private `evidence/`. The final broad Sent search missed a message that the full-subject search found; do not treat broad search counts as authoritative delivery evidence.

Cleanup: UI restored exactly, isolated account removed (zero accounts), task browser tabs closed and task gateway/dev processes stopped. Private encrypted gateway tokens retained for the authorized follow-up; no Office grant revoked. Server retains only the described synthetic messages/Sent copies and two dedicated folders; no permanent deletion. Before resumption, restore only the private profile rollback if needed, ensure all three local ports are free, remove the private coordinator stop marker and start using the documented isolated environment. Never launch this test through the existing profile or reuse stale authorization codes.

Future resumption reference — use only after explicit owner reopening (private gateway/profile only): set `b` to the private follow-up path above, `XDG_CONFIG_HOME="$b/config"`, `XDG_DATA_HOME="$b/data"`, `XDG_CACHE_HOME="$b/cache"`, `XDG_RUNTIME_DIR="$(cat "$b/runtime-path")"`, `TMPDIR="$b/tmp"`, `DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/no-session-bus"`, `MAILVEN_DEBUG=1` and `MAILVEN_DEV_LOG="$b/evidence/mailven-resume.log"`; invoke `.agents/skills/run-mailven/dev.sh start` with that environment. Verify encrypted credential fallback again. Start private `gateway-run.py` only after removing its `stop` marker. Gateway coordinator uses existing encrypted tokens; no new browser authorization should be necessary. The profile currently has zero accounts; its rollback is under `rollback-before-account-removal/`, with the original encrypted credential key material retained in the private data tree. Restore only while stopped, or recreate only the explicitly isolated test account. Avoid frontend-mounted probes during serialized debugging; `mailbox-action.cjs` demonstrates the bindings-only method. Stop with the same TMPDIR helper and coordinator stop marker.

Historical experiment verification (before deferral): `python3 tools/project-check.py --base 1e1368e2c70ac5ddd5c6ab75a531a22a18884d9b --run` passed whitespace and documentation links (five Markdown files, zero errors); the post-review rerun also passed. Independent `project-acceptance-checker` review agrees with D1–D6/D9 pass and D7/D8 fail, confirms broad grant disclosure, open DAV1–DAV3, no sensitive-data/stub patterns and active milestone status. Browser/MFA/webmail observations rely on the parent session and were not independently replayed. Direct per-file diff review corrected stale roadmap dependencies/status; no application source or simplification change was warranted. Automated application suites and native-window checks remain unrun. The private cleanup record confirms no task listeners, zero test accounts, restored UI and retained encrypted tokens/rollback.

Preservation/deferral verification: `python3 tools/project-check.py --base main --run` passed whitespace and documentation links (six Markdown files, zero errors); `python3 tools/check-doc-links.py --base origin/main` passed eight Markdown files. Independent acceptance review confirmed deferral scope, unresolved DAV1–DAV3, failed D7/D8, retained evidence and no credential values. Future launch instructions remain reference-only behind explicit reopening. No application source changed in this task; application suites and new live/native checks were not run. No simplification was applicable. The landing helper retains local recovery branches; main is the durable experiment record after landing.
