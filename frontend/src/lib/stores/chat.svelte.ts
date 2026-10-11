// Chat list state: one paged stream of chats (or search results) for the
// current scope and filter, refreshed on sync and chat-state events.
// Scope (folder selection) stays in App; ChatList passes it in via setScope.

import { GetChats, GetChatCount, SearchChats, SearchChatsUnifiedInbox, GetSearchCount, GetSearchCountUnifiedInbox } from '../../../wailsjs/go/app/App'
import { message } from '../../../wailsjs/go/models'
// @ts-ignore - wailsjs runtime
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { accountStore } from './accounts.svelte'
import { isDialogGuardActive } from './dialogGuard'

export type ChatFilter = 'all' | 'unread' | 'low' | 'snoozed'

// Search results carry no pin/snooze/low state, so only these filters apply.
export function searchSupports(filter: ChatFilter): boolean {
  return filter === 'all' || filter === 'unread'
}

export interface ChatPerson {
  name: string
  email: string
}

// ChatItem is the row model for both chats and search results.
export interface ChatItem {
  key: string // accountId|threadId; unique across the unified inbox
  // The thread to open; a sender chat's is its chat key (sender:<email>),
  // which ChatView loads with GetSenderChat.
  threadId: string
  threadKey: string
  senderEmail: string // set only for sender chats
  accountId: string
  accountName: string
  accountColor: string
  folderId: string
  subject: string
  snippet: string
  latestDate: Date
  participants: ChatPerson[]
  recipients: ChatPerson[]
  messageIds: string[]
  messageCount: number
  unreadCount: number
  hasAttachments: boolean
  isStarred: boolean
  isEncrypted: boolean
  isPinned: boolean
  snoozedUntil: Date | null
  isLowPriority: boolean
  lastFromMe: boolean
  highlightedSubject?: string
  highlightedSnippet?: string
}

const PAGE_SIZE = 50
const SENDER_CHAT_PREFIX = 'sender:'
const RELOAD_DELAY_MS = 300

function threadKeyOf(threadId: string): string {
  return threadId.replace(/[<>]/g, '')
}

// Sync stores snippets with HTML entities (&amp;) left in. Escaping "<"
// first keeps literal "<tag>" text; DOMParser documents are inert.
let entityParser: DOMParser | undefined
function decodeEntities(text: string): string {
  if (!text.includes('&')) return text
  entityParser ??= new DOMParser()
  return entityParser.parseFromString(text.replaceAll('<', '&lt;'), 'text/html').documentElement.textContent || ''
}

function toItem(c: message.Chat | message.ChatSearchResult | message.ConversationSearchResult, fallbackAccountId: string, fallbackFolderId: string): ChatItem {
  const chat = c as Partial<message.Chat & message.ChatSearchResult>
  const search = c as Partial<message.ConversationSearchResult>
  const accountId = c.accountId || fallbackAccountId
  const senderEmail = chat.senderEmail || ''
  const threadId = senderEmail ? chat.threadKey! : c.threadId
  return {
    key: `${accountId}|${threadId}`,
    threadId,
    threadKey: chat.threadKey || threadKeyOf(c.threadId),
    senderEmail,
    accountId,
    accountName: c.accountName || '',
    accountColor: c.accountColor || '',
    folderId: c.folderId || fallbackFolderId,
    subject: c.subject || '',
    snippet: decodeEntities(c.snippet || ''),
    latestDate: new Date(c.latestDate),
    participants: (c.participants || []).map((p) => ({ name: p.name || '', email: p.email || '' })),
    recipients: (chat.recipients || []).map((p) => ({ name: p.name || '', email: p.email || '' })),
    messageIds: c.messageIds || [],
    messageCount: c.messageCount || 0,
    unreadCount: c.unreadCount || 0,
    hasAttachments: !!c.hasAttachments,
    isStarred: !!c.isStarred,
    isEncrypted: !!c.isEncrypted,
    isPinned: !!chat.isPinned,
    snoozedUntil: chat.snoozedUntil ? new Date(chat.snoozedUntil) : null,
    isLowPriority: !!chat.isLowPriority,
    lastFromMe: !!chat.lastFromMe,
    highlightedSubject: search.highlightedSubject || undefined,
    highlightedSnippet: search.highlightedSnippet || undefined,
  }
}

// senderEmailOf returns the sender of a ChatItem.threadId that is a sender
// chat key, or '' for a thread.
export function senderEmailOf(threadId: string | null | undefined): string {
  return threadId?.startsWith(SENDER_CHAT_PREFIX) ? threadId.slice(SENDER_CHAT_PREFIX.length) : ''
}

