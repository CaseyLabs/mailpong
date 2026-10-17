// ChatThread — data and side effects for the open chat: loading the thread,
// debounced refresh on sync events, auto mark-as-read, on-demand body fetch,
// S/MIME/PGP processing, and read receipts. Ported from ConversationViewer
// (which stays untouched for upstream merges); ChatView owns the rendering.

import {
  GetConversation, GetSenderChat, GetReadReceiptResponsePolicy, SendReadReceipt, IgnoreReadReceipt, GetMarkAsReadDelay,
  ProcessSMIMEMessage, ProcessPGPMessage, FetchMessageBody, MarkAsRead,
} from '../../../../wailsjs/go/app/App'
// @ts-ignore - wailsjs runtime
import { EventsOn } from '../../../../wailsjs/runtime/runtime'
import { message as messageModels } from '../../../../wailsjs/go/models'
import { isDialogGuardActive } from '$lib/stores/dialogGuard'
import { senderEmailOf } from '$lib/stores/chat.svelte'
import { toasts } from '$lib/stores/toast'
import { _ } from '$lib/i18n'
import { get } from 'svelte/store'
import { nearBottom } from './chatScroll.svelte'

export interface DecryptedAttachment {
  filename: string
  contentType: string
  size: number
  isInline: boolean
  contentId: string
}

export interface SMIMEViewResult {
  bodyHtml: string
  bodyText: string
  smimeStatus: string
  smimeSignerEmail: string
  smimeSignerSubject: string
  smimeEncrypted: boolean
  chat?: messageModels.ChatText
  inlineAttachments?: Record<string, string>
  attachments?: DecryptedAttachment[]
}

export interface PGPViewResult {
  bodyHtml: string
  bodyText: string
  pgpStatus: string
  pgpSignerEmail: string
  pgpSignerKeyId: string
  pgpEncrypted: boolean
  chat?: messageModels.ChatText
  inlineAttachments?: Record<string, string>
  attachments?: DecryptedAttachment[]
}

export type ReadReceiptPolicy = 'never' | 'ask' | 'always'

interface Hooks {
  // All messages left the thread (moved/deleted/emptied).
  onGone: (autoSelectNext: boolean) => void
  // A full load finished; the view scrolls to the first unread message.
  onLoaded: () => void
  scroller: () => HTMLElement | null
}

const REFRESH_DELAY_MS = 300

function t(key: string): string {
  return get(_)(key)
}

export class ChatThread {
  conversation = $state<messageModels.Conversation | null>(null)
  loading = $state(false)
  error = $state<string | null>(null)
  smimeResults = $state<Record<string, SMIMEViewResult>>({})
  smimeLoading = $state<Set<string>>(new Set())
  pgpResults = $state<Record<string, PGPViewResult>>({})
  pgpLoading = $state<Set<string>>(new Set())
  // Messages whose body download failed; their bubble offers Retry.
  bodyErrors = $state<Set<string>>(new Set())
  readReceiptPolicy = $state<ReadReceiptPolicy>('ask')
  handledReadReceipts = $state<Set<string>>(new Set())
  sendingReadReceipt = $state<Set<string>>(new Set())
  // Message IDs whose unread state was set by this view; their
  // messages:readChanged echo is applied locally instead of reloading.
  pendingReadIds = new Set<string>()

  readonly messages = $derived(this.conversation?.messages ?? [])

  private threadId: string | null = null
  private folderId: string | null = null
  private accountId: string | null = null
  private markAsReadDelay = 1000
  private markAsReadTimer: ReturnType<typeof setTimeout> | null = null
  private refreshTimer: ReturnType<typeof setTimeout> | null = null
  private refreshDeferred = false
  private guardInterval: ReturnType<typeof setInterval> | null = null
  private loadSeq = 0
  private cleanups: (() => void)[] = []

  constructor(private hooks: Hooks) {}

