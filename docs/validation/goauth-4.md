# GOAUTH-4 — Guided Google Cloud setup

Owner selected GOAUTH-4 on 2026-10-15. [Roadmap](../../PLAN.md#goauth-4); pre-commit base `28d9aa7`, branch `feat/goauth-4`. No linked issues.

## Result

- `internal/oauth2/google_setup.go` holds the six console steps (project, enable APIs, branding, audience, data access, clients) with a deep link, the console home as fallback, and the menu path. `GoogleSetupScopes()` is the union of the Mail, Contacts and Calendar provider scopes plus `gmail.modify` and the contacts write scope. `App.GetGoogleSetupGuide` exposes both.
- `GoogleSetupGuide.svelte` is an eight-step checklist behind a **Setup checklist** button, in Settings → Accounts → OAuth Credentials and on the Google sign-in form. Each step has a done checkbox, an **Open** button that uses the system browser, and "Page not right?" text with the menu path and a console-home button. The scope step lists the scopes with **Copy scopes**.
- The custom-credentials guide now follows the same eight steps as numbered sections 1–8 (Calendar and Contacts extension setup follow as 9–10) and names the checklist.

## Verification

- `go test ./internal/oauth2 -run 'GoogleSetup|GoogleCredentialsGuide'`: pass. Tests fail when a provider scope, or any Google scope literal in non-test Go code under `app`, `internal` or `extensions`, is missing from the list (the Gmail API scopes always requested with `https://mail.google.com/` are exempt), and when the guide's scope table differs from the list.
- `node tests/ui/run.mjs google-setup-guide`: pass for Accounts expanded and collapsed (keyboard open, links, fallback, copy, done checkbox toggled with Space). `make test-ui`: 95 passed.
- `npm run lint`, `npm run check`: pass. `python3 tools/check-doc-links.py --changed`: pass. `go test ./internal/oauth2 ./app`: pass.

## Limits

- The live check used only navigation and the read-only Next step of the enable flow; nothing was created, enabled or saved.

## Live link check (2026-10-15)

Owner authorized; owner's Google account in Chrome, signed in with an existing project selected. Each link opened in the last-selected project (the console adds `?project=`), so step 1 tells the user to keep the new project selected.

| Step | Link | Result |
| --- | --- | --- |
| Project | `console.cloud.google.com/projectcreate` | Pass: New Project form |
| APIs | `…/flows/enableapi?apiid=gmail.googleapis.com,people.googleapis.com,calendar-json.googleapis.com` | Pass: redirects to `apis/enableflow`; Confirm project, then "You are about to enable: Gmail API, People API, Google Calendar API" (Enable not clicked) |
| Branding | `…/auth/branding` | Pass: Branding page (App information) |
| Audience | `…/auth/audience` | Pass: Audience page (Publishing status, User type) |
| Data Access | `…/auth/scopes` | Pass: Data Access page with Add or remove scopes |
| Clients | `…/auth/clients` | Pass: Clients page with Create client |
| Fallback | `console.cloud.google.com/` | Pass: console home loads |

- The checklist's done marks are not saved between sessions.
