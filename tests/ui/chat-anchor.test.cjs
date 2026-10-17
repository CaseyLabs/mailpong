// Opening a chat at its first unread message: when that message starts a subject on a new day, its day label
// sits below the sticky subject heading instead of under it (UI8).
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// 14 messages: trip-0..6 are "Hikes" on one day; trip-7 (unread) opens "Gear" on the next day.
const twoSubjects = `{ const A = window.go.app.App, get = A.GetConversation
  A.GetConversation = async (id) => {
    const c = await get(id)
    if (id !== 't-trip') return c
    const base = c.messages[0], t = Date.parse(base.date)
    const messages = Array.from({ length: 14 }, (_, i) => ({ ...base, id: 'trip-' + i, uid: i, subject: i < 7 ? 'Hikes' : 'Gear',
      date: new Date(t + (i < 7 ? 0 : 86400e3) + i * 60e3).toISOString(), isRead: i !== 7, mine: false, fromName: 'Priya Shah', fromEmail: 'priya@example.net',
      bodyText: 'Message ' + i, snippet: 'Message ' + i, chat: { text: 'Message ' + i + '. ' + 'More words. '.repeat(20), hasQuoted: false, isRich: false } }))
    return { ...c, messages, messageCount: messages.length }
  } }`

const boxes = (page) => page.evaluate(() => {
  const msg = document.querySelector('[data-message-id="trip-7"]')
  const heading = document.querySelector('[data-subject-heading]')
  const stuck = [...document.querySelectorAll('[data-subject-heading]')].find((h) => h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < 1)
  let node = msg
  while (!node.previousElementSibling && node.parentElement) node = node.parentElement
  const label = node.previousElementSibling.querySelector('[role="separator"] span')
  const top = (el) => el.getBoundingClientRect()
  return { heading: top(stuck ?? heading).bottom, label: top(label).top, labelBottom: top(label).bottom, msg: top(msg).top, hasLabel: !!label }
})

describe('chat unread anchor', () => {
  for (const accounts of ['expanded', 'collapsed']) {
    it(`the day label clears the sticky heading after the jump (Accounts ${accounts})`, ui.run({ height: 500, accounts, beforeMount: twoSubjects }, async (page) => {
      await page.locator('[data-message-id="trip-7"]').waitFor()
      await page.waitForTimeout(2300)
      const b = await boxes(page)
      assert.ok(b.hasLabel, 'the unread message has a day label')
      assert.ok(b.label >= b.heading - 0.5, `day label top ${b.label} is below the heading bottom ${b.heading}`)
      assert.ok(b.msg > b.labelBottom - 0.5, 'the message follows its label')
    }))
  }
})
