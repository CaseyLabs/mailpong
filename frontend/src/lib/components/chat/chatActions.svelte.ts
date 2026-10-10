// ChatActions — triage actions on the open chat (read state, pin, snooze,
// sender priority, spam, delete), shared by the header and App's shortcuts.

import { MarkAsSpam, MarkAsNotSpam, MarkAsRead, MarkAsUnread } from '../../../../wailsjs/go/app/App'
import { toasts } from '$lib/stores/toast'
import type { ChatItem } from '$lib/stores/chat.svelte'
import type { ChatThread } from './chatThread.svelte'
import type { ChatScroll } from './chatScroll.svelte'
import type { Person } from './chatFormat'
import { t, archiveChat, deleteMessagesPermanently, pinChat, setSenderLow, snoozeChat, trashMessages, undoAction, unsnoozeChat } from './chatTriage'

interface Deps {
  thread: ChatThread
  scroll: ChatScroll
  accountId: () => string | null
  threadKey: () => string
  messageIds: () => string[]
  chatItem: () => ChatItem | null
  people: () => Person[]
  allRead: () => boolean
  isTrash: () => boolean
  isSpam: () => boolean
  onActionComplete: (autoSelectNext?: boolean) => void
}

export class ChatActions {
  constructor(private d: Deps) {}

  private get ready() {
    return !!this.d.accountId() && !!this.d.threadKey()
  }

  private triageTarget() {
    return { accountId: this.d.accountId() ?? '', threadKey: this.d.threadKey(), messageIds: this.d.messageIds() }
  }

  afterUndo = () => {
    this.d.thread.reload()
    this.d.onActionComplete()
  }

  async archive() {
    if (await archiveChat(this.d.messageIds(), this.afterUndo)) this.d.onActionComplete(true)
  }

  // Moves to Trash (undoable), or deletes permanently in Trash. wholeThread
  // advances to the next chat; single messages reload via events.
  async deleteMessages(ids: string[], wholeThread: boolean) {
    if (!(await (this.d.isTrash() ? deleteMessagesPermanently(ids) : trashMessages(ids, this.afterUndo)))) return
    this.d.scroll.focusedId = null
    if (wholeThread) this.d.onActionComplete(true)
  }

  async toggleRead() {
    const { thread } = this.d
    const allRead = this.d.allRead()
    // Only messages that change, so every pending id gets a readChanged echo.
    const ids = thread.messages.filter((m) => !!m.isRead === allRead).map((m) => m.id)
    if (ids.length === 0) return
    // Tag as our own change so the readChanged echo updates flags in place.
    ids.forEach((id) => thread.pendingReadIds.add(id))
    try {
      await (allRead ? MarkAsUnread(ids) : MarkAsRead(ids))
      toasts.success(t(allRead ? 'toast.markedAsUnread' : 'toast.markedAsRead'))
    } catch (err) {
      console.error('Read status toggle failed:', err)
      toasts.error(t('toast.failedToUpdateReadStatus'))
      ids.forEach((id) => thread.pendingReadIds.delete(id))
    }
  }

  togglePin() {
    if (this.ready) void pinChat(this.triageTarget(), !this.d.chatItem()?.isPinned, this.afterUndo)
  }

  async snooze(until: Date) {
    if (this.ready && (await snoozeChat(this.triageTarget(), until, this.afterUndo))) this.d.onActionComplete(true)
  }

  unsnooze() {
    if (this.ready) void unsnoozeChat(this.triageTarget(), this.afterUndo)
  }

  toggleSenderLow() {
    const accountId = this.d.accountId()
    const item = this.d.chatItem()
    const sender = this.d.people()[0]
    if (accountId && item && sender) void setSenderLow(accountId, sender, !item.isLowPriority)
  }

  async spam() {
    const ids = this.d.messageIds()
    const isSpam = this.d.isSpam()
    try {
      if (isSpam) {
        await MarkAsNotSpam(ids)
        toasts.success(t('toast.markedAsNotSpam'), undoAction(this.afterUndo))
      } else {
        const moved = await MarkAsSpam(ids)
        toasts.success(t(moved ? 'toast.markedAsSpam' : 'toast.deletedFromFolder'), moved ? undoAction(this.afterUndo) : [])
      }
      this.d.onActionComplete(true)
    } catch (err) {
      console.error('Spam toggle failed:', err)
      toasts.error(t(isSpam ? 'toast.failedToMarkAsNotSpam' : 'toast.failedToMarkAsSpam'))
    }
  }
}
