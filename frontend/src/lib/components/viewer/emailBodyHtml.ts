// Pure HTML transforms for the sandboxed email body iframe (moved verbatim from EmailBody.svelte).

// Loading placeholder SVG
export const loadingPlaceholder = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='120' height='80' viewBox='0 0 120 80'%3E%3Crect fill='%23f3f4f6' width='120' height='80' rx='4'/%3E%3Cg transform='translate(60,40)'%3E%3Ccircle cx='0' cy='0' r='12' fill='none' stroke='%239ca3af' stroke-width='2' stroke-dasharray='20 10'%3E%3CanimateTransform attributeName='transform' type='rotate' from='0' to='360' dur='1s' repeatCount='indefinite'/%3E%3C/circle%3E%3C/g%3E%3Ctext x='60' y='65' text-anchor='middle' fill='%239ca3af' font-size='9' font-family='sans-serif'%3ELoading...%3C/text%3E%3C/svg%3E`

// Regex pattern for CSS url() with remote http(s) URLs.
// Handles all quote styles: raw ' or ", decimal &#39;/&#34;, hex &#x27;/&#x22;, named &apos;/&quot;
// Used as a string so we can create fresh RegExp instances (avoids lastIndex issues with /g)
export const CSS_QUOTE = `(?:['"]|&#(?:39|x27|34|x22);|&(?:apos|quot);)?`
export const CSS_REMOTE_URL_PATTERN = `url\\(\\s*${CSS_QUOTE}\\s*https?://[^)]*?${CSS_QUOTE}\\s*\\)`

export function checkForRemoteImages(html: string): boolean {
  if (!html) return false
  // Check <img> tags with remote src
  if (/<img[^>]+src=["'](https?:\/\/[^"']+)["']/i.test(html)) return true
  // Check CSS url() references with remote URLs (background-image, background, etc.)
  if (new RegExp(CSS_REMOTE_URL_PATTERN, 'i').test(html)) return true
  // Check HTML background attribute with remote URLs
  if (/\bbackground\s*=\s*["'](https?:\/\/[^"']+)["']/i.test(html)) return true
  return false
}

export function processCidReferences(html: string): string {
  if (!html) return html
  return html.replace(
    /src=["']cid:([^"']+)["']/gi,
    (match, contentId) => `src="${loadingPlaceholder}" data-cid="${contentId}"`
  )
}

export function processHtml(html: string, blockImages: boolean): string {
  if (!html) return ''
  let processed = processCidReferences(html)
  if (blockImages) {
    // Block <img> tags with remote sources
    processed = processed.replace(
      /(<img[^>]+)src=["'](https?:\/\/[^"']+)["']([^>]*>)/gi,
      (match, before, src, after) => {
        const placeholder = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='60' viewBox='0 0 100 60'%3E%3Crect fill='%23e5e7eb' width='100' height='60'/%3E%3Ctext x='50' y='35' text-anchor='middle' fill='%239ca3af' font-size='10' font-family='sans-serif'%3EImage blocked%3C/text%3E%3C/svg%3E`
        return `${before}src="${placeholder}" data-blocked-src="${encodeURIComponent(src)}"${after}`
      }
    )
    // Block remote URLs in CSS url() references (covers background-image, background, etc.)
    // Handles all quote encodings: raw, decimal entities, hex entities, named entities
    processed = processed.replace(new RegExp(CSS_REMOTE_URL_PATTERN, 'gi'), 'url()')
    // Block HTML background attribute with remote URLs
    processed = processed.replace(
      /\bbackground\s*=\s*["'](https?:\/\/[^"']+)["']/gi,
      'background=""'
    )
  }
  return processed
}

// Reads the active theme's chrome lightness so the dark-mail filter can tune
// its invert amount — pure black is too dark against most themes' chrome.
// For themes whose --background hue carries enough chromaticity that the
// partial-invert + hue-rotate composes into visible color shifts, declare
// --dark-mail-bg-l (number 0-100) to override with an explicit value.
export function getChromeBgLightness(): number {
  const styles = getComputedStyle(document.documentElement)
  const override = styles.getPropertyValue('--dark-mail-bg-l').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n)) return Math.max(0, Math.min(1, n / 100))
  }
  const bg = styles.getPropertyValue('--background').trim()
  const lMatch = bg.match(/(\d+(?:\.\d+)?)%\s*$/)
  if (lMatch) {
    const l = parseFloat(lMatch[1])
    if (!Number.isNaN(l)) return Math.max(0, Math.min(1, l / 100))
  }
  return 0
}

// Returns the chrome bg as `hsl(...)` for the iframe element's outer
// background, matching the rendered dark-mail surface so the rounded
// corner clip has no color seam.
export function getChromeBgHsl(): string {
  const styles = getComputedStyle(document.documentElement)
  const bg = styles.getPropertyValue('--background').trim()
  return bg ? `hsl(${bg})` : '#000'
}

// Per-theme dark-mail saturation override. <1 desaturates the inverted
// email surface to cancel out the warm/red chromatic shift that partial
// invert + hue-rotate(180deg) leaves on the residual chromaticity of
// most input content. Default 1 (no change).
export function getChromeBgSaturate(): number {
  const override = getComputedStyle(document.documentElement)
    .getPropertyValue('--dark-mail-saturate').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n) && n > 0) return n
  }
  return 1
}

// Per-theme dark-mail hue rotation override (degrees, signed). Shifts the
// residual chromaticity of the inverted email surface toward a specific
// hue family — e.g., -20 to cool down a cool-toned theme's surface that
// would otherwise read as warm gray. Default 0 (no shift, GPU-skipped).
export function getChromeBgHueRotate(): number {
  const override = getComputedStyle(document.documentElement)
    .getPropertyValue('--dark-mail-hue').trim()
  if (override) {
    const n = parseFloat(override)
    if (!Number.isNaN(n)) return n
  }
  return 0
}

// Extract domain from email address
export function extractDomain(email: string): string {
  const parts = email.split('@')
  return parts.length === 2 ? parts[1] : ''
}

export function linkifyText(text: string): string {
  if (!text) return ''
  // One pass with alternation so an address inside a URL stays part of
  // that link instead of getting a nested mailto anchor.
  const linkPattern = /(https?:\/\/[^\s<>"{}|\\^`[\]]+)|([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/g
  const escaped = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return escaped.replace(linkPattern, (_match, url: string | undefined, email: string | undefined) =>
    url
      ? `<a href="${url}" target="_blank" rel="noopener noreferrer" class="text-primary hover:underline">${url}</a>`
      : `<a href="mailto:${email}" class="text-primary hover:underline">${email}</a>`,
  )
}
