// Mail's unmodified keys: Escape, list and sidebar navigation, Enter/Space,
// chat triage and the single-key actions. Called by the global handler once
// the modifier shortcuts, dialogs and text inputs are ruled out.

import { KEY } from '$lib/keyboard/shortcuts'
import { getFocusedPane } from '$lib/stores/keyboard.svelte'
import { getLayoutMode, getResponsiveView, hideSidebar, hideViewer, isResponsive } from '$lib/stores/layout.svelte'
import {
  clearConversation,
  exitFocusMode,
  focusMessage,
  mailSelection,
  toggleThreadFocus,
} from '$lib/stores/mailSelection.svelte'
import type { ShortcutContext } from './shortcutContext'

// Chat list J/K opens each chat as it is selected, except in the narrow
// layout, where opening would cover the list.
function opensOnMove(): boolean {
  return getLayoutMode() !== 'narrow'
}

// Get the last message ID from the current conversation (for reply/forward)
export function getLastMessageId(ctx: ShortcutContext): string | null {
  return ctx.viewer()?.getLastMessageId() ?? null
}

// handleChatKey runs the chat triage shortcuts. They act on the chat
// selected in the list (the open one, after J/K or a click), falling back
// to the open chat. Returns whether the key was handled.
function handleChatKey(ctx: ShortcutContext, e: KeyboardEvent, hasConversation: boolean): boolean {
  const list = ctx.list()
  const viewer = ctx.viewer()
  const hasSelection = !!list?.hasSelection()
  if (KEY.CHAT_REPLY(e)) {
    if (!hasConversation) return false
    viewer?.focusComposer(e.shiftKey)
    return true
  }
  if (KEY.CHAT_SEARCH(e)) {
    ctx.focusListSearch()
    return true
  }
  if (KEY.CHAT_DONE(e)) {
    if (hasSelection) list!.archiveSelected()
    else if (hasConversation) viewer?.archive()
    return hasSelection || hasConversation
  }
  if (KEY.CHAT_SNOOZE(e)) {
    if (hasSelection) list!.openSnoozeForSelected()
    else if (hasConversation) viewer?.openSnooze()
    return hasSelection || hasConversation
  }
  if (!hasSelection) return false
  if (KEY.CHAT_UNREAD(e)) list!.markSelectedUnread()
  else if (KEY.CHAT_PIN(e)) list!.pinSelected()
  else if (KEY.CHAT_LOW_PRIORITY(e)) list!.toggleSelectedSenderLow()
  else if (KEY.CHAT_DELETE(e)) list!.requestDelete(list!.getSelectedMessageIds())
  else if (KEY.CHAT_MOVE(e)) list!.toggleMoveToDialog()
  else return false
  return true
}

// Escape is context-dependent and progressive: focus mode first, then
// responsive overlays, then the open chat.
function handleEscape(ctx: ShortcutContext) {
  if (mailSelection.focusMode !== 'off') {
    exitFocusMode()
    return
  }
  if (isResponsive() && getResponsiveView() === 'viewer') {
    hideViewer()
    return
  }
  if (isResponsive() && getResponsiveView() === 'sidebar') {
    hideSidebar()
    return
  }
  // In the open chat: back to the chat list, keeping the chat open.
  if (getFocusedPane() === 'viewer' && mailSelection.threadId) {
    ctx.list()?.focusList()
    return
  }
  // Close the open chat
  if (mailSelection.threadId) clearConversation()
}

// A button handles Enter/Space itself only inside the focused pane; elsewhere
// the key is handled here. Returns true when the button keeps the key.
function buttonKeepsKey(e: KeyboardEvent, focusedPane: string): boolean {
  if (document.activeElement?.tagName !== 'BUTTON') return false
  const btn = document.activeElement as HTMLElement
  const inMessageList = btn.closest('[data-pane="messageList"]')
  const inViewer = btn.closest('[data-pane="viewer"]')
  if ((focusedPane === 'messageList' && inMessageList) || (focusedPane === 'viewer' && inViewer)) {
    return true
  }
  // Otherwise, prevent the button click and handle the key with our logic
  e.preventDefault()
  return false
}

// Enter and Space: button-vs-pane disambiguation stays here (kit components
// handle their own Enter/Space locally). Returns true when the key was taken.
function handleEnterSpace(ctx: ShortcutContext, e: KeyboardEvent, focusedPane: string): boolean {
  const sidebar = ctx.sidebar()
  const list = ctx.list()
  const viewer = ctx.viewer()
  if (e.key === 'Enter') {
    if (buttonKeepsKey(e, focusedPane)) return true
    if (focusedPane === 'sidebar' && sidebar?.hasFocusedAccount()) {
      e.preventDefault()
      sidebar.toggleFocusedAccount()
    } else if (focusedPane === 'sidebar' && sidebar?.hasSelectedFolderWithChildren()) {
      e.preventDefault()
      sidebar.toggleSelectedFolderCollapse()
    } else if (focusedPane === 'messageList' && list?.hasSelection()) {
      // Open the chat and start a reply.
      e.preventDefault()
      list.openSelected()
      viewer?.focusComposer()
    } else if (focusedPane === 'viewer' && mailSelection.threadId) {
      e.preventDefault()
      viewer?.focusComposer()
    }
    return true
  }
  if (e.key === ' ') {
    // Space: expand/collapse the focused account or folder
    if (buttonKeepsKey(e, focusedPane)) return true
    e.preventDefault()
    if (focusedPane === 'sidebar' && sidebar?.hasFocusedAccount()) {
      sidebar.toggleFocusedAccount()
    } else if (focusedPane === 'sidebar' && sidebar?.hasSelectedFolderWithChildren()) {
      sidebar.toggleSelectedFolderCollapse()
    }
    return true
  }
  return false
}

