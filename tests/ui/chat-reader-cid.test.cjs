// Embedded pictures in decrypted reader mail (MAINT-17): the reader shows a cid: picture from the PGP or S/MIME
// result's inline attachments with no network request, drops unknown content IDs, and keeps to the size budget.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=='
const big = 'data:image/png;base64,' + 'A'.repeat(300 << 10)

// The trip chat's last message becomes encrypted; the PGP result carries a rich reader with cid: pictures.
const fake = (kind, inline) => `{ const A = window.go.app.App, get = A.GetConversation
  A.GetConversation = async (id) => {
    const c = await get(id)
    if (id !== 't-trip') return c
    const last = c.messages.length - 1
    c.messages[last] = { ...c.messages[last], ${kind === 'smime' ? 'hasSMIME' : 'hasPGP'}: true, chat: undefined, bodyHtml: '', bodyText: 'MIME envelope' }
    return c
  }
  const img = (cid, alt) => ({ kind: 'image', src: 'cid:' + cid, alt })
  const result = { bodyHtml: '<p>x</p>', bodyText: 'x', ${kind === 'smime' ? "smimeStatus: '', smimeSignerEmail: '', smimeSignerSubject: '', smimeEncrypted: true" : "pgpStatus: '', pgpSignerEmail: '', pgpSignerKeyId: '', pgpEncrypted: true"},
    inlineAttachments: ${inline},
    chat: { text: 'Receipt', hasQuoted: false, isRich: true, reader: [
      { kind: 'heading', spans: [{ text: 'Receipt' }] },
      img('logo@x', 'Shop logo'), img('Missing@x', 'Unknown picture'), img('huge@x', 'Huge picture'),
      { kind: 'text', spans: [{ text: 'Thanks for your order.' }] },
    ] } }
  A.${kind === 'smime' ? 'ProcessSMIMEMessage' : 'ProcessPGPMessage'} = async () => result
}`
const inline = `{ 'LOGO@x': '${PNG}', 'huge@x': '${big}' }`

const lastId = (page) => page.locator('[data-message-id]').last().getAttribute('data-message-id')

describe('embedded pictures in decrypted reader mail', () => {
  for (const kind of ['pgp', 'smime']) {
    it(`${kind} result shows its cid pictures without a request`, ui.run({ beforeMount: fake(kind, inline) }, async (page) => {
      const requests = []
      page.on('request', (r) => { if (/^https?:/.test(r.url()) && !r.url().startsWith('http://localhost') && !r.url().startsWith('http://127.0.0.1')) requests.push(r.url()) })
      const msg = page.locator(`[data-message-id="${await lastId(page)}"]`)
      await msg.getByText('Thanks for your order.').waitFor()
      const imgs = msg.locator('[data-chat-reader] img')
      assert.equal(await imgs.count(), 1, 'only the resolvable, in-budget picture is shown')
      assert.equal(await imgs.first().getAttribute('src'), PNG)
      assert.equal(await imgs.first().getAttribute('alt'), 'Shop logo')
      assert.equal(await msg.getByText('Unknown picture').count(), 0, 'unknown content ID dropped')
      assert.equal(await msg.getByText('Huge picture').count(), 0, 'over-budget picture dropped')
      assert.equal(await msg.getByRole('button', { name: /load images/i }).count(), 0, 'no remote-image banner')
      assert.deepEqual(requests, [])
    }))
  }
})
