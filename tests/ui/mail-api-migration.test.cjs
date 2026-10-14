const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = `
{
  const A = window.go.app.App
  const getAccounts = A.GetAccounts
  A.GetAccounts = async () => (await getAccounts()).map((a, i) => i === 0 ? { ...a, authType: 'oauth2' } : a)
  let route = { route: 'gmail-api', state: 'active', shared: false, authorized: true, failure: '' }
  let resolveRollback
  window.__rollbackCalls = 0
  A.GetMailAPIRouteStatus = async () => ({ ...route })
  A.PlanMailAPIMigration = async () => ({ accountId: 'acc1', route: route.route, routeState: route.state, counts: { folders: 1, imapMessages: 2, apiMessages: 2, drafts: 0, queuedSubmissions: 0 }, pendingActions: 0, blockers: [], migration: { state: 'completed', snapshotPath: '/tmp/snapshot.db' } })
  A.RollbackMailAPIMigration = async () => {
    window.__rollbackCalls++
    return new Promise(resolve => { resolveRollback = report => { route = { ...route, state: 'staged' }; resolve(report) } })
  }
  A.CancelMailAPIConsent = async () => undefined
  window.__finishRollback = syncRequested => resolveRollback({ accountId: 'acc1', snapshotPath: '/tmp/snapshot.db', unfinished: [], appliedSince: 2, syncRequested })
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`rollback reports sync request state (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: 'Edit account', exact: true }).first().click()
    await page.getByRole('tab', { name: 'Server', exact: true }).click()
    const section = page.getByRole('region', { name: 'Mail API route', exact: true })
    await section.getByRole('button', { name: 'Return to IMAP / SMTP', exact: true }).click()
    await section.getByRole('button', { name: 'Cancel', exact: true }).click()
    assert.equal(await page.evaluate(() => window.__rollbackCalls), 0, 'cancelling confirmation must not call rollback')

    await section.getByRole('button', { name: 'Return to IMAP / SMTP', exact: true }).click()
    const confirm = section.getByRole('button', { name: 'Return to IMAP / SMTP', exact: true }).last()
    await confirm.click()
    await section.locator('[aria-live="polite"]').getByText('Returning to IMAP / SMTP…').waitFor()
    assert.equal(await section.getByRole('button', { name: 'Returning to IMAP / SMTP…', exact: true }).isDisabled(), true, 'confirmation stays disabled while rollback is pending')
    await page.evaluate(() => window.__finishRollback(true))
    await section.getByText(/Returned to IMAP \/ SMTP\. 2 API actions were applied since migration\. A background sync was requested; its completion is not confirmed\./).waitFor()
    assert.equal(await section.getByText(/sync .*complete|sync .*finished/i).count(), 0, 'success copy must not claim sync completion')
    await section.getByText('IMAP / SMTP', { exact: true }).waitFor()
  }))
}

test('rollback explains when no sync was requested', ui.run({ beforeMount: fakes }, async page => {
  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
  await page.getByRole('button', { name: 'Edit account', exact: true }).first().click()
  await page.getByRole('tab', { name: 'Server', exact: true }).click()
  const section = page.getByRole('region', { name: 'Mail API route', exact: true })
  await section.getByRole('button', { name: 'Return to IMAP / SMTP', exact: true }).click()
  await section.getByRole('button', { name: 'Return to IMAP / SMTP', exact: true }).last().click()
  await section.locator('[aria-live="polite"]').getByText('Returning to IMAP / SMTP…').waitFor()
  await page.evaluate(() => window.__finishRollback(false))
  await section.getByText(/A sync was not requested; use Sync manually\./).waitFor()
}))
