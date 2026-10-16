// Pure chat logic (chatFormat.ts, chatSnooze.ts) and account form defaults, imported in the page through Vite so the app's own
// TypeScript runs unchanged. Each test evaluates the functions in the browser and asserts in Node.
const { describe, it } = require('node:test')
const assert = require('node:assert/strict')
const ui = require('./harness.cjs')

// Browser-side message factory shared by the tests below.
const helpers = `
  window.__msg = (id, from, o = {}) => ({
    id, fromName: from.name, fromEmail: from.email, toList: JSON.stringify(o.to ?? []), ccList: JSON.stringify(o.cc ?? []),
    replyTo: o.replyTo ?? '', subject: o.subject ?? 'Plans', date: o.date ?? '2026-10-14T10:00:00Z',
    mine: !!o.mine, isDraft: !!o.draft,
  })
  window.__people = { me: { name: 'Alex Morgan', email: 'alex@example.com' }, sam: { name: 'Sam Rivera', email: 'sam@example.org' },
    priya: { name: 'Priya Shah', email: 'priya@example.net' } }
`
const opts = { beforeMount: helpers }
const formatModule = '/src/lib/components/chat/chatFormat.ts'

describe('chat logic', () => {
  it('reply and reply all recipients', ui.run(opts, async (page) => {
    const r = await page.evaluate(async (mod) => {
      const { replyRecipients } = await import(mod)
      const { me, sam, priya } = window.__people
      const self = new Set([me.email])
      const msgs = [
        window.__msg('m1', sam, { to: [me, priya], cc: [{ name: '', email: 'lee@example.com' }] }),
        window.__msg('m2', priya, { to: [me], replyTo: 'list@example.net' }),
        window.__msg('m3', me, { to: [sam], mine: true }),
        window.__msg('m4', sam, { to: [{ name: '', email: 'ALEX@example.com' }, { name: '', email: 'priya@example.net' }] }),
      ]
      return Object.fromEntries(['m1', 'm2', 'm3', 'm4'].map((id) => [id, replyRecipients(msgs, id, self)]))
    }, formatModule)
    const emails = (list) => list.map((p) => p.email)
    assert.deepEqual(emails(r.m1.reply), ['sam@example.org'], 'Reply answers only the sender')
    assert.deepEqual(emails(r.m1.all), ['sam@example.org', 'priya@example.net', 'lee@example.com'], 'Reply All adds To and Cc, without me')
    assert.deepEqual(emails(r.m2.reply), ['list@example.net'], 'Reply-To wins over the sender')
    assert.deepEqual(emails(r.m3.reply), ['alex@example.com'], 'replying to my own message falls back to its sender')
    assert.deepEqual(emails(r.m4.all), ['sam@example.org', 'priya@example.net'], 'self is removed case-insensitively')
    assert.equal(r.m4.all[1].name, 'Priya Shah', 'a bare address takes the name the thread uses')
  }))

  it('reply target and Reply All default', ui.run(opts, async (page) => {
    const r = await page.evaluate(async (mod) => {
      const { replyTarget } = await import(mod)
      const { me, sam, priya } = window.__people
      const self = new Set([me.email])
      const one = [window.__msg('a', sam, { to: [me] }), window.__msg('b', me, { to: [sam], mine: true })]
      const group = [window.__msg('c', sam, { to: [me, priya] }), window.__msg('d', priya, { to: [me], draft: true })]
      const mineOnly = [window.__msg('e', me, { to: [sam], mine: true })]
      return { one: replyTarget(one, self), group: replyTarget(group, self), mineOnly: replyTarget(mineOnly, self), empty: replyTarget([], self) }
    }, formatModule)
    assert.deepEqual(r.one, { messageId: 'a', defaultReplyAll: false }, 'one-to-one chat answers their latest message with Reply')
    assert.deepEqual(r.group, { messageId: 'c', defaultReplyAll: true }, 'group chat defaults to Reply All; drafts never count')
    assert.deepEqual(r.mineOnly, { messageId: 'e', defaultReplyAll: true }, 'with only my messages, reply to my latest recipients')
    assert.equal(r.empty, null)
  }))

  it('sender identity for replies', ui.run(opts, async (page) => {
    const r = await page.evaluate(async (mod) => {
      const { replyAlias } = await import(mod)
      const { sam } = window.__people
      const ids = [{ email: 'alex@example.com', isDefault: true }, { email: 'Alex.Shop@example.com ', isDefault: false }]
      return {
        alias: replyAlias(window.__msg('a', sam, { cc: [{ name: '', email: 'alex.shop@EXAMPLE.com' }] }), ids),
        primary: replyAlias(window.__msg('b', sam, { to: [{ name: '', email: 'alex@example.com' }] }), ids),
        none: replyAlias(window.__msg('c', sam, { to: [{ name: '', email: 'other@example.com' }] }), ids),
      }
    }, formatModule)
    assert.equal(r.alias, 'Alex.Shop@example.com', 'a reply goes out from the non-default identity it was addressed to')
    assert.equal(r.primary, '', 'the default identity needs no alias')
    assert.equal(r.none, '')
  }))

  it('bubble grouping, incoming/outgoing and subjects', ui.run(opts, async (page) => {
    const r = await page.evaluate(async (mod) => {
      const { buildThreadItems, displaySubject } = await import(mod)
      const { me, sam } = window.__people
      const at = (h, m = 0) => `2026-10-14T${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:00Z`
      const thread = [
        window.__msg('1', sam, { date: at(9) }),
        window.__msg('2', sam, { date: at(9, 3) }), // same sender within 5 minutes: same run
        window.__msg('3', me, { date: at(9, 4), mine: true }), // sender change
        window.__msg('4', me, { date: at(9, 30), mine: true }), // gap over 5 minutes
        window.__msg('5', sam, { date: '2026-10-15T09:00:00Z' }), // new day
      ]
      const sender = [
        window.__msg('a', sam, { subject: 'Invoice 12', date: at(9) }),
        window.__msg('b', sam, { subject: 'RE: invoice 12', date: at(9, 1) }),
        window.__msg('c', sam, { subject: 'Fwd[2]: Delivery', date: at(9, 2) }),
        window.__msg('d', sam, { subject: '', date: at(9, 3) }),
      ]
      const pick = (items) => items.map((i) => ({ id: i.msg.id, mine: i.mine, newDay: i.newDay, groupStart: i.groupStart, subject: i.subject ?? null }))
      return {
        thread: pick(buildThreadItems(thread)),
        sender: pick(buildThreadItems(sender, true)),
        oneSubject: pick(buildThreadItems(sender.slice(0, 2), true)),
        subjects: ['Re: Re: Hi', 'AW: Hallo', 'SV:Hej', 'Fwd: Re[3]: x'].map(displaySubject),
      }
    }, formatModule)
    assert.deepEqual(r.thread.map((i) => i.groupStart), [true, false, true, true, true], 'runs split on sender change, 5-minute gap and new day')
    assert.deepEqual(r.thread.map((i) => i.mine), [false, false, true, true, false], 'incoming and outgoing follow msg.mine')
    assert.deepEqual(r.thread.map((i) => i.newDay), [true, false, false, false, true])
    assert.deepEqual(r.sender.map((i) => i.subject), ['Invoice 12', null, 'Delivery', null], 'a sender chat labels each subject run; Re: continues it')
    assert.deepEqual(r.sender.map((i) => i.groupStart), [true, false, true, false], 'a new subject starts a new run')
    assert.ok(r.oneSubject.every((i) => i.subject === null), 'one distinct subject needs no headings')
    assert.deepEqual(r.subjects, ['Hi', 'Hallo', 'Hej', 'x'])
  }))

  it('list groups, sender chat toggle and snooze presets', ui.run(opts, async (page) => {
    const r = await page.evaluate(async (mod) => {
      const { trimGroup, canToggleSenderChat } = await import(mod)
      const { snoozePresets } = await import('/src/lib/components/chat/chatSnooze.ts')
      const rows = ['a', 'b', 'c', 'd', 'e'].map((key) => ({ key }))
      const keys = (l) => l.map((x) => x.key).join('')
      const ids = (d) => snoozePresets(d).map((p) => p.id)
      return {
        fewer: keys(trimGroup(rows, true, null)), fewerSelected: keys(trimGroup(rows, true, 'e')), all: keys(trimGroup(rows, false, null)),
        toggle: [canToggleSenderChat('news@example.com', 'inbox'), canToggleSenderChat('', 'inbox'), canToggleSenderChat('', 'sent')],
        // Local dates: Wednesday 09:00, Wednesday 22:00, Saturday 09:00, Sunday 09:00.
        wedMorning: ids(new Date(2026, 9, 14, 9)), wedNight: ids(new Date(2026, 9, 14, 22)),
        saturday: ids(new Date(2026, 9, 17, 9)), sunday: ids(new Date(2026, 9, 18, 9)),
        tomorrow: snoozePresets(new Date(2026, 9, 14, 9)).find((p) => p.id === 'tomorrow').until.getHours(),
      }
    }, formatModule)
    assert.equal(r.fewer, 'abc', 'Show fewer keeps the first three rows')
    assert.equal(r.fewerSelected, 'abce', 'the selected chat stays visible')
    assert.equal(r.all, 'abcde')
    assert.deepEqual(r.toggle, [true, true, false], 'sender chats split; Sent rows cannot combine')
    assert.deepEqual(r.wedMorning, ['laterToday', 'tomorrow', 'weekend', 'nextWeek'])
    assert.deepEqual(r.wedNight, ['tomorrow', 'weekend', 'nextWeek'], 'no "Later today" once it would land tomorrow')
    assert.deepEqual(r.saturday, ['laterToday', 'tomorrow', 'nextWeek'], 'no "This weekend" at the weekend')
    assert.deepEqual(r.sunday, ['laterToday', 'tomorrow'], 'no "Next week" on Sunday')
    assert.equal(r.tomorrow, 8)
  }))

  it('row status glyph priority', ui.run(opts, async (page) => {
    const r = await page.evaluate(async () => {
      const { rowStatus } = await import('/src/lib/components/chat/chatStatus.ts')
      const later = new Date(Date.now() + 36e5)
      const st = (unreadCount, snoozedUntil, sending) => rowStatus({ unreadCount, snoozedUntil }, sending)
      return [st(2, later, true), st(2, later, false), st(2, null, false), st(0, later, false), st(0, null, false), st(0, null, true)]
    })
    assert.deepEqual(r, ['sending', 'snoozed', 'unread', 'snoozed', null, 'sending'],
      'sending outranks snoozed, which outranks unread; a read, unsnoozed chat has no glyph')
  }))
})

describe('account form logic', () => {
  it('new accounts default to a one-month sync period', ui.run(opts, async (page) => {
    const days = await page.evaluate(async () => {
      const { AccountFormState } = await import('/src/lib/components/settings/account/accountFormState.svelte.ts')
      return new AccountFormState({ editAccount: null }).syncPeriodDays
    })
    assert.equal(days, '30', 'the sync period starts at 1 month')
  }))
})
