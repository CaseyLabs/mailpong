// The Accounts sidebar's labeled actions (UIX-3): "New message" and "Search" at the top, and the list header's
// icon-only Compose that remains when the sidebar is collapsed.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

describe('sidebar actions', () => {
  it('Search focuses the chat search field', ui.run({}, async (page) => {
    const sidebar = page.locator('aside').first()
    await sidebar.getByRole('button', { name: 'New message' }).waitFor()
    await sidebar.getByRole('button', { name: 'Search', exact: true }).click()
    const focused = await page.evaluate(() => document.activeElement?.getAttribute('placeholder'))
    assert.equal(focused, 'Search chats...', 'the sidebar action moves focus to the list search')
  }))

  it('New message opens the composer', ui.run({}, async (page) => {
    const sidebar = page.locator('aside').first()
    await sidebar.getByRole('button', { name: 'New message' }).click()
    await page.getByRole('region', { name: 'Email composer' }).waitFor()
  }))

  it('the list header keeps Compose when the sidebar is collapsed', ui.run({ accounts: 'collapsed' }, async (page) => {
    await page.getByRole('button', { name: 'Compose' }).first().waitFor()
    assert.equal(await page.getByRole('button', { name: 'New message' }).count(), 0, 'labeled actions only live in the expanded sidebar')
  }))
})
