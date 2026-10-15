// Closed narrow sidebars must reject focus while retaining their mounted state.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const rejectsFocus = async (sidebar) => {
  const result = await sidebar.evaluate((el) => {
    // Clear focus left on the close button before testing fresh focus attempts.
    document.activeElement?.blur()
    const controls = [...el.querySelectorAll('button, a[href], input, [tabindex]')]
    for (const control of controls) {
      control.focus()
      if (el.contains(document.activeElement)) return { rejected: false, count: controls.length }
    }
    return { rejected: true, count: controls.length }
  })
  assert.ok(result.count > 0, 'exercise mounted sidebar controls')
  assert.ok(result.rejected, 'closed sidebar must reject focus on every control')
}

const acceptsFocus = async (sidebar) => {
  const button = sidebar.getByRole('button').first()
  await button.focus()
  assert.ok(await button.evaluate((el) => document.activeElement === el), 'visible sidebar remains focusable')
}

describe('responsive sidebar focus', () => {
  for (const view of ['mail', 'contacts']) {
    it(`${view}: closed, open, closed again and resized`, ui.run({ width: 700 }, async (page) => {
      if (view === 'mail') {
        await page.getByRole('button', { name: 'Back', exact: true }).click()
      } else {
        // Exercise the real shared SidebarFrame through Contacts with the guarded backend.
        await page.evaluate(async () => {
          const state = await import('/src/lib/stores/uiState.svelte.ts')
          state.setActiveExtension('contacts')
        })
      }
      const sidebar = page.locator(view === 'mail'
        ? 'aside.responsive-sidebar-overlay[role="presentation"]'
        : 'aside.responsive-sidebar-overlay[role="navigation"]')
      await sidebar.waitFor({ state: 'attached' })
      await rejectsFocus(sidebar)
      await page.getByRole('button', { name: 'Toggle sidebar', exact: true }).click()
      await page.waitForSelector('aside.responsive-sidebar-visible')
      await acceptsFocus(sidebar)
      await sidebar.getByRole('button', { name: view === 'mail' ? 'Close sidebar' : 'Back', exact: true }).click()
      await page.waitForSelector('aside.responsive-sidebar-visible', { state: 'detached' })
      await rejectsFocus(sidebar)
      for (const width of [900, 1400]) {
        await page.setViewportSize({ width, height: 800 })
        const expanded = page.locator(view === 'mail' ? 'aside[role="presentation"]' : 'aside[role="navigation"]')
        await page.waitForFunction(() => !document.querySelector('aside.responsive-sidebar-overlay'))
        await acceptsFocus(expanded)
      }
    }))
  }
})
