# SEND-2 — Gmail API Bcc and size investigation

Status: local investigation complete; live Gmail account use/recipient discovery authorized, **no live test mail sent or received**. Review date: 2026-10-14. [Execution checkpoint](../plans/send-2.md), [milestone](../../PLAN.md#send-2), [MA1](../../ISSUES.md#ma1). Microsoft work is deferred and excluded. No route, credential, tenant, mailbox or limit was changed.

## Findings and source limits

Google's [`users.messages.send` reference](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/send) identifies To, Cc and Bcc headers as the recipient source. Mailven therefore computes hidden envelope recipients and prepends a Bcc header to the MIME sent to this endpoint. That is not proof of what a delivered copy contains.

[Gmail Help](https://support.google.com/mail/answer/2819488?hl=en) describes hidden-recipient privacy, including preventing one blind recipient from seeing another. This supports the expected product behavior. The checked API reference and [sending guide](https://developers.google.com/workspace/gmail/api/guides/sending) do not specify the exact per-recipient raw-header transformation. “No API-specific raw-header guarantee found in these sources” is narrower than “Google never documents Bcc privacy.” The required Mailven delivered-copy check remains unrun. A sender's Sent copy may legitimately retain Bcc and is not a recipient-privacy test.

The [attachment help page](https://support.google.com/mail/answer/6584?hl=en) currently distinguishes personal Gmail's 25 MB attachment allowance from administrator-set work/school limits. Its automatic Google Drive substitution describes Gmail's own compose behavior; Mailven does not implement that substitution. An attachment allowance is not a final RFC822 MIME or JSON request ceiling.

The [Gmail discovery document](https://gmail.googleapis.com/$discovery/rest?version=v1), revision `20261012` observed during this review, reports `users.messages.send.mediaUpload.maxSize = 36700160` (35 MiB). The [upload guide](https://developers.google.com/workspace/gmail/api/guides/uploads) distinguishes media upload protocols. Mailven uses the non-upload JSON `raw` request, not a media-upload path. This field does not prove its effective JSON request limit, an account's mail allowance, or successful delivery. No automatic increase to 35 MiB is justified by this research.

## What Mailven actually measures

| Measure | Current implementation / meaning |
| --- | --- |
| Composer attachment bytes | A separate per-file resource guard; not a total message allowance |
| `req.Raw` | Final serialized MIME after signing/encryption, before API-only Bcc augmentation |
| Local Gmail ceiling | `25 << 20` = **26,214,400 bytes (25 MiB)**; exactly the ceiling passes, one byte above fails |
| API MIME | `req.Raw` plus a prepended hidden-recipient Bcc header, if needed; not rechecked against the ceiling |
| HTTP body | Unpadded base64url of that API MIME in `{"raw":"..."}`; the JSON body is larger than the MIME |
| Provider acceptance | A separate live observation; a 2xx response is not delivery or privacy proof |

Source: `internal/gmailapi/send.go` (`Preflight`, `Sender.Send`, `MaxRawBytes`), `internal/submission/api.go` (`HiddenBcc`, `WithBcc`, `CheckSize`, `Post`), `app/compose.go` (active-route branch after signing/encryption), and `app/mail_api_send.go` (`submitViaAPI`).

For `N` API-MIME bytes, unpadded base64url length is `4*(N//3) + [0,2,3][N%3]`; the current JSON wrapper adds 10 bytes. Without extra Bcc, 26,214,400 MIME bytes produce **34,952,534 base64url bytes / 34,952,544 HTTP-body bytes**. A single illustrative `Bcc: hidden@example.test` header adds 26 bytes including CRLF, producing 26,214,426 API-MIME bytes and a 34,952,578-byte JSON body. This arithmetic describes the code's encoding, not a Gmail acceptance threshold.

The 25 MiB constant is retained as an existing conservative local policy, not a provider-derived exact limit. Its source comment equates a Gmail Help figure with the send ceiling; attachment bytes, decimal MB, binary MiB, final MIME and encoded HTTP bytes must not be conflated. The 40 MiB Gmail read-response cap is independent of this send guard and includes encoded response overhead; the send cap does not establish a maximum received-message size.

## Static findings retained for follow-up

- **MA1:** Bcc augmentation can grow an allowed `req.Raw` past 25 MiB; neither that augmented MIME nor the JSON request is measured by the current guard. Existing tests prove the original-MIME boundary, hidden-recipient insertion and a 3 MiB transport round trip, not a real-provider threshold. Source comments that call routes inert pending live privacy verification are stale: the application already submits through an active API route. The outstanding live gate is an acceptance requirement, not a new runtime block added here.
- **PD10:** Gmail submission does not call SMTP's exact-final-MIME validator. Composer input validation still checks addresses/header controls, but that is distinct from validating final transformed MIME and the prepended Bcc field. See the [separate finding](../../ISSUES.md#pd10). No behavioral fix is bundled into this investigation; PD10 was fixed separately on 2026-10-16.

<a id="controlled-live-protocol"></a>

## Controlled live protocol — execution pending

### Preconditions and authorization record

The owner has authorized live Gmail account use and inbox search for test candidates. Ownership of the junk account is confirmed; the custom-domain address is forwarding-only. Gmail browser access exists, but the current Mailven profile contains no Gmail accounts. No test send, route activation or account migration has occurred. Complete the concrete execution record below before sending; do not ask again for the already-granted account-use permission. Addresses and exports stay in a private local record:

- Sending account identifier, personal Gmail or Workspace, exact primary/verified-alias From, selected `gmail_api` route and confirmation that the owner controls it.
- Four distinct controlled recipient mailboxes: visible To (`T`), visible Cc (`C`), and blind recipients (`H1`, `H2`). Prefer Gmail and independently controlled non-Microsoft external delivery among them. Aliases/plus-addresses that collapse into one mailbox do not prove independent recipient copies. Confirm permission to inspect/export each delivered original.
- Case IDs, maximum submissions, execution window, maximum original/API-MIME/request bytes for size cases, permitted mailbox read/export scope and private evidence directory.
- An already active route is required. If unavailable, staging, activation or migration needs its own reviewed authorization/backup plan; it is not implied by sending-test approval. No credential values belong in the approval, repository, logs or report.

Approval is per account type, route, From and selected cases. Personal Gmail evidence does not establish Workspace, aliases, SMTP, Microsoft, or signed/encrypted behavior. No external participants, distribution lists or uncontrolled forwarding targets are allowed.

### Cases and bounds

| ID | Exact case | Expected proof / limit |
| --- | --- | --- |
| G1 | Small synthetic plain-text message: To T, Cc C, Bcc H1 and H2; unique case token and Message-ID | One submission, four delivered originals. T/C must reveal neither hidden address; H1 must not reveal H2 and H2 must not reveal H1. Own-address delivery trace headers are allowed |
| G2 | Small synthetic plain-text message: Bcc H1 and H2 only; distinct token/Message-ID | One submission, two delivered originals; both receive it and neither learns the other hidden address |
| S1 | Valid generated MIME targeting exactly 26,214,400 original bytes, no Bcc; To T only | Optional, one submission; 34,952,544-byte JSON body. Record provider response and delivered original separately; never presume it will fit the account |
| S2 | Same original-byte target with To T and Bcc H1; unique content token | Optional, one submission, two expected copies. Precompute the extra Bcc header using the approved real address. Record original/API-MIME/JSON byte counts; require separate approval of the resulting maximum request size |
| O1 | Offline preflight only: original MIME at ceiling−1, ceiling, ceiling+1 | Below/at pass the size check; above fails. Do not submit the above-limit message to a provider or bypass the guard |

Initial recommended live approval is **G1/G2 only: two submissions, six recipient copies**, each original message at most 16 KiB. S1/S2 require separate explicit approval of two large submissions and their byte budgets. Do not binary-search a provider boundary, raise/bypass the ceiling, add a media-upload path, or substitute SMTP. Signed/encrypted or alias variants require separately selected cases and prepared keys/identities; they are not covered by these plain-text cases.

### Capture and verdict

For each selected case, preserve original compose inputs, From/recipient roles, local journal/Message-ID, route, UTC timestamps, exact pre-Bcc MIME length/hash, added-header length, final API-MIME/request lengths and the provider result/error. Do not log bearer tokens or authentication headers. Where exact application payload capture is not available, mark that evidence missing and prepare an offline measurement method before execution; do not infer sizes from attachment filenames or UI totals.

Obtain **raw delivered originals** from every recipient mailbox, independently of the sender's Sent folder. Keep private originals plus hashes and a sanitized verdict table; do not commit real mailbox content or addresses. Unfold/decode address-bearing headers and inspect complete headers plus decoded MIME for the other hidden addresses; a UI screenshot or missing Bcc header alone is insufficient. The synthetic subject/body must contain no recipient addresses. Preserve the sender's Sent copy separately for correlation, without treating its Bcc list as a leak. S1/S2 also require content completeness checks.

Stop immediately on any hidden-address exposure, missing expected copy, wrong From/route, unapproved recipient/configuration change, rate limiting, provider rejection or uncertain submission. Record failure/unknown/unrun separately. A timeout, transport failure after write, 429 or 5xx is not permission to resend; reconcile the journal and controlled mailboxes before any further approval. Stop after the approved submission budget and record the agreed observation window; delayed/missing originals leave acceptance unresolved.

Pass only the tested account/route/From/cases when all expected recipient originals are present and privacy/completeness checks pass. Record size outcomes as observations at specific byte counts, not proof of a universal maximum. Missing or failed evidence leaves MA1 and SEND-2 blocked; any failure requires review before resumption. Cleanup/deletion or rollback of account state requires separately authorized scope.

## Offline validation

- **Passed:** `go test ./internal/gmailapi ./internal/submission ./app -run '^(TestSend|TestPreflight$|TestHiddenBcc$|TestPost|TestMailAPISend)' -count=1` — all three package groups passed. Existing fixtures cover original-MIME at/over the cap, hidden-recipient insertion, byte preservation, local HTTP results and journaling. The exact ceiling−1 case is supported by the size comparison/source arithmetic; no new boundary fixture was added. The 3 MiB transport fixture is not a live near-limit message.
- **Passed:** whitespace and documentation links via `python3 tools/project-check.py --base d42736a47d67f4160bc092ffdeed6c42447cb0dc --run` (7 Markdown files, 0 link errors); final documentation checks repeated after recording results. A new generated-anchor link initially failed; an explicit stable anchor corrected it.
- **Review:** independent local-scope acceptance PASS; full delivered-copy acceptance UNVERIFIABLE. Direct per-file review complete; no implementation code changed or simplification needed. Confirmed findings left unfixed are recorded as MA1, PD10 and DOC6.
- **Unrun:** full Go suite, frontend/UI/native checks (no application changes), all live Gmail privacy/size cases, and all Microsoft checks (deferred).

Fixture logs and combined check evidence are under `/home/user/.cache/mailven-project-checks/run-cfnthoi6/`. Existing fixtures use local `httptest` endpoints and synthetic tokens; they cannot establish provider stripping or live size acceptance. SEND-2 remains blocked on the separately authorized protocol above.
