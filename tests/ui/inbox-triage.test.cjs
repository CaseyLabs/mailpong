// Inbox management through the chat list: the row context menu, the toast's Undo and the Low priority group.
// Writes resolve only for the bindings each test allows; assertions read the recorded binding calls.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const row = (page, text) => page.locator('[data-chat-row]', { hasText: text })

// Opens a row's context menu and picks an item by its exact accessible name. The chat's own items come
// first, before the shared message items (which repeat Archive; ISSUES.md UI3).
async function menu(page, rowText, item) {
  await row(page, rowText).click({ button: 'right' })
  await page.getByRole('menuitem', { name: item, exact: true }).first().click()
}

const undo = (page) => page.getByRole('button', { name: 'Undo', exact: true }).first().click()

// Waits for the named binding to have been called and returns its argument lists.
async function argsOf(page, name, count = 1) {
  await page.waitForFunction(([n, c]) => window.__calls.filter((x) => x.name === n).length >= c, [name, count], { timeout: 5000 })
    .catch(() => {})
  return (await ui.calls(page, name)).map((c) => c.args)
}

describe('inbox triage', () => {
  it('pin and unpin, with undo', ui.run({ allow: ['PinChat', 'Undo'] }, async (page) => {
    await menu(page, 'Omar Haddad', 'Pin')
    assert.deepEqual(await argsOf(page, 'PinChat'), [['acc1', 't-omar', true]], 'Pin pins the thread by its key')
    await undo(page)
    assert.equal((await argsOf(page, 'Undo')).length, 1, 'the toast Undo calls the backend Undo')
    await menu(page, 'Weekend trip to the coast', 'Unpin')
    assert.deepEqual((await argsOf(page, 'PinChat', 2))[1], ['acc1', 't-trip', false], 'a pinned chat offers Unpin')
  }))

  it('snooze and unsnooze', ui.run({
    allow: ['SnoozeChat', 'UnsnoozeChat'],
    // Grace's chat is snoozed, so its menu offers Unsnooze.
    beforeMount: `{ const A = window.go.app.App, get = A.GetChats
      A.GetChats = async (...a) => (await get(...a)).map((c) => c.threadId === 't-grace' ? { ...c, snoozedUntil: new Date(Date.now() + 864e5).toISOString() } : c) }`,
  }, async (page) => {
    await menu(page, 'Omar Haddad', 'Snooze...')
    await page.getByRole('menuitem', { name: /^Tomorrow/ }).click()
    const [[account, key, until]] = await argsOf(page, 'SnoozeChat')
    assert.deepEqual([account, key], ['acc1', 't-omar'])
    const at = new Date(until) // the browser runs in UTC (harness)
    assert.ok(until > Date.now() && at.getUTCHours() === 8 && at.getUTCMinutes() === 0, `Tomorrow snoozes to 08:00 tomorrow, got ${at.toISOString()}`)
    await menu(page, 'Grace Okafor', 'Unsnooze')
    assert.deepEqual(await argsOf(page, 'UnsnoozeChat'), [['acc1', 't-grace']])
  }))

  it('a snoozed chat with unread mail keeps an unread dot on its glyph', ui.run({
    beforeMount: `{ const A = window.go.app.App, get = A.GetChats
      A.GetChats = async (...a) => (await get(...a)).map((c) => c.threadId === 't-grace' ? { ...c, unreadCount: 2, snoozedUntil: new Date(Date.now() + 864e5).toISOString() } : c.threadId === 't-omar' ? { ...c, unreadCount: 0, snoozedUntil: new Date(Date.now() + 864e5).toISOString() } : c) }`,
  }, async (page) => {
    await row(page, 'Grace Okafor').locator('[data-row-status="snoozed"] [data-row-unread]').waitFor()
    assert.equal(await row(page, 'Omar Haddad').locator('[data-row-status="snoozed"] [data-row-unread]').count(), 0, 'a read snoozed chat has no dot')
  }))

  it('archive and undo', ui.run({ allow: ['Archive', 'Undo'] }, async (page) => {
    await menu(page, 'Omar Haddad', 'Archive')
    assert.deepEqual(await argsOf(page, 'Archive'), [[['t-omar-m']]], 'Archive moves every listed message of the chat')
    await page.getByText('Conversation archived').waitFor()
    await undo(page)
    assert.equal((await argsOf(page, 'Undo')).length, 1)
  }))

  it('read and unread', ui.run({ allow: ['MarkAsRead', 'MarkAsUnread'] }, async (page) => {
    await menu(page, 'Omar Haddad', 'Mark as Unread')
    assert.deepEqual(await argsOf(page, 'MarkAsUnread'), [[['t-omar-m']]], 'a read chat offers Mark as Unread')
    await menu(page, 'Jamie Chen', 'Mark as Read')
    assert.deepEqual(await argsOf(page, 'MarkAsRead'), [[['t-jamie-m']]], 'an unread chat offers Mark as Read')
  }))

  it('automatic Low priority group', ui.run({}, async (page) => {
    const header = page.getByRole('button', { name: 'Low priority (2)' })
    await header.waitFor()
    assert.equal(await header.getAttribute('aria-expanded'), 'false', 'the fixture starts with the Low group collapsed')
    assert.equal(await row(page, 'Trail Magazine').count(), 0, 'Low priority chats stay out of the main list')
    await header.click()
    await row(page, 'Trail Magazine').waitFor()
    await row(page, 'Corner Grocer').waitFor()
    assert.equal(await row(page, 'Omar Haddad').count(), 1, 'Priority chats remain listed once')
  }))

  it('manual sender priority override, with undo', ui.run({ allow: ['SetSenderCategory'] }, async (page) => {
    await menu(page, 'Omar Haddad', 'Move sender to Low priority')
    await page.getByRole('button', { name: 'Low priority (2)' }).click()
    await menu(page, 'Trail Magazine', 'Move sender to Priority')
    await undo(page)
    assert.deepEqual(await argsOf(page, 'SetSenderCategory', 3), [
      ['acc1', 'omar.h@example.org', 'low'],
      ['acc1', 'news@trailmag.example', 'priority'],
      ['acc1', 'news@trailmag.example', ''],
    ], 'overrides set low/priority by sender; Undo clears the override')
  }))
})
