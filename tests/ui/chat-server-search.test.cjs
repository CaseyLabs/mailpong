// "Search all of Gmail" under a chat search: offered for one Gmail-API account's folder, stores older matches
// through SearchGmailServer, and reloads the normal search.
const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = (route, added) => `
{
  const A = window.go.app.App
  A.GetMailAPIRouteStatus = async () => ({ route: ${JSON.stringify(route)}, state: 'active', shared: false, authorized: true, failure: '' })
  window.__serverSearches = 0
  A.SearchGmailServer = async (acct, query, limit) => { window.__serverSearches++; return { found: ${added}, added: ${added} } }
}
`

async function search(page, query) {
  await page.getByPlaceholder('Search chats...').fill(query)
}

for (const accounts of ['expanded', 'collapsed']) {
  test(`offers a Gmail server search for a Gmail-API account (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes('gmail-api', 3) }, async page => {
    await search(page, 'invoice')
    const button = page.getByRole('button', { name: 'Search all of Gmail' })
    await button.waitFor()
    await button.click()
    await page.getByRole('status').filter({ hasText: 'Added 3 older messages from Gmail' }).waitFor()
    assert.deepEqual(await page.evaluate(() => window.__serverSearches), 1)
  }))
}

test('does not offer it for a protocol account', ui.run({ beforeMount: fakes('protocol', 0) }, async page => {
  await search(page, 'invoice')
  await page.getByText(/No chats match/).waitFor()
  assert.equal(await page.getByRole('button', { name: 'Search all of Gmail' }).count(), 0)
}))
