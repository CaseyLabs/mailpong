// Message actions the Mail shortcuts run on the keyboard-focused chats
// (archive, spam, read state, star) with their toasts and undo.

import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
// @ts-ignore - wailsjs path
import { MarkAsRead, MarkAsUnread, Star, Unstar, Archive, MarkAsSpam, MarkAsNotSpam, Undo } from '../../../wailsjs/go/app/App.js'

// The part of the chat list the actions call back into.
export interface BulkActionTarget {
  handleActionComplete(autoSelectNext?: boolean): void
}

export function createBulkActions(getList: () => BulkActionTarget | null, getFolderType: () => string | null) {
  const t = (key: string, values?: Record<string, string>) => get(_)(key, values ? { values } : undefined)

  async function undo() {
    try {
      const description = await Undo()
      addToast({ type: 'success', message: t('toast.undone', { description }) })
      getList()?.handleActionComplete()
    } catch (err) {
      console.error('Undo failed:', err)
      addToast({ type: 'error', message: t('toast.undoFailed') })
    }
  }

  async function archive(messageIds: string[]) {
    try {
      await Archive(messageIds)
      addToast({ type: 'success', message: t('toast.archived'), actions: [{ label: t('common.undo'), onClick: undo }] })
      getList()?.handleActionComplete(true)
    } catch (err) {
      console.error('Archive failed:', err)
      addToast({ type: 'error', message: t('toast.failedToArchive') })
    }
  }

  // In the Spam folder the action is "not spam".
  async function spam(messageIds: string[]) {
    const isSpamFolder = getFolderType() === 'spam'
    try {
      if (isSpamFolder) {
        await MarkAsNotSpam(messageIds)
        addToast({ type: 'success', message: t('toast.markedAsNotSpam'), actions: [{ label: t('common.undo'), onClick: undo }] })
      } else {
        await MarkAsSpam(messageIds)
        addToast({ type: 'success', message: t('toast.markedAsSpam'), actions: [{ label: t('common.undo'), onClick: undo }] })
      }
      getList()?.handleActionComplete(true)
    } catch (err) {
      console.error('Spam toggle failed:', err)
      addToast({ type: 'error', message: t(isSpamFolder ? 'toast.failedToMarkAsNotSpam' : 'toast.failedToMarkAsSpam') })
    }
  }

  async function markRead(messageIds: string[]) {
    try {
      await MarkAsRead(messageIds)
      addToast({ type: 'success', message: t('toast.markedAsRead') })
      getList()?.handleActionComplete()
    } catch (err) {
      console.error('Mark as read failed:', err)
      addToast({ type: 'error', message: t('toast.failedToMarkAsRead') })
    }
  }

  async function markUnread(messageIds: string[]) {
    try {
      await MarkAsUnread(messageIds)
      addToast({ type: 'success', message: t('toast.markedAsUnread') })
      getList()?.handleActionComplete()
    } catch (err) {
      console.error('Mark as unread failed:', err)
      addToast({ type: 'error', message: t('toast.failedToMarkAsUnread') })
    }
  }

  async function toggleStar(messageIds: string[], shouldStar: boolean) {
    try {
      if (shouldStar) {
        await Star(messageIds)
        addToast({ type: 'success', message: t('toast.starred') })
      } else {
        await Unstar(messageIds)
        addToast({ type: 'success', message: t('toast.starRemoved') })
      }
      getList()?.handleActionComplete()
    } catch (err) {
      console.error('Star toggle failed:', err)
      addToast({ type: 'error', message: t('toast.failedToUpdateStar') })
    }
  }

  return { archive, spam, markRead, markUnread, toggleStar, undo }
}

export type BulkActions = ReturnType<typeof createBulkActions>
