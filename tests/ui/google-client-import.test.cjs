const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// The file picker returns a fixed path; the backend fake accepts one path and
// rejects another with a specific message, as the Go parser would.
const fakes = `
{
  const A = window.go.app.App
  window.__import = { picked: 0, imports: [] }
  A.PickGoogleClientFile = async () => { window.__import.picked++; return window.__import.path }
  A.ImportGoogleClientFile = async (path, ids) => {
    window.__import.imports.push({ path, ids: JSON.parse(JSON.stringify(ids)) })
    if (path === '/tmp/web.json') throw new Error('client file: this is a Web application client; create a Desktop app client and download its JSON')
    return { slots: ids, clientIdFingerprint: '….com' }
  }
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`Import JSON applies a Google client to the chosen slots and shows errors (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: /OAuth Credentials/ }).click()

    const importBtn = page.getByRole('button', { name: 'Import JSON…', exact: true })
    await importBtn.focus()
    await page.keyboard.press('Enter')
    assert.equal(await page.getByRole('checkbox').count(), 3)
    assert.equal(await page.getByRole('checkbox', { name: 'Google Mail', exact: true }).isChecked(), true)

    // Web-type file: specific error, no success toast, nothing saved by the fake.
    await page.evaluate(() => { window.__import.path = '/tmp/web.json' })
    await page.getByRole('button', { name: 'Choose file…', exact: true }).click()
    await page.getByRole('alert').getByText(/Web application client/).waitFor()

    // Desktop file onto Mail and Calendar only.
    await page.getByRole('checkbox', { name: 'Google Contacts', exact: true }).uncheck()
    await page.evaluate(() => { window.__import.path = '/tmp/ok.json' })
    await page.getByRole('button', { name: 'Choose file…', exact: true }).click()
    await page.getByText(/Google client ….com saved to 2 slots/).waitFor()
    const imports = await page.evaluate(() => window.__import.imports)
    assert.deepEqual(imports[1], { path: '/tmp/ok.json', ids: ['google-mail', 'google-calendar'] })
    assert.equal(await page.getByRole('checkbox').count(), 0, 'panel closes after a successful import')
  }))
}
