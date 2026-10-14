// Guarded Sent policy checks; requires run-mailven's disposable dev session.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { withFakePage } = require('../.agents/skills/run-mailven/fake-page.js')
const out = process.argv[2]
if (!out) throw Error('Usage: node tools/sent-policy-check.cjs <output-directory>')
fs.mkdirSync(out, { recursive: true })
const beforeMount = `
const A = window.go.app.App;
const route = JSON.stringify(['smtp','smtp.custom.example',587,'starttls','plain','password','acc1','alex@example.com','acc1','imap.example.com',993,'tls','alex@example.com','']);
window.__savedPolicy = { route, destinations: [] };
window.__policyWrites = [];
window.__copies = [
 {attemptId:'accepted',messageId:'<stable@example.com>',accountId:'acc1',folderPath:'Sent',mode:'append',state:'unknown',evidence:'APPEND reply lost'},
 {attemptId:'accepted',messageId:'<stable@example.com>',accountId:'acc2',folderPath:'Sent',mode:'provider',state:'pending',evidence:'Awaiting sync'}
];
A.GetSentPolicy = async () => structuredClone(window.__savedPolicy);
A.GetSubmissionCopies = async () => structuredClone(window.__copies);
A.SetSentPolicy = async (id,p) => { if(window.__changedRoute) throw Error('account route changed; reload and review Sent policy'); window.__policyWrites.push(JSON.parse(JSON.stringify(p))); window.__savedPolicy = JSON.parse(JSON.stringify(p)); };
A.GetSendAsStatus = async () => ({supported:false});
A.GetSenderEvidence = async () => null;
`
async function main() {
 await withFakePage({ width: 1400, height: 1100 }, async p => {
  await p.getByRole('button', { name: 'Show sidebar', exact: true }).click()
  await p.waitForTimeout(350)
  await p.screenshot({ path: path.join(out, 'mail-expanded.png') })
  await p.getByRole('button', { name: 'Hide sidebar', exact: true }).click()
  await p.waitForTimeout(350)
  await p.screenshot({ path: path.join(out, 'mail-collapsed.png') })
  console.log('PASS Mail Accounts expanded/collapsed')
  await p.getByRole('button', { name: 'Settings', exact: true }).click()
  await p.getByRole('tab', { name: 'Accounts', exact: true }).click()
  await p.getByRole('button', { name: 'Edit account', exact: true }).first().click()
  await p.getByRole('tab', { name: 'Identity', exact: true }).click()
  const s = p.getByRole('region', { name: 'Sent copy policy', exact: true })
  await s.getByText(/Policy unresolved/).waitFor()
  await s.getByText(/APPEND unconfirmed; review before any manual copy/).waitFor()
  await s.scrollIntoViewIfNeeded()
  await p.screenshot({ path: path.join(out, 'sent-unresolved.png') })
  await s.getByLabel('Destination folder', { exact:true }).selectOption('Sent')
  await s.getByLabel('Copy policy', { exact:true }).selectOption('provider')
  await s.getByRole('button', { name:'Add destination',exact:true }).click()
  await s.getByLabel('Destination mailbox', { exact:true }).selectOption('acc2')
  await s.getByLabel('Destination folder', { exact:true }).selectOption('Sent')
  await s.getByLabel('Copy policy', { exact:true }).selectOption('append')
  await s.getByRole('button', { name:'Add destination',exact:true }).click()
  await s.getByRole('button', { name:'Save reviewed Sent policy',exact:true }).focus()
  await p.keyboard.press('Enter')
  await p.waitForFunction(() => window.__policyWrites.length === 1)
  const saved = await p.evaluate(() => window.__savedPolicy)
  assert.deepEqual(saved.destinations.map(d => [d.accountId,d.folderPath,d.mode]), [['acc1','Sent','provider'],['acc2','Sent','append']])
  await p.screenshot({ path: path.join(out, 'sent-reviewed.png') })
  console.log('PASS custom route; explicit provider/APPEND destinations; keyboard policy save')
  await p.evaluate(() => { window.__copies = window.__copies.map(c => ({...c,state:'observed'})) })
  await s.getByRole('button', { name:'Refresh copy evidence',exact:true }).click()
  await p.waitForFunction(() => [...document.querySelectorAll('[aria-label="Sent copy policy"] p')].filter(x => x.textContent.includes('Copy observed')).length === 2)
  console.log('PASS delayed exact-destination evidence display without replay')
  await p.evaluate(() => { window.__changedRoute = true })
  await s.getByRole('button', { name:'Save reviewed Sent policy',exact:true }).click()
  await s.getByRole('alert').getByText(/account route changed/).waitFor()
  assert.equal(await p.evaluate(() => window.__policyWrites.length),1)
  console.log('PASS stale route save rejection preserves reviewed policy')
 }, { beforeMount })
}
main().catch(error => { console.error(error); process.exitCode=1 })
