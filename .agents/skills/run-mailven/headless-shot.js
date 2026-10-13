// Headless README shot and Accounts-column crops. See "Playwright checks" in SKILL.md.
// Usage: node headless-shot.js <outdir>   (dev server on :34115, DISPLAY not needed)
// Writes <outdir>/ss.png (1040x660, README shot) and <outdir>/vc-<theme>-<expanded|collapsed>.png (title bar + sidebar).
const { withFakePage } = require('./fake-page')
const out = process.argv[2]
if (!out) { console.error('usage: node headless-shot.js <outdir>'); process.exit(1) }
withFakePage({ width: 1040, height: 660 }, async (p) => {
  await p.mouse.move(1039, 659)
  await p.screenshot({ path: `${out}/ss.png` })
  // Theme x Accounts-column check. Dev builds force dark, so set data-theme by hand for light.
  const toggle = p.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
  for (const theme of ['dark', 'light']) {
    await p.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme)
    for (const state of ['collapsed', 'expanded']) { // starts collapsed, so the first toggle expands
      await p.waitForTimeout(600)
      await p.screenshot({ path: `${out}/vc-${theme}-${state}.png`, clip: { x: 0, y: 0, width: 520, height: 330 } })
      await toggle.click()
    }
  }
}).catch(error => { console.error(error); process.exitCode = 1 })