  // start loads settings and subscribes to backend events; returns cleanup.
  start(): () => void {
    Promise.all([GetReadReceiptResponsePolicy(), GetMarkAsReadDelay()])
      .then(([policy, delay]) => {
        this.readReceiptPolicy = policy as ReadReceiptPolicy
        this.markAsReadDelay = delay
      })
      .catch((err) => console.error('Failed to load settings:', err))

    const sameFolder = (d: { accountId: string; folderId: string }) =>
      !!this.threadId && d.accountId === this.accountId && d.folderId === this.folderId

    this.cleanups.push(
      EventsOn('messages:readChanged', (d: { messageIds: string[]; isRead: boolean }) => this.onReadChanged(d)),
      EventsOn('messages:moved', (d: { messageIds: string[] }) => this.onRemoved(d.messageIds)),
      EventsOn('messages:deleted', (ids: string[]) => this.onRemoved(ids)),
      EventsOn('undo:completed', () => this.reload()),
      EventsOn('messages:updated', (d: { accountId: string; folderId: string }) => {
        if (sameFolder(d)) this.scheduleRefresh()
      }),
      EventsOn('folder:synced', (d: { accountId: string; folderId: string }) => {
        if (!sameFolder(d)) return
        if (!this.conversation || this.error) {
          this.reload()
          return
        }
        this.scheduleRefresh()
      }),
      EventsOn('sent:synced', (d: { accountId: string }) => {
        if (this.threadId && d.accountId === this.accountId) this.scheduleRefresh()
      }),
    )

    // Flush refreshes deferred while a dialog (e.g. folder picker) was open.
    this.guardInterval = setInterval(() => {
      if (this.refreshDeferred && !isDialogGuardActive()) {
        this.refreshDeferred = false
        this.scheduleRefresh()
      }
    }, 500)

    return () => {
      this.clearMarkAsRead()
      if (this.refreshTimer) clearTimeout(this.refreshTimer)
      if (this.guardInterval) clearInterval(this.guardInterval)
      this.cleanups.forEach((fn) => fn())
      this.cleanups = []
    }
  }

  // open switches to a thread (or clears with null ids).
  open(threadId: string | null, folderId: string | null, accountId: string | null) {
    if (this.refreshTimer) {
      clearTimeout(this.refreshTimer)
      this.refreshTimer = null
    }
    if (threadId !== this.threadId || folderId !== this.folderId) {
      // Drop the old thread so its messages can't be acted on while the new one loads.
      this.loadSeq++
      this.conversation = null
      this.smimeResults = {}
      this.smimeLoading = new Set()
      this.pgpResults = {}
      this.pgpLoading = new Set()
      this.bodyErrors = new Set()
    }
    this.threadId = threadId
    this.folderId = folderId
    this.accountId = accountId
    if (threadId && folderId) {
      void this.load()
      return
    }
    this.clearMarkAsRead()
    this.loadSeq++
    this.conversation = null
  }

  reload() {
    if (this.threadId && this.folderId) void this.load()
  }

  // refreshFlags re-fetches to pick up star/read changes made elsewhere.
  async refreshFlags() {
    if (!this.threadId || !this.folderId) return
    try {
      this.conversation = await this.fetch(this.threadId, this.folderId)
    } catch {
      // best-effort
    }
  }

  // load fetches the thread; markRead=false skips auto mark-as-read, so a
  // reload after "Mark as unread" doesn't undo it.
  async load({ markRead = true } = {}) {
    const tid = this.threadId!
    const seq = ++this.loadSeq
    this.clearMarkAsRead()
    this.loading = true
    this.error = null
    try {
      const result = await this.fetch(tid, this.folderId!)
      if (seq !== this.loadSeq) return
      this.conversation = result
      this.afterMessagesChanged(tid, true, markRead)
    } catch (err) {
      if (seq !== this.loadSeq) return
      console.error('Failed to load conversation:', err)
      this.error = t('viewer.failedToLoad')
    } finally {
      if (seq === this.loadSeq) {
        this.loading = false
        this.hooks.onLoaded()
      }
    }
  }

  // fetch loads a thread, or a sender chat (threadId sender:<email>) with
  // the sender's threads in the folder.
  private fetch(threadId: string, folderId: string) {
    const sender = senderEmailOf(threadId)
    return sender ? GetSenderChat(this.accountId ?? '', sender, folderId) : GetConversation(threadId, folderId)
  }

  private afterMessagesChanged(tid: string, fetchBodies: boolean, markRead = true) {
    const msgs = this.conversation?.messages
    if (!msgs) return
    if (markRead) this.scheduleMarkAsRead(tid, msgs)
    this.processSMIME(msgs)
    this.processPGP(msgs)
    if (fetchBodies) void this.fetchUnfetchedBodies(msgs)
  }