// J/K and the arrows (with their checkbox variants) move through the pane
// that has focus.
function handleListMove(ctx: ShortcutContext, e: KeyboardEvent, focusedPane: string, forward: boolean) {
  e.preventDefault()
  const sidebar = ctx.sidebar()
  const list = ctx.list()
  const viewer = ctx.viewer()
  if (focusedPane === 'sidebar') {
    if (forward) sidebar?.selectNextFolder()
    else sidebar?.selectPreviousFolder()
  } else if (focusedPane === 'messageList') {
    if (forward) list?.selectNext(opensOnMove())
    else list?.selectPrevious(opensOnMove())
  } else if (focusedPane === 'viewer') {
    // J/K moves to the next/previous chat; the arrows scroll the open one.
    if (forward) {
      if (e.key === 'ArrowDown') viewer?.scrollDown()
      else list?.selectNext(true)
    } else if (e.key === 'ArrowUp') viewer?.scrollUp()
    else list?.selectPrevious(true)
  }
}

// Mail-domain plain keys. isMailActive guards each: the kit panes handle their
// own navigation locally, so these only matter when an event slips through
// while an extension is the active rail pane.
export function handleMailKey(ctx: ShortcutContext, e: KeyboardEvent, isMailActive: () => boolean) {
  const focusedPane = getFocusedPane()
  const hasConversation = mailSelection.threadId !== null
  const list = ctx.list()
  const viewer = ctx.viewer()

  if (e.key === 'Escape') {
    handleEscape(ctx)
    return
  }

  if (KEY.LIST_PREV(e) || KEY.LIST_PREV_CHECK(e)) {
    if (isMailActive()) handleListMove(ctx, e, focusedPane, false)
    return
  }
  if (KEY.LIST_NEXT(e) || KEY.LIST_NEXT_CHECK(e)) {
    if (isMailActive()) handleListMove(ctx, e, focusedPane, true)
    return
  }

  if (handleEnterSpace(ctx, e, focusedPane)) return

  // g / Shift+G — jump to first / last loaded message (shared predicates —
  // kit list panes consume the same KEY.LIST_FIRST/LIST_LAST locally)
  if (KEY.LIST_FIRST(e) || KEY.LIST_LAST(e)) {
    if (!isMailActive()) return
    if (focusedPane !== 'messageList') return
    e.preventDefault()
    if (KEY.LIST_FIRST(e)) list?.selectFirst()
    else list?.selectLast()
    return
  }

  // Chat triage keys (V here is Move, not the kit lists' V = open).
  if (isMailActive() && handleChatKey(ctx, e, hasConversation)) {
    e.preventDefault()
    return
  }

  // Single-key shortcuts
  switch (e.key) {
    case 's': {
      // Toggle star on the keyboard-focused chat
      const focusedIds = list?.getSelectedMessageIds() ?? []
      if (focusedIds.length > 0) {
        ctx.bulk.toggleStar(focusedIds, !(list?.isSelectedStarred() ?? false))
      }
      return
    }
    case 'f':
      // Toggle thread focus mode (only with a conversation open)
      if (!hasConversation) return
      e.preventDefault()
      toggleThreadFocus()
      return
    case 'F': {
      // Shift+F: toggle message focus on the currently Tab-focused message
      // (falls back to last message if none focused)
      if (!hasConversation) return
      e.preventDefault()
      if (mailSelection.focusMode === 'message') {
        exitFocusMode()
        return
      }
      const targetId = (focusedPane === 'viewer' && viewer?.hasFocusedMessage())
        ? viewer.getFocusedMessageId()
        : getLastMessageId(ctx)
      if (targetId) focusMessage(targetId)
      return
    }
    case 'd': // alias of Delete: move focused/checked message(s) to Trash
    case 'Backspace':
    case 'Delete': {
      if (focusedPane === 'viewer' && viewer?.hasFocusedMessage()) {
        if (e.shiftKey) viewer.deletePermanently()
        else viewer.trash()
        return
      }
      const focusedMessageIds = list?.getSelectedMessageIds() ?? []
      if (focusedMessageIds.length > 0) {
        list?.requestDelete(focusedMessageIds, e.shiftKey)
      }
      return
    }
  }
}
