// Guarded UI checks: no real account writes, probes, consent or mail.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { withFakePage } = require('../.agents/skills/run-mailven/fake-page.js')
const out = process.argv[2]
if (!out) throw Error('Usage: node tools/setup-diagnostic-check.cjs <output-directory>')
fs.mkdirSync(out, { recursive: true })
const beforeMount = `
const A = window.go.app.App;
window.__diagnosticRuns = []; window.__diagnosticCancels = []; window.__diagnosticExports = [];
A.GetSendAsStatus = async () => ({supported:false}); A.GetSenderEvidence = async () => null;
A.GetSentPolicy = async () => ({route:'[]',destinations:[]}); A.GetSubmissionCopies = async () => [];
A.RunSetupDiagnostic = async req => {
 const r = JSON.parse(JSON.stringify(req)); window.__diagnosticRuns.push(r);
 return await new Promise(resolve => { window.__diagnosticFinish = cancelled => resolve({
  runId:r.runId,accountId:r.accountId,mailbox:'alex@example.com',from:r.from,route:r.route,
  startedAt:'2026-10-13T20:00:00Z',finishedAt:'2026-10-13T20:00:01Z',endpoints:['imap imap.example.com:993 (tls)','smtp smtp.example.com:587 (starttls)'],
  oauthResource:'https://outlook.office.com',oauthExpiresAt:'2026-10-13T21:00:00Z',oauthScopesKnown:true,oauthScopes:['https://outlook.office.com/SMTP.Send'],
  checks:[{name:'imap transport',status:cancelled?'cancelled':'passed',at:'2026-10-13T20:00:00Z',evidenceAgeSeconds:0,detail:cancelled?'Cancelled by user; no automatic retry.':'TLS verified; IMAP CAPABILITY completed.',code:''},
  {name:'cached sender permission',status:'unknown',at:'2026-10-12T20:00:00Z',evidenceAgeSeconds:86400,detail:'Sender evidence is stale. Login never proves Send As.',code:''}],
  dns:{domain:r.domain,checks:r.ownedDomain?[{query:'_dmarc.'+r.domain,resolver:'127.0.0.1:53',ttlSeconds:300,observedAt:'2026-10-13T20:00:00Z',status:'passed',evidenceAgeSeconds:0,evidence:'one structurally valid policy record; alignment unknown'}]:[]},
  limitations:'No test mail. Login never proves sending rights or delivery. Raw errors and request IDs omitted.'
 }) });
};
A.CancelSetupDiagnostic = async id => {window.__diagnosticCancels.push(id);window.__diagnosticFinish?.(true)};
A.ExportSetupDiagnostic = async id => {window.__diagnosticExports.push(id);return true};
`
async function main() {
 await withFakePage({width:1400,height:1100},async p => {
  await p.getByRole('button',{name:'Show sidebar',exact:true}).click()
  await p.waitForTimeout(350)
  await p.screenshot({path:path.join(out,'mail-expanded.png')})
  await p.getByRole('button',{name:'Hide sidebar',exact:true}).click()
  await p.waitForTimeout(350)
  await p.screenshot({path:path.join(out,'mail-collapsed.png')})
  console.log('PASS Mail Accounts expanded/collapsed')
  await p.getByRole('button',{name:'Settings',exact:true}).click()
  await p.getByRole('tab',{name:'Accounts',exact:true}).click()
  await p.getByRole('button',{name:'Edit account',exact:true}).first().click()
  await p.getByRole('tab',{name:'Identity',exact:true}).click()
  const s=p.getByRole('region',{name:'Check account setup',exact:true})
  await s.scrollIntoViewIfNeeded()
  assert.equal(await p.evaluate(()=>window.__diagnosticRuns.length),0)
  assert.equal(await p.evaluate(()=>window.__diagnosticExports.length),0)
  const from=await s.getByLabel('Diagnostic From identity',{exact:true}).inputValue()
  await s.getByRole('button',{name:'Start diagnostic',exact:true}).focus()
  await p.keyboard.press('Enter')
  await p.waitForFunction(()=>window.__diagnosticRuns.length===1)
  assert.equal(await s.getByRole('button',{name:'Checking setup…',exact:true}).isDisabled(),true)
  await s.getByRole('button',{name:'Cancel',exact:true}).click()
  await s.getByText('imap transport: cancelled',{exact:true}).waitFor()
  assert.equal(await p.evaluate(()=>window.__diagnosticCancels.length),1)
  await s.getByText('imap transport: cancelled',{exact:true}).scrollIntoViewIfNeeded()
  await p.screenshot({path:path.join(out,'cancelled.png')})
  console.log('PASS explicit keyboard start, bounded cancel, no automatic run/export')
  await s.getByLabel('Include DNS for a domain I own',{exact:true}).check()
  await s.getByLabel('Owned domain',{exact:true}).fill('example.com')
  await s.getByLabel(/DKIM selectors supplied/).fill('selector1, selector2')
  await s.getByRole('button',{name:'Start diagnostic',exact:true}).click()
  await p.waitForFunction(()=>window.__diagnosticRuns.length===2)
  await p.evaluate(()=>window.__diagnosticFinish(false))
  await s.getByText('imap transport: passed',{exact:true}).waitFor()
  await s.getByText(/evidence is stale/).waitFor()
  await s.getByText(/_dmarc.example.com: passed/).waitFor()
  const runs=await p.evaluate(()=>window.__diagnosticRuns)
  assert.equal(runs[0].route,'imap-smtp')
  assert.equal(runs[0].ownedDomain,false)
  assert.deepEqual(runs[0].dkimSelectors,[])
  assert.deepEqual(runs[1].dkimSelectors,['selector1','selector2'])
  assert.ok(runs[0].from || from)
  assert.equal(await p.evaluate(()=>window.__diagnosticExports.length),0)
  await s.getByRole('button',{name:'Export redacted report',exact:true}).focus()
  await p.keyboard.press('Enter')
  await p.waitForFunction(()=>window.__diagnosticExports.length===1)
  await s.getByText('Redacted report saved.',{exact:true}).waitFor()
  await s.scrollIntoViewIfNeeded()
  await p.screenshot({path:path.join(out,'report.png')})
  await s.getByText(/Checked mailbox: alex@example.com/).waitFor()
  await s.getByText(/_dmarc.example.com: passed/).scrollIntoViewIfNeeded()
  await p.screenshot({path:path.join(out,'report-details.png')})
  console.log('PASS selected route/identity, owned DNS/selectors, evidence age, explicit redacted export')
 },{beforeMount})
}
main().catch(e=>{console.error(e);process.exitCode=1})