// mergeRows appends a page, folding a row whose chat is already listed into
// it: SearchChats merges a sender chat's threads only within one page. The
// folded row keeps the newer row's subject and snippet.
function mergeRows(items: ChatItem[], page: ChatItem[]): ChatItem[] {
  const byKey = new Map(items.map((c, i) => [c.key, i]))
  const out = [...items]
  for (const c of page) {
    const i = byKey.get(c.key)
    if (i === undefined) {
      byKey.set(c.key, out.length)
      out.push(c)
      continue
    }
    const o = out[i]
    const have = new Set(o.messageIds)
    const ids = c.messageIds.filter((id) => !have.has(id))
    if (ids.length === 0) continue
    const people = new Set(o.participants.map((p) => p.email.toLowerCase()))
    const newer = c.latestDate > o.latestDate ? c : o
    out[i] = {
      ...o,
      subject: newer.subject,
      snippet: newer.snippet,
      latestDate: newer.latestDate,
      highlightedSubject: newer.highlightedSubject,
      highlightedSnippet: newer.highlightedSnippet,
      participants: [...o.participants, ...c.participants.filter((p) => !people.has(p.email.toLowerCase()))],
      messageIds: [...o.messageIds, ...ids],
      messageCount: o.messageCount + ids.length,
      unreadCount: o.unreadCount + c.unreadCount,
      hasAttachments: o.hasAttachments || c.hasAttachments,
      isStarred: o.isStarred || c.isStarred,
      isEncrypted: o.isEncrypted || c.isEncrypted,
    }
  }
  return out
}

function isInboxFolder(accountId: string, folderId: string): boolean {
  return accountStore.getFolder(accountId, folderId)?.type === 'inbox'
}

class ChatListStore {
  accountId = $state<string | null>(null)
  folderId = $state<string | null>(null)
  filter = $state<ChatFilter>('all')
  query = $state('')
  items = $state<ChatItem[]>([])
  total = $state(0)
  lowCount = $state(0)
  loading = $state(false)
  error = $state<string | null>(null)
  // Settings → Chat: when off, All loads only priority chats (no Low group).
  showLowGroup = $state(true)

  // Bumped on every reload; responses from older requests are dropped.
  private generation = 0
  private reloadTimer: ReturnType<typeof setTimeout> | null = null
  // Set while a reload waits for a dialog to close; holds its keepWindow.
  private pendingKeepWindow: boolean | null = null

  readonly isUnified = $derived(this.accountId === 'unified' && this.folderId === 'inbox')
  readonly isSearch = $derived(this.query.trim().length > 0)
  // Backend rows fetched so far. A search page counts as full: SearchChats
  // can return fewer rows than it consumed, after merging sender chats.
  private fetched = $state(0)
  readonly hasMore = $derived(this.fetched < this.total)
  readonly effectiveFilter = $derived<ChatFilter>(this.isSearch && !searchSupports(this.filter) ? 'all' : this.filter)

  // GetChats scope: "" for the unified inbox, otherwise the folder ID.
  private get scopeId(): string {
    return this.isUnified ? '' : this.folderId || ''
  }

  // GetChats section for the current filter.
  private get section(): string {
    return this.filter === 'all' && !this.showLowGroup ? 'priority' : this.filter
  }

  // lowGroupChats loads every chat of the collapsed Low priority group (low,
  // not pinned) in the current scope, for its bulk actions.
  async lowGroupChats(): Promise<ChatItem[]> {
    const accountId = this.accountId || ''
    const folderId = this.folderId || ''
    const count = await GetChatCount(this.scopeId, 'low')
    if (count === 0) return []
    const chats = await GetChats(this.scopeId, 'low', 0, count)
    return (chats || []).map((c) => toItem(c, accountId, folderId)).filter((c) => !c.isPinned)
  }

  setScope(accountId: string | null, folderId: string | null) {
    if (accountId === this.accountId && folderId === this.folderId) return
    this.accountId = accountId
    this.folderId = folderId
    this.clear()
    void this.reload()
  }

  setFilter(filter: ChatFilter) {
    if (filter === this.filter) return
    this.filter = filter
    this.clear()
    void this.reload()
  }

  setShowLowGroup(show: boolean) {
    if (show === this.showLowGroup) return
    this.showLowGroup = show
    if (this.filter !== 'all') return
    this.clear()
    void this.reload()
  }

  setQuery(query: string) {
    this.query = query
    // Drop in-flight pages for the old query; loadMore waits for the reload.
    this.invalidate()
    this.scheduleReload(false)
  }

  // invalidate drops the results of in-flight requests.
  private invalidate() {
    this.generation++
    this.loading = false
  }

  private clear() {
    this.invalidate()
    this.items = []
    this.total = 0
    this.fetched = 0
    this.error = null
  }

  private async fetchPage(offset: number, limit: number): Promise<{ items: ChatItem[]; total: number }> {
    const accountId = this.accountId || ''
    const folderId = this.folderId || ''
    if (this.isSearch) {
      const q = this.query.trim()
      const mode = this.effectiveFilter === 'unread' ? 'unread' : ''
      const [results, total] = await Promise.all(this.isUnified
        ? [SearchChatsUnifiedInbox(q, offset, limit, mode), GetSearchCountUnifiedInbox(q, mode)] as const
        : [SearchChats(folderId, q, offset, limit, mode), GetSearchCount(accountId, folderId, q, mode)] as const)
      return { items: (results || []).map((r) => toItem(r, accountId, folderId)), total }
    }
    const [chats, total] = await Promise.all([GetChats(this.scopeId, this.section, offset, limit), GetChatCount(this.scopeId, this.section)])
    return { items: (chats || []).map((c) => toItem(c, accountId, folderId)), total }
  }

