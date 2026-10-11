// Headless UI check for cloud sessions (no Claude-in-Chrome). See "Headless (cloud) path" in SKILL.md.
// Usage: node headless-shot.js <outdir>   (dev server on :34115, serve-fakes.py on :8765, DISPLAY not needed)
// Writes <outdir>/ss.png (1040x660, README shot) and <outdir>/vc-<theme>-<expanded|collapsed>.png (title bar + sidebar).
const { chromium } = require('/opt/node-tools/node_modules/playwright')
const out = process.argv[2]
;(async () => {
  const b = await chromium.launch({ executablePath: '/opt/pw-browsers/chromium' })
  const p = await b.newPage({ viewport: { width: 1300, height: 700 }, locale: 'en-GB' }) // en-GB gives 24-hour times
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
  const ui0 = await p.evaluate(async () => JSON.stringify(await window.go.app.App.GetUIState()))
  await p.evaluate(async () => {
    ;(0, eval)(await (await fetch('http://127.0.0.1:8765/screenshot-fakes.js')).text())
    document.body.innerHTML = ''; document.body.style.margin = '0'
    const f = document.createElement('iframe')
    f.style.cssText = 'width:1040px;height:660px;border:0;position:fixed;left:0;top:0'
    document.body.appendChild(f); f.src = location.href
    const poll = setInterval(() => { try { const w = f.contentWindow
      if (w.location.href !== 'about:blank' && w.go?.app?.App && !w.__faked) { w.__faked = true; __installFakes(w); clearInterval(poll) } } catch {} }, 1)
    await new Promise((r) => setTimeout(r, 8000))
  })
  await p.mouse.move(1250, 400)
  await p.screenshot({ path: `${out}/ss.png`, clip: { x: 0, y: 0, width: 1040, height: 660 } })
  // Theme x Accounts-column check. Dev builds force nord-dark, so set data-theme by hand for light.
  const fr = p.frames().find((f) => f !== p.mainFrame())
  const toggle = fr.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
  for (const theme of ['nord-dark', 'light']) {
    await fr.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme)
    for (const state of ['collapsed', 'expanded']) { // starts collapsed, so the first toggle expands
      await p.waitForTimeout(600)
      await p.screenshot({ path: `${out}/vc-${theme}-${state}.png`, clip: { x: 0, y: 0, width: 520, height: 330 } })
      await toggle.click()
    }
  }
  console.log('ui state unchanged:', ui0 === (await p.evaluate(async () => JSON.stringify(await window.go.app.App.GetUIState()))))
  await b.close()
})()
