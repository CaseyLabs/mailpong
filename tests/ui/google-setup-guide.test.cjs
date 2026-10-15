const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = `
{
  const A = window.go.app.App
  window.__guide = { opened: [], copied: '' }
  const c = 'https://console.cloud.google.com/'
  A.GetGoogleSetupGuide = async () => ({
    steps: [
      { id: 'project', url: c + 'projectcreate', fallbackUrl: c, menuPath: 'Project picker → New Project' },
      { id: 'apis', url: c + 'flows/enableapi?apiid=gmail.googleapis.com', fallbackUrl: c, menuPath: 'APIs & Services → Library' },
      { id: 'scopes', url: c + 'auth/scopes', fallbackUrl: c, menuPath: 'Google Auth Platform → Data Access' },
    ],
    scopes: ['https://mail.google.com/', 'openid'],
  })
  A.OpenURL = async url => { window.__guide.opened.push(url) }
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async t => { window.__guide.copied = t } } })
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`Setup checklist opens console links, offers a fallback and copies the scopes (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: /OAuth Credentials/ }).click()

    // Keyboard: focus the toggle and open it with Enter.
    await page.getByRole('button', { name: 'Setup checklist', exact: true }).focus()
    await page.keyboard.press('Enter')
    assert.equal(await page.getByRole('checkbox', { name: /^Mark done:/ }).count(), 8)

    await page.getByRole('button', { name: 'Create project', exact: true }).click()
    await page.getByRole('button', { name: 'Enable APIs', exact: true }).click()
    await page.getByRole('button', { name: 'the console' }).first().click()
    const opened = await page.evaluate(() => window.__guide.opened)
    assert.deepEqual(opened, [
      'https://console.cloud.google.com/projectcreate',
      'https://console.cloud.google.com/flows/enableapi?apiid=gmail.googleapis.com',
      'https://console.cloud.google.com/',
    ])

    await page.getByRole('button', { name: 'Copy scopes', exact: true }).click()
    assert.equal(await page.evaluate(() => window.__guide.copied), 'https://mail.google.com/\nopenid')

    const doneBox = page.getByRole('checkbox', { name: 'Mark done: Create a project' })
    await doneBox.focus()
    await page.keyboard.press('Space')
    assert.equal(await doneBox.isChecked(), true)
  }))
}
