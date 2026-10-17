// Conversation compaction: runs of older, read messages fold into one expandable row; the latest messages
// and unread messages never fold; the rows work by keyboard and hold the scroll position.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// The trip chat gets 14 messages: trip-4 is unread, so the read runs are trip-0..3 and trip-5..8 and the
// last five (trip-9..13) are the recent tail.
const longTrip = `{ const A = window.go.app.App, get = A.GetConversation
  A.GetConversation = async (id) => {
    const c = await get(id)
    if (id !== 't-trip') return c
    const base = c.messages[0], t = Date.parse(base.date)
    const messages = Array.from({ length: 14 }, (_, i) => ({ ...base, id: 'trip-' + i, uid: i, date: new Date(t + i * 60e3).toISOString(),
      isRead: i !== 4, mine: i % 3 === 1, fromName: i % 3 === 1 ? 'Alex Morgan' : 'Priya Shah', fromEmail: i % 3 === 1 ? 'alex@example.com' : 'priya@example.net',
      bodyText: 'Message ' + i, snippet: 'Message ' + i, chat: { text: 'Message ' + i + '. ' + 'More words. '.repeat(8), hasQuoted: false, isRich: false } }))
    return { ...c, messages, messageCount: messages.length }
  } }`

const articles = (page) => page.locator('[role="article"][data-message-id]').evaluateAll((els) => els.map((e) => e.dataset.messageId))
const rows = (page) => page.locator('[data-compact-row]')

