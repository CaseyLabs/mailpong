// Undo-send check on fake data: chat reply, main-window composer, detached composer.
// Usage: node undo-send-check.js <outdir>   (dev server on :34115, serve-fakes.py on :8765)
// Prints PASS/FAIL per step with the fake binding log, and writes screenshots to <outdir>.
// Chat screenshots cover the Accounts column expanded and collapsed. Takes about a minute.
const path = require('path')
const { launch } = require('./pw')
const out = process.argv[2]
const fakes = path.join(__dirname, 'composer-fakes.js')
let failed = 0
const check = (name, ok, detail) => { if (!ok) failed++; console.log(`${ok ? 'PASS' : 'FAIL'} ${name}${detail ? ' :: ' + detail : ''}`) }

// Main window inside an iframe with the read fakes and the composer fakes installed before mount.
async function mainWindow(b) {
  const p = await b.newPage({ viewport: { width: 1300, height: 700 }, locale: 'en-GB' })
  p.on('pageerror', (e) => { if (!/blocked: |reading 'nodes'/.test(e.message)) console.log('pageerror:', e.message) })
  await p.goto('http://localhost:34115'); await p.waitForTimeout(3000)
  const ui0 = await p.evaluate(async () => JSON.stringify(await window.go.app.App.GetUIState()))
  await p.evaluate(async () => {
    for (const f of ['screenshot-fakes.js', 'composer-fakes.js']) (0, eval)(await (await fetch('http://127.0.0.1:8765/' + f)).text())
    document.body.innerHTML = ''; document.body.style.margin = '0'
    const f = document.createElement('iframe')
    f.style.cssText = 'width:1300px;height:700px;border:0;position:fixed;left:0;top:0'
    document.body.appendChild(f); f.src = location.href
    const poll = setInterval(() => { try { const w = f.contentWindow
      if (w.location.href !== 'about:blank' && w.go?.app?.App && !w.__faked) { w.__faked = true; __installFakes(w); __installComposerFakes(w); clearInterval(poll) } } catch {} }, 1)
  })
  const fr = await (await p.waitForSelector('iframe')).contentFrame()
  await fr.waitForSelector('.chat-row', { timeout: 30000 })
  const log = () => fr.evaluate(() => window.__log.join(' | '))
  const uiSame = async () => ui0 === (await p.evaluate(async () => JSON.stringify(await window.go.app.App.GetUIState())))
  return { p, fr, log, uiSame }
}

async function chat(b) {
  const { p, fr, log, uiSame } = await mainWindow(b)
  await fr.locator('.chat-row').first().click(); await p.waitForTimeout(1500)
  const box = fr.locator('textarea').last()
  await box.fill('Undo test reply'); await box.press('Enter'); await p.waitForTimeout(1500)
  const status = await fr.locator('[role=status]').last().innerText().catch(() => '')
  check('chat: queued bubble counts down', /Sending in \d+s/.test(status), status.replace(/\s+/g, ' '))
  await fr.getByRole('button', { name: 'Undo' }).click(); await p.waitForTimeout(800)
  check('chat: undo puts the text back', (await box.inputValue()) === 'Undo test reply', await log())
  await box.press('Enter'); await p.waitForTimeout(800)
  const toggle = fr.locator('[title="Hide sidebar"],[aria-label="Hide sidebar"],[title="Show sidebar"],[aria-label="Show sidebar"]').first()
  for (const n of [1, 2]) { await p.screenshot({ path: `${out}/chat-queued-${n}.png` }); await toggle.click(); await p.waitForTimeout(700) }
  await p.waitForTimeout(10500)
  check('chat: resend goes out after the delay', /fire send-2/.test(await log()), await log())
  await p.screenshot({ path: `${out}/chat-sent.png` })
  check('chat: saved UI state unchanged', await uiSame())
  await p.close()
}

async function composer(b) {
  const { p, fr, log, uiSame } = await mainWindow(b)
  const reg = fr.getByRole('region', { name: 'Email composer' }) // the chat reply box has its own Send
  const editor = reg.locator('[contenteditable=true]:visible').first()
  await fr.getByRole('button', { name: /Compose/ }).first().click(); await p.waitForTimeout(2500)
  await fr.getByPlaceholder('Subject').fill('Undo toast test')
  const to = fr.getByPlaceholder(/recipients/i).first(); await to.fill('sam@example.com'); await to.press('Enter')
  await editor.click(); await editor.type('Hello there'); await p.waitForTimeout(500)
  await reg.getByRole('button', { name: /^Send$/ }).click(); await p.waitForTimeout(1500)
  await p.screenshot({ path: `${out}/composer-toast.png` })
  check('composer: closes and queues against the draft', (await fr.getByPlaceholder('Subject').count()) === 0 && /queue send-1 draft-1 Undo toast test/.test(await log()), await log())
  await fr.getByRole('button', { name: 'Undo' }).first().click(); await p.waitForTimeout(2000)
  const subject = await fr.getByPlaceholder('Subject').inputValue().catch(() => '')
  const body = await editor.innerText().catch(() => '')
  check('composer: undo reopens the message', subject === 'Undo toast test' && body.includes('Hello there'), `subject=${subject} body=${body.trim()}`)
  await reg.getByRole('button', { name: /^Send$/ }).click(); await p.waitForTimeout(11500)
  const l = await log()
  check('composer: resend goes out after the delay, never directly', /fire send-2/.test(l) && !/SEND /.test(l), l)
  check('composer: saved UI state unchanged', await uiSame())
  await p.close()
}

async function detached(b) {
  const p = await b.newPage({ viewport: { width: 900, height: 700 }, locale: 'en-GB' })
  p.on('pageerror', (e) => console.log('pageerror:', e.message))
  await p.addInitScript({ path: fakes })
  await p.goto('http://localhost:34115/composer.html')
  const reg = p.getByRole('region', { name: 'Email composer' })
  await reg.waitFor({ timeout: 30000 }); await p.waitForTimeout(1500)
  const log = () => p.evaluate(() => window.__log.join(' | '))
  const editor = reg.locator('[contenteditable=true]:visible').first()
  await p.getByPlaceholder('Subject').fill('Detached test')
  const to = p.getByPlaceholder(/recipients/i).first(); await to.fill('sam@example.com'); await to.press('Enter')
  await editor.click(); await editor.type('Hi')
  await reg.getByRole('button', { name: /^Send$/ }).click(); await p.waitForTimeout(1500)
  await p.screenshot({ path: `${out}/detached-countdown.png` })
  check('detached: counts down without sending', (await reg.getByRole('button', { name: 'Undo' }).count()) === 1 && !/SEND /.test(await log()), await log())
  await reg.getByRole('button', { name: 'Undo' }).click(); await p.waitForTimeout(800)
  check('detached: undo returns to editing', (await reg.getByRole('button', { name: /^Send$/ }).count()) === 1 && !/SEND /.test(await log()))
  await reg.getByRole('button', { name: /^Send$/ }).click(); await p.waitForTimeout(1000)
  await reg.getByRole('button', { name: /Send now/ }).click(); await p.waitForTimeout(2000)
  const l = await log()
  check('detached: send now sends and closes', /SEND Detached test/.test(l) && /closeWindow/.test(l), l)
  await p.close()
}

;(async () => {
  const b = await launch()
  for (const run of [chat, composer, detached]) await run(b).catch((e) => check(run.name, false, e.message.split('\n')[0]))
  await b.close()
  console.log(failed ? `${failed} failed` : 'all passed')
  process.exit(failed ? 1 : 0)
})()
