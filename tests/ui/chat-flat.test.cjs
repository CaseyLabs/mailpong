// Flat transcript mode: chat_display_style 'flat' drops the bubbles and avatar column, shows a name and time
// line for every message and marks my messages with an accent rule; 'bubbles' (the default) is unchanged.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

const flatStyle = `{ window.go.app.App.GetChatDisplayStyle = async () => 'flat' }`
// Settings → Save writes every setting; the fake rejects writes unless allowed.
const SETTINGS_SAVE = ['SetReadReceiptResponsePolicy', 'SetMarkAsReadDelay', 'SetAccentBarUnread', 'SetAccentUnreadStyle', 'SetAlwaysLoadImages', 'SetAlwaysShowMessageCheckbox', 'SetAutostart', 'SetChatAutoAdvance', 'SetChatCombineSenders', 'SetChatDisplayStyle', 'SetChatIncludeQuote', 'SetChatNotifyPriorityOnly', 'SetChatReadingWidth', 'SetChatSendKey', 'SetChatShowLowGroup', 'SetComposerFormat', 'SetComposerMode', 'SetDarkComposerBody', 'SetDarkMailContent', 'SetLanguage', 'SetMailtoMode', 'SetMessageListDensity', 'SetNativeTitleBar', 'SetRunBackground', 'SetShowMessageListCircles', 'SetShowMessageListProfilePics', 'SetShowMessagePreview', 'SetShowTitleBar', 'SetShowViewerCircles', 'SetSpellcheckEnabled', 'SetSpellcheckLanguages', 'SetStartHidden', 'SetThemeMode']

const bubbleBackground = (page) => page.locator('[role="article"][data-message-id]').evaluateAll((els) =>
  els.map((e) => ({ id: e.dataset.messageId, bubble: !!e.querySelector('.bg-muted, .bg-bubble-own'), avatar: !!e.querySelector('[aria-hidden="true"].w-8'),
    named: /\w/.test(e.querySelector('.text-xs')?.textContent ?? ''), rule: !!e.querySelector('.border-l-2') })))

describe('chat flat transcript', () => {
  it('defaults to bubbles with the avatar column', ui.run({}, async (page) => {
    await page.locator('[data-message-id="trip-5"]').waitFor()
    const all = await bubbleBackground(page)
    assert.ok(all.some((m) => m.bubble), 'bubble backgrounds render by default')
    assert.ok(all.every((m) => !m.rule), 'no accent rule in bubble mode')
    assert.ok(all.every((m) => m.avatar), 'every message keeps the avatar column')
  }))

  it('flat mode has no bubble or avatar column and rules only my messages', ui.run({ beforeMount: flatStyle }, async (page) => {
    await page.locator('[data-message-id="trip-5"]').waitFor()
    const all = await bubbleBackground(page)
    assert.ok(all.length > 0)
    assert.ok(all.every((m) => !m.bubble), 'no bubble background')
    assert.ok(all.every((m) => !m.avatar), 'no avatar column')
    const mine = await page.evaluate(async () => {
      const c = await window.go.app.App.GetConversation('t-trip')
      return c.messages.filter((m) => m.mine).map((m) => m.id)
    })
    for (const m of all) assert.equal(m.rule, mine.includes(m.id), `accent rule only on my messages (${m.id})`)
    assert.ok(await page.locator('[data-message-id] .font-medium').count() > 0, 'a name line shows')
  }))

  it('settings choose the style and save it', ui.run({ allow: SETTINGS_SAVE }, async (page) => {
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await page.getByRole('tab', { name: 'Chat' }).click()
    await page.getByRole('button', { name: 'Message style' }).click()
    await page.getByRole('option', { name: 'Flat' }).click()
    await page.getByRole('button', { name: 'Save' }).click()
    await page.waitForFunction(() => document.querySelector('[role="article"][data-message-id]') && !document.querySelector('[role="article"] .bg-muted'))
    const calls = await ui.calls(page, 'SetChatDisplayStyle')
    assert.ok(calls.length >= 1 && JSON.stringify(calls[0]).includes('flat'), 'Flat is saved')
  }))
})
