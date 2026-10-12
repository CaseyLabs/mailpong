// ChatComposer — state for the docked reply box: text, reply-all choice and
// attachments per chat, debounced draft autosave (restored when the chat is
// reopened), and sends shown as pending bubbles until the thread has them.
// Sends wait out the backend's undo delay; Undo puts the reply back in the box.

// @ts-ignore - wailsjs path
import { SaveChatDraft, GetChatDraft, QueueChatReply, ReleaseChatDraft, PickAttachmentFiles, DeleteDraft } from '../../../../wailsjs/go/app/App'
import { UNDO_SEND_MS, undoSend, whenSent } from '$lib/stores/sendQueue'
import { app, smtp } from '../../../../wailsjs/go/models'
import type { Person, ReplyRecipients, ReplyTarget } from './chatFormat'

const SAVE_DELAY_MS = 600
// A sent bubble that never shows up in the thread (Sent sync failed) goes away after this.
const SENT_BUBBLE_MAX_MS = 5 * 60 * 1000

export interface PendingSend {
  id: number
  key: string
  text: string
  attachmentNames: string[]
  status: 'queued' | 'sent' | 'failed'
  error: string
  // While queued: when the send goes out. Once sent: when it went.
  sentAt: number
  // Backend send ID while queued, for undo.
  sendId: string
  reply: app.ChatReply
}

interface Snapshot {
  key: string
  accountId: string
  threadKey: string
  messageId: string
  text: string
  replyAll: boolean
  attachments: smtp.Attachment[]
}

interface Options {
  // Latest reply target of the open chat; null while the thread loads.
  target: () => ReplyTarget | null
  // Who the target's reply and reply all reach; null while unknown.
  recipients: () => ReplyRecipients | null
}

let nextPendingId = 1

// Mounted composers, flushed together at shutdown.
const live = new Set<ChatComposer>()

export async function flushChatComposers(): Promise<void> {
  await Promise.allSettled([...live].map((c) => c.flush()))
}

export class ChatComposer {
  text = $state('')
  // null = follow the target's default
  replyAllChoice = $state<boolean | null>(null)
  attachments = $state<smtp.Attachment[]>([])
  draftId = $state('')
  // Pending sends of every chat, so switching chats keeps them.
  pending = $state<PendingSend[]>([])

  #opts: Options
  #accountId = ''
  #threadKey = ''
  // Message the box's text answers, captured while editing this chat.
  #messageId = ''
  #dirty = false
  #timer: ReturnType<typeof setTimeout> | null = null
  // Saves run one at a time so each sees the draft ID of the previous one.
  #saving: Promise<void> = Promise.resolve()
  #draftIds = new Map<string, string>()
  // Drafts of failed sends merged into the box, deleted once a save holds
  // their text.
  #orphans = new Map<string, string[]>()
  #openGen = 0

  constructor(opts: Options) {
    this.#opts = opts
    live.add(this)
  }

  dispose() {
    void this.flush()
    live.delete(this)
  }

  get key(): string {
    return this.#accountId && this.#threadKey ? `${this.#accountId}|${this.#threadKey}` : ''
  }

  get replyAll(): boolean {
    return this.replyAllChoice ?? this.#opts.target()?.defaultReplyAll ?? false
  }

  // Who reply and reply all reach; null while unknown.
  get choices(): ReplyRecipients | null {
    return this.#opts.recipients()
  }

  // Who a send would reach in the current reply mode.
  get recipients(): Person[] {
    const r = this.choices
    if (!r) return []
    return this.replyAll ? r.all : r.reply
  }

  get isEmpty(): boolean {
    return this.text.trim() === '' && this.attachments.length === 0
  }

  get pendingHere(): PendingSend[] {
    const key = this.key
    return this.pending.filter((p) => p.key === key)
  }

