// Guarded UI checks: fake bindings only; no consent, migration or account write reaches a backend.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { withFakePage } = require('../.agents/skills/run-mailven/fake-page.js')
const out = process.argv[2]
if (!out) throw Error('Usage: node tools/mail-api-migration-check.cjs <output-directory>')
fs.mkdirSync(out, { recursive: true })
const beforeMount = `
const A = window.go.app.App;
const eventHandlers = new Map();
window.runtime.EventsOnMultiple = (name, callback) => { const handlers = eventHandlers.get(name) || []; handlers.push(callback); eventHandlers.set(name, handlers); return () => { const current = eventHandlers.get(name) || []; eventHandlers.set(name, current.filter(fn => fn !== callback)); }; };
window.runtime.EventsOff = (...names) => names.forEach(name => eventHandlers.delete(name));
window.runtime.EventsOffAll = () => eventHandlers.clear();
window.runtime.EventsEmit = (name, ...args) => { for (const callback of eventHandlers.get(name) || []) callback(...args); };
const base = A.GetAccounts;
A.GetAccounts = async () => (await base()).map((a, i) => i === 0 ? { ...a, authType: 'oauth2' } : a);
A.GetSendAsStatus = async () => ({supported:false}); A.GetSenderEvidence = async () => null;
A.GetSentPolicy = async () => ({route:'[]',destinations:[]}); A.GetSubmissionCopies = async () => [];
window.__calls = [];
let route = { route: 'protocol', state: '', shared: false, authorized: false, failure: '' };
let record = null;
const counts = { folders: 4, imapMessages: 120, apiMessages: 118, drafts: 2, queuedSubmissions: 0 };
window.__blockers = [];
A.GetMailAPIRouteStatus = async () => JSON.parse(JSON.stringify(route));
A.PlanMailAPIMigration = async () => ({ accountId: 'x', route: route.route, routeState: route.state, counts, pendingActions: 1, blockers: window.__blockers, migration: record });
A.StartMailAPIConsent = async (id, shared) => { window.__accountId = id; window.__calls.push(['consent', id, shared]); route = { ...route, route: 'gmail-api', state: route.state === 'active' ? 'active' : 'staged', shared: false, authorized: true, failure: '' }; setTimeout(() => window.runtime.EventsEmit && 0, 0); window.__consentDone = () => window.__emit('mail-api:success', { accountId: id }) };
A.CancelMailAPIConsent = async () => { window.__calls.push(['cancel-consent']); window.__emit('mail-api:cancelled', null) };
A.MigrateMailAPIAccount = async id => { window.__calls.push(['migrate', id]); route = { ...route, state: 'active' }; record = { accountId: id, state: 'completed', route: 'gmail-api', snapshotPath: '/home/u/snapshots/pre.db', correlated: 110, uncorrelated: 8, error: '' }; return record };
A.RollbackMailAPIMigration = async id => { window.__calls.push(['rollback', id]); route = { ...route, state: 'staged' }; record = { ...record, state: 'rolled-back' }; return { accountId: id, snapshotPath: record.snapshotPath, appliedSince: 3, syncRequested: true, unfinished: [{ op: 'move', status: 'pending', error: '' }] } };
A.ClearMailAPIRoute = async id => { window.__calls.push(['clear', id]); route = { route: 'protocol', state: '', shared: false, authorized: false, failure: '' } };
`
async function main() {
 await withFakePage({width:1400,height:1100},async p => {
  // Expose the runtime event bus to the check.
  await p.evaluate(() => { window.__emit = (n, d) => window.runtime.EventsEmit(n, d) })
  await p.getByRole('button',{name:'Show sidebar',exact:true}).click()
  await p.getByRole('button',{name:'Hide sidebar',exact:true}).waitFor()
  await p.screenshot({path:path.join(out,'mail-expanded.png')})
  await p.getByRole('button',{name:'Hide sidebar',exact:true}).click()
  await p.getByRole('button',{name:'Show sidebar',exact:true}).waitFor()
  await p.screenshot({path:path.join(out,'mail-collapsed.png')})
  console.log('PASS Mail Accounts expanded/collapsed')
  await p.getByRole('button',{name:'Settings',exact:true}).click()
  await p.getByRole('tab',{name:'Accounts',exact:true}).click()
  await p.getByRole('button',{name:'Edit account',exact:true}).first().click()
  await p.getByRole('tab',{name:'Server',exact:true}).click()
  const s = p.getByRole('region',{name:'Mail API route',exact:true})
  await s.scrollIntoViewIfNeeded()
  await s.getByText('IMAP / SMTP',{exact:true}).waitFor()
  assert.equal(await p.evaluate(()=>window.__calls.length),0)
  await s.getByRole('button',{name:'Authorize API access',exact:true}).focus()
  await p.keyboard.press('Enter')
  await p.waitForFunction(()=>window.__calls.length===1)
  await p.evaluate(()=>window.__consentDone())
  await s.getByRole('button',{name:'Review migration',exact:true}).waitFor()
  await s.getByText(/120 IMAP messages/).waitFor()
  // Blockers disable review.
  await p.evaluate(()=>{window.__blockers=['An IMAP sync is in progress.']})
  await p.evaluate(()=>window.__emit('mail-api:success',{accountId:'x'}))
  await s.getByRole('button',{name:'Authorize API access',exact:true}).click()
  await p.evaluate(()=>window.__consentDone())
  await s.getByText('An IMAP sync is in progress.').waitFor()
  assert.equal(await s.getByRole('button',{name:'Review migration',exact:true}).isDisabled(),true)
  await p.screenshot({path:path.join(out,'blocked.png')})
  await p.evaluate(()=>{window.__blockers=[]})
  await s.getByRole('button',{name:'Authorize API access',exact:true}).click()
  await p.evaluate(()=>window.__consentDone())
  await s.getByRole('button',{name:'Review migration',exact:true}).click()
  const migrate = s.getByRole('button',{name:'Migrate this account',exact:true})
  assert.equal(await migrate.isDisabled(),true)
  await s.getByRole('button',{name:'Cancel',exact:true}).click()
  await s.getByRole('button',{name:'Review migration',exact:true}).click()
  await s.getByRole('checkbox').check()
  await p.screenshot({path:path.join(out,'confirm.png')})
  await migrate.focus(); await p.keyboard.press('Enter')
  await s.getByText(/110 messages matched, 8 added once/).waitFor()
  assert.equal(await p.evaluate(()=>window.__calls.filter(c=>c[0]==='migrate').length),1)
  await s.getByText('This account uses its mail API.').waitFor()
  await s.getByText(/Pre-migration snapshot: \/home\/u\/snapshots\/pre.db/).waitFor()
  await p.screenshot({path:path.join(out,'migrated.png')})
  await s.getByRole('button',{name:'Return to IMAP / SMTP',exact:true}).click()
  await s.getByRole('button',{name:'Cancel',exact:true}).click()
  assert.equal(await p.evaluate(()=>window.__calls.filter(c=>c[0]==='rollback').length),0)
  await s.getByRole('button',{name:'Return to IMAP / SMTP',exact:true}).click()
  await s.getByRole('button',{name:'Return to IMAP / SMTP',exact:true}).last().click()
  await s.getByText(/3 API actions were applied/).waitFor()
  await s.getByText(/move: pending/).waitFor()
  await p.screenshot({path:path.join(out,'rolled-back.png')})
  console.log('PASS authorize, blockers, consent gate, migrate, rollback confirm')

  // Active API grants can be renewed in place after a provider rejection.
  await p.evaluate(()=>window.__emit('mail-api:success',{accountId:'x'}))
  await s.getByRole('button',{name:'Authorize API access',exact:true}).click()
  await p.evaluate(()=>window.__consentDone())
  await s.getByRole('button',{name:'Review migration',exact:true}).click()
  await s.getByRole('checkbox').check()
  await s.getByRole('button',{name:'Migrate this account',exact:true}).click()
  await s.getByText('This account uses its mail API.').waitFor()
  await p.evaluate(()=>window.__emit('mail-api:reauth-required',{accountId:window.__accountId,failure:'reauthorize',error:'grant rejected'}))
  await s.getByText('The API authorization needs to be renewed. Authorize again to continue.').waitFor()
  const reauthorize = s.getByRole('button',{name:'Reauthorize API access',exact:true})
  await reauthorize.click()
  await s.getByRole('button',{name:'Cancel',exact:true}).last().click()
  assert.equal(await p.evaluate(()=>window.__calls.filter(c=>c[0]==='cancel-consent').length),1)
  await s.getByText('This account uses its mail API.').waitFor()
  await reauthorize.click()
  await p.evaluate(()=>window.__consentDone())
  await s.getByText('This account uses its mail API.').waitFor()
  assert.equal(await s.getByRole('button',{name:'Return to IMAP / SMTP',exact:true}).count(),1)
  console.log('PASS active-route reauthorization, cancellation, and in-place recovery')
 },{beforeMount})
}
main().catch(e=>{console.error(e);process.exitCode=1})
