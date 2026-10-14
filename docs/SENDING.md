# Sending and recovery

Gmail API and delegated Microsoft Graph routes are opt-in per account (Settings →
Accounts → Edit → Server → Mail API route). Accounts keep sending through their
configured SMTP route until the user migrates them, and there is no automatic
fallback between routes. No route promises delivery or inbox placement.
See [provider requirements](PROVIDERS.md) and [API integration and migration](MAIL_APIS.md).

Mailven records each submission attempt locally with an attempt ID and a stable
Message-ID. The server's acceptance means it accepted responsibility for the
message; it does not confirm delivery or inbox placement.

- **Not submitted:** preparation, connection, or transport setup stopped before
  a message body could be submitted. The draft is retained.
- **Rejected temporarily/permanently:** an explicit SMTP 4xx/5xx response
  established non-acceptance. Fix permanent failures before trying again.
  Temporary rejection retries require a new user action and use a persisted
  backoff starting at five seconds, doubling up to five minutes.
- **Accepted:** the server returned a positive final DATA reply. Sent-copy,
  contact-update, and draft-cleanup failures are local recovery problems. Do not
  resend to repair them. Mailven reports cleanup warnings separately.
- **Unknown:** the DATA write or final reply was interrupted, or the process
  stopped after a durable pre-DATA checkpoint. The server may have accepted the
  message. The draft is retained. Check Sent and confirm with recipients before
  deciding whether to submit again: another submission can create duplicates.

Mailven has no automatic replay, retry, or route fallback. A reopened unknown or
accepted draft requires review and an explicit **Submit again** confirmation on
a new Send/Retry action. Editing that draft or selecting another sending account does not clear its
previous outcome.
Previous attempts remain recorded; a new submission gets new identifiers.
Recipient records distinguish accepted, rejected, and unattempted RCPT commands.
If any recipient rejects the envelope, Mailven aborts the entire transaction
before DATA, so the other recipients receive no subset of that message.

Queued sends retain their existing ten-second Undo delay and have a local draft
before queueing. After a restart, interrupted pre-DATA attempts remain unsent;
open the retained draft and choose Send if wanted. Unknown and accepted attempts
are never replayed. Read receipts use the same journal and block repeat
submission after unknown/accepted outcomes, including a failed handled-flag
update. Their original message remains available; an unknown receipt is not
sent again automatically.

The journal stores outcomes, recipient responses, and a content fingerprint,
not another copy of the body or attachments. Existing encrypted draft storage,
account routes, credentials, and signing/encryption choices are retained. A
pre-submission recovery copy that cannot honor requested encryption stops the
submission rather than storing a new plaintext recovery copy. Existing draft
autosave/account-ownership limitations are tracked separately in ISSUES.md.
SMTP failure and restart behavior is verified with disposable local fixtures.
Live provider behavior remains subject to separately authorized checks.

Before MAIL FROM, RCPT TO or DATA, Mailven validates the final serialized MIME
and envelope, after signing and encryption. Invalid required headers, threading,
addresses, injected header controls and overlong physical lines stop submission.
The original draft remains editable; content is never truncated to pass a check.
Bcc recipients stay in the envelope and are omitted from outgoing headers.
IDN domains are converted to ASCII in the journal's Message-ID, before
serialization/signing, and in the SMTP envelope. International local parts
(or raw international headers) require the server to advertise SMTPUTF8.
Unicode subjects, names and bodies are retained.
Missing encryption keys or unreadable encryption policies stop submission.
No alternate plaintext or transport fallback is attempted.

