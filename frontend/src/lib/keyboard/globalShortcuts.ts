// The window-level keydown/keyup handlers. Gating first (open menus, modal
// dialogs, extension shortcuts, the composer), then Ctrl/Cmd shortcuts, the
// context-menu keys and Alt navigation; plain keys go to mailKeys.ts.

import { KEY } from '$lib/keyboard/shortcuts'
import { getActiveExtension, setActiveExtension } from '$lib/stores/uiState.svelte'
import { getRailTabs } from '$lib/stores/extensionRegistry.svelte'
import { isDialogGuardActive } from '$lib/stores/dialogGuard'
import { dispatchExtensionShortcut } from '$lib/stores/extensionShortcuts.svelte'
import { composer, openNewMessage, openReply } from '$lib/stores/appComposer.svelte'
import { mailSelection } from '$lib/stores/mailSelection.svelte'
import {
  focusNextPane,
  focusPreviousPane,
  getFocusedPane,
  getPaneNav,
  isInputElement,
  setFocusedPane,
} from '$lib/stores/keyboard.svelte'
import {
  getLayoutMode,
  getResponsiveView,
  hideSidebar,
  hideViewer,
  isResponsive,
  showSidebar,
  showViewer,
  toggleActiveSidebar,
} from '$lib/stores/layout.svelte'
import { getLastMessageId, handleMailKey } from './mailKeys'
import type { ShortcutContext } from './shortcutContext'

// After a synthetic contextmenu event, bits-ui mounts the portal asynchronously.
// Poll until [role="menu"] appears, then focus the first menuitem.
function focusContextMenu() {
  let attempts = 0
  const tryFocus = () => {
    const menu = document.querySelector('[role="menu"]') as HTMLElement | null
    if (menu) {
      const firstItem = menu.querySelector('[role="menuitem"]:not([data-disabled])') as HTMLElement | null
      ;(firstItem || menu).focus()
      return
    }
    if (attempts++ < 10) {
      requestAnimationFrame(tryFocus)
    }
  }
  requestAnimationFrame(tryFocus)
}

const isMailActive = () => getActiveExtension() === 'mail'

