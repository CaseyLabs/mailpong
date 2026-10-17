// UI quality audit over the layout matrix: theme × width × Accounts column × previews.
// 1400px is the full layout and 900px the medium one; both keep the Accounts column toggle.
// 700px is narrow: the sidebar becomes an on-demand overlay and the persisted collapse flag is ignored.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')
const audit = require('./ui-audit.cjs')
const allow = require('./ui-audit-allow.cjs')
const { readerFake } = require('./reader-fake.cjs')

const matrix = []
for (const theme of ['light', 'dark']) {
  for (const width of [1400, 900, 700]) {
    for (const accounts of width < 768 ? ['overlay'] : ['expanded', 'collapsed']) {
      for (const previews of [false, true]) matrix.push({ theme, width, accounts, previews })
    }
  }
}

// Flat transcript mode (chat_display_style) is audited in both themes at the full and medium widths,
// with the Accounts column expanded and collapsed.
const flatStyle = `{ window.go.app.App.GetChatDisplayStyle = async () => 'flat' }`
for (const theme of ['light', 'dark']) {
  for (const width of [1400, 900]) {
    for (const accounts of ['expanded', 'collapsed']) matrix.push({ theme, width, accounts, previews: false, style: 'flat', beforeMount: flatStyle })
  }
}

// The open chat ends with a newsletter in the reader view (chat_rich_style 'reader', the default).
for (const theme of ['light', 'dark']) {
  for (const accounts of ['expanded', 'collapsed']) matrix.push({ theme, width: 1400, accounts, previews: false, style: 'reader', beforeMount: readerFake })
}

describe('UI quality audit', () => {
  const used = new Set()
  let completed = 0
  for (const config of matrix) {
    it(ui.describeConfig(config), ui.run(config, async (page, c) => {
      const check = async (view) => {
        const { findings, reached, focusable } = await audit.audit(page)
        const filtered = audit.filter(findings, allow, c)
        const { unexpected } = filtered
        for (const entry of filtered.used) used.add(entry)
        const label = `${c.label}; ${view}`
        assert.ok(reached > 5, `[${label}] keyboard Tab reached only ${reached} of ${focusable} focusable controls`)
        assert.equal(unexpected.length, 0, audit.format(unexpected, label))
      }
      const toggle = (name) => page.locator(`[title="${name}"],[aria-label="${name}"]`).first()
      if (c.accounts !== 'overlay') {
        // The Accounts column toggle reflects the persisted collapse flag.
        const name = c.accounts === 'expanded' ? 'Hide sidebar' : 'Show sidebar'
        assert.ok(await toggle(name).isVisible(), `[${c.label}] expected a visible "${name}" toggle for the Accounts column`)
        await check('list and viewer')
        completed++
        return
      }
      // Narrow: the selected chat fills the window; Back returns to the list, whose toggle slides the sidebar in.
      const back = page.getByRole('button', { name: 'Back' })
      assert.ok(await back.isVisible(), `[${c.label}] expected the viewer to fill the window with a Back button`)
      await check('viewer')
      await back.click()
      assert.ok(await toggle('Toggle sidebar').isVisible(), `[${c.label}] expected a "Toggle sidebar" button in the list after Back`)
      await check('list')
      await toggle('Toggle sidebar').click()
      await page.waitForSelector('aside.responsive-sidebar-visible', { timeout: 5000 })
      await check('sidebar overlay open')
      completed++
    }))
  }

  // An entry that matches nothing across the matrix is stale (its issue is fixed or the markup changed); deleting
  // it lets the audit guard the fix. Only meaningful when every configuration ran to the end.
  it('every allowlist entry still matches a finding', (t) => {
    if (completed < matrix.length) return t.skip(`${matrix.length - completed} configuration(s) did not complete`)
    const stale = allow.filter((a) => !used.has(a))
    assert.equal(stale.length, 0, `stale ui-audit-allow.cjs entries:\n${stale.map((a) => `  ${a.rule} ${a.element} (${a.reason})`).join('\n')}`)
  })
})
