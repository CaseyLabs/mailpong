// Guarded sender evidence/review checks; run-mailven dev session required.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { withFakePage } = require('../.agents/skills/run-mailven/fake-page.js')
const { launch } = require('../.agents/skills/run-mailven/pw.js')
const out = process.argv[2]
if (!out) throw Error('Usage: node tools/sender-check.cjs <output-directory>')
fs.mkdirSync(out, { recursive: true })
const composerFakes = fs.readFileSync(path.join(__dirname, '../.agents/skills/run-mailven/composer-fakes.js'), 'utf8')
const localEvents = `const handlers = {};
window.runtime.EventsOnMultiple = (name, cb) => { (handlers[name] ??= []).push(cb); return () => { handlers[name] = handlers[name].filter(x => x !== cb) } };
window.runtime.EventsEmit = (name, ...data) => (handlers[name] ?? []).forEach(cb => cb(...data));`
const beforeMount = `${localEvents}\n${composerFakes}\nwindow.__installComposerFakes(window);`
const evidence = (state, action) => ({ email: 'alex@example.com', state, observedState: state === 'stale' ? 'verified' : state, action, source: 'Gmail send-as settings', route: 'smtp://smtp.gmail.com:587 (starttls)', observedAt: '2026-10-12T10:00:00Z', guidance: action === 'block' ? 'Verify or add this sender in Gmail; selected From preserved.' : 'Authorization needs review; provider enforces permission.', reviewToken: 'route-from-review', sendAs: 'unknown', sendOnBehalf: 'unknown', sharedRead: 'unknown' })
const review = p => p.getByRole('alertdialog').filter({ hasText: 'Review sender authorization' })
const logs = p => p.evaluate(() => window.__log.join(' | '))
const populate = async p => {
  // Composer finishes initialization with a delayed To-field autofocus.
  await p.getByRole('region', { name: 'Email composer' }).waitFor()
  await p.waitForTimeout(300)
  await p.getByPlaceholder('Subject').fill('Sender evidence draft')
  const composer = p.getByRole('region', { name: 'Email composer' })
  const to = p.getByPlaceholder(/recipients/i).first()
  await to.fill('sam@example.com'); await to.press('Enter')
  await composer.getByRole('button', { name: 'Bcc', exact: true }).click()
  const bcc = p.getByPlaceholder(/recipients/i).last()
  await bcc.fill('hidden@example.com'); await bcc.press('Enter')
  await composer.locator('.ProseMirror[contenteditable=true]:visible').fill('Preserved body')
  return composer
}
const preserved = async (p, composer) => {
  assert.equal(await p.getByPlaceholder('Subject').inputValue(), 'Sender evidence draft')
  assert.equal(await composer.locator('.ProseMirror[contenteditable=true]:visible').innerText(), 'Preserved body')
  await composer.getByText('hidden@example.com', { exact: true }).waitFor()
  await composer.getByText(/Alex Morgan/).first().waitFor()
}
async function main() {
  await withFakePage({ width: 1400, height: 1000 }, async p => {
    await p.getByRole('button', { name: 'Show sidebar', exact: true }).click()
    await p.waitForTimeout(350)
    await p.screenshot({ path: path.join(out, 'mail-expanded.png') })
    await p.getByRole('button', { name: 'Hide sidebar', exact: true }).click()
    await p.waitForTimeout(350)
    await p.screenshot({ path: path.join(out, 'mail-collapsed.png') })
    console.log('PASS Mail Accounts expanded/collapsed')
    await p.evaluate(e => { window.__senderEvidence = e; window.__sendDelay = 60000 }, evidence('unknown', 'review'))
    await p.getByRole('button', { name: /Compose/ }).first().click()
    const composer = await populate(p)
    await composer.getByText(/Source: Gmail send-as settings/).waitFor()
    await p.evaluate(() => { window.__log = [] })
    await composer.locator('.ProseMirror[contenteditable=true]:visible').fill('Preserved body')
    await composer.getByRole('button', { name: 'Send', exact: true }).click()
    await review(p).waitFor()
    assert(!/queue |SEND /.test(await logs(p)))
    await review(p).getByRole('button', { name: 'Cancel', exact: true }).focus()
    await p.keyboard.press('Enter')
    await review(p).waitFor({ state: 'hidden' })
    await preserved(p, composer)
    await p.waitForFunction(() => window.__log.includes('saveDraft'))
    assert(!/queue |SEND /.test(await logs(p)))
    await composer.getByRole('button', { name: 'Send', exact: true }).click()
    await review(p).getByRole('button', { name: 'Use this From and send', exact: true }).focus()
    await p.keyboard.press('Enter')
    await p.waitForFunction(() => window.__log.some(x => x.startsWith('queue ')))
    const msg = await p.evaluate(() => window.__lastSentMessage)
    assert.equal(msg.from.address, 'alex@example.com')
    assert.equal(msg.bcc[0].address, 'hidden@example.com')
    assert.equal(msg.sender_review_token, 'route-from-review')
    console.log('PASS main unknown review keyboard cancel/confirm; From/Bcc and queue acknowledgement preserved')
  }, { beforeMount })
  await withFakePage({ width: 1400, height: 1000 }, async p => {
    await p.evaluate(e => { window.__senderEvidence = { ...e, source: 'Microsoft delegation lookup unavailable; local configuration', sharedRead: 'configured' } }, evidence('unknown', 'review'))
    await p.getByRole('button', { name: 'Settings', exact: true }).click()
    await p.getByRole('tab', { name: 'Accounts', exact: true }).click()
    await p.getByRole('button', { name: 'Edit account', exact: true }).first().click()
    await p.getByRole('tab', { name: 'Identity', exact: true }).click()
    await p.getByText(/Send As: unknown. Send on Behalf: unknown. Shared read: locally configured./).waitFor()
    await p.getByText(/minutes ago/).waitFor()
    await p.screenshot({ path: path.join(out, 'account-evidence.png') })
    console.log('PASS account identity evidence source/route/age and separate delegation states')
  }, { beforeMount })
  const browser = await launch()
  try {
    for (const [state, action] of [['pending', 'block'], ['not-configured', 'block'], ['stale', 'review'], ['verified', 'allow'], ['unknown', 'review'], ['lookup-error', 'error']]) {
      const p = await browser.newPage({ viewport: { width: 1100, height: 950 } })
      await p.addInitScript({ path: path.join(__dirname, '../.agents/skills/run-mailven/composer-fakes.js') })
      await p.addInitScript(e => { window.__senderEvidence = e; if (e.action === 'error') window.__senderEvidenceError = 'Sender evidence lookup unavailable' }, evidence(state === 'lookup-error' ? 'unknown' : state, action))
      await p.goto('http://localhost:34115/composer.html')
      const composer = await populate(p)
      if (action === 'error') await p.getByText(/Sender evidence could not be read/).waitFor()
      await composer.getByRole('button', { name: 'Send', exact: true }).click()
      await p.getByRole('button', { name: /Send now/ }).click()
      if (action === 'block' || action === 'error') {
        await p.getByText(action === 'block' ? 'Error: Verify or add this sender in Gmail; selected From preserved.' : 'Error: Sender evidence lookup unavailable', { exact: true }).last().waitFor()
        await preserved(p, composer)
        assert(!/SEND /.test(await logs(p)))
      } else if (action === 'review') {
        await review(p).waitFor()
        await p.screenshot({ path: path.join(out, `detached-${state}.png`) })
        await review(p).getByRole('button', { name: 'Cancel', exact: true }).focus()
        await p.keyboard.press('Enter')
        await preserved(p, composer)
        assert(!/SEND /.test(await logs(p)))
      } else {
        await p.waitForFunction(() => window.__log.some(x => x.startsWith('SEND ')))
        const msg = await p.evaluate(() => window.__lastSentMessage)
        assert.equal(msg.from.address, 'alex@example.com')
        assert.equal(msg.bcc[0].address, 'hidden@example.com')
        assert.equal(msg.sender_review_token, undefined)
      }
      if (action === 'block' || action === 'error') await p.screenshot({ path: path.join(out, `detached-${state}.png`) })
      console.log(`PASS detached ${state}: ${action}; From/Bcc/body/draft preservation`)
      await p.close()
    }
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