  // open switches to a chat: saves the previous chat's text and restores
  // this one's draft.
  async open(accountId: string | null, threadKey: string | null) {
    const next = accountId && threadKey ? `${accountId}|${threadKey}` : ''
    if (next === this.key) return
    void this.flush()
    const gen = ++this.#openGen
    this.#accountId = accountId ?? ''
    this.#threadKey = threadKey ?? ''
    this.#reset()
    if (!next) return
    await this.#saving
    try {
      const d = await GetChatDraft(this.#accountId, this.#threadKey)
      if (gen !== this.#openGen || !d) return
      // Typing started before the draft loaded: keep both, in one draft.
      this.text = this.#dirty && this.text ? `${d.text}\n${this.text}` : d.text
      this.replyAllChoice ??= d.replyAll
      this.attachments = [...(d.attachments ?? []), ...this.attachments]
      this.draftId = d.draftId
      this.#messageId = d.messageId
      this.#draftIds.set(next, d.draftId)
      // A failed send whose draft is back in the box no longer needs its bubble.
      this.pending = this.pending.filter((p) => !(p.status === 'failed' && p.reply.draftId === d.draftId))
    } catch (err) {
      console.error('Failed to load chat draft:', err)
    }
  }

  setText(text: string) {
    this.text = text
    this.#changed()
  }

  toggleReplyAll() {
    this.replyAllChoice = !this.replyAll
    this.#changed()
  }

  async attach() {
    try {
      const files: app.ComposerAttachment[] = (await PickAttachmentFiles()) ?? []
      if (files.length === 0) return
      this.attachments = [
        ...this.attachments,
        ...files.map((f) => new smtp.Attachment({ filename: f.filename, content_type: f.contentType, content_base64: f.data, inline: false })),
      ]
      this.#changed()
    } catch (err) {
      console.error('Failed to attach files:', err)
      throw err
    }
  }

  removeAttachment(index: number) {
    this.attachments = this.attachments.filter((_, i) => i !== index)
    this.#changed()
  }

  // flush saves unsaved text now; resolves when every queued save is done.
  flush(): Promise<void> {
    this.#clearTimer()
    if (!this.#dirty) return this.#saving
    const snap = this.#snapshot(this.#messageId)
    if (!snap) return this.#saving // thread not loaded yet: stays dirty
    this.#dirty = false
    const orphans = this.#orphans.get(snap.key) ?? []
    this.#orphans.delete(snap.key)
    this.#saving = this.#saving.then(() => this.#save(snap, orphans))
    return this.#saving
  }

  // send sends the box's content as a reply.
  async send() {
    const snap = this.#snapshot(this.#opts.target()?.messageId)
    if (!snap || this.isEmpty) return
    // Clear the box now; a save already running still lands before the send.
    this.#reset()
    await this.#saving
    const reply = new app.ChatReply({
      accountId: snap.accountId,
      threadKey: snap.threadKey,
      messageId: snap.messageId,
      text: snap.text,
      replyAll: snap.replyAll,
      attachments: snap.attachments,
      draftId: this.#draftIds.get(snap.key) ?? '',
    })
    this.#draftIds.delete(snap.key)
    if (snap.key === this.key) this.draftId = ''
    const p: PendingSend = {
      id: nextPendingId++,
      key: snap.key,
      text: snap.text.trim(),
      attachmentNames: snap.attachments.map((a) => a.filename),
      status: 'queued',
      error: '',
      sentAt: Date.now(),
      sendId: '',
      reply,
    }
    this.pending = [...this.pending, p]
    await this.#deliver(p.id)
  }

  retry(id: number) {
    void this.#deliver(id)
  }

  // undo stops a queued send and puts it back in the box; it does nothing
  // once the send has started.
  async undo(id: number) {
    const p = this.pending.find((x) => x.id === id)
    if (!p?.sendId || p.status !== 'queued' || p.key !== this.key) return
    if (!(await undoSend(p.sendId))) return
    // Switched chats while cancelling: keep the text as a failed bubble there.
    if (p.key !== this.key) {
      this.pending = this.pending.map((x) => (x.id === id ? { ...x, status: 'failed' } : x))
      return
    }
    this.edit(id)
  }

  // edit moves a failed or undone send back into the box.
  edit(id: number) {
    const p = this.pending.find((x) => x.id === id)
    if (!p || p.key !== this.key) return
    this.pending = this.pending.filter((x) => x.id !== id)
    this.text = this.isEmpty ? p.reply.text : `${this.text}\n${p.reply.text}`
    this.attachments = [...this.attachments, ...(p.reply.attachments ?? [])]
    this.replyAllChoice = p.reply.replyAll
    const own = this.#draftIds.get(p.key)
    if (p.reply.draftId && !own) {
      this.#draftIds.set(p.key, p.reply.draftId)
      this.draftId = p.reply.draftId
    }
    this.#changed()
    // The box keeps its own draft: once that holds the reply's text too, the
    // reply's draft would only linger in Drafts.
    if (own && p.reply.draftId && p.reply.draftId !== own) {
      this.#orphans.set(p.key, [...(this.#orphans.get(p.key) ?? []), p.reply.draftId])
      void this.flush()
    }
  }

  // prune drops sent bubbles once the thread holds a message of mine at least
  // as new as the send.
  prune(latestMineAt: number) {
    const now = Date.now()
    const keep = this.pending.filter(
      (p) => !(p.status === 'sent' && p.key === this.key && (latestMineAt >= p.sentAt - 60_000 || now - p.sentAt > SENT_BUBBLE_MAX_MS)),
    )
    if (keep.length !== this.pending.length) this.pending = keep
  }

  // release saves the text and hands its draft to the full composer,
  // emptying the box. Returns the draft ID, or "" when there was nothing.
  async release(): Promise<string> {
    const key = this.key
    await this.flush()
    const id = this.#draftIds.get(key) ?? ''
    if (!id || key !== this.key) return ''
    await ReleaseChatDraft(this.#accountId, this.#threadKey)
    this.#draftIds.delete(key)
    this.#reset()
    return id
  }

  async #deliver(id: number) {
    const update = (patch: Partial<PendingSend>) => {
      this.pending = this.pending.map((x) => (x.id === id ? { ...x, ...patch } : x))
    }
    const p = this.pending.find((x) => x.id === id)
    if (!p) return
    update({ status: 'queued', error: '', sentAt: Date.now() + UNDO_SEND_MS, sendId: '' })
    let error: string
    try {
      const sendId: string = await QueueChatReply(p.reply)
      update({ sendId })
      const r = await whenSent(sendId)
      error = r.error
      // The backend saves an unsent reply without a draft as one.
      if (r.draftId) p.reply.draftId = r.draftId
    } catch (err) {
      error = String(err)
    }
    if (error) {
      console.error('Chat reply failed:', error)
      update({ status: 'failed', error, sendId: '' })
      return
    }
    update({ status: 'sent', sentAt: Date.now(), sendId: '' })
  }

  async #save(snap: Snapshot, orphans: string[]) {
    try {
      const id = await SaveChatDraft(
        new app.ChatReply({ ...snap, draftId: this.#draftIds.get(snap.key) ?? '' }),
      )
      if (id) this.#draftIds.set(snap.key, id)
      else this.#draftIds.delete(snap.key)
      if (snap.key === this.key) this.draftId = id
      for (const o of orphans) {
        DeleteDraft(o).catch((err: unknown) => console.error('Failed to delete merged chat draft:', err))
      }
    } catch (err) {
      console.error('Failed to save chat draft:', err)
      if (snap.key === this.key) this.#dirty = true
      if (orphans.length) this.#orphans.set(snap.key, [...orphans, ...(this.#orphans.get(snap.key) ?? [])])
    }
  }

  #snapshot(messageId: string | undefined): Snapshot | null {
    messageId ||= this.#opts.target()?.messageId
    if (!this.key || !messageId) return null
    return {
      key: this.key,
      accountId: this.#accountId,
      threadKey: this.#threadKey,
      messageId,
      text: this.text,
      replyAll: this.replyAll,
      attachments: this.attachments,
    }
  }

  #changed() {
    this.#messageId = this.#opts.target()?.messageId || this.#messageId
    this.#dirty = true
    this.#clearTimer()
    this.#timer = setTimeout(() => void this.flush(), SAVE_DELAY_MS)
  }

  #clearTimer() {
    if (this.#timer) clearTimeout(this.#timer)
    this.#timer = null
  }

  #reset() {
    this.#clearTimer()
    this.#dirty = false
    this.#messageId = ''
    this.text = ''
    this.replyAllChoice = null
    this.attachments = []
    this.draftId = this.#draftIds.get(this.key) ?? ''
  }
}
