// "Load older mail" at the end of a chat list: offered for Gmail-API accounts (LoadOlderGmail) and protocol
// accounts (LoadOlderIMAP), never Graph; loads a page, reloads the list, and reports the end once nothing is older.
const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = (route, steps) => `
{
  const A = window.go.app.App
  A.GetMailAPIRouteStatus = async () => ({ route: ${JSON.stringify(route)}, state: 'active', shared: false, authorized: true, failure: '' })
  const steps = ${JSON.stringify(steps)}
  window.__olderLoads = 0
  A.LoadOlderGmail = A.LoadOlderIMAP = async () => ({ found: 0, ...steps[Math.min(window.__olderLoads++, steps.length - 1)] })
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`loads older Gmail mail until the end (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes('gmail-api', [{ added: 5, done: false }, { added: 0, done: true }]) }, async page => {
    const button = page.getByRole('button', { name: 'Load older mail' })
    await button.waitFor()
    await button.click()
    await button.waitFor() // more remains: the button stays after a page that added mail
    await button.click()
    await page.getByRole('status').filter({ hasText: 'All older mail is loaded' }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Load older mail' }).count(), 0)
    assert.equal(await page.evaluate(() => window.__olderLoads), 2)
  }))
}

test('loads older mail for a protocol account', ui.run({ beforeMount: fakes('protocol', [{ added: 2, done: true }]).replace("state: 'active'", "state: ''") }, async page => {
  await page.getByRole('button', { name: 'Load older mail' }).click()
  await page.getByRole('status').filter({ hasText: 'All older mail is loaded' }).waitFor()
  assert.equal(await page.evaluate(() => window.__olderLoads), 1)
}))

test('does not offer it for a Graph account', ui.run({ beforeMount: fakes('graph-api', [{ added: 0, done: true }]) }, async page => {
  await page.getByRole('listbox').first().waitFor()
  assert.equal(await page.getByRole('button', { name: 'Load older mail' }).count(), 0)
}))
