// Pure helpers for the chat thread view: participants, grouping, day labels,
// recipient parsing, and URL linkification (rendered as text segments, never HTML).

import { get } from 'svelte/store'
import { format, isThisYear, isToday, isYesterday } from 'date-fns'
import { _ } from '$lib/i18n'
import type { message as messageModels } from '../../../../wailsjs/go/models'
import type { AccountWithFolders } from '$lib/stores/accounts.svelte'

export type ReplyMode = 'reply' | 'reply-all' | 'forward'

export interface Person {
  name: string
  email: string
}

// Consecutive messages from one sender within this window share a group.
const GROUP_WINDOW_MS = 5 * 60 * 1000

export function parseRecipients(raw: string | undefined): Person[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    // Rows synced before group syntax was dropped (e.g. "undisclosed-recipients:;")
    // can hold entries with neither a name nor an address.
    return parsed
      .map((r: { name?: string; email?: string }) => ({ name: r.name || '', email: r.email || '' }))
      .filter((p) => p.name || p.email)
  } catch {
    return []
  }
}

export function displayName(p: Person): string {
  return p.name || p.email.split('@')[0] || p.email
}

// chatPeople lists a chat's participants once each, latest sender first,
// without my own addresses. When I sent every message (Sent folder rows) it
// falls back to the recipients, and to me only for a note to self. allMine
// says every sender is me even when the address is an unknown alias.
export function chatPeople(participants: Person[], myEmails: Set<string>, recipients: Person[] = [], allMine = false): Person[] {
  const seen = new Set<string>()
  const all: Person[] = []
  for (const p of participants) {
    const key = p.email.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    all.push(p)
  }
  const notMe = (p: Person) => !myEmails.has(p.email.toLowerCase())
  const others = allMine ? [] : all.filter(notMe)
  if (others.length > 0) return others
  const to = recipients.filter(notMe)
  return to.length > 0 ? to : all
}

// chatSender names a sender chat's sender from the people in it.
export function chatSender(people: Person[], email: string): Person {
  return people.find((p) => p.email.toLowerCase() === email) ?? { name: '', email }
}

// rowPeople names a chat row's people: a sender chat just its sender, a
// Sent row its recipients.
export function rowPeople(chat: { participants: Person[]; recipients: Person[]; senderEmail: string }, myEmails: Set<string>, sent: boolean): Person[] {
  return chat.senderEmail ? [chatSender(chat.participants, chat.senderEmail)] : chatPeople(chat.participants, myEmails, chat.recipients, sent)
}

// canToggleSenderChat: a sender chat can split; a Sent row's people are
// recipients, so it has no sender to combine.
export function canToggleSenderChat(senderEmail: string, folderType: string | null | undefined): boolean {
  return !!senderEmail || folderType !== 'sent'
}

// myAddresses lists every account address, lowercased, to leave me out of
// participant and recipient lists. Whether a message is mine comes from the
// backend's msg.mine flag.
export function myAddresses(accounts: AccountWithFolders[]): Set<string> {
  const emails = new Set<string>()
  for (const a of accounts) {
    if (a.account.email) emails.add(a.account.email.toLowerCase())
  }
  return emails
}

// People in the thread other than me, in first-appearance order. Falls back
// to recipients when every message is mine (e.g. a Sent-only thread).
export function threadPeople(messages: messageModels.Message[], myEmails: Set<string>): Person[] {
  const seen = new Set<string>()
  const people: Person[] = []
  const add = (p: Person) => {
    const key = p.email.toLowerCase()
    if (!key || seen.has(key) || myEmails.has(key)) return
    seen.add(key)
    people.push(p)
  }
  for (const m of messages) {
    if (!m.mine) add({ name: m.fromName || '', email: m.fromEmail || '' })
  }
  if (people.length === 0) {
    for (const m of messages) {
      parseRecipients(m.toList).forEach(add)
      parseRecipients(m.ccList).forEach(add)
    }
  }
  return people
}

export interface ReplyTarget {
  messageId: string
  defaultReplyAll: boolean
}

// replyTarget picks the message a chat reply answers: the latest message
// from someone else, else my latest one (replying to its recipients). Reply
// all is the default when the answered message involves more than one other
// person. Drafts never count.
export function replyTarget(messages: messageModels.Message[], myEmails: Set<string>): ReplyTarget | null {
  let theirs: messageModels.Message | null = null
  let last: messageModels.Message | null = null
  for (const m of messages) {
    if (m.isDraft) continue
    last = m
    if (!m.mine) theirs = m
  }
  if (!theirs) return last ? { messageId: last.id, defaultReplyAll: true } : null
  const others = new Set<string>()
  for (const p of [{ name: '', email: theirs.fromEmail || '' }, ...parseRecipients(theirs.toList), ...parseRecipients(theirs.ccList)]) {
    const email = p.email.toLowerCase()
    if (email && !myEmails.has(email)) others.add(email)
  }
  return { messageId: theirs.id, defaultReplyAll: others.size > 1 }
}

export interface ReplyRecipients {
  reply: Person[]
  all: Person[]
}

