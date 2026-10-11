// Applies the persisted spellcheck settings to the worker. Call on app startup
// (after loadSettings), whenever the settings change, and on composer mount.
// Disabled → tear the worker down (no squiggles). Enabled with an explicit
// language set → use it; empty set → fall back to the system language's English variant.
import { getSpellcheckEnabled, getSpellcheckLanguages, getSpellcheckCustomWords, setSpellcheckCustomWords } from '$lib/stores/settings.svelte'
import { AddSpellcheckCustomWord, RemoveSpellcheckCustomWord } from '$wailsjs/go/app/App.js'
import { appLocaleToDict } from './locales'
import { spellcheck } from './client'

// The effective default dictionary when the user hasn't chosen one: the system
// language's English variant (en-CA and en-GB map to their own dictionaries,
// everything else to en-US). Used by both the engine and the settings UI so
// they agree on what's "on" out of the box.
export function defaultSpellcheckLanguages(): string[] {
  const nav = typeof navigator === 'undefined' ? '' : navigator.language
  return [appLocaleToDict(nav) ?? 'en']
}

export function syncSpellcheckLanguages(): void {
  if (!getSpellcheckEnabled()) {
    spellcheck.setLanguages([])
    return
  }
  const langs = getSpellcheckLanguages()
  spellcheck.setLanguages(langs.length ? langs : defaultSpellcheckLanguages())
  // Re-seed the (possibly freshly created) worker with the user dictionary.
  for (const w of getSpellcheckCustomWords()) spellcheck.addWord(w)
}

// "Add to dictionary": persist the word, seed the worker, and update the store.
export function addCustomWord(word: string): void {
  spellcheck.addWord(word)
  if (getSpellcheckCustomWords().includes(word)) return
  setSpellcheckCustomWords([...getSpellcheckCustomWords(), word])
  AddSpellcheckCustomWord(word).catch(() => {})
}

// Remove a word from the user dictionary: drop it from the store + backend and
// tell the worker to flag it again.
export function removeCustomWord(word: string): void {
  setSpellcheckCustomWords(getSpellcheckCustomWords().filter((w) => w !== word))
  spellcheck.removeWord(word)
  RemoveSpellcheckCustomWord(word).catch(() => {})
}

// Live-apply settings changes only when a composer worker is already running
// (a composer is open). Otherwise do nothing — the worker stays lazy and the
// next composer mount applies the current settings. Keeps unrelated settings
// saves from spinning up the spellcheck worker.
export function syncSpellcheckLanguagesIfActive(): void {
  if (spellcheck.isActive) syncSpellcheckLanguages()
}
