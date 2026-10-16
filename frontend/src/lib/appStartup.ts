// App.svelte's startup: backend event listeners for the main window and the
// boot sequence (settings, theme, persisted UI state, window show, mailto).

import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
import { accountStore } from '$lib/stores/accounts.svelte'
import { loadSettings } from '$lib/stores/settings.svelte'
import { loadImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
import { initTheme, handleSystemThemeEvent, handleMediaQueryChange } from '$lib/stores/theme.svelte'
import { loadUIState, setActiveExtension, type UIState } from '$lib/stores/uiState.svelte'
import { refreshExtensionRegistry } from '$lib/stores/extensionRegistry.svelte'
import { setPendingDeepLink } from '$lib/stores/extensionDeepLink.svelte'
import { initLayout } from '$lib/stores/layout.svelte'
import { openMailto, type MailtoData } from '$lib/stores/appComposer.svelte'
import { restoreSelection, selectFromNotification } from '$lib/stores/mailSelection.svelte'
// @ts-ignore - wailsjs path
import { GetPendingMailto, GetTermsAccepted, SetTermsAccepted, RefreshWindowConstraints, GetStartHiddenActive, GetSystemTheme, NotifyStartupComplete } from '../../wailsjs/go/app/App.js'
// @ts-ignore - wailsjs path
import { folder } from '../../wailsjs/go/models'
// @ts-ignore - wailsjs runtime
import { WindowShow, WindowHide, EventsOn } from '../../wailsjs/runtime/runtime'

export interface StartupHooks {
  // Highlight a thread in the message list.
  selectThreadInList(threadId: string): void
  // Give keyboard focus to the message list.
  focusMessageList(): void
  // Restore the persisted pane widths.
  applyUIState(uiState: UIState): void
}

// Helper to find folder info by ID from account store
function findFolderById(accountId: string, folderId: string): { name: string; type: string; path: string } | null {
  const acc = accountStore.accounts.find(a => a.account.id === accountId)
  if (!acc) return null

  function searchTree(trees: folder.FolderTree[]): { name: string; type: string; path: string } | null {
    for (const tree of trees) {
      if (tree.folder?.id === folderId) {
        return { name: tree.folder.name, type: tree.folder.type, path: tree.folder.path }
      }
      if (tree.children) {
        const found = searchTree(tree.children)
        if (found) return found
      }
    }
    return null
  }

  // Check if folders are loaded before searching
  if (!acc.folders || acc.folders.length === 0) return null
  return searchTree(acc.folders)
}

function registerEvents(hooks: StartupHooks) {
  // Notification click: show the conversation.
  EventsOn('notification:clicked', (data: { accountId: string; folderId: string; threadId: string }) => {
    // Switch the rail back to mail in case the user was on an extension
    // tab when the notification fired — without this the selection below
    // would update but stay hidden behind the extension pane.
    setActiveExtension('mail')
    selectFromNotification(data.accountId, data.folderId, data.threadId, findFolderById(data.accountId, data.folderId))
    // Highlight the thread in the list (small delay to ensure the list has loaded)
    setTimeout(() => hooks.selectThreadInList(data.threadId), 100)
  })

  // Generic extension-routed notification clicks. The host switches the
  // rail tab here AND stashes the path in a pending-deep-link buffer.
  // The target extension's pane drains the buffer on mount (it isn't
  // mounted yet at the moment we set the tab). Extension-specific path
  // parsing lives in each extension's own pane component.
  EventsOn('extension:open', (data: { extensionId: string; path: string }) => {
    if (!data.extensionId) return
    if (data.path) setPendingDeepLink(data.extensionId, data.path)
    setActiveExtension(data.extensionId)
  })

  // Window show requests (single-instance activation, notification clicks)
  EventsOn('window:show', () => {
    window.focus()
  })

  // External mailto from a second instance (routed through the backend)
  EventsOn('mailto:external', (data: MailtoData) => {
    openMailto(data)
  })

  // Toast confirmation when a detached composer sends a message
  EventsOn('composer:messageSent', () => {
    addToast({ type: 'success', message: get(_)('composer.messageSent') })
  })

  // EmailBody asks for focus back when keyboard navigation leaves its iframe.
  window.addEventListener('escape-iframe-focus', hooks.focusMessageList)
}

// Start the main window: register listeners, load persisted state, show the
// window. Resolves when startup is complete.
export async function startApp(hooks: StartupHooks) {
  registerEvents(hooks)

  // Load application settings (including theme mode) and apply theme
  const storedThemeMode = await loadSettings()
  await initTheme(storedThemeMode, GetSystemTheme)

  // Load image allowlist cache for synchronous checks in EmailBody
  loadImageAllowlist()

  // No startup pop-ups: Terms of Use, the missing-OAuth notice and What's New
  // never open at launch. Terms are linked from Settings > About, and
  // continued use counts as acceptance, so record it silently.
  GetTermsAccepted()
    .then((accepted: boolean) => (accepted ? undefined : SetTermsAccepted(true)))
    .catch((err: unknown) => console.error('Failed to record terms acceptance:', err))

  const uiState = await loadUIState()

  // Load extension registry (enabled extensions, rail tabs) so the rail can
  // render synchronously when the layout mounts.
  await refreshExtensionRegistry()

  // Pane widths are already validated/clamped by loadUIState.
  hooks.applyUIState(uiState)
  restoreSelection(uiState)

  // System theme changes from the backend (XDG Settings Portal)
  EventsOn('theme:system-preference', (newTheme: string) => {
    handleSystemThemeEvent(newTheme)
  })

  // System theme changes via matchMedia (fallback when the portal is unavailable)
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', (e) => {
    handleMediaQueryChange(e.matches)
  })

  // App is fully initialized — dismiss the inline boot splash from
  // index.html. The CSS transition fades it out; we remove the element
  // shortly after to free the DOM. Done BEFORE the start-hidden check
  // so users in background mode don't see the splash linger on screen
  // before the window hides.
  const splash = document.getElementById('boot-splash')
  if (splash) {
    splash.hidden = true
    setTimeout(() => splash.remove(), 250)
  }

  // main.ts called WindowShow() at module load so the splash was visible
  // during slow startup work (migrations etc.). If the user has start-
  // hidden background mode, undo that now. Otherwise the window is already
  // visible — calling WindowShow again is harmless.
  const shouldStartHidden = await GetStartHiddenActive()
  if (shouldStartHidden) {
    WindowHide()
  } else {
    WindowShow()
  }

  // Clear the desktop-environment startup indicator. Called after WindowShow()
  // so KDE/Plasma sees the placeholder → real window handoff cleanly (#154).
  // Fired unconditionally so the indicator clears even when starting hidden.
  NotifyStartupComplete()

  // Remove GTK max size constraints that Wails v2 sets at startup
  RefreshWindowConstraints()

  // Initialize responsive layout breakpoint listeners
  initLayout()

  // Check for pending mailto: URL from command line
  try {
    const mailtoData = await GetPendingMailto()
    if (mailtoData && (mailtoData.to?.length > 0 || mailtoData.subject || mailtoData.body)) {
      // Wait a moment for accounts to load
      await new Promise(resolve => setTimeout(resolve, 100))
      openMailto(mailtoData)
    }
  } catch (err) {
    console.error('Failed to check pending mailto:', err)
  }
}