describe('chat compaction', () => {
  it('compactItems keeps the tail, unread, drafts and headings; shows the sender after a fold', ui.run({}, async (page) => {
    const r = await page.evaluate(async () => {
      const { compactItems } = await import('/src/lib/components/chat/chatCompact.ts')
      // o[i].day sets the message's day (default day 0); newDay follows buildThreadItems.
      const mk = (n, o = {}) => Array.from({ length: n }, (_, i) => ({ msg: { id: 'm' + i, isRead: true, isDraft: false, ...(o[i] ?? {}) },
        subject: o[i]?.subject, groupStart: false, date: new Date(2026, 0, 5 + (o[i]?.day ?? 0), 9, i) }))
        .map((it, i, all) => ({ ...it, newDay: i === 0 || it.date.getDate() !== all[i - 1].date.getDate() }))
      // * marks a sender line (groupStart), @ a date separator (newDay).
      const shape = (es) => es.map((e) => (e.kind === 'run' ? 'run' + e.count + (e.open ? '+' : '') : e.item.msg.id + (e.item.groupStart ? '*' : '') + (e.item.newDay ? '@' : '')))
      const days = (list) => Object.fromEntries(list.map((d, i) => [i, { day: d }]))
      return {
        short: shape(compactItems(mk(7), new Set())),
        long: shape(compactItems(mk(10), new Set())),
        open: shape(compactItems(mk(10), new Set(['m0']))),
        unread: shape(compactItems(mk(10, { 2: { isRead: false } }), new Set())),
        draft: shape(compactItems(mk(10, { 1: { isDraft: true } }), new Set())),
        heading: shape(compactItems(mk(10, { 0: { subject: 'Hikes' } }), new Set())),
        keep: shape(compactItems(mk(10), new Set(), new Set(['m2']))),
        // m0 (a heading, day 0) stays; m1-m4 (days 1-2) fold; m5 is on day 2 like m4.
        hiddenDay: shape(compactItems(mk(10, { ...days([0, 1, 1, 2, 2, 2, 2, 2, 2, 2]), 0: { subject: 'Hikes', day: 0 } }), new Set())),
        sameDay: shape(compactItems(mk(10, { ...days([0, 0, 0, 0, 0, 1, 1, 1, 1, 1]), 0: { subject: 'Hikes', day: 0 } }), new Set())),
      }
    })
    assert.deepEqual(r.short, ['m0@', 'm1', 'm2', 'm3', 'm4', 'm5', 'm6'], 'two older messages are below the run minimum')
    assert.deepEqual(r.long, ['run5', 'm5*@', 'm6', 'm7', 'm8', 'm9'], 'five older fold; the message after starts a run and, first shown, carries the date')
    assert.deepEqual(r.open.slice(0, 7), ['run5+', 'm0*@', 'm1', 'm2', 'm3', 'm4', 'm5'], 'an open run lists its messages, the first with its sender')
    assert.deepEqual(r.unread, ['m0@', 'm1', 'm2', 'm3', 'm4', 'm5', 'm6', 'm7', 'm8', 'm9'], 'an unread message splits the older messages into short runs')
    assert.deepEqual(r.draft, ['m0@', 'm1', 'run3', 'm5*', 'm6', 'm7', 'm8', 'm9'], 'a draft never folds')
    assert.deepEqual(r.heading, ['m0@', 'run4', 'm5*', 'm6', 'm7', 'm8', 'm9'], 'a subject heading message stays')
    assert.deepEqual(r.hiddenDay, ['m0@', 'run4', 'm5*@', 'm6', 'm7', 'm8', 'm9'], 'a run that hid a day change dates the next message')
    assert.deepEqual(r.sameDay, ['m0@', 'run4', 'm5*@', 'm6', 'm7', 'm8', 'm9'], 'a message on a new day keeps its date')
    assert.deepEqual(r.keep.slice(0, 5), ['m0@', 'm1', 'm2', 'm3', 'm4'], 'a kept message splits the run below the minimum')
  }))

  it('folds runs of earlier read messages and never the unread or recent ones', ui.run({ beforeMount: longTrip }, async (page) => {
    await rows(page).first().waitFor()
    assert.deepEqual(await articles(page), ['trip-0', 'trip-4', 'trip-9', 'trip-10', 'trip-11', 'trip-12', 'trip-13'], 'only the subject-heading opener, the unread message and the latest five show')
    assert.deepEqual(await rows(page).allInnerTexts(), ['3 earlier messages', '4 earlier messages'])
    assert.equal(await rows(page).first().getAttribute('aria-expanded'), 'false')
  }))

  it('short chats do not fold', ui.run({}, async (page) => {
    await page.locator('[data-message-id="trip-5"]').waitFor()
    assert.equal(await rows(page).count(), 0, 'six read messages stay visible')
  }))

  it('rows expand and collapse from the keyboard', ui.run({ beforeMount: longTrip }, async (page) => {
    const first = rows(page).first()
    await first.waitFor()
    await first.focus()
    await page.keyboard.press('Enter')
    assert.equal(await first.getAttribute('aria-expanded'), 'true')
    assert.match(await first.innerText(), /Hide 3 earlier messages/)
    assert.deepEqual((await articles(page)).slice(0, 5), ['trip-0', 'trip-1', 'trip-2', 'trip-3', 'trip-4'], 'the run shows in order')
    assert.match(await page.locator('[data-message-id="trip-3"]').innerText(), /Message 3\./, "expanded messages render")
    await page.keyboard.press('Space')
    assert.equal(await first.getAttribute('aria-expanded'), 'false')
    assert.deepEqual((await articles(page)).slice(0, 3), ['trip-0', 'trip-4', 'trip-9'], 'and fold again')
  }))

  it('Tab reaches folded-run rows between messages, in order', ui.run({ beforeMount: longTrip }, async (page) => {
    await rows(page).first().waitFor()
    await page.locator('[data-message-id="trip-4"]').focus()
    const at = () => page.evaluate(() => document.activeElement?.dataset.messageId ?? (document.activeElement?.hasAttribute('data-compact-row') ? 'row' : document.activeElement?.tagName))
    const steps = []
    for (const key of ['Tab', 'Tab', 'Shift+Tab', 'Shift+Tab', 'Shift+Tab']) { await page.keyboard.press(key); steps.push(await at()) }
    assert.deepEqual(steps, ['row', 'trip-9', 'row', 'trip-4', 'row'], 'Tab and Shift+Tab stop on both runs around trip-4')
    assert.equal(await page.evaluate(() => document.activeElement === document.querySelectorAll('[data-compact-row]')[0]), true, 'the last stop is the first run')
  }))

  it('scrolling or sending ends the hold on an expanded row', ui.run({}, async (page) => {
    const r = await page.evaluate(async () => {
      const { ChatScroll } = await import('/src/lib/components/chat/chatScroll.svelte.ts')
      const held = async (act) => {
        const s = new ChatScroll(), sc = document.createElement('div'), el = document.createElement('div')
        sc.append(el); document.body.append(sc); s.scroller = sc
        await s.holdPosition(el, () => {})
        const before = s.keep !== null
        await act(s)
        sc.remove()
        return [before, s.keep !== null]
      }
      return { by: await held((s) => s.by(1)), toEnd: await held((s) => s.toEnd()) }
    })
    assert.deepEqual(r.by, [true, false], 'keyboard scrolling releases the row')
    assert.deepEqual(r.toEnd, [true, false], 'scrolling to a sent reply releases the row')
  }))

  it('a message read while open stays visible', ui.run({ beforeMount: longTrip }, async (page) => {
    await page.locator('[data-message-id="trip-4"]').waitFor()
    // Mark trip-4 read through the same event the backend emits.
    await page.evaluate(() => window.runtime.EventsEmit?.('messages:readChanged', { messageIds: ['trip-4'], isRead: true }))
    await page.waitForTimeout(300)
    assert.ok((await articles(page)).includes('trip-4'), 'trip-4 does not fold under the reader')
    assert.equal(await rows(page).count(), 2, 'the two runs do not merge around it')
  }))

  it('expanding holds the row in place', ui.run({ height: 420, beforeMount: longTrip }, async (page) => {
    const first = rows(page).first()
    await first.waitFor()
    await page.waitForTimeout(2300)
    const scroller = page.locator('[role="log"]')
    await scroller.evaluate((el) => { el.scrollTop = el.scrollHeight })
    await page.waitForTimeout(200)
    const top = () => first.evaluate((el) => el.getBoundingClientRect().top)
    const before = await top()
    await first.evaluate((el) => el.click())
    await page.waitForTimeout(400)
    assert.equal(await first.getAttribute('aria-expanded'), 'true')
    assert.ok(Math.abs((await top()) - before) <= 1, 'the row keeps its place after expanding')
  }))
})