export function createGlobalShortcuts(ctx: ShortcutContext) {
  // Track Left Alt held state for the Left Alt + Right Alt combo
  let leftAltHeld = false

  // Ctrl/Cmd+Tab cycles forward through the rail items (Mail + enabled
  // extensions); Ctrl+` cycles backward, because webkit2gtk intercepts
  // Ctrl+Shift+Tab before the keydown reaches us.
  function cycleRail(e: KeyboardEvent) {
    const order = ['mail', ...getRailTabs().map(t => t.extensionId)]
    if (order.length <= 1) return // only Mail — nothing to cycle
    const idx = order.indexOf(getActiveExtension())
    const step = e.key === '`' ? -1 : 1
    setActiveExtension(order[(idx + step + order.length) % order.length])
  }

  // Mail-domain Ctrl/Cmd shortcuts. The caller has checked that mail is the
  // active rail pane (otherwise Ctrl+R would silently reply to the hidden mail
  // thread while the user is browsing contacts).
  function handleMailCtrlKey(e: KeyboardEvent) {
    const focusedPane = getFocusedPane()
    const hasConversation = mailSelection.threadId !== null
    const list = ctx.list()
    const viewer = ctx.viewer()
    const focusedIds = () => list?.getSelectedMessageIds() ?? []

    switch (e.key.toLowerCase()) {
      case 'n':
        // Ctrl/Cmd+N — new mail composer. Mail-domain only: when an
        // extension rail is active (e.g., calendar), that extension's
        // shortcut registry handles Ctrl+N before we reach this switch.
        e.preventDefault()
        openNewMessage()
        return
      case 'r': {
        if (!hasConversation) return
        e.preventDefault()
        if (focusedPane === 'viewer' && viewer?.hasFocusedMessage()) {
          if (e.shiftKey) viewer.replyAll()
          else viewer.reply()
          return
        }
        const msgId = getLastMessageId(ctx)
        if (!msgId) return
        openReply(e.shiftKey ? 'reply-all' : 'reply', msgId, viewer?.isImagesLoaded(msgId) || false)
        return
      }
      case 'f': {
        if (!hasConversation) return
        e.preventDefault()
        if (focusedPane === 'viewer' && viewer?.hasFocusedMessage()) {
          viewer.forward()
          return
        }
        const msgId = getLastMessageId(ctx)
        if (msgId) openReply('forward', msgId, viewer?.isImagesLoaded(msgId) || false)
        return
      }
      case 's':
        // Ctrl+S (no shift) is handled globally. Only Ctrl+Shift+S (sync
        // current folder) is mail-domain.
        if (!e.shiftKey) return
        e.preventDefault()
        list?.toggleFolderSync()
        return
      case 'a':
        e.preventDefault()
        if (e.shiftKey) {
          // Ctrl-Shift-A: sync all accounts, or open the sync status dialog if a sync is running
          ctx.sidebar()?.toggleSync()
        } else if (focusedPane === 'viewer') {
          // Ctrl-A: select all text in the viewer, or all messages in the list
          viewer?.selectAllText()
        } else {
          list?.selectAll()
        }
        return
      case 'l':
        e.preventDefault()
        // Ctrl-Shift-L opens the "Always Load" dropdown; Ctrl-L loads images
        if (e.shiftKey) viewer?.openAlwaysLoadDropdown()
        else viewer?.loadImages()
        return
      case 'u':
        e.preventDefault()
        // Mark the keyboard-focused chat as read/unread
        if (focusedIds().length > 0) {
          if (e.shiftKey) ctx.bulk.markUnread(focusedIds())
          else ctx.bulk.markRead(focusedIds())
        }
        return
      case 'k':
        e.preventDefault()
        // Archive the keyboard-focused chat
        if (focusedIds().length > 0) ctx.bulk.archive(focusedIds())
        return
      case 'j':
        e.preventDefault()
        // Spam the keyboard-focused chat
        if (focusedIds().length > 0) ctx.bulk.spam(focusedIds())
        return
    }
  }

  // Ctrl/Cmd shortcuts. GLOBAL cases fire regardless of which rail pane is
  // active, then a guard returns when an extension is the active pane, then
  // the mail-domain cases. The kit's components handle their own list/sidebar
  // shortcuts locally with stopPropagation, so those events never get here.
  function handleCtrlKey(e: KeyboardEvent) {
    if (KEY.SIDEBAR_TOGGLE(e)) {
      e.preventDefault()
      if (!e.repeat) toggleActiveSidebar()
      return
    }
    switch (e.key.toLowerCase()) {
      case 'q':
        e.preventDefault()
        ctx.quit()
        return
      case 'tab':
      case '`':
        e.preventDefault()
        cycleRail(e)
        return
    }

    // Ctrl+S (no shift) — focus the active pane's search input. Mail
    // dispatches to the list; extensions via the pane-nav registry.
    if (e.key.toLowerCase() === 's' && !e.shiftKey) {
      e.preventDefault()
      if (isMailActive()) {
        ctx.focusListSearch()
        return
      }
      getPaneNav('messageList')?.focusSearch?.()
      setFocusedPane('messageList')
      return
    }

    if (!isMailActive()) return
    handleMailCtrlKey(e)
  }

  // Right Alt or ContextMenu key: open the context menu for the focused item
  // in the current pane. Left Alt + Right Alt always targets the folder.
  function handleContextMenuKey(e: KeyboardEvent) {
    e.preventDefault()
    const focusedPane = getFocusedPane()

    if (leftAltHeld || focusedPane === 'sidebar') {
      const folderId = mailSelection.folderId
      if (!folderId) return
      const folderEl = document.querySelector(
        `[data-sidebar-item="folder"][data-folder-id="${folderId}"], ` +
        `[data-sidebar-item="unified-account"][data-folder-id="${folderId}"]`
      ) as HTMLElement | null
      if (!folderEl) return
      const rect = folderEl.getBoundingClientRect()
      folderEl.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: rect.right,
        clientY: rect.top + rect.height / 2,
      }))
      focusContextMenu()
      return
    }

    if (focusedPane === 'messageList') {
      ctx.list()?.openContextMenu()
      focusContextMenu()
    } else if (focusedPane === 'viewer') {
      ctx.viewer()?.openContextMenu()
      focusContextMenu()
    }
  }

  // Alt+Left (focus previous pane), narrowed in responsive layouts to close the
  // viewer or open the sidebar first.
  function focusPrevious() {
    if (isResponsive()) {
      const view = getResponsiveView()
      if (view === 'viewer') {
        hideViewer()
        return
      }
      if (getLayoutMode() === 'narrow' && view === 'default') {
        showSidebar()
        return
      }
    }
    focusPreviousPane()
  }

  function focusNext() {
    if (isResponsive()) {
      const view = getResponsiveView()
      if (getLayoutMode() === 'narrow' && view === 'sidebar') {
        hideSidebar()
        return
      }
      if (view === 'default' && mailSelection.threadId) {
        showViewer()
        return
      }
    }
    focusNextPane()
  }

  // Sidebar item navigation. Dispatches to mail's concrete ref when mail is
  // active; otherwise to the kit pane that registered as 'sidebar' via
  // registerPaneNav, so extensions get the same behavior regardless of which
  // pane is currently DOM-focused.
  function sidebarNav(
    e: KeyboardEvent,
    mail: 'selectPreviousFolder' | 'selectNextFolder' | 'selectFirstFolder' | 'selectLastFolder',
    kit: 'navigatePrev' | 'navigateNext' | 'navigateFirst' | 'navigateLast'
  ) {
    e.preventDefault()
    if (isMailActive()) {
      ctx.sidebar()?.[mail]()
      return
    }
    getPaneNav('sidebar')?.[kit]?.()
  }

  // Alt shortcuts (pane/folder navigation, always work)
  function handleAltKey(e: KeyboardEvent) {
    // Pane navigation is meaningless in focus mode (other panes hidden)
    if (mailSelection.focusMode !== 'off') return

    // Pane focus cycling (shared predicate — kit's pane components react to
    // the same focusedPane store, so cycling works across mail and extensions).
    if (KEY.PANE_FOCUS_PREV(e)) {
      e.preventDefault()
      focusPrevious()
      return
    }
    if (KEY.PANE_FOCUS_NEXT(e)) {
      e.preventDefault()
      focusNext()
      return
    }

    if (KEY.SIDEBAR_PREV(e)) return sidebarNav(e, 'selectPreviousFolder', 'navigatePrev')
    if (KEY.SIDEBAR_NEXT(e)) return sidebarNav(e, 'selectNextFolder', 'navigateNext')
    // Alt+G / Alt+Shift+G — first sidebar item (All Inboxes when present) or
    // the last visible one (last account header when all trees are collapsed).
    if (KEY.SIDEBAR_FIRST(e)) return sidebarNav(e, 'selectFirstFolder', 'navigateFirst')
    if (KEY.SIDEBAR_LAST(e)) return sidebarNav(e, 'selectLastFolder', 'navigateLast')

    // Alt+M / Alt+C — move/copy folder picker for the keyboard-focused
    // message. Open path only: the picker sets the dialog guard, so the
    // close/switch press is handled pre-guard in handleKeyDown.
    if (KEY.LIST_MOVE_TO(e)) {
      if (!isMailActive()) return
      e.preventDefault()
      ctx.list()?.toggleMoveToDialog()
      return
    }
    if (KEY.LIST_COPY_TO(e)) {
      if (!isMailActive()) return
      e.preventDefault()
      ctx.list()?.toggleCopyToDialog()
      return
    }

    // Alt+Enter — expand/collapse the focused account header or selected folder
    if (e.key === 'Enter') {
      const sidebar = ctx.sidebar()
      if (sidebar?.hasFocusedAccount()) {
        e.preventDefault()
        sidebar.toggleFocusedAccount()
      } else if (sidebar?.hasSelectedFolderWithChildren()) {
        e.preventDefault()
        sidebar.toggleSelectedFolderCollapse()
      }
    }
  }

  function onKeyUp(e: KeyboardEvent) {
    if (e.code === 'AltLeft') {
      leftAltHeld = false
    }
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.code === 'AltLeft') {
      leftAltHeld = true
    }
    const inInput = isInputElement(e.target)

    // Don't intercept keyboard events when a context menu or dropdown is open
    // (bits-ui portals mount [role="menu"] only while open)
    if (document.querySelector('[role="menu"]')) return

    // Alt+M/Alt+C toggle the message list's folder picker. While it's open the
    // dialog guard below swallows all keys, so close/switch is handled here.
    const list = ctx.list()
    if (list?.isFolderPickerOpen()) {
      if (KEY.LIST_MOVE_TO(e)) {
        e.preventDefault()
        list.toggleMoveToDialog()
        return
      }
      if (KEY.LIST_COPY_TO(e)) {
        e.preventDefault()
        list.toggleCopyToDialog()
        return
      }
    }

    // Don't intercept while a modal dialog has the guard active — keystrokes
    // (especially Ctrl+A) should target dialog inputs, not the background.
    if (isDialogGuardActive()) return

    // When the active rail pane is NOT mail, the extension's registered
    // shortcuts run first; a match handles the event. Skipped while typing in
    // an input element so extension shortcuts don't fire from text fields.
    // See [[extension-sdk-pattern]] and extensionShortcuts.svelte.ts.
    if (!inInput && dispatchExtensionShortcut(e)) {
      e.preventDefault()
      e.stopPropagation()
      return
    }

    // When the composer is open it handles its own shortcuts; only block the
    // reply/forward keys that might conflict.
    if (composer.show) {
      if ((e.ctrlKey || e.metaKey) && ['r', 'f'].includes(e.key.toLowerCase())) {
        e.preventDefault()
      }
      return
    }

    if (e.ctrlKey || e.metaKey) {
      handleCtrlKey(e)
      return
    }

    if (e.key === 'ContextMenu' || (e.key === 'Alt' && e.code === 'AltRight')) {
      handleContextMenuKey(e)
      return
    }

    if (e.altKey) {
      handleAltKey(e)
      return
    }

    // Skip single-key shortcuts if in input field
    if (inInput) return

    handleMailKey(ctx, e, isMailActive)
  }

  return { onKeyDown, onKeyUp }
}
