# Review Guidance

Repo-specific rules for code review. General conventions live in `AGENTS.md`.

## Report as important

- Extension, core API (`internal/core/`), or kit (`internal/kit/`, `frontend/src/lib/components/kit/`) changes that break a rule in `docs/EXT_RULES.md`. Cite the rule number.
- Credentials, tokens, keys, or message content written to logs, errors, or events.
- Weakened checks in `internal/{certificate,credentials,crypto,keyring,oauth2,pgp,smime}/`, or in TLS/STARTTLS handling. Hold findings here to a high bar of evidence, but do not drop them for style reasons.
- Lost keyboard access in a UI change: a control or pane that can no longer be reached or operated from the keyboard.

## Nits

- Svelte components over 500 lines.
- Missing comments on exported Go functions.

## Skip

- Generated files: `frontend/wailsjs/`, `frontend/dist/`.
- Non-English locale files under `*/i18n/locales/`. MailPong updates English only, so missing or stale translations are expected.
- Formatting that `gofmt` or ESLint already enforces.
