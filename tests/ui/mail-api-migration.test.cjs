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

const sharedFakes = `
{
  const A = window.go.app.App
  const getAccounts = A.GetAccounts
  A.GetAccounts = async () => (await getAccounts()).map((a, i) => i === 0 ? { ...a, authType: 'oauth2', imapHost: 'outlook.office365.com' } : a)
  A.GetMicrosoftSharedMailboxes = async () => (await getAccounts()).slice(1, 2).map(a => ({ ...a, name: 'Support', email: 'support@example.com', authType: 'oauth2', sharedMailboxParentId: 'acc1' }))
  window.__consentArgs = null
  A.GetMailAPIRouteStatus = async () => ({ route: 'protocol', state: '', shared: false, authorized: false, failure: '' })
  A.StartMailAPIConsent = async (id, shared) => { window.__consentArgs = [id, shared] }
  A.CancelMailAPIConsent = async () => undefined
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`linked shared mailbox offers shared API consent (Accounts ${accounts})`, ui.run({ accounts, beforeMount: sharedFakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: 'Edit account', exact: true }).first().click()
    await page.getByRole('tab', { name: 'Identity', exact: true }).click()
    await page.getByRole('button', { name: 'Edit', exact: true }).last().click()
    const editor = page.getByRole('dialog', { name: 'Edit Shared Mailbox' })
    await editor.getByRole('tab', { name: 'Server', exact: true }).click()
    const section = editor.getByRole('region', { name: 'Mail API route', exact: true })
    await section.getByText(/shared mailbox uses your parent account/).waitFor()
    assert.equal(await section.getByRole('button', { name: /Review migration|Migrate/ }).count(), 0, 'no migration is offered before consent')
    await section.getByRole('button', { name: 'Authorize API access', exact: true }).click()
    const args = await page.evaluate(() => window.__consentArgs)
    assert.equal(args[1], true, 'consent is started in shared mode')
  }))
}
