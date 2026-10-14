// Guarded preflight guidance/error UI checks; run-mailven dev session required.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { withFakePage } = require('../.agents/skills/run-mailven/fake-page.js')
const { launch } = require('../.agents/skills/run-mailven/pw.js')
const out = process.argv[2]
if (!out) throw Error('Usage: node tools/preflight-check.cjs <output-directory>')
fs.mkdirSync(out, { recursive: true })
const sizeError = 'encoded message is 30000 bytes, exceeding the server SIZE limit of 20000 bytes; reduce attachments or content (draft retained)'
const utf8Error = 'this message requires SMTPUTF8, which the sending server does not advertise; use an ASCII mailbox or another configured account'
async function main() {
  await withFakePage({ width: 1400, height: 1000 }, async p => {
    await p.getByRole('button', { name: 'Show sidebar', exact: true }).click()
    await p.waitForTimeout(350)
    await p.screenshot({ path: path.join(out, 'mail-expanded.png') })
    await p.getByRole('button', { name: 'Hide sidebar', exact: true }).click()
    await p.waitForTimeout(350)
    await p.screenshot({ path: path.join(out, 'mail-collapsed.png') })
    console.log('PASS Mail Accounts expanded/collapsed')
  })
  const browser = await launch()
  try {
    const p = await browser.newPage({ viewport: { width: 1100, height: 900 } })
    await p.addInitScript({ path: path.join(__dirname, '../.agents/skills/run-mailven/composer-fakes.js') })
    await p.goto('http://localhost:34115/composer.html')
    const composer = p.getByRole('region', { name: 'Email composer' })
    await composer.waitFor()
    await p.getByText(/Encoded message size includes/).waitFor()
    assert.match(await p.getByText(/Encoded message size includes/).innerText(), /inline images.*encryption overhead.*unknown/)
    await p.getByPlaceholder('Subject').fill('Editable preflight draft')
    const to = p.getByPlaceholder(/recipients/i).first()
    await to.fill('sam@example.com'); await to.press('Enter')
    await composer.locator('[contenteditable=true]:visible').first().fill('Preserved body')
    for (const [name, error] of [['size', sizeError], ['smtputf8', utf8Error]]) {
      await p.evaluate(error => { window.go.app.App.SendMessage = async () => { window.__log.push('fake preflight rejection'); return Promise.reject(error) } }, error)
      await composer.getByRole('button', { name: 'Send', exact: true }).click()
      await p.getByRole('button', { name: /Send now/ }).click()
      await p.getByText(error, { exact: true }).waitFor()
      assert.equal(await p.getByPlaceholder('Subject').inputValue(), 'Editable preflight draft')
      assert.equal(await composer.locator('[contenteditable=true]:visible').first().innerText(), 'Preserved body')
      assert.equal(await p.evaluate(() => window.__log.includes('closeWindow')), false)
      await p.getByPlaceholder('Subject').focus()
      await p.keyboard.press('End'); await p.keyboard.type('!')
      await p.getByPlaceholder('Subject').fill('Editable preflight draft')
      await p.waitForTimeout(350)
      await p.screenshot({ path: path.join(out, `composer-${name}.png`) })
      console.log(`PASS ${name} rejection displayed; subject/body preserved and editable; composer stays open`)
    }
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
