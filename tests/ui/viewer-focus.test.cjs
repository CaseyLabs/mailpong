// Closed responsive viewers must reject focus while retaining their mounted state (UI4).
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const controls = 'button, a[href], input, textarea, select, [tabindex]'

const rejectsFocus = async (viewer) => {
  const result = await viewer.evaluate((el, selector) => {
    // Clear focus left on the Back button before testing fresh focus attempts.
    document.activeElement?.blur()
    const all = [...el.querySelectorAll(selector)]
    for (const control of all) {
      control.focus()
      if (el.contains(document.activeElement)) return { rejected: false, count: all.length }
    }
    return { rejected: true, count: all.length }
  }, controls)
  assert.ok(result.count > 0, 'exercise mounted viewer controls')
  assert.ok(result.rejected, 'closed viewer must reject focus on every control')
}

const acceptsFocus = async (viewer) => {
  const button = viewer.getByRole('button').first()
  await button.focus()
  assert.ok(await button.evaluate((el) => document.activeElement === el), 'visible viewer remains focusable')
}

// One synthetic local contact for the Contacts list and detail pane.
const contactFake = `{ const A = window.go.app.App
  const ada = { id: 'c-ada', name: 'Ada Lovelace', emails: ['ada@example.test'], sourceId: 'local', updatedAt: null }
  A.Contacts_ListContactsForBrowse = async () => [ada]
  A.Contacts_GetContactDetail = async () => ada
}`

const back = (viewer) => viewer.getByRole('button', { name: 'Back', exact: true })

// After Back, focus is back in the list and Tab never enters the closed viewer.
const tabLeaves = async (page, viewer) => {
  const handed = await viewer.evaluate((el) => document.activeElement !== document.body && !el.contains(document.activeElement))
  assert.ok(handed, 'closing the viewer hands focus to the list')
  let reached = false
  for (let i = 0; i < 3; i++) {
    await page.keyboard.press('Tab')
    const where = await viewer.evaluate((el) => {
      const a = document.activeElement
      return { inside: el.contains(a), control: a !== document.body && a.checkVisibility() }
    })
    assert.ok(!where.inside, 'Tab must not enter the closed viewer')
    reached ||= where.control
  }
  assert.ok(reached, 'Tab after closing the viewer reaches a visible control')
}

describe('responsive viewer focus', () => {
  for (const width of [700, 900]) {
    it(`mail at ${width}px: open, closed, reopened, focus mode and full width`, ui.run({ width }, async (page) => {
      const viewer = page.locator('main[data-pane="viewer"]')
      // The restored conversation opens as the viewer overlay.
      await page.waitForSelector('main.responsive-viewer-visible')
      await acceptsFocus(viewer)
      await back(viewer).click()
      await page.waitForSelector('main.responsive-viewer-visible', { state: 'detached' })
      await tabLeaves(page, viewer)
      await rejectsFocus(viewer)
      await page.locator('[data-chat-row]').first().click()
      await page.waitForSelector('main.responsive-viewer-visible')
      await acceptsFocus(viewer)
      await back(viewer).click()
      await page.waitForSelector('main.responsive-viewer-visible', { state: 'detached' })
      await rejectsFocus(viewer)
      // Thread focus mode shows the viewer overlay even when the responsive view is closed.
      await page.keyboard.press('f')
      await page.waitForSelector('main.responsive-viewer-visible')
      await acceptsFocus(viewer)
      await page.keyboard.press('Escape')
      await page.waitForSelector('main.responsive-viewer-visible', { state: 'detached' })
      await rejectsFocus(viewer)
      await page.setViewportSize({ width: 1400, height: 800 })
      await page.waitForFunction(() => !document.querySelector('main.responsive-viewer-overlay'))
      await acceptsFocus(viewer)
    }))
  }

  for (const width of [700, 900]) {
    it(`shared detail pane at ${width}px: open, closed, reopened and full width`, ui.run({ width, beforeMount: contactFake }, async (page) => {
      // Exercise the real kit DetailPane through Contacts.
      await page.evaluate(async () => {
        const state = await import('/src/lib/stores/uiState.svelte.ts')
        state.setActiveExtension('contacts')
      })
      const viewer = page.locator('section.pane-detail.responsive-viewer-overlay')
      const row = page.getByText('Ada Lovelace', { exact: true }).first()
      await viewer.waitFor({ state: 'attached' })
      await rejectsFocus(viewer)
      await row.click()
      await page.waitForSelector('section.responsive-viewer-visible')
      await acceptsFocus(viewer)
      await back(viewer).click()
      await page.waitForSelector('section.responsive-viewer-visible', { state: 'detached' })
      await tabLeaves(page, viewer)
      await rejectsFocus(viewer)
      await row.click()
      await page.waitForSelector('section.responsive-viewer-visible')
      await acceptsFocus(viewer)
      await page.setViewportSize({ width: 1400, height: 800 })
      await page.waitForFunction(() => !document.querySelector('section.responsive-viewer-overlay'))
      await acceptsFocus(page.locator('section.pane-detail'))
    }))
  }
})
