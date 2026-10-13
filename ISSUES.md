# Issues

## Deferred follow-ups

Found by the full audit on 2026-10-08. Each entry gives a severity, the
location, and the failure. "Unverified" means a reviewer reported it and it
has not yet been confirmed against the code.

### Dependencies

- **D2 low** `frontend/package.json`: TypeScript 7 remains blocked upstream.
  Verified on 2026-10-12 with `npm view <package>@latest version peerDependencies --json`:
  TypeScript 7.0.2 is published, but latest `typescript-eslint` 8.71.1 requires
  `>=4.8.4 <6.1.0` and `svelte-check` 4.7.6 requires `^5.0.0 || ^6.0.0`.
  Both tools are already current here; retain TypeScript ~6.0.3 until both accept 7.
  Recheck using the commands in `CONTRIBUTING.md`.

### AI guidance portability (reviewed 2026-10-12)

- **AI5 medium** `.agents/skills/project-handoff/SKILL.md`: G2 corrected checkpoint-before-commit ordering and now requires branch/revision, dirty state, validation provenance and blockers in the shared handoff. Bidirectional continuation, including uncommitted work, is still unverified; complete the deferred bidirectional runtime trials in PLAN.md G4 on a future branch before closing this item. The user closed the current branch on 2026-10-12; resume only when requested.

### Provider compatibility and deliverability (P1, 2026-10-12)

Confirmed by static inspection; all remain unfixed because P1 is research-only. Details, provider sources, applicability and live-check limits are in [the provider review](docs/validation/provider-deliverability-review.md). Architecture and follow-up prioritization belong to P2.

- **PD1 medium** `docs/user-guide/getting-started/first-account/gmail.md:77–84`, `outlook.md:30–35,51–58`: setup guidance prescribes manual reauthorization for routine token expiry despite automatic refresh in `app/compose.go:74–146`; Outlook's mail permissions list also claims contacts access absent from the Microsoft mail grant (`internal/oauth2/providers.go:51–58`). Correct the existing guides; distinguish rejected refresh grants and separate contacts authorization.
- **PD2 medium** `app/oauth.go:179–192`, `docs/user-guide/getting-started/first-account/outlook.md:20–49`: all Microsoft OAuth accounts use Exchange Online's SMTP endpoint, rather than Outlook.com's documented `smtp-mail.outlook.com`; the guide omits personal Outlook.com's IMAP opt-in and makes an absolute app-password claim despite Microsoft's device-specific caveat. Published-default mismatch is confirmed; current endpoint failure is not. Verify compatibility before changing existing accounts.
- **PD3 medium** `internal/smtp/client.go:375–386`, `app/send_queue.go:23–28,106–120`: generic write/final-DATA-reply errors expose no distinction between definite rejection and possibly accepted mail. A user retry after a lost final reply can duplicate a message. Preserve unknown-outcome state; verify with controlled SMTP fixtures rather than blind automatic resends.
- **PD4 low** `internal/smtp/message.go:213–234,256–259`: whitespace folding leaves a long unbroken ASCII Subject intact and performs no total Subject-value limit check. A 1,100-character unbroken subject produces a line exceeding RFC/Gmail limits. Future checks must cover both physical-line and folded-value limits without silently altering content.
- **PD5 medium** `app/sendas.go:46–57,94–100`, `frontend/src/lib/components/composer/SendAsWarning.svelte:20–35`, `app/compose.go:475–481`: Gmail OAuth sender status is advisory, failed lookups hide warnings, and selected From is sent without an authorization gate; equivalent Microsoft permission evidence is absent from this path. Known-unverified and unknown states need distinct guidance; lack of a warning is not proof of sender authorization.
- **PD6 medium** `internal/smtp/client.go:351–386`, `frontend/src/lib/components/composer/Composer.svelte:365–366`: SMTP SIZE/final encoded-message size is not preflighted; the UI allows individual attachments up to 100 MB regardless of provider limits or combined MIME/encryption overhead. Users can prepare messages that their selected server rejects. Effective account thresholds still require verification.
- **PD7 medium** `app/compose.go:484–491,1242–1255`: Sent-copy suppression depends on an IMAP-host substring rather than actual SMTP route and server copy settings. Exchange Online can disable SMTP Sent copies; the heuristic cannot represent that setting or custom/shared destinations, risking missing or duplicate copies. Verify actual mailbox behavior before changing the policy.
- **PD8 low** `internal/smtp/client.go:351–372`: raw internationalized envelope addresses are sent without checking that SMTPUTF8 is available or normalizing international domains. Go's `net/smtp.Client.Mail` already adds SMTPUTF8 when advertised; the gap is absence of a capability-required preflight when it is not advertised. Verify Unicode domains separately from international local-parts and preserve international message content.
- **PD9 medium** `app/oauth.go:214–216,482–486`, `internal/oauth2/flow.go:38`: persisted scope metadata records requested provider scopes rather than the token response's granted scopes. Partial consent can be represented as full consent. Verify feature handling with a controlled partial-grant response; downstream account failures are not established by this audit.

## Recurring problems

None.
