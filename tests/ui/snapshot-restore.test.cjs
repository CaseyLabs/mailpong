const test = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const fakes = `
{
  const A = window.go.app.App
  const getAccounts = A.GetAccounts
  A.GetAccounts = async () => (await getAccounts()).map((a, i) => i === 0 ? { ...a, authType: 'oauth2' } : a)
  const route = { route: 'gmail-api', state: 'active', shared: false, authorized: true, failure: '' }
  A.GetMailAPIRouteStatus = async () => ({ ...route })
  A.PlanMailAPIMigration = async () => ({ accountId: 'acc1', route: route.route, routeState: route.state, counts: { folders: 1, imapMessages: 2, apiMessages: 2, drafts: 0, queuedSubmissions: 0 }, pendingActions: 0, blockers: [], migration: { state: 'completed', snapshotPath: '/tmp/snapshot.db' } })
  A.PreviewMailAPISnapshotRestore = async () => ({ snapshotPath: '/tmp/snapshot.db', sha256: 'x', createdAt: Date.UTC(2026, 9, 12, 12) / 1000, accountId: 'acc1', accounts: [{ id: 'acc1', email: 'user@example.test', messages: 12, drafts: 1 }], addedSince: ['new@example.test'], removedSince: [], backupDir: '/data/restore-backups', undo: false })
  window.__restoreCalls = []
  A.RestoreMailAPISnapshot = async (id, ack) => { window.__restoreCalls.push([id, ack]); return {} }
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`snapshot restore states loss semantics and needs acknowledgement (Accounts ${accounts})`, ui.run({ accounts, beforeMount: fakes }, async page => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await page.getByRole('button', { name: 'Edit account', exact: true }).first().click()
    await page.getByRole('tab', { name: 'Server', exact: true }).click()
    const section = page.getByRole('region', { name: 'Mail API route', exact: true })
    await section.getByRole('button', { name: 'Restore database to snapshot…', exact: true }).click()
    const dialog = section.getByRole('group', { name: 'Restore database to snapshot', exact: true })
    await dialog.getByText(/every account/).waitFor()
    await dialog.getByText(/user@example\.test: 12 messages, 1 drafts/).waitFor()
    await dialog.getByText(/new@example\.test/).waitFor()
    await dialog.getByText(/Changes already made at your mail provider are not undone/).waitFor()
    await dialog.getByText(/must be reopened by you/).waitFor()
    await dialog.getByText(/\/data\/restore-backups/).waitFor()
    const action = dialog.getByRole('button', { name: /^Restore database to .*2026$/ })
    assert.equal(await action.isDisabled(), true, 'restore stays disabled until acknowledged')
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
    assert.equal((await page.evaluate(() => window.__restoreCalls)).length, 0)

    await section.getByRole('button', { name: 'Restore database to snapshot…', exact: true }).click()
    await dialog.getByRole('checkbox').check()
    await action.click()
    await dialog.getByText(/Mailven is closing/).waitFor()
    assert.deepEqual(await page.evaluate(() => window.__restoreCalls), [['acc1', true]])
  }))
}

const bannerFakes = `
{
  const A = window.go.app.App
  window.__bannerCalls = []
  A.GetSnapshotRestoreResult = async () => ({ id: 'r1', status: 'restored', phase: 'post', error: '', undo: false, backupPath: '/data/restore-backups/pre-restore-r1.db', snapshotCreatedAt: Date.UTC(2026, 9, 12, 12) / 1000, finishedAt: 0, heldActions: 3, queuedSubmissions: 1, needsReauth: ['user@example.test'], orphanedAccounts: [{ id: 'gone', email: 'old@example.test' }], notice: '' })
  A.DismissSnapshotRestoreResult = async () => { window.__bannerCalls.push('dismiss') }
  A.CleanRestoreOrphanCredentials = async () => { window.__bannerCalls.push('clean') }
  A.PreviewUndoSnapshotRestore = async () => ({})
  A.UndoSnapshotRestore = async ack => { window.__bannerCalls.push('undo:' + ack); return {} }
}
`

for (const accounts of ['expanded', 'collapsed']) {
  test(`restore result banner reports held work and offers undo (Accounts ${accounts})`, ui.run({ accounts, beforeMount: bannerFakes }, async page => {
    const banner = page.getByRole('region', { name: 'Restore database to snapshot', exact: true })
    await banner.getByText(/Database restored to the snapshot from/).waitFor()
    await banner.getByText('3 queued API actions are held, not sent.').waitFor()
    await banner.getByText(/1 queued outgoing messages were not sent automatically/).waitFor()
    await banner.getByText('Sign in again for: user@example.test.').waitFor()
    await banner.getByText(/old@example\.test/).waitFor()
    await banner.getByRole('button', { name: 'Delete those credentials', exact: true }).click()
    await banner.getByRole('button', { name: 'Undo restore', exact: true }).click()
    const undo = banner.getByRole('button', { name: 'Undo restore and close Mailven', exact: true })
    assert.equal(await undo.isDisabled(), true)
    await banner.getByRole('checkbox').check()
    await undo.click()
    await banner.getByText(/Mailven is closing/).waitFor()
    assert.deepEqual(await page.evaluate(() => window.__bannerCalls), ['clean', 'undo:true'])
  }))
}
