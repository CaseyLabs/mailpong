// UI check matrix: theme x width x Accounts column x previews, plus hovered and focused row crops.
// See "Playwright checks" in SKILL.md.
// Usage: node ui-matrix.js <outdir> <prefix>   (dev server on :34115, serve-fakes.py on :8765)
// Writes <outdir>/<prefix>-<theme>-<width>-<expanded|collapsed>-<previews|nopreviews>.png and, at 1400 px,
// <prefix>-<theme>-<expanded|collapsed>-<previews|nopreviews>-row-<hover|focus>.png.
const { launch } = require('./pw')
const [out, prefix] = process.argv.slice(2)
if (!out || !prefix) { console.error('usage: node ui-matrix.js <outdir> <prefix>'); process.exit(1) }

const HEIGHT = 800
const WIDTHS = [1400, 900, 700]
const THEMES = ['dark', 'light']

;(async () => {
  const b = await launch()
  const p = await b.newPage({ viewport: { width: 1400, height: HEIGHT }, locale: 'en-GB' })
  p.on('pageerror', (e) => console.log('pageerror:', e.message))
  await p.goto('http://localhost:34115'); await p.waitForTimeout(3000)
  // Fresh data dir: dismiss Terms/OAuth/What's New, and enable the extensions, on the plain page (the fakes block writes).
  const sw = p.getByRole('switch')
  for (let i = 0; i < 5; i++) {
    if (await sw.count()) await sw.first().click().catch(() => {})
    const btn = p.getByRole('button', { name: /^(OK|Accept & Continue)$/ })
    if (!(await btn.count())) break
    await btn.first().click(); await p.waitForTimeout(1200)
  }
  await p.evaluate(async () => { const A = window.go.app.App; await A.SetExtensionEnabled('contacts', true); await A.SetExtensionEnabled('calendar', true) })
  await p.evaluate(async (h) => {
    ;(0, eval)(await (await fetch('http://127.0.0.1:8765/screenshot-fakes.js')).text())
    document.body.innerHTML = ''; document.body.style.margin = '0'
    const f = document.createElement('iframe')
    f.id = 'app'
    f.style.cssText = `width:1400px;height:${h}px;border:0;position:fixed;left:0;top:0`
    document.body.appendChild(f); f.src = location.href
    const poll = setInterval(() => { try { const w = f.contentWindow
      if (w.location.href !== 'about:blank' && w.go?.app?.App && !w.__faked) { w.__faked = true; __installFakes(w); clearInterval(poll) } } catch {} }, 1)
    await new Promise((r) => setTimeout(r, 8000))
  }, HEIGHT)
  const fr = p.frames().find((f) => f !== p.mainFrame())
  const setWidth = async (w) => { await p.evaluate((w) => { document.getElementById('app').style.width = w + 'px' }, w); await p.waitForTimeout(500) }
  const setPreviews = (on) => fr.evaluate(async (on) => {
    const url = performance.getEntriesByType('resource').map((e) => new URL(e.name).pathname + new URL(e.name).search)
      .filter((u) => u.includes('settings.svelte.ts')).pop() ?? '/src/lib/stores/settings.svelte.ts'
    ;(await import(url)).setShowMessagePreview(on)
  }, on)
  const toggle = fr.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
  const isCollapsed = async () => (await toggle.getAttribute('aria-label') ?? await toggle.getAttribute('title')).startsWith('Show')
  const setAccounts = async (state) => {
    if (!(await toggle.count())) return false
    if ((await isCollapsed()) !== (state === 'collapsed')) { await toggle.click(); await p.waitForTimeout(500) }
    return true
  }
  const rows = fr.locator('.chat-row')

  for (const theme of THEMES) {
    await fr.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme)
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
  await b.close()
  console.log('done')
})()
