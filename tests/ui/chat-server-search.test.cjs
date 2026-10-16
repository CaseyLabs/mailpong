// "Search the whole mailbox" under a chat search: offered for a Gmail-API account (SearchGmailServer) and a protocol
// account (SearchIMAPServer), never for Graph; it stores older matches and reloads the normal search.
const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = (route, added) => `
{
  const A = window.go.app.App
  A.GetMailAPIRouteStatus = async () => ({ route: ${JSON.stringify(route)}, state: 'active', shared: false, authorized: true, failure: '' })
  window.__serverSearches = 0
  A.SearchGmailServer = async () => { window.__serverSearches++; window.__via = 'gmail'; return { found: ${added}, added: ${added} } }
  A.SearchIMAPServer = async () => { window.__serverSearches++; window.__via = 'imap'; return { found: ${added}, added: ${added} } }
}
`

async function search(page, query) {
  await page.getByPlaceholder('Search chats...').fill(query)
}

for (const accounts of ['expanded', 'collapsed']) {
  test(`offers a Gmail server search for a Gmail-API account (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes('gmail-api', 3) }, async page => {
    await search(page, 'invoice')
    const button = page.getByRole('button', { name: 'Search the whole mailbox' })
    await button.waitFor()
    await button.click()
    await page.getByRole('status').filter({ hasText: 'Added 3 older messages from the server' }).waitFor()
    assert.deepEqual(await page.evaluate(() => [window.__serverSearches, window.__via]), [1, 'gmail'])
  }))
}

test('searches a protocol account over IMAP', ui.run({ beforeMount: fakes('protocol', 2).replace("route: \"protocol\", state: 'active'", "route: \"protocol\", state: ''") }, async page => {
  await search(page, 'invoice')
  await page.getByRole('button', { name: 'Search the whole mailbox' }).click()
  await page.getByRole('status').filter({ hasText: 'Added 2 older messages from the server' }).waitFor()
  assert.deepEqual(await page.evaluate(() => [window.__serverSearches, window.__via]), [1, 'imap'])
}))

test('does not offer it for a Graph account', ui.run({ beforeMount: fakes('graph-api', 0) }, async page => {
  await search(page, 'invoice')
  await page.getByText(/No chats match/).waitFor()
  assert.equal(await page.getByRole('button', { name: 'Search the whole mailbox' }).count(), 0)
}))
