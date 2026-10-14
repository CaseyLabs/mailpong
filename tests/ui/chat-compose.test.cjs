// Chat composition in the docked reply box: Reply vs Reply All, draft autosave and restore, undo-send, the
// identity a reply goes out from, and a refused reply (ISSUES.md R1). Sends use composer-fakes.js, so nothing
// leaves the page: queued sends fire a fake send:finished event after window.__sendDelay ms.
const fs = require('node:fs')
const path = require('node:path')
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const composerFakes = fs.readFileSync(path.resolve(__dirname, '../../.agents/skills/run-mailven/composer-fakes.js'), 'utf8')
const withComposer = (extra = '') => `${composerFakes}\nwindow.__installComposerFakes(window)\n${extra}`

const box = (page) => page.getByPlaceholder('Type a reply...')
const replyLine = (page) => box(page).locator('xpath=ancestor::div[contains(@class,"border-t")][1]')
const log = (page) => page.evaluate(() => window.__log.join(' | '))
// Wait up to 5 s for the reply box to hold text; on a timeout the assertion that follows reports the actual value.
const boxHolds = async (page, text) => {
  for (const end = Date.now() + 5000; Date.now() < end; await page.waitForTimeout(50)) {
    if (await box(page).inputValue({ timeout: 1000 }).catch(() => null) === text) return
  }
}
const argsOf = async (page, name) => (await ui.calls(page, name)).map((c) => c.args)

describe('chat compose', () => {
  it('reply and reply all', ui.run({ beforeMount: withComposer() }, async (page) => {
    // The trip chat is a group chat, so its reply defaults to Reply All (Priya and Sam).
    const line = replyLine(page)
    await line.getByText('2 recipients').waitFor()
    const toggle = line.getByRole('button', { name: /^All/ })
    await toggle.click()
    await line.getByRole('button', { name: /^Priya Shah/ }).waitFor()
    assert.equal(await line.getByText('2 recipients').count(), 0, 'a plain reply names its one recipient instead of a count')
    await box(page).fill('Count me in.')
    await box(page).press('Enter')
    await page.waitForFunction(() => window.__calls.some((c) => c.name === 'QueueChatReply'))
    const [[reply]] = await argsOf(page, 'QueueChatReply')
    assert.deepEqual([reply.messageId, reply.replyAll, reply.text], ['trip-5', false, 'Count me in.'],
      'the reply answers the latest incoming message, with the chosen mode')
  }))

  it('draft autosave and restore', ui.run({
    // A stateful draft store: SaveChatDraft keeps the reply by thread, GetChatDraft returns it.
    beforeMount: withComposer(`{ const A = window.go.app.App, drafts = {}
      A.SaveChatDraft = async (r) => { drafts[r.threadKey] = { ...r, draftId: 'draft-' + r.threadKey }; return drafts[r.threadKey].draftId }
      A.GetChatDraft = async (acc, key) => drafts[key] ?? null }`),
  }, async (page) => {
    await box(page).fill('Half-written reply')
    await replyLine(page).getByRole('button', { name: /^All/ }).click()
    await page.waitForFunction(() => window.__calls.some((c) => c.name === 'SaveChatDraft' && c.args[0].text === 'Half-written reply'))
    await page.locator('[data-chat-row]', { hasText: 'Omar Haddad' }).click()
    await page.waitForFunction(() => window.__calls.some((c) => c.name === 'GetChatDraft' && c.args[1] === 't-omar'))
    assert.equal(await box(page).inputValue(), '', 'another chat starts with its own empty box')
    await page.locator('[data-chat-row]', { hasText: 'Weekend trip to the coast' }).click()
    await boxHolds(page, 'Half-written reply')
    assert.equal(await box(page).inputValue(), 'Half-written reply', 'reopening the chat restores its draft')
    await replyLine(page).getByRole('button', { name: /^Priya Shah/ }).waitFor()
    const saved = (await argsOf(page, 'SaveChatDraft')).at(-1)[0]
    assert.deepEqual([saved.accountId, saved.threadKey, saved.replyAll], ['acc1', 't-trip', false], 'the draft keeps the reply mode')
  }))

  it('undo send puts the reply back, then a resend goes out', ui.run({ beforeMount: withComposer() }, async (page) => {
    await box(page).fill('Undo test reply')
    await box(page).press('Enter')
    const pending = page.getByRole('status').filter({ hasText: 'Undo test reply' })
    await pending.getByText(/Sending in \d+s/).waitFor()
    assert.equal(await box(page).inputValue(), '', 'sending clears the box')
    await pending.getByRole('button', { name: 'Undo' }).click()
    await boxHolds(page, 'Undo test reply')
    assert.equal(await box(page).inputValue(), 'Undo test reply', 'Undo puts the text back in the box')
    assert.match(await log(page), /undo send-1 true/)
    assert.doesNotMatch(await log(page), /fire send-1/, 'an undone send never goes out')
    await page.evaluate(() => { window.__sendDelay = 200 })
    await box(page).press('Enter')
    await pending.getByText('Accepted by server; delivery not confirmed').waitFor()
    assert.match(await log(page), /fire send-2/, 'the resend goes out after the delay')
  }))

  it('reply identity: the alias the message reached', ui.run({
    beforeMount: withComposer(`{ const A = window.go.app.App, ids = A.GetIdentities
      A.GetIdentities = async (acc) => [...await ids(acc), { id: 'id-shop', accountId: acc, email: 'alex.shop@example.com', name: 'Alex Morgan', isDefault: false }]
      A.ChatReplyFrom = async () => 'alex.shop@example.com' }`),
  }, async (page) => {
    await replyLine(page).getByText('from alex.shop@example.com').waitFor()
    assert.deepEqual(await argsOf(page, 'ChatReplyFrom'), [['trip-5']], 'with several identities the backend resolves the alias once')
  }))

  it('reply identity: one identity needs no lookup', ui.run({ beforeMount: withComposer() }, async (page) => {
    await replyLine(page).getByText('2 recipients').waitFor()
    assert.equal(await replyLine(page).getByText(/^from /).count(), 0, 'the default identity is not named')
    assert.equal((await argsOf(page, 'ChatReplyFrom')).length, 0)
  }))

  it('a refused reply (sender needs review) can be edited', ui.run({
    // QueueChatReply refuses replies whose sender needs review (ISSUES.md R1); the error text is synthetic.
    beforeMount: withComposer(`window.go.app.App.QueueChatReply = async () => { throw new Error('sender needs review: send from the full composer') }`),
  }, async (page) => {
    await box(page).fill('Refused reply')
    await box(page).press('Enter')
    const pending = page.getByRole('status').filter({ hasText: 'Refused reply' })
    await pending.getByText('Submission needs attention').waitFor()
    assert.match(await pending.innerText(), /sender needs review/, 'the refusal reason is shown')
    await pending.getByRole('button', { name: 'Edit' }).click()
    assert.equal(await box(page).inputValue(), 'Refused reply', 'Edit puts the reply back in the box')
    assert.equal(await pending.count(), 0)
  }))
})
