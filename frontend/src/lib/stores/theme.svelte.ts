// Theme store - centralizes all theme application and system theme detection logic
//
// Used by both App.svelte (main window) and ComposerApp.svelte (detached composer).
// The OS theme probe is injected by the caller because each Wails process binds a
// different Go struct (App vs ComposerApp), and importing the wrong binding at the
// module level silently fails at runtime.

import { getThemeMode, type ThemeMode } from './settings.svelte'
import { WindowSetBackgroundColour } from '$wailsjs/runtime/runtime'

export type { ThemeMode }

// Internal state for portal-based system theme (XDG Settings Portal on Linux)
let portalThemeAvailable = false
let portalTheme: 'light' | 'dark' = 'light'

// Reactive flag mirroring the `.dark` class on <html>. Consumers (e.g., the
// email-content dark-filter toggle) need a Svelte-reactive way to observe it.
let isDarkActive = $state<boolean>(false)

export function getIsDarkActive(): boolean {
  return isDarkActive
}

/** Apply a resolved theme to the document element. The dark/light classification
 *  is read from the CSS-declared `color-scheme` property on the matching
 *  [data-theme="..."] block, so each theme owns its own scheme — no JS list to
 *  maintain. We mirror it as the `.dark` class so Tailwind `dark:` variants and
 *  any `.dark mark`-style selectors keep working.
 *  Fork: dev builds (`make dev`) always render Dark, whatever theme is
 *  saved, so dev sessions and screenshots match the fork's default look. The
 *  backend-free test mode (window.__fakeBackend) keeps the requested theme so
 *  UI tests can cover every theme through the real theme path. */
export function applyTheme(themeName: ThemeMode) {
  const fakeBackend = (window as { __fakeBackend?: boolean }).__fakeBackend
  if (import.meta.env.DEV && !fakeBackend) themeName = 'dark'
  document.documentElement.setAttribute('data-theme', themeName)
  const scheme = getComputedStyle(document.documentElement).colorScheme.trim()
  const dark = scheme === 'dark'
  document.documentElement.classList.toggle('dark', dark)
  isDarkActive = dark
  syncWindowBackground()
}

/** Match the native window background to the theme's --background, so the
 *  area the webview has not painted yet while the window grows shows the theme
 *  colour instead of a light strip. Read through a probe element because the
 *  body's background-color transitions between themes. */
function syncWindowBackground() {
  const probe = document.createElement('div')
  probe.style.backgroundColor = 'hsl(var(--background))'
  document.body.appendChild(probe)
  const colour = getComputedStyle(probe).backgroundColor
  probe.remove()
  // Only an opaque rgb() colour is usable: a missing or translucent
  // --background resolves to rgba(), which would paint the window black.
  const rgb = colour.match(/^rgb\((\d+(?:\.\d+)?), (\d+(?:\.\d+)?), (\d+(?:\.\d+)?)\)$/)
  if (!rgb) return
  const [r, g, b] = rgb.slice(1).map((c) => Math.round(Number(c)))
  try {
    WindowSetBackgroundColour(r, g, b, 255)
  } catch {
    // No Wails runtime (plain browser): nothing to sync.
  }
}

/** Resolve a ThemeMode (which may be 'system') to a concrete theme and apply it. */
export function applyThemeFromMode(mode: ThemeMode) {
  if (mode !== 'system') {
    applyTheme(mode)
    return
  }

  // System mode: use portal-based theme if available, otherwise fall back to matchMedia
  if (portalThemeAvailable) {
    applyTheme(portalTheme)
    return
  }

  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  applyTheme(mediaQuery.matches ? 'dark' : 'light')
}

/**
 * Initialize the theme on mount.
 * Probes the XDG Settings Portal for system theme via the caller-supplied binding,
 * then applies the stored mode.
 */
export async function initTheme(
  storedMode: ThemeMode,
  getSystemTheme: () => Promise<string>,
) {
  try {
    const sysTheme = await getSystemTheme()
    if (sysTheme === 'light' || sysTheme === 'dark') {
      portalThemeAvailable = true
      portalTheme = sysTheme
    }
  } catch {
    // Portal not available, will use matchMedia fallback
  }

  applyThemeFromMode(storedMode)
}

/** Handle backend 'theme:system-preference' events (XDG Settings Portal changes). */
export function handleSystemThemeEvent(newTheme: string) {
  if (newTheme !== 'light' && newTheme !== 'dark') return

  portalThemeAvailable = true
  portalTheme = newTheme
  if (getThemeMode() === 'system') {
    applyTheme(portalTheme)
  }
}

/** Handle matchMedia 'change' events (fallback when portal is unavailable). */
export function handleMediaQueryChange(matches: boolean) {
  if (getThemeMode() !== 'system' || portalThemeAvailable) return
  applyTheme(matches ? 'dark' : 'light')
}

/** Handle 'theme:changed' IPC events for composer windows. */
export function handleThemeChanged(newTheme: string) {
  if (newTheme === 'system') {
    applyThemeFromMode('system')
    return
  }
  applyTheme(newTheme as ThemeMode)
}
