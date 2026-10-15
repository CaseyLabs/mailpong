# GOAUTH-3 — Import a client_secret JSON file

Owner selected GOAUTH-3 on 2026-10-15. [Roadmap](../../PLAN.md#goauth-3); pre-commit base `2a971f2`, branch `feat/goauth-disabled-api`. No linked issues.

## Result

- `oauth2.ParseGoogleClientJSON` accepts only Google's Desktop shape (top-level `installed` with `client_id` and `client_secret`). It rejects files over 64 KB, non-UTF-8 text, malformed JSON, `web` clients (with a message to create a Desktop app client), other shapes and missing fields. Its errors never contain file content, and `GoogleClient` redacts the secret from every fmt verb.
- `App.PickGoogleClientFile` opens the native dialog. `App.ImportGoogleClientFile(path, slots)` limits the read, parses, then saves through `credStore.SetUserClientCreds` and marks each slot's choice Custom. Only `google-mail`, `google-contacts` and `google-calendar` are accepted. The result carries the slot list and the client ID's last four characters, never the secret, and the log line lists slots only.
- `GoogleClientImport.svelte` sits above the Google Mail editor in Settings → Accounts → OAuth Credentials (not in the kit editor, which stays provider-agnostic). **Import JSON…** reveals three checkboxes (all ticked) and **Choose file…**; a failure shows an alert and the panel stays open; success toasts and remounts the Mail editor.
- Docs: [custom Google credentials guide](../user-guide/getting-started/first-account/google-oauth-credentials.md).

## Deviations from the plan

- The button is not inside `OAuthCredsSlotEditor`: the slot choice needs Google-specific parsing, and EXT_RULES R25/R26 keep kit primitives generic. No `internal/extensions`, `coreapi` or kit file changed, so the EXT_RULES review found nothing to redesign.
- Slots are applied one by one, not atomically; a store failure midway reports how many were saved.

## Verification

- `go test ./internal/oauth2 ./app -run 'GoogleClient|ImportGoogle'`: pass. Table tests cover desktop, BOM, web, other types, malformed, wrong types, missing fields, non-UTF-8 and oversized input, and that no error or formatted value contains the secret. The app test uses the mock keyring, saves only the chosen slots, and checks that rejected imports and bad slot lists change nothing.
- `node tests/ui/run.mjs google-client-import`: pass for Accounts expanded and collapsed. It covers keyboard opening, the web-file error, unchecking a slot and the exact slots sent.
- `tools/project-check.py --base 2a971f2 --run`: whitespace, doc links, `npm run lint`, `npm run check`, `npm run build`, `make test` and `make test-ui` (93 passed) pass. `make lint` then reported one staticcheck finding in the new test, fixed, and passes with 0 issues.

## Limits

- The native file dialog and a real Google download are unexercised (fakes only).
- A developer machine's real keyring holds user credentials, so the app test must call `gokeyring.MockInit()`; a first draft without it read the real entry before it could write.
