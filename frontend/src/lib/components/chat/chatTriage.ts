// Chat triage actions shared by the chat list (rows, keyboard) and the open
// chat's header. Each runs the backend call and shows a toast, with Undo
// where the backend records one. They resolve to whether the action ran.

import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { toasts } from '$lib/stores/toast'
import { Archive, DeletePermanently, MarkAsRead, MarkAsUnread, PinChat, SnoozeChat, UnsnoozeChat, SetSenderCategory, SetSenderChat, Trash, Undo } from '../../../../wailsjs/go/app/App'
import { displayName, type Person } from './chatFormat'
import { formatSnoozePreset } from './chatSnooze'

// The parts of a chat the actions need; ChatItem satisfies it.
export interface TriageChat {
  accountId: string
  threadKey: string
  messageIds: string[]
}

// Triage handlers a chat row calls, implemented by ChatList.
export interface ChatRowActions {
  onArchive: () => void
  onPin: () => void
  onSnooze: (until: Date) => void
  onUnsnooze: () => void
  onMarkUnread: () => void
  onToggleSenderLow: () => void
  onToggleSenderChat: () => void
}

type After = () => void

export function t(key: string, values?: Record<string, string | number>): string {
  return get(_)(key, values ? { values } : undefined)
}

export async function undoLast(after?: After): Promise<void> {
  try {
    const description = await Undo()
    toasts.success(t('toast.undone', { description }))
    after?.()
  } catch (err) {
    console.error('Undo failed:', err)
    toasts.error(t('toast.undoFailed'))
  }
}

export function undoAction(after?: After) {
  return [{ label: t('common.undo'), onClick: () => void undoLast(after) }]
}

// trashMessages moves messages to Trash, with Undo when they moved (some
// folders delete outright).
export async function trashMessages(ids: string[], after?: After): Promise<boolean> {
  try {
    const moved = await Trash(ids)
    toasts.success(t(moved ? 'toast.movedToTrash' : 'toast.deletedFromFolder'), moved ? undoAction(after) : [])
    return true
  } catch (err) {
    console.error('Delete failed:', err)
    toasts.error(t('toast.failedToDelete'))
    return false
  }
}

export async function deleteMessagesPermanently(ids: string[]): Promise<boolean> {
  try {
    await DeletePermanently(ids)
    toasts.success(t('toast.permanentlyDeleted'))
    return true
  } catch (err) {
    console.error('Permanent delete failed:', err)
    toasts.error(t('toast.failedToDelete'))
    return false
  }
}

async function run(action: () => Promise<unknown>, success: string, failure: string, undo: boolean, after?: After): Promise<boolean> {
  try {
    await action()
    toasts.success(success, undo ? undoAction(after) : [])
    return true
  } catch (err) {
    console.error('Chat action failed:', err)
    toasts.error(failure)
    return false
  }
}

// archiveChat is Done: archive every listed message of the thread.
export function archiveChat(messageIds: string[], afterUndo?: After): Promise<boolean> {
  if (messageIds.length === 0) return Promise.resolve(false)
  return run(() => Archive(messageIds), t('toast.conversationArchived'), t('toast.failedToArchive'), true, afterUndo)
}

// archiveChats archives several chats at once (the Low priority group's
// Archive all) as one undoable move.
export function archiveChats(chats: TriageChat[], afterUndo?: After): Promise<boolean> {
  const ids = chats.flatMap((c) => c.messageIds)
  if (ids.length === 0) return Promise.resolve(false)
  const msg = t('chat.chatsArchivedToast', { count: chats.length })
  return run(() => Archive(ids), msg, t('toast.failedToArchive'), true, afterUndo)
}

// markChatsRead marks every message of several chats read. Like mark
// unread, the backend records no undo.
export function markChatsRead(chats: TriageChat[]): Promise<boolean> {
  const ids = chats.flatMap((c) => c.messageIds)
  if (ids.length === 0) return Promise.resolve(false)
  return run(() => MarkAsRead(ids), t('chat.chatsMarkedReadToast', { count: chats.length }), t('toast.failedToUpdateReadStatus'), false)
}

export function pinChat(c: TriageChat, pinned: boolean, afterUndo?: After): Promise<boolean> {
  const msg = t(pinned ? 'chat.pinnedToast' : 'chat.unpinnedToast')
  return run(() => PinChat(c.accountId, c.threadKey, pinned), msg, t('chat.stateChangeFailed'), true, afterUndo)
}

export function snoozeChat(c: TriageChat, until: Date, afterUndo?: After): Promise<boolean> {
  const msg = t('chat.snoozedToast', { time: formatSnoozePreset(until) })
  return run(() => SnoozeChat(c.accountId, c.threadKey, until.getTime()), msg, t('chat.stateChangeFailed'), true, afterUndo)
}

export function unsnoozeChat(c: TriageChat, afterUndo?: After): Promise<boolean> {
  return run(() => UnsnoozeChat(c.accountId, c.threadKey), t('chat.unsnoozedToast'), t('chat.stateChangeFailed'), true, afterUndo)
}

// markChatUnread has no backend undo; marking read again is one key away.
export function markChatUnread(messageIds: string[]): Promise<boolean> {
  if (messageIds.length === 0) return Promise.resolve(false)
  return run(() => MarkAsUnread(messageIds), t('chat.markedUnreadToast'), t('toast.failedToUpdateReadStatus'), false)
}

// setSenderLow overrides a sender's classification. Its Undo clears the
// override (back to header-based classification), since the backend keeps
// no history for sender categories.
export async function setSenderLow(accountId: string, sender: Person, low: boolean): Promise<boolean> {
  const name = displayName(sender)
  try {
    await SetSenderCategory(accountId, sender.email, low ? 'low' : 'priority')
  } catch (err) {
    console.error('Sender category change failed:', err)
    toasts.error(t('chat.stateChangeFailed'))
    return false
  }
  const undo = async () => {
    try {
      await SetSenderCategory(accountId, sender.email, '')
    } catch (err) {
      console.error('Sender category undo failed:', err)
      toasts.error(t('toast.undoFailed'))
    }
  }
  toasts.success(t(low ? 'chat.senderMovedToLow' : 'chat.senderMovedToPriority', { sender: name }), [
    { label: t('common.undo'), onClick: () => void undo() },
  ])
  return true
}

// setSenderChat combines a sender's threads into one sender chat, or splits
// it back into threads. Undoable. Resolves false, with no Undo, when the
// sender already was that way (for example a thread I started that the
// sender replied to, while the sender already combines).
export async function setSenderChat(accountId: string, sender: Person, combined: boolean, afterUndo?: After): Promise<boolean> {
  const name = displayName(sender)
  try {
    if (!(await SetSenderChat(accountId, sender.email, combined))) {
      toasts.info(t(combined ? 'chat.senderAlreadyCombined' : 'chat.senderAlreadySplit', { sender: name }))
      return false
    }
  } catch (err) {
    console.error('Sender chat change failed:', err)
    toasts.error(t('chat.stateChangeFailed'))
    return false
  }
  toasts.success(t(combined ? 'chat.senderCombinedToast' : 'chat.senderSplitToast', { sender: name }), undoAction(afterUndo))
  return true
}
