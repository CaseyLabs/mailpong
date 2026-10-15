const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// First run: Gmail disabled, People missing scope, Calendar not connected.
// After Enable and a second Check, everything works. The fake counts calls.
const fakes = `
{
  const A = window.go.app.App
  const getAccounts = A.GetAccounts
  A.GetAccounts = async () => (await getAccounts()).map((a, i) => i === 0 ? { ...a, authType: 'oauth2' } : a)
  A.GetSendAsStatus = async () => ({ supported: true, accountEmail: 'user@example.test', fetched: true, identities: {}, available: [] })
  window.__check = { runs: 0, opened: [] }
  A.CheckGoogleSetup = async id => {
    window.__check.runs++
    const ok = window.__check.opened.length > 0
    return { accountId: id, checkedAt: 0, results: ok
      ? [{ api: 'gmail', state: 'ok' }, { api: 'people', state: 'ok' }, { api: 'calendar', state: 'not-connected', detail: 'Calendar access has not been granted for this account.' }]
      : [{ api: 'gmail', state: 'disabled', detail: 'x', enableUrl: 'https://console.developers.google.com/apis/api/gmail.googleapis.com/overview?project=1234', project: '1234' },
         { api: 'people', state: 'missing-scope', detail: 'Google denied access.' },
         { api: 'calendar', state: 'error', detail: 'HTTP 500: oops' }] }
  }
  A.OpenURL = async url => { window.__check.opened.push(url) }
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`Check Google setup renders each state and recovers after Enable (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: 'Edit account', exact: true }).first().click()
    await page.getByRole('tab', { name: 'Identity', exact: true }).click()
    const section = page.getByRole('region', { name: 'Google setup check', exact: true })
    assert.equal((await page.evaluate(() => window.__check.runs)), 0, 'nothing runs until asked')

    await section.getByRole('button', { name: 'Check Google setup', exact: true }).focus()
    await page.keyboard.press('Enter')
    await section.getByText(/Disabled in Google Cloud project 1234/).waitFor()
    await section.getByText(/Missing permission: Google denied access/).waitFor()
    await section.getByText(/Error: HTTP 500/).waitFor()
    await section.getByRole('button', { name: 'Re-authorize', exact: true }).waitFor()

    await section.getByRole('button', { name: 'Enable API', exact: true }).focus()
    await page.keyboard.press('Enter')
    await page.waitForFunction(() => window.__check.opened.length === 1)
    assert.match(await page.evaluate(() => window.__check.opened[0]), /^https:\/\/console\.developers\.google\.com\/apis\/api\/gmail\.googleapis\.com\/overview\?project=1234$/)

    await section.getByRole('button', { name: 'Check Google setup', exact: true }).click()
    await section.getByText('Working').first().waitFor()
    await section.getByText(/Calendar access has not been granted/).waitFor()
    assert.equal(await section.getByRole('button', { name: 'Enable API', exact: true }).count(), 0)
    assert.equal(await section.getByRole('button', { name: 'Re-authorize', exact: true }).count(), 0)
    assert.equal((await page.evaluate(() => window.__check.runs)), 2)
  }))
}
