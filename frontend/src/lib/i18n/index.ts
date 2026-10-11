import { register, init, waitLocale, locale, _ } from 'svelte-i18n'

// Register CORE locale files with lazy loading. Extensions register their own
// locales via Vite glob auto-discovery in initI18n() — see below.
// en must be first — it's the fallback/default.
register('en', () => import('./locales/en.json'))

// Vite-discovered extension i18n modules. Each enabled extension that ships
// translations exports a registerExtensionI18n() function from its
// frontend/i18n/index.ts. Adding a new extension's i18n is purely a file
// drop — no edit to this file needed. See docs/EXTENSIONS.md § Extension
// i18n for the contract.
const extensionI18nModules = import.meta.glob<{
  registerExtensionI18n: () => void
}>('../../../../extensions/*/frontend/i18n/index.ts', { eager: true })

// Supported locales for the language picker
// en must be first — it's the fallback/default.
export const supportedLocales = [
  { code: 'en', name: 'English' },
] as const

/**
 * Map any locale code to one the UI ships. The UI is English-only, so this
 * returns 'en' for everything else, including a non-English language saved in
 * an imported Aerion database. Message lookup would fall back to 'en' anyway;
 * resolving up front avoids an unregistered locale ever reaching svelte-i18n.
 */
export function resolveLocale(code: string | undefined | null): string {
  return supportedLocales.find(l => l.code === code)?.code ?? 'en'
}

/**
 * Detect the UI locale. The UI is English-only, so this is always 'en'.
 * Add a case here (and a supportedLocales entry) when a locale is shipped.
 */
export function detectSystemLocale(): string {
  return 'en'
}

/**
 * Initialize i18n and wait for the initial locale to load.
 * Must be awaited before mounting the Svelte app, otherwise $_ throws.
 * @param savedLocale - Previously saved locale code from backend settings, or undefined for auto-detect
 */
export async function initI18n(savedLocale?: string): Promise<void> {
  const initialLocale = resolveLocale(savedLocale || detectSystemLocale())

  // Register every discovered extension's locale loaders before init() so
  // their messages are merged into the active locale on first wait.
  for (const mod of Object.values(extensionI18nModules)) {
    mod.registerExtensionI18n?.()
  }

  init({
    fallbackLocale: 'en',
    initialLocale,
  })

  await waitLocale()
}

/**
 * Change the active locale at runtime.
 */
export function setLocale(code: string) {
  locale.set(resolveLocale(code))
}

export { _, locale }