  // Debounced refresh coalesces rapid sync events; deferred while a dialog is open.
  private scheduleRefresh() {
    if (isDialogGuardActive()) {
      this.refreshDeferred = true
      return
    }
    if (this.refreshTimer) clearTimeout(this.refreshTimer)
    this.refreshTimer = setTimeout(() => {
      this.refreshTimer = null
      void this.refresh()
    }, REFRESH_DELAY_MS)
  }

  // refresh only replaces the thread when its messages changed, keeping scroll.
  private async refresh() {
    const tid = this.threadId
    if (!tid || !this.folderId) return
    try {
      const updated = await this.fetch(tid, this.folderId)
      if (this.threadId !== tid) return
      if (!updated?.messages?.length) {
        this.dismiss()
        return
      }
      const current = this.conversation?.messages
      if (!current) return
      if (updated.messages.length === current.length && updated.messages.at(-1)?.id === current.at(-1)?.id) return

      const scroller = this.hooks.scroller()
      const atBottom = !!scroller && nearBottom(scroller)
      const scrollTop = scroller?.scrollTop ?? 0
      this.conversation = updated
      this.afterMessagesChanged(tid, true)
      // New messages arrive at the bottom: follow them if the user was there.
      requestAnimationFrame(() => {
        if (!scroller) return
        scroller.scrollTop = atBottom ? scroller.scrollHeight : scrollTop
      })
    } catch (err) {
      console.error('Failed to refresh conversation:', err)
    }
  }

  dismiss() {
    this.clearMarkAsRead()
    this.conversation = null
    this.hooks.onGone(true)
  }

  private onReadChanged(d: { messageIds: string[]; isRead: boolean }) {
    const conv = this.conversation
    if (d.messageIds.length > 0 && d.messageIds.every((id) => this.pendingReadIds.has(id))) {
      d.messageIds.forEach((id) => this.pendingReadIds.delete(id))
      if (!d.isRead) this.clearMarkAsRead()
      if (!conv?.messages) return
      for (const m of conv.messages) {
        if (d.messageIds.includes(m.id)) m.isRead = d.isRead
      }
      const delta = d.isRead ? -d.messageIds.length : d.messageIds.length
      conv.unreadCount = Math.max(0, (conv.unreadCount || 0) + delta)
      this.conversation = { ...conv } as messageModels.Conversation
      return
    }
    if (!conv?.messages?.some((m) => d.messageIds.includes(m.id))) return
    if (!d.isRead) {
      // Marked unread elsewhere: close so auto mark-as-read doesn't undo it.
      this.clearMarkAsRead()
      this.conversation = null
      return
    }
    this.reload()
  }

  private onRemoved(ids: string[]) {
    const msgs = this.conversation?.messages
    if (!msgs?.some((m) => ids.includes(m.id))) return
    if (msgs.every((m) => ids.includes(m.id))) {
      this.dismiss()
      return
    }
    this.reload()
  }

  private async fetchUnfetchedBodies(messages: messageModels.Message[]) {
    for (const msg of messages) {
      if (msg.bodyFetched !== false || msg.bodyHtml || msg.bodyText) continue
      // Failed bodies wait for Retry instead of refetching on every reload.
      if (this.bodyErrors.has(msg.id)) continue
      await this.fetchBody(msg.id)
    }
  }

  retryBody(id: string) {
    const errors = new Set(this.bodyErrors)
    errors.delete(id)
    this.bodyErrors = errors
    void this.fetchBody(id)
  }

  private async fetchBody(id: string) {
    const tid = this.threadId
    try {
      const updated = await FetchMessageBody(id)
      if (this.threadId !== tid) return
      const conv = this.conversation
      const idx = conv?.messages?.findIndex((m) => m.id === id) ?? -1
      if (conv?.messages && idx >= 0 && updated) {
        conv.messages[idx] = updated
        this.conversation = { ...conv } as messageModels.Conversation
      }
    } catch (err) {
      console.error('Failed to fetch body for message:', id, err)
      // A message deleted on the server leaves with the next sync's reload.
      if (this.threadId === tid) this.bodyErrors = new Set([...this.bodyErrors, id])
    }
  }

