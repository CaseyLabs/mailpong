// Headless README shots and Accounts-column crops. See "Playwright checks" in SKILL.md.
// Usage: node headless-shot.js <outdir>   (dev server on :34115, DISPLAY not needed)
// Writes the README shots <outdir>/ss.png (1040x660, dark), ss-light.png (1040x690, light, Accounts expanded)
// and ss-narrow.png (480x750, single-pane layout), plus <outdir>/vc-<theme>-<expanded|collapsed>.png (title bar + sidebar).
const { withFakePage } = require('./fake-page')
const out = process.argv[2]
if (!out) { console.error('usage: node headless-shot.js <outdir>'); process.exit(1) }
const toggleOf = (p) => p.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
// Scroll the open chat to its top so the shot starts at the subject heading, not a clipped bubble.
const scrollChatToTop = (p) => p.evaluate(() => { document.querySelector('.flex-1.min-h-0.overflow-y-auto.scrollbar-thin').scrollTop = 0 })
;(async () => {
  await withFakePage({ width: 1040, height: 660 }, async (p) => {
    await p.mouse.move(1039, 659)
    await p.waitForTimeout(600)
    await p.screenshot({ path: `${out}/ss.png` })
    // Theme x Accounts-column check. Dev builds force dark, so set data-theme by hand for light.
    const toggle = toggleOf(p)
    for (const theme of ['dark', 'light']) {
      await p.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme)
      for (const state of ['collapsed', 'expanded']) { // starts collapsed, so the first toggle expands
        await p.waitForTimeout(600)
        await p.screenshot({ path: `${out}/vc-${theme}-${state}.png`, clip: { x: 0, y: 0, width: 520, height: 330 } })
        await toggle.click()
      }
    }
  })
  // The heights end the chat between messages once it is scrolled to the top.
  await withFakePage({ width: 1040, height: 690 }, async (p) => {
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'))
    await toggleOf(p).click()
    await p.waitForTimeout(800)
    await scrollChatToTop(p)
    await p.mouse.move(1039, 689)
    await p.waitForTimeout(300)
    await p.screenshot({ path: `${out}/ss-light.png` })
  })
  await withFakePage({ width: 480, height: 750 }, async (p) => {
    await p.waitForTimeout(1500)
    await scrollChatToTop(p)
    await p.mouse.move(479, 749)
    await p.waitForTimeout(300)
    await p.screenshot({ path: `${out}/ss-narrow.png` })
  })
})().catch(error => { console.error(error); process.exitCode = 1 })
