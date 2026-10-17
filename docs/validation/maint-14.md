# MAINT-14 — Quoted names in Copy addresses

Owner approved MAINT-14 on 2026-10-16 (unattended `make milestone` run). [Roadmap](../../PLAN.md#maint-14); base `cab7d10`, branch `milestone/maint-14`.

## Result

- `chatFormat.ts` `formatAddress` renders `Name <email>`, quoting the name as an RFC 5322 quoted-string (escaping `\` and `"`) when it holds a special (`()<>[]:;@\,."`); no name gives the address, no address the display name.
- `chatClipboard.ts` `copyText` is the one clipboard-and-toast helper; `ChatPeopleMenu` (Copy addresses) and `ChatBubble` (copy text, copy source) use it, and the bubble's recipients title uses `formatAddress`.
- Independent acceptance check (`project-acceptance-checker`): all criteria pass; lint/check/build passed in `tools/project-check.py --run`. Authorization: owner-started runner, 2026-10-16, MAINT-14 only; no live or native step involved.
- UI7 is resolved. MA10 stays open for MAINT-18 (generated view-result types, redundant `subjectSections` test).

## Verification

- `node tests/ui/run.mjs logic`: 8 pass, including the table-driven `formatAddress` test (plain, comma, quote, angle bracket, no name, no address).
- `node tests/ui/run.mjs chat-header`: 8 pass, including Copy addresses with a `Shah, Priya` sender copying `"Shah, Priya" <priya@example.net>, …`.
- `cd frontend && npm run lint && npm run check`: 0 errors, 0 warnings.
- `make test-ui`: 136 passed, 0 failed, 0 skipped (includes the audit in both Accounts states).

## Limits

Chromium with fake data only; no native WebKitGTK check.
