// Maps an app locale or BCP 47 tag (e.g. "en", "en-CA", "en-GB") to a hunspell
// dictionary key, or null when Mailven ships no dictionary for the language.
// The UI is English-only; the dictionaries cover its English variants.
const APP_TO_DICT: Record<string, string> = {
  en: 'en',
}

// Dictionaries bundled as static assets under /spellcheck/<key>.{aff,dic}.
// Keep in step with LOCALES in scripts/copy-dictionaries.mjs.
export const SPELLCHECK_DICTS = ['en', 'en-gb', 'en-ca'] as const

// Display names for the dictionaries in the settings list. 'en' is the US
// SCOWL wordlist, so disambiguate it from the other English variants.
export const DICT_NAMES: Record<string, string> = {
  en: 'English (US)',
  'en-gb': 'English (UK)',
  'en-ca': 'English (Canada)',
}

export function appLocaleToDict(locale: string | null | undefined): string | null {
  if (!locale) return null
  const lower = locale.toLowerCase()
  // Already a dictionary key (the settings toggles store these) or a system
  // en-GB/en-CA tag. Base-splitting first would collapse these to the 'en'
  // (US) dictionary and spellcheck the wrong variant.
  if ((SPELLCHECK_DICTS as readonly string[]).includes(lower)) return lower
  const base = lower.split('-')[0]
  return APP_TO_DICT[base] ?? null
}

// Drops saved dictionary keys that are no longer shipped (e.g. de, fr, nl
// from an imported Aerion database).
export function filterSupportedDicts(keys: readonly string[]): string[] {
  return keys.filter((k) => (SPELLCHECK_DICTS as readonly string[]).includes(k))
}
