# Localization architecture

Mailven's interface is English-only. The i18n machinery from Aerion is kept so a locale could be added later, and email content is unaffected: messages in any language, charset and script are parsed, displayed and sent as before. That handling lives in the Go backend (`internal/sync/parse.go`, `internal/smtp`, ...) and does not use the UI locale files.

## How strings work

- Every UI string is a key in `frontend/src/lib/i18n/locales/en.json`, read with `$_('namespace.key')` (svelte-i18n). Extensions ship their own `extensions/<name>/frontend/i18n/locales/en.json` under a top-level namespace; `frontend/src/lib/i18n/index.ts` discovers each extension's `i18n/index.ts` through `import.meta.glob`.
- Update the English files only when adding or changing a string. Check `npm run check` and `npm run build`.
- `supportedLocales` in `i18n/index.ts` lists the shipped UI locales. With one entry, Settings → General hides the language picker. A saved language that is not listed (for example from an imported Aerion database) resolves to `en` without error.
- `i18n/dateFnsLocale.ts` loads the date-fns locale for a non-English UI locale; English uses date-fns' default.

## Spellcheck

Dictionaries are the `dictionary-en`, `dictionary-en-gb` and `dictionary-en-ca` npm packages. `scripts/copy-dictionaries.mjs` gzips them into `frontend/public/spellcheck/` at build time. `SPELLCHECK_DICTS` and `DICT_NAMES` in `frontend/src/lib/spellcheck/locales.ts` list and name them, and Settings → Composer lets the user enable any combination. With none chosen, the system language picks en-US, en-GB or en-CA. Saved dictionary keys that are no longer shipped are ignored.

## Adding a locale later

1. Add `locales/<code>.json` to the core and, optionally, to each extension (missing extension keys fall back to English).
2. `register('<code>', ...)` in `frontend/src/lib/i18n/index.ts` and in each extension's `i18n/index.ts`.
3. Add the locale to `supportedLocales` and, if system detection should pick it, to `detectSystemLocale()`.
4. Add a `case` to `loadDateFnsLocale()` in `dateFnsLocale.ts`.
5. If it needs spellcheck, add a `dictionary-*` package, then update `LOCALES` in `scripts/copy-dictionaries.mjs`, `SPELLCHECK_DICTS`/`DICT_NAMES` and `APP_TO_DICT` in `spellcheck/locales.ts`. Check each dictionary's licence first (see `NOTICE`).
6. Translate the desktop entry (`build/linux/`) and AppStream metainfo (`xml:lang`) if wanted.
7. Run `npm run check`, `npm run build` and `go test ./...`; switch the language in the running app, including the detached composer.

## Key Namespaces

### Core (`frontend/src/lib/i18n/locales/en.json`)

| Namespace | Description |
|-----------|-------------|
| `common` | Shared buttons and labels (Save, Cancel, Delete, etc.) |
| `sidebar` | Sidebar navigation (Compose, All Inboxes, folder names) |
| `messageList` | Message list UI (select all, no messages, loading) |
| `viewer` | Message viewer (reply, forward, attachments, error states, S/MIME/PGP banners) |
| `composer` | Email composer (To, Cc, Subject, Send, formatting) |
| `contextMenu` | Right-click context menus (Reply, Archive, Mark as Read) |
| `toast` | Toast notification messages (clean translated messages without raw error details) |
| `responsive` | Responsive layout labels (back, folders) |
| `settings` | Settings dialog tabs and titles |
| `settingsAbout` | About tab in settings |
| `settingsAccounts` | Accounts tab in settings |
| `settingsGeneral` | General settings tab (theme, density, read receipts) |
| `editor` | TipTap editor toolbar labels |
| `account` | Account dialog and management |
| `identity` | Identity editor (email address management, display names, signatures) |
| `security` | S/MIME and PGP security settings |
| `contactSource` | CardDAV contact source management |
| `certificate` | TLS certificate trust dialog |
| `terms` | Terms of service dialog |
| `dialog` | Generic dialog strings (confirmations, warnings) |
| `date` | Date/time labels (just now, yesterday, etc.) |
| `aria` | Accessibility labels (screen reader text) |
| `window` | Window management (minimize, maximize, close) |
| `attachment` | Attachment handling (download, save, open) |
| `search` | Search UI |
| `sort` | Sort options (newest first, oldest first) |
| `oauth` | OAuth flow UI |

### Extensions (`extensions/<name>/frontend/i18n/locales/en.json`)

Each extension namespaces its keys under the extension id. For example, the Contacts extension uses `contacts.*`:

| Namespace | Description |
|-----------|-------------|
| `contacts` | Contacts extension UI (browse, edit, source management, account-setup hook panel) |

Future extensions add their own top-level namespace (e.g., `calendar.*` for a Calendar extension). Extensions never reuse core namespaces — duplicate `common.save` strings are intentional, the cost is negligible, and it keeps each extension's translation file self-contained.

## Key Conventions

- **Error/failure messages**: Use clean, translated messages. Do not include raw error details or `{error}` interpolation tokens in failure messages — keep them user-friendly (e.g., `"Failed to save."` not `"Failed to save: {error}"`).
- **Placeholder tokens**: `{placeholder}` tokens are used for dynamic values. Common tokens include:
  - `{name}` — account or contact source name
  - `{email}` / `{emails}` — email address(es)
  - `{count}` — numeric count (messages, attachments, etc.)
  - `{mode}` — composer mode (reply, forward, etc.)
  - `{time}` — relative time string (from date-fns)
  - `{folder}` — folder name
  - `{percentage}` — sync progress percentage
  - `{version}` — app version string
  - `{provider}` — OAuth provider name (Google, Microsoft)
  - `{query}` — search query text
  - `{domain}` / `{sender}` — email domain or sender address
  - `{description}` — undo action description
  - `{filename}` — attachment filename
- **Token positioning**: Keep each sentence's tokens together so another language could reorder them; don't build sentences by concatenating fragments.

