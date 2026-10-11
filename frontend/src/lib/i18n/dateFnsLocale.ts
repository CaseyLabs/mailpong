import type { Locale } from 'date-fns'

// Lazy-loaded date-fns locale cache
const localeCache = new Map<string, Locale>()

/**
 * Get the date-fns locale for a given i18n locale code.
 * Returns undefined if the locale hasn't been loaded yet (use loadDateFnsLocale first).
 */
export function getDateFnsLocale(code: string): Locale | undefined {
  return localeCache.get(code)
}

/**
 * Load and cache the date-fns locale for a given i18n locale code.
 * English doesn't need an explicit locale (date-fns defaults to en-US).
 */
export async function loadDateFnsLocale(code: string): Promise<Locale | undefined> {
  if (code === 'en') return undefined
  if (localeCache.has(code)) return localeCache.get(code)

  // The UI ships English only, so there is nothing else to load. To ship a
  // locale, import its date-fns locale here and store it in localeCache, e.g.
  //   const mod = await import('date-fns/locale/de')
  //   localeCache.set(code, mod.de)
  return undefined
}
