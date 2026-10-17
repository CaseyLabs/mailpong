// Signed and decrypted mail (UIX-8): the chat shows the chat text from the S/MIME or PGP view result as a
// bubble with its security badge, not the full body. Original layout and Show original keep the full body,
// and a failed decryption still shows its banner.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const bubble = (page, id) => page.locator(`[data-message-id="${id}"]`)
const chat = `{ text: 'See you at the trailhead.', quoted: 'On Monday Ana wrote: old plan', hasQuoted: true, isRich: false }`

// The trip chat's last message becomes signed (S/MIME) or encrypted (PGP); its stored body is the raw envelope.
const secureFake = (kind, result) => `{ const A = window.go.app.App, get = A.GetConversation
  A.GetConversation = async (id) => {
    const c = await get(id)
    if (id !== 't-trip') return c
    const last = c.messages.length - 1
    c.messages[last] = { ...c.messages[last], ${kind === 'smime' ? 'hasSMIME' : 'hasPGP'}: true, chat: undefined,
      bodyHtml: '', bodyText: 'MIME envelope' }
    return c
  }
  A.${kind === 'smime' ? 'ProcessSMIMEMessage' : 'ProcessPGPMessage'} = async () => (${result})
}`
const signed = secureFake('smime', `{ bodyHtml: '<p>See you at the trailhead.</p>', bodyText: 'See you at the trailhead.', smimeStatus: 'signed',
  smimeSignerEmail: 'ana@example.com', smimeSignerSubject: '', smimeEncrypted: false, chat: ${chat} }`)
const decrypted = secureFake('pgp', `{ bodyHtml: '<p>See you at the trailhead.</p>', bodyText: 'See you at the trailhead.', pgpStatus: '', pgpSignerEmail: '',
  pgpSignerKeyId: '', pgpEncrypted: true, chat: ${chat} }`)
const attachmentOnly = secureFake('pgp', `{ bodyHtml: '', bodyText: '', pgpStatus: '', pgpSignerEmail: '', pgpSignerKeyId: '', pgpEncrypted: true,
  attachments: [{ filename: 'route-plan.pdf', contentType: 'application/pdf', size: 2048, isInline: false, contentId: '' }],
  chat: { text: '', hasQuoted: false, isRich: false } }`)
const failed = secureFake('pgp', `{ bodyHtml: '', bodyText: '', pgpStatus: 'decrypt_failed', pgpSignerEmail: '', pgpSignerKeyId: '', pgpEncrypted: true }`)
const originalLayout = `{ window.go.app.App.GetChatRichStyle = async () => 'card' }`

const lastId = (page) => page.locator('[data-message-id]').last().getAttribute('data-message-id')

describe('signed and encrypted mail in chat', () => {
  for (const accounts of ['expanded', 'collapsed']) {
    it(`signed mail shows as a chat bubble with its badge (Accounts ${accounts})`, ui.run({ accounts, beforeMount: signed }, async (page) => {
      const msg = bubble(page, await lastId(page))
      await msg.getByText('See you at the trailhead.').waitFor()
      assert.equal(await msg.locator('iframe').count(), 0, 'no full HTML body')
      assert.equal(await msg.locator('.bg-muted, .bg-bubble-own').count() > 0, true, 'renders in a bubble')
      await msg.locator('[title="Signed by ana@example.com with S/MIME"]').waitFor()
      assert.doesNotMatch(await msg.innerText(), /old plan/, 'quoted history hidden')
      await msg.getByRole('button', { name: 'Show quoted text' }).click()
      await msg.getByText('On Monday Ana wrote: old plan').waitFor()
    }))
  }

  it('decrypted mail shows as a chat bubble; Show original shows it in full', ui.run({ beforeMount: decrypted }, async (page) => {
    const msg = bubble(page, await lastId(page))
    await msg.getByText('See you at the trailhead.').waitFor()
    assert.equal(await msg.locator('iframe').count(), 0, 'no full body')
    await msg.locator('[title="This message was encrypted with PGP"]').waitFor()
    await msg.hover()
    await msg.getByRole('button', { name: 'Message actions' }).click()
    await page.getByRole('menuitem', { name: 'Show original' }).click()
    await msg.locator('iframe').waitFor()
    assert.equal(await msg.getByRole('button', { name: 'Show quoted text' }).count(), 0, 'the chat bubble is replaced')
  }))

  it('decrypted attachment-only mail shows just its attachments', ui.run({ beforeMount: attachmentOnly }, async (page) => {
    const msg = bubble(page, await lastId(page))
    await msg.getByText('route-plan.pdf').waitFor()
    assert.equal(await msg.locator('iframe').count(), 0, 'no empty full body')
    assert.equal(await msg.locator('.bg-muted.rounded-bl-md, .bg-bubble-own.rounded-br-md').count(), 0, 'no empty bubble')
  }))

  it('Original layout keeps signed mail in full', ui.run({ beforeMount: `${signed}\n${originalLayout}` }, async (page) => {
    const msg = bubble(page, await lastId(page))
    await msg.locator('[title="Signed by ana@example.com with S/MIME"]').waitFor()
    assert.equal(await msg.getByRole('button', { name: 'Show quoted text' }).count(), 0, 'no chat bubble')
    await msg.hover()
    await msg.getByRole('button', { name: 'Message actions' }).click()
    assert.equal(await page.getByRole('menuitem', { name: 'Show original' }).count(), 0, 'no chat view to leave')
  }))

  it('a failed decryption keeps its banner and the full body', ui.run({ beforeMount: failed }, async (page) => {
    const msg = bubble(page, await lastId(page))
    await msg.getByText('PGP decryption failed').waitFor()
    assert.equal(await msg.getByRole('button', { name: 'Show quoted text' }).count(), 0, 'no chat bubble')
  }))
})