  private processSMIME(messages: messageModels.Message[]) {
    // Only new messages: refreshes keep earlier results instead of decrypting again.
    const ids = messages.filter((m) => m.hasSMIME && !this.smimeResults[m.id] && !this.smimeLoading.has(m.id)).map((m) => m.id)
    if (ids.length === 0) return
    this.smimeLoading = new Set([...this.smimeLoading, ...ids])
    const tid = this.threadId
    for (const id of ids) {
      ProcessSMIMEMessage(id)
        .then((r) => {
          if (this.threadId === tid) this.smimeResults = { ...this.smimeResults, [id]: r }
        })
        .catch((err) => console.error('Failed to process S/MIME message:', id, err))
        .finally(() => {
          const next = new Set(this.smimeLoading)
          next.delete(id)
          this.smimeLoading = next
        })
    }
  }

  private processPGP(messages: messageModels.Message[]) {
    const ids = messages.filter((m) => m.hasPGP && !this.pgpResults[m.id] && !this.pgpLoading.has(m.id)).map((m) => m.id)
    if (ids.length === 0) return
    this.pgpLoading = new Set([...this.pgpLoading, ...ids])
    const tid = this.threadId
    for (const id of ids) {
      ProcessPGPMessage(id)
        .then((r) => {
          if (this.threadId === tid) this.pgpResults = { ...this.pgpResults, [id]: r }
        })
        .catch((err) => console.error('Failed to process PGP message:', id, err))
        .finally(() => {
          const next = new Set(this.pgpLoading)
          next.delete(id)
          this.pgpLoading = next
        })
    }
  }

  private clearMarkAsRead() {
    if (this.markAsReadTimer) {
      clearTimeout(this.markAsReadTimer)
      this.markAsReadTimer = null
    }
  }

  // Opening a chat marks it read after the user's delay (-1 = manual only).
  private scheduleMarkAsRead(tid: string, messages: messageModels.Message[]) {
    this.clearMarkAsRead()
    const unreadIds = messages.filter((m) => !m.isRead).map((m) => m.id)
    if (unreadIds.length === 0 || this.markAsReadDelay < 0) return

    const mark = () => {
      unreadIds.forEach((id) => this.pendingReadIds.add(id))
      MarkAsRead(unreadIds).catch((err) => {
        console.error('Failed to mark messages as read:', err)
        unreadIds.forEach((id) => this.pendingReadIds.delete(id))
      })
    }
    if (this.markAsReadDelay === 0) {
      mark()
      return
    }
    this.markAsReadTimer = setTimeout(() => {
      this.markAsReadTimer = null
      if (this.threadId === tid) mark()
    }, this.markAsReadDelay)
  }

  shouldShowReadReceipt(msg: messageModels.Message): boolean {
    if (this.readReceiptPolicy === 'never' || !msg.readReceiptTo) return false
    return !msg.readReceiptHandled && !this.handledReadReceipts.has(msg.id)
  }

  // Auto-send for the 'always' policy once a message is shown.
  autoSendReadReceipts() {
    if (this.readReceiptPolicy !== 'always') return
    for (const m of this.messages) {
      if (this.shouldShowReadReceipt(m)) void this.sendReadReceipt(m)
    }
  }

  async sendReadReceipt(msg: messageModels.Message) {
    if (this.sendingReadReceipt.has(msg.id)) return
    this.sendingReadReceipt = new Set([...this.sendingReadReceipt, msg.id])
    try {
      await SendReadReceipt(msg.accountId, msg.id)
      this.handledReadReceipts = new Set([...this.handledReadReceipts, msg.id])
      toasts.success(t('viewer.readReceiptSent'))
    } catch (err) {
      console.error('Failed to send read receipt:', err)
      toasts.error(String(err))
    } finally {
      const next = new Set(this.sendingReadReceipt)
      next.delete(msg.id)
      this.sendingReadReceipt = next
    }
  }

  async ignoreReadReceipt(msg: messageModels.Message) {
    try {
      await IgnoreReadReceipt(msg.accountId, msg.id)
      this.handledReadReceipts = new Set([...this.handledReadReceipts, msg.id])
    } catch (err) {
      console.error('Failed to ignore read receipt:', err)
    }
  }
}
