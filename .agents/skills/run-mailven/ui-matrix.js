// UI check matrix: theme x width x Accounts column x previews, plus hovered and focused row crops.
// See "Playwright checks" in SKILL.md.
// Usage: node ui-matrix.js <outdir> <prefix>   (dev server on :34115)
// Writes <outdir>/<prefix>-<theme>-<width>-<expanded|collapsed>-<previews|nopreviews>.png and, at 1400 px,
// <prefix>-<theme>-<expanded|collapsed>-<previews|nopreviews>-row-<hover|focus>.png.
const { withFakePage } = require('./fake-page')
const [out, prefix] = process.argv.slice(2)
if (!out || !prefix) { console.error('usage: node ui-matrix.js <outdir> <prefix>'); process.exit(1) }

const HEIGHT = 800
const WIDTHS = [1400, 900, 700]
const THEMES = ['dark', 'light']

withFakePage({ width: 1400, height: HEIGHT }, async (p) => {
  const setWidth = async (w) => { await p.setViewportSize({ width: w, height: HEIGHT }); await p.waitForTimeout(500) }
  const setPreviews = (on) => p.evaluate(async (on) => {
    const url = performance.getEntriesByType('resource').map((e) => new URL(e.name).pathname + new URL(e.name).search)
      .filter((u) => u.includes('settings.svelte.ts')).pop() ?? '/src/lib/stores/settings.svelte.ts'
    ;(await import(url)).setShowMessagePreview(on)
  }, on)
  const toggle = p.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
  const isCollapsed = async () => (await toggle.getAttribute('aria-label') ?? await toggle.getAttribute('title')).startsWith('Show')
  const setAccounts = async (state) => {
    if (!(await toggle.count())) return false
    if ((await isCollapsed()) !== (state === 'collapsed')) { await toggle.click(); await p.waitForTimeout(500) }
    return true
  }
  const rows = p.locator('.chat-row')

  for (const theme of THEMES) {
    await p.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme)
    for (const previews of ['previews', 'nopreviews']) {
      await setPreviews(previews === 'previews')
      for (const state of ['expanded', 'collapsed']) {
        for (const w of WIDTHS) {
          await setWidth(w)
          const ok = await setAccounts(state)
          // Overlay layouts (below 1025 px of app width) have no Accounts toggle: shoot once, as "expanded".
          if (!ok && state === 'collapsed') continue
          await p.mouse.move(1399, HEIGHT - 1)
          await p.waitForTimeout(300)
          await p.screenshot({ path: `${out}/${prefix}-${theme}-${w}-${state}-${previews}.png`, clip: { x: 0, y: 0, width: w, height: HEIGHT } })
          if (w !== 1400) continue
          // Row crops: hover the third row; click the second (selects it, focuses the list pane) for the focus ring.
          const n = await rows.count()
          if (n < 3) { console.log('too few rows for crops:', n); continue }
          const crop = async (row, name) => {
            const bb = await row.boundingBox()
            if (bb) await p.screenshot({ path: `${out}/${prefix}-${theme}-${state}-${previews}-row-${name}.png`, clip: { x: Math.max(0, bb.x - 4), y: bb.y - 4, width: bb.width + 8, height: bb.height + 8 } })
          }
          await rows.nth(2).hover(); await p.waitForTimeout(300); await crop(rows.nth(2), 'hover')
          await rows.nth(1).click(); await p.mouse.move(1399, HEIGHT - 1); await p.waitForTimeout(300); await crop(rows.nth(1), 'focus')
        }
      }
    }
  }
  console.log('done')
}).catch(error => { console.error(error); process.exitCode = 1 })