  // reload fetches the first page again. keepWindow keeps as many rows as
  // are loaded now, so a sync refresh doesn't collapse the scrolled list.
  async reload(keepWindow = false): Promise<void> {
    if (!this.accountId || !this.folderId) return
    const gen = ++this.generation
    const limit = keepWindow ? Math.max(this.fetched, PAGE_SIZE) : PAGE_SIZE
    this.loading = true
    this.error = null
    try {
      const wantLow = this.section === 'all' && !this.isSearch
      const [page, lowCount] = await Promise.all([
        this.fetchPage(0, limit),
        wantLow ? GetChatCount(this.scopeId, 'low') : Promise.resolve(0),
      ])
      if (gen !== this.generation) return
      this.items = mergeRows([], page.items)
      this.total = page.total
      this.fetched = this.consumed(0, limit, page)
      this.lowCount = lowCount
    } catch (err) {
      if (gen !== this.generation) return
      console.error('Failed to load chats:', err)
      this.error = String(err)
    } finally {
      if (gen === this.generation) this.loading = false
    }
  }

  async loadMore(): Promise<void> {
    if (this.loading || !this.hasMore || this.error || this.reloadTimer) return
    const gen = this.generation
    this.loading = true
    try {
      const offset = this.fetched
      const page = await this.fetchPage(offset, PAGE_SIZE)
      if (gen !== this.generation) return
      this.items = mergeRows(this.items, page.items)
      this.total = page.total
      this.fetched = this.consumed(offset, PAGE_SIZE, page)
    } catch (err) {
      if (gen !== this.generation) return
      console.error('Failed to load more chats:', err)
      this.error = String(err)
    } finally {
      if (gen === this.generation) this.loading = false
    }
  }

  // consumed is the backend offset after fetching page at offset.
  private consumed(offset: number, limit: number, page: { items: ChatItem[]; total: number }): number {
    return this.isSearch ? Math.min(offset + limit, page.total) : offset + page.items.length
  }

  // scheduleReload coalesces bursts of events into one reload, and waits
  // while a dialog (e.g. the folder picker) is open.
  scheduleReload(keepWindow = true) {
    if (this.reloadTimer) clearTimeout(this.reloadTimer)
    this.reloadTimer = setTimeout(() => {
      this.reloadTimer = null
      if (isDialogGuardActive()) {
        this.pendingKeepWindow = (this.pendingKeepWindow ?? true) && keepWindow
        return
      }
      void this.reload(keepWindow)
    }, RELOAD_DELAY_MS)
  }

  private inScope(accountId: string, folderId?: string): boolean {
    if (this.isUnified) return folderId === undefined || isInboxFolder(accountId, folderId)
    return accountId === this.accountId && (folderId === undefined || folderId === this.folderId)
  }

  private applyReadChange(messageIds: string[], isRead: boolean) {
    const changed = new Set(messageIds)
    for (const c of this.items) {
      let n = 0
      for (const id of c.messageIds) if (changed.has(id)) n++
      if (n === 0) continue
      c.unreadCount = Math.max(0, c.unreadCount + (isRead ? -n : n))
    }
    // Rows read under the Unread filter stay until the next reload, so the
    // chat being read doesn't vanish from under the selection.
  }

  // start subscribes to backend events; returns the unsubscribe function.
  start(): () => void {
    const offs = [
      EventsOn('folder:synced', (d: { accountId: string; folderId: string }) => {
        if (this.inScope(d.accountId, d.folderId)) this.scheduleReload()
      }),
      EventsOn('messages:updated', (d: { accountId: string; folderId: string }) => {
        if (this.inScope(d.accountId, d.folderId)) this.scheduleReload()
      }),
      EventsOn('chats:changed', (d: { accountId: string }) => {
        if (this.inScope(d.accountId)) this.scheduleReload()
      }),
      // A Sent sync can change which chats were last answered by me.
      EventsOn('sent:synced', (d: { accountId: string }) => {
        if (this.inScope(d.accountId)) this.scheduleReload()
      }),
      EventsOn('messages:readChanged', (d: { messageIds: string[]; isRead: boolean }) => {
        this.applyReadChange(d.messageIds || [], d.isRead)
      }),
    ]
    const guardTimer = setInterval(() => {
      if (this.pendingKeepWindow !== null && !isDialogGuardActive()) {
        const keepWindow = this.pendingKeepWindow
        this.pendingKeepWindow = null
        void this.reload(keepWindow)
      }
    }, 500)
    return () => {
      offs.forEach((off) => off())
      clearInterval(guardTimer)
      if (this.reloadTimer) clearTimeout(this.reloadTimer)
      this.reloadTimer = null
    }
  }
}

export const chatList = new ChatListStore()