SMTP SIZE is checked against the complete encoded message, including MIME,
inline and regular attachments, signature/encryption overhead and canonical
CRLF line endings. Base64 usually adds about one third to attachment bytes.
An absent SIZE advertisement, SIZE without a number, or SIZE 0 does not establish
unlimited delivery capacity: account, administrator and recipient limits can
remain unknown. Mailven's 100 MB per-file attachment cap is a local resource
limit, not a provider allowance. Mailven does not infer a mailbox limit from
its provider name: [Gmail attachment limits](https://support.google.com/mail/answer/6584?hl=en)
vary by account/plan, and [Exchange Online limits](https://learn.microsoft.com/en-us/office365/servicedescriptions/exchange-online-service-description/exchange-online-limits)
can be configured by administrators. The sending server can still reject a
message below its advertised ceiling.

All physical message lines must fit within 998 bytes excluding CRLF. On Gmail
SMTP routes and when sending to gmail.com/googlemail.com, separate
[Gmail header policy](https://knowledge.workspace.google.com/admin/gmail/gmail-message-header-limits?hl=en)
checks enforce 512000 total header bytes, 5000 fields, 32768 bytes per unfolded
field value (except To, Cc and References), 500 characters after the Message-ID
colon and 998 after the Subject colon. Continuation whitespace and the leading
space after a colon count toward field values. These checks use wire header
values, including encoded words; folding never excuses an oversized value.
Other custom-domain Gmail recipients cannot be identified from an address alone.
SMTPUTF8 and SIZE behavior follow [RFC 6531](https://www.rfc-editor.org/rfc/rfc6531)
and [RFC 1870](https://www.rfc-editor.org/rfc/rfc1870). Provider receipt, privacy
and encryption parity still require separately authorized live checks.

Sender evidence is shown in account identities and the composer for the selected
From and configured SMTP route, with its source and observation age. Fresh Gmail
send-as evidence allows primary addresses and explicitly accepted aliases. A
pending or absent address blocks submission until it is verified/configured and
refreshed, or another From is selected. Blank or unrecognized alias status,
lookup denial/error, and missing evidence are unknown. Evidence older than 24
hours is stale. A last successful pending/absent observation remains blocked
until a successful refresh proves setup corrected, even after aging or lookup
failure; the old state and observation age remain visible. Lookup failure is recorded locally across restarts and never
turns an older verified result into current authorization. A verified alias proves
setup only, not domain alignment or delivery. See Google's [send-as resource](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs).

Unknown/stale evidence requires a new explicit **Use this From and send** review
in either composer. Cancel retains the selected From, recipients (including Bcc),
and editable draft. The acknowledgement applies to that account, From, route,
and evidence observation; the backend checks again before SMTP submission,
including after the Undo queue. Changing routes or refreshing to known-invalid
evidence cannot bypass the check. Saved/reopened drafts carry no acknowledgement.
A configured password-account primary address is described as local configuration,
not a provider grant; custom aliases and routes remain usable after review.
OAuth protocol scopes are checked separately and review cannot grant a missing scope.

Microsoft Send As, Send on Behalf, and shared read access are separate permissions.
Mailven has no authoritative delegation lookup or complete sendable-mailbox list;
login and SMTP/IMAP scope consent do not prove delegation. A configured shared
mailbox records read setup only. The current SMTP route sends the selected From
without a Sender-on-behalf transformation and requires Send As for another mailbox;
Send on Behalf is not a substitute. Confirm rights with an administrator and review
unknown evidence. Microsoft documents the [separate delegation permissions](https://learn.microsoft.com/en-us/exchange/recipients-in-exchange-online/manage-permissions-for-recipients).
Quick replies and read receipts also pass the backend gate: when sender evidence
needs review, use the full composer for a reply; an automatic receipt cannot
acknowledge unknown rights and remains unsent. Mailven never silently changes From
or chooses another route. Provider enforcement remains authoritative. Actual
alias/delegation authorization still requires separately authorized live checks.

## Sent copies and linked authentication

In account settings → Identity → **Sent copy policy**, review the actual SMTP
route, authenticating identity, mailbox and intended folders. No receive-host
heuristic establishes whether server copying is enabled. Existing account and
folder settings remain intact; an unreviewed or changed route has unresolved
copy policy and produces a separate Sent warning after successful submission.
Choose provider copying only when its setting/destination is known; choose
client IMAP APPEND only after reviewing server copying to avoid duplicates.
Multiple distinct intended destinations are supported, including shared/custom
folders. A saved policy applies to future submissions; editing it never replays
old messages. Removed/renamed destinations need review.

Each submission snapshots destinations before SMTP, then records a one-shot
APPEND claim before contacting IMAP. Interrupted or failed APPEND remains
unconfirmed and is never automatically retried, including after restart. Accepted
submission and missing-copy evidence are separate. **Refresh copy evidence**
checks already synced messages by stable Message-ID in each intended folder;
absence can mean delayed sync, and a Sent copy does not prove recipient delivery.
This SMTP copy policy does not promise exactly one global copy or automatically
resubmit messages. Active API routes use provider filing instead of IMAP APPEND. Actual provider/shared mailbox destinations and counts remain
separately authorized live checks.

Linked Microsoft mailboxes use the parent's authentication ownership and refresh
lock; IMAP still addresses the shared mailbox SASL identity while SMTP uses the
parent authenticating identity. Explicit reauthorization propagates updated token
metadata to linked accounts. Refresh-token rotation alone does not establish
revocation of an old Microsoft refresh token.

## Check account setup

In Settings → Accounts, edit the account and open Identity → Check account setup.
Choose an existing From identity and the saved IMAP/SMTP route, then select Start
diagnostic. Cancel stops network work; closing the tab also requests cancellation.
Nothing runs until Start. The diagnostic never sends a test message, starts an SMTP
mail transaction, reads mail, requests consent, changes DNS, or retries automatically.
Only local results are retained in memory; a new run replaces them.

The report separates TLS/capability, login, recorded OAuth protocol scopes, cached
sender evidence, and unknown provider/tenant policy. It shows effective endpoints,
UTC observations, token expiry and actual known grants (legacy requested scopes
remain unknown). Cached sender evidence includes its observation age and sender-evidence
limitations described above. A successful login is not Send As, Send on Behalf or delivery evidence.
Opaque custom OAuth metadata and raw provider errors/request IDs are redacted.

Probes require TLS/STARTTLS with system certificate verification; they do not bypass
hostname, expiry or trust failures. Review the saved hostname, device clock and
certificate chain/trust settings when TLS fails. An authentication rejection alone
cannot distinguish bad credentials from disabled protocols, missing delegation or
tenant restrictions; consult the administrator. Existing user-approved certificate
exceptions are not applied by this stricter diagnostic. Each protocol probe is bounded
to 12 seconds, DNS to 5 seconds, and the run to 35 seconds. Wait at least 60 seconds
before manually retrying timeouts/temporary failures.

The diagnostic reads the shared authentication owner's stored credentials. It does
not refresh tokens: existing refresh locks have no caller deadline and may trigger
reauthorization events on rejection. Tokens expiring before the probe can finish
skip authentication. Let the normal account connection refresh them or use the
account's explicit Re-authorize action, then retry. No consent is started here.

Optionally confirm ownership and enter a domain to inspect SPF/DMARC TXT. DKIM is
queried only for up to ten selectors you supply; Mailven never guesses selectors.
Consumer domains are provider-managed. DNS results include query, resolver, TTL
when available and UTC time. They distinguish absent, NXDOMAIN, timeout and
malformed/multiple policy records; on platforms with only the system resolver,
TTL and NXDOMAIN/NODATA distinction are explicitly unavailable. Structural record
presence does not prove signing, SPF/DKIM/DMARC pass or alignment; SPF includes and
full policy semantics are not evaluated. Mailven does not prescribe replacement SPF.

Export redacted report is a separate user action that saves JSON through the native file dialog. It excludes
account IDs, mailbox/From addresses, usernames, passwords, tokens, message content
and raw server replies. Endpoints and explicitly selected domain/selector queries
remain useful diagnostic context. Review the report before sharing. There is no
delivery score or promise of inbox placement. Real-account diagnostics and live
provider compatibility require separately authorized checks.

## API submission

After a user authorizes and migrates an eligible account, sends use Gmail API or
Microsoft Graph. Staged accounts and accounts without an active API route continue
using their configured SMTP route. API sends reuse the submission journal: an
`unknown` checkpoint is written before each HTTP request and the final result is
committed before local cleanup. An accepted API response means the provider took
the message, not that it was delivered. Unknown outcomes are never replayed or
sent over SMTP. See [API integration](MAIL_APIS.md) for provider limits and unsupported API
operations. Outlook.com personal accounts and shared-mailbox API routes are not
supported.
