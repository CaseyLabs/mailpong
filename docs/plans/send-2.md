# SEND-2 — Gmail API privacy and limits verification

## Scope and authorization

[Milestone](../../PLAN.md#send-2), [MA1](../../ISSUES.md#ma1). Owner selected SEND-2 in this session after being told it starts with Gmail research and controlled-case design, with separately authorized live tests. The owner subsequently authorized use of the live Gmail account and inbox search for controlled recipient candidates, confirmed ownership of the junk Gmail account, and identified the Casey Labs address as forwarding-only. This account-use authorization persists; do not request it again. Base: `d42736a47d67f4160bc092ffdeed6c42447cb0dc`; branch: `docs/send-2-gmail-review`.

Original local scope: Gmail static/provider-source investigation, current ceiling documentation, offline fixtures and a reviewable live-check protocol. Continuation permits live Gmail account use and recipient discovery; no test messages have been sent. Do not change send behavior, raise limits, inspect credential values, or count a connector send as Mailven-route evidence. OAuth grants and route setup must be reviewed concretely; do not migrate an existing account under the preparation step. All Microsoft work remains deferred. No provider rollout/privacy acceptance is inferred from local fixtures.

## Success criteria and proof

- Explain how Gmail recipients/Bcc reach the API; distinguish API recipient selection, Gmail product privacy descriptions, and observed delivered raw messages. Prove with source review and official provider sources, not a local mock's acceptance.
- State exactly what the 25 MiB guard measures, where header/MIME/base64/JSON overhead occurs, and why published attachment/upload numbers do not define this JSON route's effective account allowance. Prove with code, arithmetic and official provider sources; do not change the limit.
- Specify bounded Gmail-only privacy and size cases, controlled identities/recipients, raw-copy checks and stop criteria before seeking live authorization.
- Full milestone acceptance remains: “Recorded authorized delivered-copy evidence establishes Bcc handling; document the ceiling and provider uncertainty. An unrun or failed privacy check keeps the rollout gate open.” Live Gmail account use is now authorized, but the required Mailven route setup and independent recipient mapping are incomplete and no delivered-copy test evidence exists; leave it blocked.

## Progress

- [x] Record selection, exclusions, base, branch and acceptance mapping.
- [x] Finish source/provider research and record confirmed findings in MA1 or a separate owning issue.
- [x] Update technical/provider documentation and write controlled live cases with an authorization template.
- [x] Run offline fixtures and documentation checks; independently review local scope.
- [x] Commit local checkpoint (`875d5c7`) and request live scope; owner authorized live Gmail use and recipient discovery.
- [ ] Prepare Mailven Gmail route and sufficient independent controlled recipients; obtain any new OAuth permission grant at its consent screen, then execute only concrete selected cases. Do not land or mark done while required delivered-copy checks remain unrun.

## Surprises & Discoveries

- `internal/gmailapi/send.go` measures `req.Raw` before adding hidden-recipient headers; the JSON request body is larger again after URL-safe base64.
- Google discovery metadata exposes a media-upload size field, distinct from the JSON `raw` endpoint used by Mailven. Published Gmail Help attachment figures do not directly establish a final MIME ceiling.

## Decision Log

- Research and test design first; do not waive live approval or Microsoft deferral.
- Existing limits and behavior remain intact. Distinguish a documented local resource policy from provider acceptance.

> **Superseded in part (rebased 2026-10-15).** After this checkpoint, `main` recorded the static documentation review and authorized Gmail Bcc cases 1–4 ([evidence](../validation/send-2.md)), and salvaged PD10 and DOC6; on 2026-10-16 PD10 was fixed and MAINT-11 resolved DOC6. The owner landed this checkpoint on 2026-10-16 as the investigation record; SEND-2 stays blocked on its live gate. The status below is the state when this branch was written; read the Progress and Issues entries in [PLAN.md](../../PLAN.md#send-2) for the current state.

## Checkpoint

Current step: live Gmail account-use permission received; recipient discovery and Mailven setup preparation underway. The connected primary account and owner-confirmed junk account are available in the browser; the custom-domain address is forwarding-only and cannot count as an independent recipient. Another backup Gmail identity is visible in the browser account chooser as a candidate, without mailbox-content inspection. Private identity mapping and source message IDs are in `/home/user/.cache/mailven-send2/live-aompttz4/preparation.json`; do not commit real addresses or messages.

Read-only metadata query of the existing Mailven database found no Gmail accounts. A Gmail connector send would not prove Mailven's route. An isolated XDG test profile was created at `/home/user/.cache/mailven-send2/live-aompttz4/` to prepare account setup without syncing existing accounts. No test message, account addition, permission grant, route activation or migration has occurred. Casey Labs forwarding and the unconfigured Mailven route are concrete execution blockers, distinct from the now-granted live-account permission. Next: inspect the isolated app's account-setup screen and record any remaining consent/recipient requirements. Large S1/S2 submissions and all Microsoft work remain outside this continuation.

Changed files: `PLAN.md`, `ISSUES.md`, `docs/MAIL_APIS.md`, `docs/PROVIDERS.md`, `docs/SENDING.md`, this plan, `docs/validation/send-2-gmail.md`. No implementation changes. Local checkpoint commit stays on this branch; do not land while full acceptance is unproved.

Validation passed:

- `python3 tools/project-check.py --base d42736a47d67f4160bc092ffdeed6c42447cb0dc --extra-command '["go","test","./internal/gmailapi","./internal/submission","./app","-run","^(TestSend|TestPreflight$|TestHiddenBcc$|TestPost|TestMailAPISend)","-count=1"]' --run`: whitespace PASS, docs PASS (7 files, 0 errors), all three selected Go package fixture groups PASS. Evidence/logs: `/home/user/.cache/mailven-project-checks/run-cfnthoi6/`.
- Independent acceptance: local research/protocol/issue capture PASS; full delivered-copy acceptance UNVERIFIABLE. Direct per-file diff review found no further corrections; no code simplification applies.
- Final documentation checks rerun after recording these results (initial generated-anchor link failed, then corrected with an explicit anchor); the Go implementation and tests were unchanged. Full Go suite, frontend/UI/native checks, live Gmail copies/size acceptance and all Microsoft checks unrun for this documentation-only local phase.

Remaining findings: MA1 live privacy/allowance and stale source comments, PD10 final-MIME validation parity, DOC6 stale snapshot guidance. Each has an owning issue entry. No next milestone selected.

## Outcomes & Retrospective

Gmail recipient selection and the original-MIME/encoded-request byte boundaries are documented; bounded G1/G2 and optional S1/S2 cases are reviewable. Offline checks and independent local acceptance passed. Full SEND-2 and MA1 remain blocked on executable Mailven setup and independent recipient evidence, with live Gmail use authorized but no live compatibility or privacy acceptance implied.