// replyRecipients lists who a reply and a reply all to the message reach,
// mirroring PrepareReply in app/compose.go: Reply-To or the sender, plus the
// To and Cc lists for reply all, without selfEmails (the account's identity
// addresses, lowercased). Either falls back to the sender when nobody else is
// left, or to nobody when the sender has no address. A bare address picks up
// the name the thread uses for it elsewhere.
export function replyRecipients(messages: messageModels.Message[], messageId: string, selfEmails: Set<string>): ReplyRecipients | null {
  const m = messages.find((x) => x.id === messageId)
  if (!m) return null
  const fromEmail = (m.fromEmail || '').trim()
  const replyTo = (m.replyTo || '').trim()
  const sender: Person[] =
    replyTo && replyTo.toLowerCase() !== fromEmail.toLowerCase()
      ? [{ name: '', email: replyTo }]
      : [{ name: m.fromName || '', email: replyTo || fromEmail }]
  const others = (list: Person[]) => {
    const seen = new Set<string>()
    return list.filter((p) => {
      const key = p.email.toLowerCase()
      if (!key || selfEmails.has(key) || seen.has(key)) return false
      seen.add(key)
      return true
    })
  }
  let reply = others(sender)
  let all = others([...sender, ...parseRecipients(m.toList), ...parseRecipients(m.ccList)])
  const fallback = sender.filter((p) => p.email)
  if (reply.length === 0) reply = fallback
  if (all.length === 0) all = fallback
  const names = threadNames(messages, new Set([...reply, ...all].filter((p) => !p.name).map((p) => p.email.toLowerCase())))
  const named = (list: Person[]) => list.map((p) => (p.name ? p : { ...p, name: names.get(p.email.toLowerCase()) ?? '' }))
  return { reply: named(reply), all: named(all) }
}

// threadNames finds the names the thread gives the wanted addresses, stopping
// once all are found.
function threadNames(messages: messageModels.Message[], wanted: Set<string>): Map<string, string> {
  const names = new Map<string, string>()
  for (const m of messages) {
    if (names.size === wanted.size) break
    for (const p of [{ name: m.fromName || '', email: m.fromEmail || '' }, ...parseRecipients(m.toList), ...parseRecipients(m.ccList)]) {
      const key = p.email.toLowerCase()
      if (p.name && wanted.has(key) && !names.has(key)) names.set(key, p.name)
    }
  }
  return names
}

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

export function formatDay(d: Date): string {
  const t = get(_)
  if (isToday(d)) return t('chat.today')
  if (isYesterday(d)) return t('date.yesterday')
  return format(d, isThisYear(d) ? 'EEE d MMM' : 'EEE d MMM yyyy')
}

export interface ThreadItem {
  msg: messageModels.Message
  date: Date
  mine: boolean
  // First message of a new day: render a separator before it.
  newDay: boolean
  // First message of a same-sender run: show avatar and name.
  groupStart: boolean
  // Set where the subject changes after the first (sender chats only):
  // label the bubble.
  subject?: string
}

const SUBJECT_PREFIX_RE = /^\s*((re|fwd?|aw|wg|sv|vs)(\[\d+\])?\s*:\s*)+/i

// baseSubject drops reply and forward prefixes, for comparing subjects.
function baseSubject(subject: string): string {
  return subject.replace(SUBJECT_PREFIX_RE, '').trim().toLowerCase()
}

// buildThreadItems groups messages into bubble runs. labelSubjects (a
// sender chat, which mixes threads) labels each change from the previous
// non-empty subject and starts a new run there.
export function buildThreadItems(messages: messageModels.Message[], labelSubjects = false): ThreadItem[] {
  const items: ThreadItem[] = []
  let prev: ThreadItem | null = null
  let prevBase: string | null = null
  for (const msg of messages) {
    // The first subject and empty subjects get no label.
    const base = labelSubjects ? baseSubject(msg.subject || '') : ''
    const subject = base && prevBase !== null && base !== prevBase ? msg.subject : undefined
    if (base) prevBase = base
    const date = new Date(msg.date)
    const mine = !!msg.mine
    const newDay = !prev || dayKey(prev.date) !== dayKey(date)
    const sameSender = !!prev && prev.mine === mine && (prev.msg.fromEmail || '').toLowerCase() === (msg.fromEmail || '').toLowerCase()
    const groupStart = newDay || !sameSender || subject !== undefined || date.getTime() - prev!.date.getTime() > GROUP_WINDOW_MS
    const item: ThreadItem = { msg, date, mine, newDay, groupStart, subject }
    items.push(item)
    prev = item
  }
  return items
}

export interface TextSegment {
  text: string
  url?: string
}

const URL_RE = /\b(?:https?:\/\/|mailto:|www\.)[^\s<>"']+/gi
const TRAILING_PUNCT_RE = /[.,;:!?)\]}'"]+$/

// linkify splits text into plain and link segments. Trailing punctuation
// stays outside the link; a closing ")" is kept when the URL opened one.
export function linkify(text: string): TextSegment[] {
  const out: TextSegment[] = []
  let last = 0
  for (const m of text.matchAll(URL_RE)) {
    let raw = m[0]
    const trail = raw.match(TRAILING_PUNCT_RE)?.[0] ?? ''
    raw = raw.slice(0, raw.length - trail.length)
    let keep = ''
    if (trail.startsWith(')') && raw.includes('(')) keep = ')'
    raw += keep
    const start = m.index!
    if (start > last) out.push({ text: text.slice(last, start) })
    const url = raw.toLowerCase().startsWith('www.') ? `https://${raw}` : raw
    out.push({ text: raw, url })
    last = start + raw.length
  }
  if (last < text.length) out.push({ text: text.slice(last) })
  return out
}
