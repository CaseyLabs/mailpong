# MailPong Frontend

The desktop UI uses Svelte 5, TypeScript, Vite, and Tailwind CSS. Follow the downstream workflow and coding conventions in [CONTRIBUTING.md](../CONTRIBUTING.md).

## Structure

- `src/App.svelte`: main application UI.
- `src/ComposerApp.svelte`: detached composer UI.
- `src/lib/`: shared components, stores, and application logic.
- `src/assets/` and `src/themes/`: bundled assets and themes.
- `wailsjs/`: generated Go bindings and Wails runtime helpers.
- `../extensions/*/frontend/`: extension UI and translations.

## Development

Use Node.js 24 to match CI. From this directory:

```bash
npm ci
npm run build
```

Run `make dev` from the repository root to start the desktop application with hot reload. `npm run dev` starts only Vite; desktop functionality needs the Wails backend. The production output in `dist/` is embedded by Go and must exist before root Go tests or linting.

## Checks and Conventions

- `npm run check`: Svelte and TypeScript checks.
- `npm run lint`: ESLint checks.
- `npm run lint:fix`: apply automatic lint fixes; review the changes.
- `npm run build`: production frontend build.

There is no frontend test script configured. Manually verify changed desktop flows, including keyboard navigation and the detached composer where relevant.

Use two-space indentation, single quotes, no semicolons, and Svelte 5 runes. Regenerate bindings with `make generate` from the repository root after backend API changes. See [the translation guide](../docs/LANGUAGE.md) when changing locale support.
