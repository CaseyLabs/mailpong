// Contrast gate for the theme text tiers. No dependencies.
// Usage: node tools/contrast-check.mjs   (exit 1 if any text tier on a surface is below 4.5:1)
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const hsl = (h, s, l) => {
  s /= 100; l /= 100
  const k = (n) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  return [0, 8, 4].map((n) => 255 * (l - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1))))
}
const parse = (s) => {
  s = s.trim()
  let m
  if ((m = s.match(/^#([0-9a-f]{3})$/i))) return [...m[1]].map((c) => parseInt(c + c, 16))
  if ((m = s.match(/^#([0-9a-f]{6})/i))) return [0, 2, 4].map((i) => parseInt(m[1].slice(i, i + 2), 16))
  if ((m = s.match(/^rgb\(\s*(\d+)[ ,]+(\d+)[ ,]+(\d+)\s*\)$/i))) return [+m[1], +m[2], +m[3]]
  if ((m = s.match(/^(\d+(?:\.\d+)?)\s+(\d+(?:\.\d+)?)%\s+(\d+(?:\.\d+)?)%$/))) return hsl(+m[1], +m[2], +m[3])
  throw new Error(`cannot parse colour: ${s}`)
}
const lin = (c) => ((c /= 255) <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
const lum = ([r, g, b]) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
const ratio = (fg, bg) => {
  const [a, b] = [lum(parse(fg)), lum(parse(bg))]
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
}

const css = readFileSync(fileURLToPath(new URL('../frontend/src/themes/built-in.css', import.meta.url)), 'utf8')
const themes = { light: ':root,\\s*\\[data-theme="light"\\]', dark: '\\[data-theme="dark"\\]' }
// Pane surfaces plus the selected-row fill (muted).
const surfaces = ['surface-rail', 'surface-sidebar', 'surface-list', 'surface-content', 'muted']
const tiers = ['primary', 'secondary', 'tertiary']
const MIN = 4.5

let failed = false
for (const [name, selector] of Object.entries(themes)) {
  const block = css.match(new RegExp(`${selector}\\s*\\{([^}]*)\\}`))?.[1]
  if (!block) { console.error(`theme block not found: ${name}`); process.exit(1) }
  const get = (v) => block.match(new RegExp(`--${v}:\\s*([^;/]+?)\\s*;`))?.[1]
  console.log(`\n${name}`)
  for (const t of tiers) {
    const fg = get(`fg-${t}`)
    const cells = surfaces.map((s) => {
      const bg = get(s)
      if (!fg || !bg) { failed = true; return `${s.replace('surface-', '')}: missing` }
      let r = 0
      try { r = ratio(fg, bg) } catch { failed = true; return `${s.replace('surface-', '')}: unparseable` }
      if (r < MIN) failed = true
      return `${s.replace('surface-', '')} ${r.toFixed(2)} ${r >= MIN ? 'pass' : 'FAIL'}`
    })
    console.log(`  fg-${t.padEnd(9)} ${cells.join('  |  ')}`)
  }
}
process.exit(failed ? 1 : 0)
