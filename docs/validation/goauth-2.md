# GOAUTH-2 — Google client health check

Owner selected GOAUTH-2 on 2026-10-15. [Roadmap](../../PLAN.md#goauth-2); issue [GO1](../../ISSUES.md#go1); pre-commit base `ff499ca`, branch `feat/goauth-disabled-api`.

## Result

- `internal/googlecheck` classifies one read-only GET per API into `ok`, `disabled` (typed from GOAUTH-1's `googleerr`, with the Enable URL Mailven builds), `missing-scope` (403 that is not a rate limit), `not-connected` or `error`. Gmail runs `users/me/profile`, then the send-as list, and reports the first failure. People runs `people/me/connections?personFields=names&pageSize=1`, the call `contacts.readonly` covers. Calendar lists one calendar. No probe writes.
- `App.CheckGoogleSetup(accountID)` (Google OAuth accounts only) gets the Gmail token the same way the send-as refresh does (`composeOps.sendAsToken`, extracted from `refreshSendAs`), and reaches People and Calendar through the extension auth broker. A missing grant is `not-connected`, never a consent prompt.
- `GoogleSetupCheck.svelte` (Identity tab, below the send-as section) lists the results in an `aria-live` list. Enable API calls `OpenURL`; Re-authorize runs the existing `oauthStore.reauthorize`, then checks again. Nothing runs until Check is pressed, except once after the first Google sign-in (`accountsStore.addOAuthAccount` for Google, and `addCustomOAuthAccount` for a Gmail host), which toasts only on `disabled` or `missing-scope`.
- Docs: [gmail.md](../user-guide/getting-started/first-account/gmail.md).

## Deviations from the plan

- People uses `people/me/connections`, not `people/me`: the granted `contacts.readonly` scope is documented for it, while `people/me` may need `profile`.
- The first-sign-in run lives in the frontend store, after the account exists, so the backend stays a pure on-demand call.

## Verification

- `go test ./internal/googlecheck ./app -run 'Probes|Gmail|GoogleSetup|SendAs'`: pass. Fakes cover ok, disabled, missing scope, rate limit, 401 and 500 for all three probes, first-failure stop, and GET-only requests.
- `node tests/ui/run.mjs google-setup`: pass for Accounts expanded and collapsed. It covers disabled, missing-scope, error and not-connected rendering, keyboard activation of Check and Enable API, no call before Check, the exact Enable URL, and recovery on a second Check without a restart.
- `tools/project-check.py --base ff499ca --extra-command '["make","lint"]' --run`: whitespace, doc links, `npm run lint`, `npm run check`, `npm run build`, `make test`, `make test-ui` (91 passed) and `make lint` all pass. After the simplification pass removed an unused helper, `go vet` and the focused Go tests above passed again, and `make lint` reported 0 issues.
- Independent acceptance check: all three criteria pass. Its only gap is that Re-authorize is not keyboard-tested; it is a native button like the tested ones.

## Limits

- Local fixtures only; no real Google response was exercised. The owner-run live check (a client with one API disabled) is unrun, as is a native-window check.
- Which scope a real `403` reports for People is unverified; a 403 that is not a rate limit is shown as missing permission.
- The first-sign-in run has no UI test: it needs the OAuth flow.
