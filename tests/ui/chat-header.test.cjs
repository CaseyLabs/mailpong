// The open chat's header: title, "More actions" menu beside it, Archive and Delete. Snooze and Pin live in the menu.
// Writes resolve only for the bindings each test allows; assertions read the recorded binding calls.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const header = (page) => page.locator('header', { has: page.getByRole('heading', { level: 2 }) })

// At narrow widths an open chat covers the list, so open one only when none shows.
async function open(page, text) {
  await page.locator('[data-chat-row]', { hasText: text }).waitFor()
  if (!(await header(page).isVisible())) await page.locator('[data-chat-row]', { hasText: text }).click()
  await header(page).getByRole('button', { name: 'More actions' }).waitFor()
}

async function argsOf(page, name) {
  await page.waitForFunction((n) => window.__calls.some((x) => x.name === n), name, { timeout: 5000 }).catch(() => {})
  return (await ui.calls(page, name)).map((c) => c.args)
}

describe('chat header', () => {
  for (const width of [1200, 700, 500]) {
    it(`shows only Archive, Delete and More actions at ${width}px`, ui.run({ width }, async (page) => {
      await open(page, 'Omar Haddad')
      const names = await header(page).getByRole('button').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') || e.textContent.trim()))
      for (const n of ['Archive', 'Delete', 'More actions']) assert.ok(names.includes(n), `${n} is in the header: ${names}`)
      for (const n of ['Pin', 'Unpin']) assert.ok(!names.includes(n), `${n} is not a header button`)
      assert.equal(await header(page).getByRole('button', { name: 'Snooze', exact: true }).locator('visible=true').count(), 0, 'Snooze has no header button')
    }))
  }

  it('reaches Pin, Snooze and Delete from the menu and Archive by keyboard', ui.run({ allow: ['PinChat', 'Archive', 'Undo'] }, async (page) => {
    await open(page, 'Omar Haddad')
    const more = header(page).getByRole('button', { name: 'More actions' })
    await more.click()
    for (const n of ['Snooze...', /^(Pin|Unpin)$/, 'Delete']) {
      assert.ok(await page.getByRole('menuitem', typeof n === 'string' ? { name: n, exact: true } : { name: n }).first().isVisible(), `${n} is in the menu`)
    }
    await page.getByRole('menuitem', { name: /^(Pin|Unpin)$/ }).first().click()
    assert.equal((await argsOf(page, 'PinChat')).length, 1, 'Pin from the menu pins the open chat')

    await more.click()
    await page.getByRole('menuitem', { name: 'Snooze...', exact: true }).first().click()
    await page.getByRole('menuitem', { name: /Tomorrow/ }).first().waitFor()
    await page.keyboard.press('Escape')

    const archive = header(page).getByRole('button', { name: 'Archive' })
    await archive.focus()
    await page.keyboard.press('Enter')
    assert.equal((await argsOf(page, 'Archive')).length, 1, 'Archive runs from the keyboard')
  }))
})
