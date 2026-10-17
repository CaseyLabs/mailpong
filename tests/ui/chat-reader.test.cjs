// Reader view for rich mail: a newsletter renders its sanitized reader blocks (headings, paragraphs, items,
// links, images) as chat content instead of the collapsed card. Remote images stay blocked until loaded, long
// messages fold behind "Show more", and chat_rich_style 'card' restores the card.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const bubble = (page, id) => page.locator(`[data-message-id="${id}"]`)

const { readerFake } = require('./reader-fake.cjs')
const cardStyle = `{ window.go.app.App.GetChatRichStyle = async () => 'card' }`

describe('chat reader view', () => {
  for (const accounts of ['expanded', 'collapsed']) {
    it(`a newsletter renders as readable chat content (Accounts ${accounts})`, ui.run({ accounts, beforeMount: readerFake, allow: ['OpenURL'] }, async (page) => {
      const msg = bubble(page, 'trip-5')
      const reader = msg.locator('[data-chat-reader]')
      await reader.waitFor()
      assert.equal(await msg.getByRole('button', { name: 'Show message' }).count(), 0, 'no collapsed card')
      assert.equal(await msg.locator('iframe').count(), 0, 'no full HTML body')
      await msg.getByRole('heading', { name: 'Ten autumn hikes' }).waitFor()
      assert.match(await reader.innerText(), /Coastal paths and a ridge walk this week\.[\s\S]*Boots/)
      assert.equal(await msg.locator('.bg-muted [data-chat-reader]').count(), 1, 'an incoming reader message sits in a bubble')

      // Remote images are blocked until loaded; the alt text stands in.
      assert.equal(await reader.locator('img').count(), 0, 'remote image blocked')
      await reader.getByText('A ridge at dawn').waitFor()
      await reader.getByRole('button', { name: 'Load images' }).click()
      const img = reader.locator('img')
      assert.equal(await img.getAttribute('src'), 'https://cdn.example.com/hero.jpg')
      assert.equal(await img.getAttribute('referrerpolicy'), 'no-referrer')

      // Links open through the app, not by navigating.
      await reader.getByRole('link', { name: 'a ridge walk' }).click()
      await reader.getByRole('link', { name: 'Read more' }).click()
      const opened = (await ui.calls(page, 'OpenURL')).map((c) => c.args[0])
      assert.deepEqual(opened, ['https://example.com/ridge', 'https://example.com/read'])
    }))
  }

  it('long newsletters fold behind Show more, by keyboard too', ui.run({ beforeMount: readerFake }, async (page) => {
    const reader = bubble(page, 'trip-5').locator('[data-chat-reader]')
    await reader.waitFor()
    assert.doesNotMatch(await reader.innerText(), /Trail note 10\.|Unsubscribe/, 'later blocks start folded')
    const more = reader.getByRole('button', { name: 'Show more' })
    assert.equal(await more.getAttribute('aria-expanded'), 'false')
    await more.focus()
    await page.keyboard.press('Enter')
    const less = reader.getByRole('button', { name: 'Show less' })
    assert.equal(await less.getAttribute('aria-expanded'), 'true')
    assert.match(await reader.innerText(), /Trail note 10\.[\s\S]*Unsubscribe/, 'Show more reveals the rest')
    await less.click()
    assert.doesNotMatch(await reader.innerText(), /Unsubscribe/, 'Show less folds again')
  }))

  it('Show original switches to the full body', ui.run({ beforeMount: readerFake }, async (page) => {
    const msg = bubble(page, 'trip-5')
    await msg.locator('[data-chat-reader]').waitFor()
    await msg.hover()
    await msg.getByRole('button', { name: 'Message actions' }).click()
    await page.getByRole('menuitem', { name: 'Show original' }).click()
    await msg.locator('iframe').waitFor()
    assert.equal(await msg.locator('[data-chat-reader]').count(), 0, 'the reader view is replaced')
  }))

  it('flat mode shows the reader without a bubble', ui.run({ beforeMount: `${readerFake}\n{ window.go.app.App.GetChatDisplayStyle = async () => 'flat' }` }, async (page) => {
    const msg = bubble(page, 'trip-5')
    await msg.locator('[data-chat-reader]').waitFor()
    assert.equal(await msg.locator('.bg-muted [data-chat-reader], .bg-bubble-own [data-chat-reader]').count(), 0, 'no bubble background')
  }))

  it('the card setting keeps the collapsed card', ui.run({ beforeMount: `${readerFake}\n${cardStyle}` }, async (page) => {
    const msg = bubble(page, 'trip-5')
    await msg.getByRole('button', { name: /Show message/ }).waitFor()
    assert.equal(await msg.locator('[data-chat-reader]').count(), 0, 'no reader content')
  }))
})
