const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// A first run: no accounts, so the chat list offers Add Account. The guide's
// buttons sit inside the Add Account form and must not submit it.
const fakes = `
{
  const A = window.go.app.App
  window.__guide = { opened: [], picked: 0 }
  A.GetAccounts = async () => []
  A.GetChats = async () => []
  A.GetChatCount = async () => 0
  A.GetConfiguredOAuthProviders = async () => ['google']
  A.IsOAuthConfigured = async () => true
  A.GetGoogleSetupGuide = async () => ({
    steps: [{ id: 'project', url: 'https://console.cloud.google.com/projectcreate', fallbackUrl: 'https://console.cloud.google.com/', menuPath: 'Project picker → New Project' }],
    scopes: ['openid'],
  })
  A.OpenURL = async url => { window.__guide.opened.push(url) }
  A.PickGoogleClientFile = async () => { window.__guide.picked++; return '' }
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async () => {} } })
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`No accounts: chat list offers Add Account, and the Gmail setup guide does not submit the form (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes, ready: 'body' }, async page => {
    assert.equal(await page.getByText('Inbox zero').count(), 0)
    await page.getByText('No accounts yet').last().waitFor()
    await page.getByRole('button', { name: 'Add Account', exact: true }).last().click()

    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: /Gmail/ }).click()
    await dialog.getByRole('button', { name: 'Setup checklist', exact: true }).click()
    await dialog.getByRole('button', { name: 'Create project', exact: true }).click()
    await dialog.getByRole('button', { name: 'Copy scopes', exact: true }).click()
    await dialog.getByRole('button', { name: /Import JSON/ }).click()
    await dialog.getByRole('button', { name: 'Choose file…', exact: true }).click()

    assert.deepEqual(await page.evaluate(() => window.__guide.opened), ['https://console.cloud.google.com/projectcreate'])
    assert.equal(await page.evaluate(() => window.__guide.picked), 1)
    assert.equal(await dialog.getByText(/is required|complete the sign-in/).count(), 0)
    assert.equal((await ui.calls(page, 'AddAccount')).length, 0)
  }))
}

// Release builds ship no Google client: Gmail starts on App Password, and the
// checklist's inline import makes Sign in with Google available.
const noClient = fakes + `
{
  const A = window.go.app.App
  let imported = false
  A.GetConfiguredOAuthProviders = async () => imported ? ['google'] : []
  A.IsOAuthConfigured = async p => p === 'google' && imported
  A.PickGoogleClientFile = async () => '/tmp/client_secret.json'
  A.ImportGoogleClientFile = async () => { imported = true; return { slots: ['google-mail'], clientIdFingerprint: '…abcd' } }
}
`

test('No Google client: the Gmail form offers the checklist, and importing a client enables Sign in with Google', ui.run({ accounts: 'collapsed', beforeMount: noClient, ready: 'body' }, async page => {
  await page.getByRole('button', { name: 'Add Account', exact: true }).last().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: /Gmail/ }).click()
  await dialog.getByText(/Mailven needs a Google Cloud client of your own/).waitFor()
  assert.equal(await dialog.getByRole('button', { name: 'Sign in with Google' }).count(), 0)

  await dialog.getByRole('button', { name: 'Setup checklist', exact: true }).click()
  await dialog.getByRole('button', { name: /Import JSON/ }).click()
  await dialog.getByRole('button', { name: 'Choose file…', exact: true }).click()

  await dialog.getByRole('button', { name: 'Sign in with Google' }).waitFor()
  assert.equal(await dialog.getByText(/is required/).count(), 0)
  assert.equal((await ui.calls(page, 'AddAccount')).length, 0)
}))
