# SEND-2 — API privacy and limits: static review

Date: 2026-10-15. Scope: static/provider-source review, then authorized live Bcc cases 1–4 (below). [SEND-2](../../PLAN.md#send-2) / [MA1](../../ISSUES.md#ma1) stay **blocked**: the acceptance criterion needs delivered-copy evidence from a separate recipient mailbox, which the authorized alias cases cannot give.

## Provider documentation (read 2026-10-15)

| Question | Gmail `users.messages.send` | Graph `user: sendMail` (MIME) |
| --- | --- | --- |
| Recipients come from headers | Documented: "Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers." | Not stated for MIME; the page only says to supply Internet message headers and MIME content, base64 encoded. |
| Bcc header removed from delivered copies | Not documented (reference and sending guide are silent). | Not documented. |
| Message size limit | Not documented for the API; the sending guide and reference give no figure. Gmail Help's 25 MB send limit is the only published number. | Not documented for MIME; the page defers to Exchange Online limits, which vary by tenant and administrator. |

Sources: developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/send, developers.google.com/workspace/gmail/api/guides/sending, learn.microsoft.com/en-us/graph/api/user-sendmail.

## Code under review

- `internal/submission/api.go`: `HiddenBcc` returns envelope recipients in none of To/Cc/Bcc; `WithBcc` prepends a `Bcc:` header naming only those, and passes the bytes through unchanged when there are none. Covered by `TestHiddenBcc`.
- `MaxRawBytes = 25 << 20` in `internal/gmailapi/send.go` and `internal/graphapi/send.go`; `CheckSize` rejects larger messages locally without truncating.

## Conclusions

- Only Gmail documents that Bcc recipients are read from headers; neither provider documents whether the header is stripped from delivered copies. Privacy cannot be established from documentation, so live evidence is required.
- The 25 MiB ceiling is a local conservative bound (Gmail Help's 25 MB), not a documented API or Graph allowance. It stays as is; Graph may refuse smaller messages by tenant policy, and Gmail's encoded-size accounting versus our raw-MIME check is unverified.

## Controlled live cases (to run only after separate authorization)

Use owner-controlled sender and recipient mailboxes on disposable, authorized accounts, and record raw delivered headers (view original) for each.

1. Bcc only: To = A, Bcc = B (hidden). Expect B's copy has no `Bcc:` header naming B or others; A's copy has no `Bcc:` header at all.
2. Two hidden recipients B and C: neither delivered copy, nor A's, names the other.
3. Visible plus hidden mix with a duplicate address (hidden also in Cc): no duplicate delivery, no `Bcc:` header (`HiddenBcc` excludes it).
4. Sent-copy check: the sender's Sent item may keep the `Bcc:` header; record it (not a recipient exposure).
5. Size: raw MIME just under 25 MiB and just over (local rejection), plus a mid-size (about 20 MiB) case; record the provider's accept/reject and any base64/encoding-size error.
6. Failure of any case 1–3 keeps the rollout gate closed and means the route must not send hidden recipients.

Gmail cases need an authorized Gmail account with a verified From; Microsoft cases are deferred by the 2026-10-14 owner decision.

## Live authorization (2026-10-15)

The owner authorized live Bcc cases 1–4 with their own personal Gmail account as the sender (OAuth, Gmail API route, sent through the real app path) and three owner-controlled recipient mailboxes (A visible, B and C hidden), added to the same dev install so that raw delivered headers can be read locally, read-only. Size case 5 is not authorized and stays unrun. Microsoft remains deferred. Owner choices (2026-10-15): sender `seancasey@gmail.com`, signed in to the dev install (main) with the same custom Google client the live Flatpak client uses; the live Flatpak account is not touched. Recipients: plus-aliases `seancasey+a/+b/+c@gmail.com`. Limitation: all aliases deliver to the sender's own mailbox, where Gmail keeps one copy per Message-ID (the Sent copy), so these cases cannot show what a separate recipient server receives and cannot alone close MA1. Status: cases 1–4 sent and read on 2026-10-15 (below).

Activation (2026-10-15): the dev account signed in and the Gmail API grant was given, but activating the route failed twice on Gmail's per-user per-minute quota ([SY7](../../ISSUES.md#sy7)). The reconcile listed the whole mailbox (about 42,700 messages) and gave up after about 7 seconds of retries. By owner decision the fix makes the Gmail reconcile respect the account's sync period and wait out the quota window, and the dev account's sync period was set to 30 days. The reconcile then stopped at one message over the 40 MiB download cap, now kept as a header-only row ([SY8](../../ISSUES.md#sy8)). Two defects found on the way are recorded: [MA7](../../ISSUES.md#ma7) (an interrupted activation leaves the route active) and [MA8](../../ISSUES.md#ma8) (Edit Account cannot save on an API route; the period was set directly in the dev database).

After the fixes the reconcile completed in the dev install: 1,649 API-route messages, none older than 31 days, one header-only oversize row, route active and authorized with no recorded failure. It completed in the resumed background sync, not in a single successful activation call.

## Live Bcc results (2026-10-15)

Each case was sent through `SendMessage` on the active Gmail API route and journaled as `api-send` / `accepted`. Raw source was read from the synced copy. Every message has exactly one stored copy, labeled INBOX and SENT, with two `Received:` headers: Gmail merged the alias delivery into the Sent copy, as the limitation above predicts.

| Case | Sent | Stored copy | Reading |
| --- | --- | --- | --- |
| 1 | To +a, Bcc +b | One `Bcc:` header naming +b | Expected for the Sent copy; says nothing about B's delivered copy. |
| 2 | To +a, Bcc +b, +c | One `Bcc:` header naming +b and +c | Same; cannot show whether B's copy names C. |
| 3 | To +a, Cc +b, Bcc +b | No `Bcc:` header | Passes: `HiddenBcc` drops a hidden address already visible, and no duplicate copy was stored. |
| 4 | Sent-copy check (cases 1–2) | Sent copy keeps `Bcc:` | Recorded as expected; not a recipient exposure. |

Observation: Gmail replaced the `Message-ID` Mailven generated (the journal's `<uuid@gmail.com>`) with its own `<…@mail.gmail.com>`. API sends create no `submission_copies` rows and no API send path looks the sent message up by Message-ID, so nothing in Mailven depends on the original value; replies thread on the stored (Gmail) value, which recipients also see. Not recorded as an issue.

## Results

Passed: documentation review; live cases 3 and 4; Go tests for the SY7/SY8 fixes (`make test`, golangci-lint 0 issues). Inconclusive: live cases 1 and 2, because the aliases deliver into the sender's mailbox and only the Sent copy exists, so MA1 stays open. Failed: none. Unrun: size case 5 (not authorized); separate-recipient delivered-copy evidence (needs a mailbox on another account or server).
