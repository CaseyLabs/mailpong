<script lang="ts">
  // Load offline icon data before anything else
  import './lib/iconify-offline'

  import { onMount, untrack } from 'svelte'
  import TitleBar from './lib/components/common/TitleBar.svelte'
  import SnapshotRestoreBanner from './lib/components/common/SnapshotRestoreBanner.svelte'
  import Sidebar from './lib/components/sidebar/Sidebar.svelte'
  import PaneResizeHandle from '$lib/components/kit/PaneResizeHandle.svelte'
  import ChatList from './lib/components/chat/ChatList.svelte'
  import { flushChatComposers } from '$lib/components/chat/chatComposerState.svelte'
  import ChatView from './lib/components/chat/ChatView.svelte'
  import Composer from './lib/components/composer/Composer.svelte'
  import ToastContainer from './lib/components/ui/toast/ToastContainer.svelte'
  import SpellSuggestionMenu from './lib/spellcheck/SpellSuggestionMenu.svelte'
  import CertificateDialog from './lib/components/settings/CertificateDialog.svelte'
  import ExtensionSettingsDialog from './lib/components/settings/ExtensionSettingsDialog.svelte'
  import ExtensionRail from './lib/components/rail/ExtensionRail.svelte'
  import ContactsPane from '$extensions/contacts/frontend/components/ContactsPane.svelte'
  import CalendarPane from '$extensions/calendar/frontend/components/CalendarPane.svelte'
  import { refreshExtensionRegistry, getRailTabs } from '$lib/stores/extensionRegistry.svelte'
  import { KEY } from '$lib/keyboard/shortcuts'
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { loadSettings, getThemeMode, getShowTitleBar, getNativeTitleBar, getComposerMode, getMailtoMode } from '$lib/stores/settings.svelte'
  import { loadImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import { initTheme, applyThemeFromMode, handleSystemThemeEvent, handleMediaQueryChange } from '$lib/stores/theme.svelte'
  import { loadUIState, saveUIState, getActiveExtension, setActiveExtension } from '$lib/stores/uiState.svelte'
  import { setPendingDeepLink } from '$lib/stores/extensionDeepLink.svelte'
  import {
    type FocusablePane,
    getFocusedPane,
    setFocusedPane,
    focusPreviousPane,
    focusNextPane,
    isPaneFlashing,
    isInputElement,
    setComposerOpen,
    getPaneNav
  } from '$lib/stores/keyboard.svelte'
  import { isDialogGuardActive } from '$lib/stores/dialogGuard'
  import { dispatchExtensionShortcut } from '$lib/stores/extensionShortcuts.svelte'
  import { initLayout, registerViewerSelection, getLayoutMode, getResponsiveView, showViewer, hideViewer, showSidebar, hideSidebar, isResponsive, isSidebarHidden, toggleActiveSidebar } from '$lib/stores/layout.svelte'
  // @ts-ignore - wailsjs path
  import { PrepareReply, GetPendingMailto, GetDraft, MarkAsRead, MarkAsUnread, Star, Unstar, Archive, MarkAsSpam, MarkAsNotSpam, Undo, GetTermsAccepted, SetTermsAccepted, RefreshWindowConstraints, AcceptCertificate, GetServerCertificateFailure, GetStartHiddenActive, CloseWindow, QuitApp, OpenComposerWindow, GetSystemTheme, NotifyStartupComplete, ChatDraftsFlushed } from '../wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs path
  import { smtp, folder, certificate } from '../wailsjs/go/models'
  // @ts-ignore - wailsjs runtime
  import { WindowShow, WindowHide, EventsOn } from '../wailsjs/runtime/runtime'
  import { _ } from '$lib/i18n'

  // Component refs for keyboard navigation. Plain `let` (not $state) is
  // intentional: svelte-check warns "Changing its value will not correctly
  // trigger updates" but nothing here actually reads these refs in a reactive
  // context — they're only used inside event handlers. Making them $state
  // added bookkeeping cost (visible in idle-CPU profiling) without any benefit.
  let sidebarRef: Sidebar | null = null
  let messageListRef: ChatList | null = null
  let viewerRef: ChatView | null = null
  let messageListContainerRef: HTMLElement | null = null

  // React to theme mode changes from settings store
  $effect(() => {
    const mode = getThemeMode()
    applyThemeFromMode(mode)
  })

  // Selected folder state
  // Starts on All Inboxes, so the Accounts column highlights what the list shows.
  let selectedAccountId = $state<string | null>('unified')
  let selectedFolderId = $state<string | null>('inbox')
  let selectedFolderName = $state('All Inboxes')
  let selectedFolderType = $state<string | null>('inbox')
  // Track where the selection came from: 'unified' for unified section, 'account' for account tree
  let selectionSource = $state<'unified' | 'account' | null>('unified')

  // Selected conversation state
  let selectedThreadId = $state<string | null>(null)
  let selectedConversationFolderId = $state<string | null>(null)
  let selectedConversationAccountId = $state<string | null>(null)

  // Composer state
  let showComposer = $state(false)
  let composerAccountId = $state<string | null>(null)
  let composerInitialMessage = $state<smtp.ComposeMessage | null>(null)
  let composerDraftId = $state<string | null>(null)
  let composerImagesLoaded = $state(false)

  // Mirror composer visibility into the keyboard store so the viewer can
  // suppress its Delete/Backspace shortcut during the composer's mount→focus race.
  $effect(() => {
    setComposerOpen(showComposer)
  })

  // Focus mode state — viewer (or single message) takes the whole window.
  // Always resets on conversation change, on Esc, on back-arrow, and on app reload.
  let focusMode = $state<'off' | 'thread' | 'message'>('off')
  let focusedMessageIdInFocus = $state<string | null>(null)
  // A selected conversation (restored at startup or picked in full mode or
  // before switching views) reopens as the viewer overlay in responsive
  // layouts; see registerViewerSelection.
  registerViewerSelection('mail', () => selectedThreadId !== null)

  const viewerIsOverlay = $derived(isResponsive() || focusMode !== 'off')
  const viewerIsVisible = $derived(getResponsiveView() === 'viewer' || focusMode !== 'off')
  // A closed viewer overlay is inert so its controls cannot take keyboard focus.
  const viewerIsClosed = $derived(viewerIsOverlay && !viewerIsVisible)
  let viewerContainerRef: HTMLElement | null = null

  // Hand the focused pane and DOM focus to the list as the viewer closes:
  // otherwise ChatView keeps handling Tab for its now-inert messages (Tab
  // stops moving), and focus on Back would fall to <body>.
  $effect.pre(() => {
    if (!viewerIsClosed) return
    untrack(() => {
      if (getFocusedPane() === 'viewer') setFocusedPane('messageList')
      if (viewerContainerRef?.contains(document.activeElement)) messageListContainerRef?.focus()
    })
  })

  function toggleThreadFocus() {
    if (focusMode === 'thread') {
      focusMode = 'off'
      focusedMessageIdInFocus = null
      return
    }
    focusMode = 'thread'
    focusedMessageIdInFocus = null
  }

  // Auto-reset focus mode when the conversation changes (or is closed).
  // Prevents focus state from leaking across navigation.
  $effect(() => {
    void selectedThreadId
    focusMode = 'off'
    focusedMessageIdInFocus = null
  })

  // Route keyboard pane focus to the viewer while in focus mode so j/k/arrow
  // shortcuts scroll the viewer, then restore the prior pane on exit.
  let focusedPaneBeforeFocusMode: FocusablePane | null = null
  $effect(() => {
    const mode = focusMode
    if (mode !== 'off' && focusedPaneBeforeFocusMode === null) {
      focusedPaneBeforeFocusMode = untrack(() => getFocusedPane())
      setFocusedPane('viewer')
      return
    }
    if (mode === 'off' && focusedPaneBeforeFocusMode !== null) {
      setFocusedPane(focusedPaneBeforeFocusMode)
      focusedPaneBeforeFocusMode = null
    }
  })

  // Shutdown state
  let isShuttingDown = $state(false)

  // Certificate TOFU state (for background sync cert errors)
  let showCertDialog = $state(false)
  let pendingCertificate = $state<certificate.CertificateInfo | null>(null)
  let pendingCertAccountId = $state<string | null>(null)
  let pendingCertHost = $state('')

  // Flatpak filesystem permission dialog state
  let showFlatpakFsDialog = $state(false)
  let flatpakFsDialogContext = $state<'open' | 'saveAll'>('open')

  // Handle window close button (title bar X) — hides if background mode, quits if not
  function handleClose() {
    CloseWindow()
  }

  // Handle forced quit (Ctrl+Q) — always quits regardless of background mode
  function handleQuit() {
    isShuttingDown = true
    setTimeout(() => QuitApp(), 100)
  }

  // Certificate TOFU handlers for background sync
  async function handleBgCertAccept(permanent: boolean) {
    if (!pendingCertificate) return
    try {
      const acc = accountStore.accounts.find(a => a.account.id === pendingCertAccountId)
      const host = pendingCertHost || acc?.account.imapHost || ''
      await AcceptCertificate(host, pendingCertificate, permanent)
    } catch (err) {
      console.error('Failed to accept certificate:', err)
    }
    handleBgCertDecline()
  }

  function handleBgCertDecline() {
    showCertDialog = false
    pendingCertificate = null
    pendingCertAccountId = null
    pendingCertHost = ''
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

  onMount(async () => {
    // Listen for notification click events from backend
    EventsOn('notification:clicked', (data: { accountId: string; folderId: string; threadId: string }) => {
      // Find folder info for display
      const folderInfo = findFolderById(data.accountId, data.folderId)

      // Switch the rail back to mail in case the user was on an extension
      // tab when the notification fired — without this the message-list
      // state below would update but stay hidden behind the extension pane.
      setActiveExtension('mail')

      // Navigate to the folder (use 'unified' source to highlight under Unified Inbox)
      selectedAccountId = data.accountId
      selectedFolderId = data.folderId
      selectedFolderName = folderInfo?.name || 'Inbox'
      selectedFolderType = folderInfo?.type || 'inbox'
      selectionSource = 'unified'

      // Select the conversation
      selectedThreadId = data.threadId
      selectedConversationAccountId = data.accountId
      selectedConversationFolderId = data.folderId

      // Highlight the thread in the message list (with small delay to ensure list has loaded)
      setTimeout(() => {
        messageListRef?.selectThread(data.threadId)
      }, 100)

      // Persist state
      saveUIState({
        selectedAccountId: data.accountId,
        selectedFolderId: data.folderId,
        selectedFolderName: folderInfo?.name || 'Inbox',
        selectedFolderType: folderInfo?.type || 'inbox',
        selectedThreadId: data.threadId,
        selectedConversationAccountId: data.accountId,
        selectedConversationFolderId: data.folderId,
      })
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

    // Listen for window show requests (from single-instance activation, notification clicks)
    EventsOn('window:show', () => {
      window.focus()
    })

    // Listen for shutdown event from backend (triggered by OS close signal)
    EventsOn('app:shutting-down', () => {
      isShuttingDown = true
      // Save chat composer text before the backend quits; it waits for this.
      void flushChatComposers().finally(() => ChatDraftsFlushed())
    })

    // Listen for untrusted certificate events from background sync
    EventsOn('certificate:untrusted', (data: { accountId?: string; host?: string; certificate: certificate.CertificateInfo }) => {
      // Only show if not already showing a cert dialog
      if (!showCertDialog) {
        pendingCertificate = data.certificate
        pendingCertAccountId = data.accountId ?? null
        pendingCertHost = data.host ?? ''
        showCertDialog = true
      }
    })

    // Calendar owns its scheduler and reports DAV failures through its event bus.
    EventsOn('calendar:source-error', async (data: { message: string; certificateFailure?: string }) => {
      if (showCertDialog) return
      try {
        const failure = await GetServerCertificateFailure(data.certificateFailure ?? data.message)
        if (!showCertDialog && failure.certificateRequired && failure.host && failure.certificate) {
          pendingCertificate = failure.certificate
          pendingCertAccountId = null
          pendingCertHost = failure.host
          showCertDialog = true
        }
      } catch (err) {
        console.error('Failed to resolve calendar certificate error:', err)
      }
    })

    // Listen for Flatpak filesystem permission dialog event. Optional payload
    // picks the wording: 'saveAll' (bulk attachment save, #384) vs the
    // original toast-open case (no payload).
    EventsOn('flatpak:filesystem-dialog', (context?: string) => {
      flatpakFsDialogContext = context === 'saveAll' ? 'saveAll' : 'open'
      showFlatpakFsDialog = true
    })

    // Listen for external mailto from second instance (routed through backend)
    EventsOn('mailto:external', (data: MailtoData) => {
      handleMailtoData(data)
    })

    // Toast confirmation when a detached composer sends a message
    EventsOn('composer:messageSent', () => {
      addToast({
        type: 'success',
        message: $_('composer.messageSent'),
      })
    })

    // Listen for escape-iframe-focus event (from EmailBody when navigating away from iframe)
    const handleEscapeIframeFocus = () => {
      // Focus the message list container to take keyboard focus away from iframe
      messageListContainerRef?.focus()
    }
    window.addEventListener('escape-iframe-focus', handleEscapeIframeFocus)

    // Load application settings (including theme mode) and apply theme
    const storedThemeMode = await loadSettings()
    await initTheme(storedThemeMode, GetSystemTheme)

    // Load image allowlist cache for synchronous checks in EmailBody
    loadImageAllowlist()

    // No startup pop-ups: Terms of Use, the missing-OAuth notice and What's New
    // never open at launch. Terms are linked from Settings > About, and
    // continued use counts as acceptance, so record it silently.
    GetTermsAccepted()
      .then((accepted) => (accepted ? undefined : SetTermsAccepted(true)))
      .catch((err) => console.error('Failed to record terms acceptance:', err))

    // Load persisted UI state
    const uiState = await loadUIState()

    // Load extension registry (enabled extensions, rail tabs) so the rail can
    // render synchronously when the layout mounts.
    await refreshExtensionRegistry()

    // Restore pane widths (already validated/clamped by loadUIState)
    sidebarWidth = uiState.sidebarWidth
    listWidth = uiState.listWidth

    // Restore folder selection if valid
    if (uiState.selectedAccountId && uiState.selectedFolderId) {
      // Validate account still exists (unless unified inbox)
      const isUnified = uiState.selectedAccountId === 'unified'
      const accountExists = isUnified || accountStore.accounts.some(
        a => a.account.id === uiState.selectedAccountId
      )

      if (accountExists) {
        selectedAccountId = uiState.selectedAccountId
        selectedFolderId = uiState.selectedFolderId
        selectedFolderName = uiState.selectedFolderName || 'Inbox'
        selectedFolderType = uiState.selectedFolderType
        // The saved state has no source; an account folder highlights in its tree.
        selectionSource = isUnified ? 'unified' : 'account'

        // Restore conversation selection
        if (uiState.selectedThreadId) {
          selectedThreadId = uiState.selectedThreadId
          selectedConversationAccountId = uiState.selectedConversationAccountId
          selectedConversationFolderId = uiState.selectedConversationFolderId
        }
      }
    }

    // Listen for system theme changes from backend (XDG Settings Portal)
    EventsOn('theme:system-preference', (newTheme: string) => {
      handleSystemThemeEvent(newTheme)
    })

    // Listen for system theme changes via matchMedia (fallback when portal unavailable)
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
    }
    if (!shouldStartHidden) {
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
        handleMailtoData(mailtoData)
      }
    } catch (err) {
      console.error('Failed to check pending mailto:', err)
    }
  })

  // Handle folder selection from sidebar (account tree)
  function handleFolderSelect(
    accountId: string,
    folderId: string,
    folderPath: string,
    folderName: string,
    folderType: string
  ) {
    selectedAccountId = accountId
    selectedFolderId = folderId
    selectedFolderName = folderName
    selectedFolderType = folderType
    selectionSource = 'account'
    selectedThreadId = null // Clear conversation selection when changing folders
    selectedConversationFolderId = null
    selectedConversationAccountId = null
    hideSidebar()

    // Persist state
    saveUIState({
      selectedAccountId: accountId,
      selectedFolderId: folderId,
      selectedFolderName: folderName,
      selectedFolderType: folderType,
      selectedThreadId: null,
      selectedConversationAccountId: null,
      selectedConversationFolderId: null,
    })
  }

  // Handle folder selection from unified inbox section
  function handleUnifiedFolderSelect(
    accountId: string,
    folderId: string,
    folderPath: string,
    folderName: string,
    folderType: string
  ) {
    selectedAccountId = accountId
    selectedFolderId = folderId
    selectedFolderName = folderName
    selectedFolderType = folderType
    selectionSource = 'unified'
    selectedThreadId = null
    selectedConversationFolderId = null
    selectedConversationAccountId = null
    hideSidebar()

    // Persist state
    saveUIState({
      selectedAccountId: accountId,
      selectedFolderId: folderId,
      selectedFolderName: folderName,
      selectedFolderType: folderType,
      selectedThreadId: null,
      selectedConversationAccountId: null,
      selectedConversationFolderId: null,
    })
  }

  // Handle unified inbox selection from sidebar (All Inboxes)
  function handleUnifiedInboxSelect() {
    selectedAccountId = 'unified'
    selectedFolderId = 'inbox'
    selectedFolderName = 'All Inboxes'
    selectedFolderType = 'inbox'
    selectionSource = 'unified'
    selectedThreadId = null
    selectedConversationFolderId = null
    selectedConversationAccountId = null
    hideSidebar()

    // Persist state
    saveUIState({
      selectedAccountId: 'unified',
      selectedFolderId: 'inbox',
      selectedFolderName: 'All Inboxes',
      selectedFolderType: 'inbox',
      selectedThreadId: null,
      selectedConversationAccountId: null,
      selectedConversationFolderId: null,
    })
  }

  // Handle conversation selection from list
  function handleConversationSelect(threadId: string, folderId: string, accountId: string) {
    selectedThreadId = threadId
    selectedConversationFolderId = folderId
    selectedConversationAccountId = accountId
    showViewer()

    // Persist state
    saveUIState({
      selectedThreadId: threadId,
      selectedConversationAccountId: accountId,
      selectedConversationFolderId: folderId,
    })
  }

  // Resolve an account ID that may be 'unified' to a real account ID.
  // Returns the first real account ID if the input is 'unified' or falsy.
  function resolveAccountId(id: string | null): string | undefined {
    if (id && id !== 'unified') return id
    return accountStore.accounts[0]?.account.id
  }

  function focusListSearch() {
    messageListRef?.toggleSearchFocus()
    setFocusedPane('messageList')
  }

  // Handle compose button click (new message)
  function handleCompose() {
    // Use the selected account, or the first account if none selected
    const accountId = resolveAccountId(selectedAccountId)
    if (!accountId) return

    // Check if detached mode is preferred
    if (getComposerMode() === 'detached') {
      OpenComposerWindow(accountId, 'new', '', '', '')
      return
    }

    composerAccountId = accountId
    composerInitialMessage = null
    composerDraftId = null
    showComposer = true
  }

  // Handle edit draft (opens composer with existing draft)
  async function handleEditDraft(draftId: string) {
    // Use conversation's account ID, fall back to selected account or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    try {
      // Load the draft content from backend
      const draftMessage = await GetDraft(draftId)

      composerAccountId = accountId
      composerInitialMessage = draftMessage || null
      composerDraftId = draftId
      showComposer = true
    } catch (err) {
      console.error('Failed to load draft:', err)
      addToast({
        type: 'error',
        message: $_('composer.failedToLoadDraft'),
      })
    }
  }

  // Undo on a queued send: reopen the message from its draft. A composer
  // already open keeps its place, so the message opens in a window.
  function handleUndoSend(accountId: string, message: smtp.ComposeMessage, draftId: string) {
    if (showComposer) {
      OpenComposerWindow(accountId, '', '', draftId, '')
      return
    }
    composerAccountId = accountId
    composerInitialMessage = message
    composerDraftId = draftId
    showComposer = true
  }

  // Open a chat draft handed over by the docked chat composer's Expand
  function handleExpandChatDraft(draftId: string) {
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return
    if (getComposerMode() === 'detached' || focusMode !== 'off') {
      OpenComposerWindow(accountId, '', '', draftId, '')
      return
    }
    void handleEditDraft(draftId)
  }

  // Handle compose to a specific email address (from mailto: links in emails)
  function handleComposeToAddress(toAddress: string) {
    // Use conversation's account ID, or selected account, or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    composerAccountId = accountId
    composerDraftId = null
    // Create a minimal ComposeMessage with just the To address
    composerInitialMessage = new smtp.ComposeMessage({
      from: new smtp.Address({ name: '', address: '' }),
      to: [new smtp.Address({ name: '', address: toAddress })],
      cc: [],
      bcc: [],
      subject: '',
      text_body: '',
      html_body: '',
      attachments: [],
      request_read_receipt: false,
    })
    showComposer = true
  }

  // Handle mailto: URL data (from command line launch)
  interface MailtoData {
    to?: string[]
    cc?: string[]
    bcc?: string[]
    subject?: string
    body?: string
  }

  function handleMailtoData(data: MailtoData, rawMailtoURL?: string) {
    // Use selected account or first account (resolve 'unified' to real account)
    const accountId = resolveAccountId(selectedAccountId)
    if (!accountId) {
      // No accounts available, can't compose
      addToast({
        type: 'error',
        message: $_('toast.noAccountConfigured'),
      })
      return
    }

    // Check if detached mode is preferred for mailto
    if (getMailtoMode() === 'detached' && rawMailtoURL) {
      OpenComposerWindow(accountId, 'new', '', '', rawMailtoURL)
      return
    }

    composerAccountId = accountId
    composerDraftId = null
    composerInitialMessage = new smtp.ComposeMessage({
      from: new smtp.Address({ name: '', address: '' }),
      to: (data.to || []).map(addr => new smtp.Address({ name: '', address: addr })),
      cc: (data.cc || []).map(addr => new smtp.Address({ name: '', address: addr })),
      bcc: (data.bcc || []).map(addr => new smtp.Address({ name: '', address: addr })),
      subject: data.subject || '',
      text_body: data.body || '',
      html_body: '',
      attachments: [],
      request_read_receipt: false,
    })
    showComposer = true
  }

  // Handle reply/reply-all/forward - calls backend API
  async function handleReply(mode: 'reply' | 'reply-all' | 'forward', messageId: string, imagesLoaded?: boolean) {
    // Use conversation's account ID (important for unified inbox), fall back to selected account or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    // Force detached composer when in focus mode — preserves the focused view
    if (focusMode !== 'off') {
      OpenComposerWindow(accountId, mode, messageId, '', '')
      return
    }

    try {
      // Call backend to prepare the reply message (backend gets account from message)
      const composeMessage = await PrepareReply(messageId, mode)
      composerAccountId = accountId
      composerDraftId = null
      composerInitialMessage = composeMessage
      composerImagesLoaded = imagesLoaded || false
      showComposer = true
    } catch (err) {
      console.error(`Failed to prepare ${mode}:`, err)
      addToast({
        type: 'error',
        message: $_('toast.failedToPrepare', { values: { mode } }),
      })
      // Fallback: open blank composer
      composerAccountId = accountId
      composerDraftId = null
      composerInitialMessage = null
      showComposer = true
    }
  }

  // Close composer
  function closeComposer() {
    showComposer = false
    composerAccountId = null
    composerInitialMessage = null
  }

  // Pane sizing state
  let sidebarWidth = $state(240)
  // Mail's own collapse flag — the mail tree stays mounted under other rail
  // views, so it must not follow the active extension's flag.
  const mailSidebarHidden = $derived(isSidebarHidden('mail'))

  // A hidden sidebar can't hold pane focus — covers collapsing, switching to a
  // view whose sidebar is collapsed, and leaving narrow mode.
  $effect(() => {
    if (isSidebarHidden() && getFocusedPane() === 'sidebar') {
      setFocusedPane('messageList')
    }
  })

  let listWidth = $state(420)

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

  // Track Left Alt held state for Left Alt + Right Alt combo
  let leftAltHeld = false

  function handleGlobalKeyUp(e: KeyboardEvent) {
    if (e.code === 'AltLeft') {
      leftAltHeld = false
    }
  }

  // Global keyboard shortcut handler
  function handleGlobalKeyDown(e: KeyboardEvent) {
    // Track Left Alt press
    if (e.code === 'AltLeft') {
      leftAltHeld = true
    }
    const inInput = isInputElement(e.target)
    const focusedPane = getFocusedPane()
    const hasConversation = selectedThreadId !== null

    // Don't intercept keyboard events when a context menu or dropdown is open
    // (bits-ui portals mount [role="menu"] only while open)
    if (document.querySelector('[role="menu"]')) return

    // Alt+M/Alt+C toggle the message list's folder picker. While it's open the
    // dialog guard below swallows all keys, so close/switch is handled here.
    if (messageListRef?.isFolderPickerOpen()) {
      if (KEY.LIST_MOVE_TO(e)) {
        e.preventDefault()
        messageListRef.toggleMoveToDialog()
        return
      }
      if (KEY.LIST_COPY_TO(e)) {
        e.preventDefault()
        messageListRef.toggleCopyToDialog()
        return
      }
    }

    // Don't intercept while a modal dialog has the guard active — keystrokes
    // (especially Ctrl+A) should target dialog inputs, not the background.
    if (isDialogGuardActive()) return

    // Extension shortcut dispatch: when the active rail pane is NOT mail, let
    // the extension's registered shortcuts run first. dispatchExtensionShortcut
    // returns true when a handler matched — in that case we treat the event as
    // handled and skip mail's downstream dispatch. Skipped while typing in an
    // input element (consistent with mail's inInput guard below) so extension
    // shortcuts don't fire from inside text fields. See [[extension-sdk-pattern]]
    // and frontend/src/lib/stores/extensionShortcuts.svelte.ts.
    if (!inInput && dispatchExtensionShortcut(e)) {
      e.preventDefault()
      e.stopPropagation()
      return
    }

    // When composer is open, only handle Escape (composer handles its own shortcuts)
    if (showComposer) {
      // Block compose/reply shortcuts that might conflict
      if ((e.ctrlKey || e.metaKey) && ['r', 'f'].includes(e.key.toLowerCase())) {
        e.preventDefault()
        return
      }
      return
    }

    // Handle Ctrl/Cmd shortcuts.
    //
    // Layout: GLOBAL cases first (fire regardless of which rail pane is active),
    // then a guard that returns when an extension is the active rail pane, then
    // MAIL-DOMAIN cases (fire only on mail). The kit's components (extension UI)
    // handle their own list/sidebar shortcuts via local tabindex+keydown +
    // stopPropagation, so those events never reach this handler in the first
    // place. The guard catches the case where Ctrl+R / Ctrl+K / etc. fire
    // with no kit pane DOM-focused but an extension is the active rail pane —
    // we don't want those acting on the (hidden) mail UI.
    const isMailActive = () => getActiveExtension() === 'mail'
    if (e.ctrlKey || e.metaKey) {
      // GLOBAL Ctrl/Cmd shortcuts — fire regardless of active rail pane.
      if (KEY.SIDEBAR_TOGGLE(e)) {
        e.preventDefault()
        if (!e.repeat) toggleActiveSidebar()
        return
      }
      switch (e.key.toLowerCase()) {
        case 'q':
          e.preventDefault()
          handleQuit()
          return
        case 'tab':
        case '`': {
          // Cycle through rail items: Mail + enabled extensions.
          // Ctrl+Tab        → forward
          // Ctrl+`          → backward
          // (Ctrl+Shift+Tab is intercepted by webkit2gtk before the
          //  keydown event reaches us, so we use Ctrl+` for backward.)
          e.preventDefault()
          const tabs = getRailTabs()
          const order = ['mail', ...tabs.map(t => t.extensionId)]
          if (order.length <= 1) return // only Mail — nothing to cycle
          const current = getActiveExtension()
          const idx = order.indexOf(current)
          const step = e.key === '`' ? -1 : 1
          const next = (idx + step + order.length) % order.length
          setActiveExtension(order[next])
          return
        }
      }

      // Ctrl+S (no shift) — focus the active pane's search input.
      // Mail dispatches to messageListRef; extensions dispatch via the
      // pane-nav registry. Ctrl+Shift+S (sync folder) is mail-only and
      // stays in the mail-domain switch below.
      if (e.key.toLowerCase() === 's' && !e.shiftKey) {
        e.preventDefault()
        if (isMailActive()) {
          focusListSearch()
          return
        }
        getPaneNav('messageList')?.focusSearch?.()
        setFocusedPane('messageList')
        return
      }

      // Below: MAIL-DOMAIN Ctrl/Cmd shortcuts. Guarded so they no-op when an
      // extension is the active rail pane (otherwise Ctrl+R would silently
      // reply to the hidden mail thread while the user is browsing contacts).
      if (!isMailActive()) return

      // MAIL-DOMAIN Ctrl/Cmd cases (guarded above).
      switch (e.key.toLowerCase()) {
        case 'n':
          // Ctrl/Cmd+N — new mail composer. Mail-domain only: when an
          // extension rail is active (e.g., calendar), that extension's
          // shortcut registry handles Ctrl+N before we reach the global
          // switch, opening its own "new X" dialog.
          e.preventDefault()
          handleCompose()
          return
        case 'r': {
          if (!hasConversation) return
          e.preventDefault()
          if (focusedPane === 'viewer' && viewerRef?.hasFocusedMessage()) {
            if (e.shiftKey) {
              viewerRef.replyAll()
              return
            }
            viewerRef.reply()
            return
          }
          const msgId = getLastMessageId()
          if (!msgId) return
          handleReply(e.shiftKey ? 'reply-all' : 'reply', msgId, viewerRef?.isImagesLoaded(msgId) || false)
          return
        }
        case 'f': {
          if (!hasConversation) return
          e.preventDefault()
          if (focusedPane === 'viewer' && viewerRef?.hasFocusedMessage()) {
            viewerRef.forward()
            return
          }
          const msgId = getLastMessageId()
          if (msgId) handleReply('forward', msgId, viewerRef?.isImagesLoaded(msgId) || false)
          return
        }
        case 's':
          // Ctrl+S (no shift) is handled globally above. Only Ctrl+Shift+S
          // (sync current folder) is mail-domain and lives here.
          if (!e.shiftKey) return
          e.preventDefault()
          messageListRef?.toggleFolderSync()
          return
        case 'a':
          if (e.shiftKey) {
            // Ctrl-Shift-A: sync all accounts, or open the sync status dialog if a sync is running
            e.preventDefault()
            sidebarRef?.toggleSync()
            return
          }
          // Ctrl-A: Select all text in viewer, or select all messages in list
          e.preventDefault()
          if (focusedPane === 'viewer') {
            viewerRef?.selectAllText()
            return
          }
          messageListRef?.selectAll()
          return
        case 'l':
          e.preventDefault()
          if (e.shiftKey) {
            // Ctrl-Shift-L: Open "Always Load" dropdown
            viewerRef?.openAlwaysLoadDropdown()
          } else {
            // Ctrl-L: Load images for this message
            viewerRef?.loadImages()
          }
          return
        case 'u':
          e.preventDefault()
          {
            // Mark the keyboard-focused chat as read/unread
            const focusedIds = messageListRef?.getSelectedMessageIds() ?? []
            if (focusedIds.length > 0) {
              if (e.shiftKey) {
                handleBulkMarkUnread(focusedIds)
              } else {
                handleBulkMarkRead(focusedIds)
              }
            }
          }
          return
        case 'k':
          e.preventDefault()
          {
            // Archive the keyboard-focused chat
            const focusedIds = messageListRef?.getSelectedMessageIds() ?? []
            if (focusedIds.length > 0) {
              handleBulkArchive(focusedIds)
            }
          }
          return
        case 'j':
          e.preventDefault()
          {
            // Spam the keyboard-focused chat
            const focusedIds = messageListRef?.getSelectedMessageIds() ?? []
            if (focusedIds.length > 0) {
              handleBulkSpam(focusedIds)
            }
          }
          return
      }
      return
    }

    // Right Alt or ContextMenu key: open context menu for focused item in current pane
    // Left Alt + Right Alt: always open folder context menu regardless of pane
    if (e.key === 'ContextMenu' || (e.key === 'Alt' && e.code === 'AltRight')) {
      e.preventDefault()

      // Left Alt + Right Alt combo: always target the selected folder
      if (leftAltHeld || focusedPane === 'sidebar') {
        if (!selectedFolderId) return
        const folderEl = document.querySelector(
          `[data-sidebar-item="folder"][data-folder-id="${selectedFolderId}"], ` +
          `[data-sidebar-item="unified-account"][data-folder-id="${selectedFolderId}"]`
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

      switch (focusedPane) {
        case 'messageList': {
          messageListRef?.openContextMenu()
          focusContextMenu()
          return
        }
        case 'viewer': {
          viewerRef?.openContextMenu()
          focusContextMenu()
          return
        }
      }
      return
    }

    // Handle Alt shortcuts (pane/folder navigation, always work)
    if (e.altKey) {
      // Pane navigation is meaningless in focus mode (other panes hidden)
      if (focusMode !== 'off') return

      // Pane focus cycling (shared predicate — kit's pane components react to
      // the same focusedPane store, so cycling works uniformly across mail
      // and extensions).
      if (KEY.PANE_FOCUS_PREV(e)) {
        e.preventDefault()
        if (isResponsive()) {
          const view = getResponsiveView()
          const mode = getLayoutMode()
          if (view === 'viewer') {
            hideViewer()
            return
          }
          if (mode === 'narrow' && view === 'default') {
            showSidebar()
            return
          }
        }
        focusPreviousPane()
        return
      }
      if (KEY.PANE_FOCUS_NEXT(e)) {
        e.preventDefault()
        if (isResponsive()) {
          const view = getResponsiveView()
          const mode = getLayoutMode()
          if (mode === 'narrow' && view === 'sidebar') {
            hideSidebar()
            return
          }
          if (view === 'default' && selectedThreadId) {
            showViewer()
            return
          }
        }
        focusNextPane()
        return
      }

      // Sidebar item navigation (Alt+Up/Down/J/K). Dispatches to mail's
      // concrete ref when mail is active; otherwise to the kit pane that
      // registered as 'sidebar' via registerPaneNav. This way extensions
      // get the same "global Alt+J/K navigates the sidebar regardless of
      // which pane is currently DOM-focused" behavior mail has.
      if (KEY.SIDEBAR_PREV(e)) {
        e.preventDefault()
        if (isMailActive()) {
          sidebarRef?.selectPreviousFolder()
          return
        }
        getPaneNav('sidebar')?.navigatePrev?.()
        return
      }
      if (KEY.SIDEBAR_NEXT(e)) {
        e.preventDefault()
        if (isMailActive()) {
          sidebarRef?.selectNextFolder()
          return
        }
        getPaneNav('sidebar')?.navigateNext?.()
        return
      }

      // Alt+G / Alt+Shift+G — jump to the first sidebar item (All Inboxes
      // when present) or the last visible one (last account header when all
      // folder trees are collapsed).
      if (KEY.SIDEBAR_FIRST(e)) {
        e.preventDefault()
        if (isMailActive()) {
          sidebarRef?.selectFirstFolder()
          return
        }
        getPaneNav('sidebar')?.navigateFirst?.()
        return
      }
      if (KEY.SIDEBAR_LAST(e)) {
        e.preventDefault()
        if (isMailActive()) {
          sidebarRef?.selectLastFolder()
          return
        }
        getPaneNav('sidebar')?.navigateLast?.()
        return
      }

      // Alt+M / Alt+C — move/copy folder picker for the keyboard-focused
      // message. Open path only: the picker sets the dialog guard, so the
      // close/switch press is handled pre-guard near the top of this handler.
      if (KEY.LIST_MOVE_TO(e)) {
        if (!isMailActive()) return
        e.preventDefault()
        messageListRef?.toggleMoveToDialog()
        return
      }
      if (KEY.LIST_COPY_TO(e)) {
        if (!isMailActive()) return
        e.preventDefault()
        messageListRef?.toggleCopyToDialog()
        return
      }

      // Alt+Enter — mail sidebar expand/collapse. Keep inline switch for
      // single residual case.
      switch (e.key) {
        case 'Enter':
          // Toggle expand/collapse for focused account header or selected folder with children
          if (sidebarRef?.hasFocusedAccount()) {
            e.preventDefault()
            sidebarRef.toggleFocusedAccount()
          } else if (sidebarRef?.hasSelectedFolderWithChildren()) {
            e.preventDefault()
            sidebarRef.toggleSelectedFolderCollapse()
          }
          return
      }
      return
    }

    // Skip single-key shortcuts if in input field
    if (inInput) return

    // Handle Escape (context-dependent, progressive)
    // Focus mode first, then responsive overlays, then checkboxes, then conversation
    if (e.key === 'Escape') {
      if (focusMode !== 'off') {
        focusMode = 'off'
        focusedMessageIdInFocus = null
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
      if (focusedPane === 'viewer' && selectedThreadId) {
        messageListRef?.focusList()
        return
      }
      if (selectedThreadId) {
        // Close the open chat
        selectedThreadId = null
        selectedConversationFolderId = null
        selectedConversationAccountId = null
      }
      return
    }

    // Handle pane-focused navigation shortcuts.
    //
    // Mail-domain: these run for the mail UI's focused pane. The kit's
    // components handle their own list/sidebar navigation via local
    // keydown + stopPropagation (so the events never reach this handler).
    // Guard for safety in case an event slips through while an extension
    // is the active rail pane.
    if (KEY.LIST_PREV(e) || KEY.LIST_PREV_CHECK(e)) {
      if (!isMailActive()) return
      e.preventDefault()
      if (focusedPane === 'sidebar') {
        sidebarRef?.selectPreviousFolder()
      } else if (focusedPane === 'messageList') {
        messageListRef?.selectPrevious(opensOnMove())
      } else if (focusedPane === 'viewer') {
        // K moves to the previous chat; the arrow scrolls the open one.
        if (e.key === 'ArrowUp') viewerRef?.scrollUp()
        else messageListRef?.selectPrevious(true)
      }
      return
    }
    if (KEY.LIST_NEXT(e) || KEY.LIST_NEXT_CHECK(e)) {
      if (!isMailActive()) return
      e.preventDefault()
      if (focusedPane === 'sidebar') {
        sidebarRef?.selectNextFolder()
      } else if (focusedPane === 'messageList') {
        messageListRef?.selectNext(opensOnMove())
      } else if (focusedPane === 'viewer') {
        if (e.key === 'ArrowDown') viewerRef?.scrollDown()
        else messageListRef?.selectNext(true)
      }
      return
    }

    // Enter and Space — domain-specific button-vs-pane disambiguation logic
    // stays as inline switch (kit components handle their own Enter/Space
    // locally so they don't depend on this dispatch).
    switch (e.key) {
      case 'Enter':
        // Only let buttons handle Enter if they're in the focused pane
        // This prevents sidebar buttons from intercepting Enter when messageList is focused
        if (document.activeElement?.tagName === 'BUTTON') {
          const btn = document.activeElement as HTMLElement
          const inMessageList = btn.closest('[data-pane="messageList"]')
          const inViewer = btn.closest('[data-pane="viewer"]')
          // Only let button handle Enter if it's in the currently focused pane
          if ((focusedPane === 'messageList' && inMessageList) ||
              (focusedPane === 'viewer' && inViewer)) {
            return
          }
          // Otherwise, prevent button click and handle with our logic
          e.preventDefault()
        }
        if (focusedPane === 'sidebar' && sidebarRef?.hasFocusedAccount()) {
          e.preventDefault()
          sidebarRef.toggleFocusedAccount()
        } else if (focusedPane === 'sidebar' && sidebarRef?.hasSelectedFolderWithChildren()) {
          e.preventDefault()
          sidebarRef.toggleSelectedFolderCollapse()
        } else if (focusedPane === 'messageList' && messageListRef?.hasSelection()) {
          // Open the chat and start a reply.
          e.preventDefault()
          messageListRef.openSelected()
          viewerRef?.focusComposer()
        } else if (focusedPane === 'viewer' && selectedThreadId) {
          e.preventDefault()
          viewerRef?.focusComposer()
        }
        return
      case ' ':  // Space - toggle checkbox on focused message, or expand/collapse account
        // Only let buttons handle Space if they're in the focused pane
        if (document.activeElement?.tagName === 'BUTTON') {
          const btn = document.activeElement as HTMLElement
          const inMessageList = btn.closest('[data-pane="messageList"]')
          const inViewer = btn.closest('[data-pane="viewer"]')
          if ((focusedPane === 'messageList' && inMessageList) ||
              (focusedPane === 'viewer' && inViewer)) {
            return
          }
          e.preventDefault()
        }
        e.preventDefault()
        if (focusedPane === 'sidebar' && sidebarRef?.hasFocusedAccount()) {
          sidebarRef.toggleFocusedAccount()
        } else if (focusedPane === 'sidebar' && sidebarRef?.hasSelectedFolderWithChildren()) {
          sidebarRef.toggleSelectedFolderCollapse()
        }
        return
    }

    // g / Shift+G — jump to first / last loaded message (shared predicates —
    // kit list panes consume the same KEY.LIST_FIRST/LIST_LAST locally)
    if (KEY.LIST_FIRST(e)) {
      if (!isMailActive()) return
      if (focusedPane !== 'messageList') return
      e.preventDefault()
      messageListRef?.selectFirst()
      return
    }
    if (KEY.LIST_LAST(e)) {
      if (!isMailActive()) return
      if (focusedPane !== 'messageList') return
      e.preventDefault()
      messageListRef?.selectLast()
      return
    }

    // Chat triage keys (V here is Move, not the kit lists' V = open).
    if (isMailActive() && handleChatKey(e, hasConversation)) {
      e.preventDefault()
      return
    }

    // Single-key shortcuts
    switch (e.key) {
      case 's':
        {
          // Toggle star on the keyboard-focused chat
          const focusedIds = messageListRef?.getSelectedMessageIds() ?? []
          if (focusedIds.length > 0) {
            const isStarred = messageListRef?.isSelectedStarred() ?? false
            handleBulkToggleStar(focusedIds, !isStarred)
          }
        }
        return
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
        if (focusMode === 'message') {
          focusMode = 'off'
          focusedMessageIdInFocus = null
          return
        }
        const targetId = (focusedPane === 'viewer' && viewerRef?.hasFocusedMessage())
          ? viewerRef.getFocusedMessageId()
          : getLastMessageId()
        if (!targetId) return
        focusMode = 'message'
        focusedMessageIdInFocus = targetId
        return
      }
      case 'd': // alias of Delete: move focused/checked message(s) to Trash
      case 'Backspace':
      case 'Delete': {
        if (focusedPane === 'viewer' && viewerRef?.hasFocusedMessage()) {
          if (e.shiftKey) {
            viewerRef.deletePermanently()
            return
          }
          viewerRef.trash()
          return
        }
        const focusedMessageIds = messageListRef?.getSelectedMessageIds() ?? []
        if (focusedMessageIds.length > 0) {
          messageListRef?.requestDelete(focusedMessageIds, e.shiftKey)
        }
        return
      }
    }
  }

  // Chat list J/K opens each chat as it is selected, except in the narrow
  // layout, where opening would cover the list.
  function opensOnMove(): boolean {
    return getLayoutMode() !== 'narrow'
  }

  // handleChatKey runs the chat triage shortcuts. They act on the chat
  // selected in the list (the open one, after J/K or a click), falling back
  // to the open chat. Returns whether the key was handled.
  function handleChatKey(e: KeyboardEvent, hasConversation: boolean): boolean {
    const list = messageListRef
    const hasSelection = !!list?.hasSelection()
    if (KEY.CHAT_REPLY(e)) {
      if (!hasConversation) return false
      viewerRef?.focusComposer(e.shiftKey)
      return true
    }
    if (KEY.CHAT_SEARCH(e)) {
      focusListSearch()
      return true
    }
    if (KEY.CHAT_DONE(e)) {
      if (hasSelection) list!.archiveSelected()
      else if (hasConversation) viewerRef?.archive()
      return hasSelection || hasConversation
    }
    if (KEY.CHAT_SNOOZE(e)) {
      if (hasSelection) list!.openSnoozeForSelected()
      else if (hasConversation) viewerRef?.openSnooze()
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

  // Get the last message ID from the current conversation (for reply/forward)
  function getLastMessageId(): string | null {
    return viewerRef?.getLastMessageId() ?? null
  }

  // Handle click on pane to set focus
  function handlePaneClick(pane: FocusablePane) {
    setFocusedPane(pane)
  }

  // Bulk action handlers
  async function handleBulkArchive(messageIds: string[]) {
    try {
      await Archive(messageIds)
      addToast({ type: 'success', message: $_('toast.archived'), actions: [{ label: $_('common.undo'), onClick: handleUndo }] })
      messageListRef?.handleActionComplete(true)
    } catch (err) {
      console.error('Archive failed:', err)
      addToast({ type: 'error', message: $_('toast.failedToArchive') })
    }
  }

  async function handleBulkSpam(messageIds: string[]) {
    try {
      const isSpamFolder = selectedFolderType === 'spam'

      if (isSpamFolder) {
        // If we're in spam folder, mark as NOT spam
        await MarkAsNotSpam(messageIds)
        addToast({ type: 'success', message: $_('toast.markedAsNotSpam'), actions: [{ label: $_('common.undo'), onClick: handleUndo }] })
      } else {
        // Otherwise, mark as spam
        await MarkAsSpam(messageIds)
        addToast({ type: 'success', message: $_('toast.markedAsSpam'), actions: [{ label: $_('common.undo'), onClick: handleUndo }] })
      }

      messageListRef?.handleActionComplete(true)
    } catch (err) {
      const isSpamFolder = selectedFolderType === 'spam'
      console.error('Spam toggle failed:', err)
      addToast({ type: 'error', message: $_(isSpamFolder ? 'toast.failedToMarkAsNotSpam' : 'toast.failedToMarkAsSpam') })
    }
  }

  async function handleBulkMarkRead(messageIds: string[]) {
    try {
      await MarkAsRead(messageIds)
      addToast({ type: 'success', message: $_('toast.markedAsRead') })
      messageListRef?.handleActionComplete()
    } catch (err) {
      console.error('Mark as read failed:', err)
      addToast({ type: 'error', message: $_('toast.failedToMarkAsRead') })
    }
  }

  async function handleBulkMarkUnread(messageIds: string[]) {
    try {
      await MarkAsUnread(messageIds)
      addToast({ type: 'success', message: $_('toast.markedAsUnread') })
      messageListRef?.handleActionComplete()
    } catch (err) {
      console.error('Mark as unread failed:', err)
      addToast({ type: 'error', message: $_('toast.failedToMarkAsUnread') })
    }
  }

  async function handleBulkToggleStar(messageIds: string[], shouldStar: boolean) {
    try {
      if (shouldStar) {
        await Star(messageIds)
        addToast({ type: 'success', message: $_('toast.starred') })
      } else {
        await Unstar(messageIds)
        addToast({ type: 'success', message: $_('toast.starRemoved') })
      }
      messageListRef?.handleActionComplete()
    } catch (err) {
      console.error('Star toggle failed:', err)
      addToast({ type: 'error', message: $_('toast.failedToUpdateStar') })
    }
  }

  async function handleUndo() {
    try {
      const description = await Undo()
      addToast({ type: 'success', message: $_('toast.undone', { values: { description } }) })
      messageListRef?.handleActionComplete()
    } catch (err) {
      console.error('Undo failed:', err)
      addToast({ type: 'error', message: $_('toast.undoFailed') })
    }
  }
</script>

<svelte:window onkeydown={handleGlobalKeyDown} onkeyup={handleGlobalKeyUp} />

<div class="flex flex-col h-full w-full overflow-hidden bg-background">
  <!-- Custom Title Bar -->
  {#if getShowTitleBar() && !getNativeTitleBar()}
    <TitleBar onClose={handleClose} />
  {/if}

  <SnapshotRestoreBanner />

  <!-- Main Content -->
  <div class="flex flex-1 min-h-0 overflow-hidden relative">
    <ExtensionRail />

    {#if getActiveExtension() === 'contacts'}
      <ContactsPane />
    {/if}

    {#if getActiveExtension() === 'calendar'}
      <CalendarPane />
    {/if}

    <!-- Mail layout is ALWAYS mounted; only its visibility is toggled when an
         extension takes over the pane. Unmounting+remounting the mail tree on
         every extension switch was leaking state (zombie listeners) and pinning
         the main thread on the second mount. display:contents keeps the flex
         children as direct flex items so the layout doesn't shift. -->
    <div style:display={getActiveExtension() === 'mail' ? 'contents' : 'none'}>
    <!-- Sidebar (Folder List) -->
    <aside
      inert={getLayoutMode() === 'narrow' && getResponsiveView() !== 'sidebar'}
      class="{getLayoutMode() === 'narrow' ? `responsive-sidebar-overlay w-72 border-r border-divider bg-background ${getResponsiveView() === 'sidebar' ? 'responsive-sidebar-visible' : ''}` : 'shrink-0 border-r border-divider bg-surface-sidebar'} {mailSidebarHidden ? 'hidden' : ''}"
      style="{getLayoutMode() !== 'narrow' ? `width: ${sidebarWidth}px` : ''}"
      role="presentation"
      onclick={() => handlePaneClick('sidebar')}
    >
      <Sidebar
        bind:this={sidebarRef}
        onFolderSelect={handleFolderSelect}
        onUnifiedFolderSelect={handleUnifiedFolderSelect}
        onUnifiedInboxSelect={handleUnifiedInboxSelect}
        onMessagesMoved={() => messageListRef?.handleActionComplete(false)}
        onCompose={handleCompose}
        onSearch={focusListSearch}
        selectedAccountId={selectedAccountId}
        selectedFolderId={selectedFolderId}
        selectionSource={selectionSource}
        isFocused={getFocusedPane() === 'sidebar'}
        isFlashing={isPaneFlashing('sidebar')}
        showBackButton={getLayoutMode() === 'narrow'}
        onBack={hideSidebar}
      />
    </aside>

    <!-- Scrim for narrow sidebar overlay -->
    {#if getLayoutMode() === 'narrow'}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        role="button"
        tabindex="-1"
        class="responsive-scrim {getResponsiveView() === 'sidebar' ? 'responsive-scrim-visible' : ''}"
        onclick={hideSidebar}
        aria-label={$_('aria.closeSidebar')}
      ></div>
    {/if}

    <!-- Sidebar Resize Handle -->
    {#if getLayoutMode() !== 'narrow' && !mailSidebarHidden}
      <PaneResizeHandle
        width={sidebarWidth}
        kind="sidebar"
        label={$_('aria.resizeSidebar')}
        onresize={(w) => { sidebarWidth = w }}
        oncommit={(w) => saveUIState({ sidebarWidth: w })}
      />
    {/if}

    <!-- Chat List -->
    <section
      bind:this={messageListContainerRef}
      class="{isResponsive() ? 'flex-1 min-w-0' : 'pane-list-resizable'} border-r border-divider bg-surface-list"
      style="{getLayoutMode() === 'full' ? `width: ${listWidth}px` : ''}"
      role="presentation"
      data-pane="messageList"
      tabindex="-1"
      onclick={() => handlePaneClick('messageList')}
    >
      <ChatList
        bind:this={messageListRef}
        accountId={selectedAccountId}
        folderId={selectedFolderId}
        folderName={selectedFolderName}
        folderType={selectedFolderType || 'inbox'}
        onConversationSelect={handleConversationSelect}
        onReply={handleReply}
        onRowActionComplete={() => viewerRef?.refreshFlags()}
        onCompose={handleCompose}
        onUnifiedInboxSelect={handleUnifiedInboxSelect}
        onFolderSelect={handleFolderSelect}
        isFocused={getFocusedPane() === 'messageList'}
        isFlashing={isPaneFlashing('messageList')}
        openThreadId={selectedThreadId}
        openAccountId={selectedConversationAccountId}
      />
    </section>

    <!-- List Resize Handle -->
    {#if getLayoutMode() === 'full'}
      <PaneResizeHandle
        width={listWidth}
        kind="list"
        label={$_('aria.resizeMessageList')}
        onresize={(w) => { listWidth = w }}
        oncommit={(w) => saveUIState({ listWidth: w })}
      />
    {/if}

    <!-- Conversation Viewer -->
    <main
      bind:this={viewerContainerRef}
      inert={viewerIsClosed}
      class="{viewerIsOverlay ? `responsive-viewer-overlay bg-surface-content ${viewerIsVisible ? 'responsive-viewer-visible' : ''}` : 'pane-detail bg-surface-content'}"
      role="presentation"
      data-pane="viewer"
      onclick={() => handlePaneClick('viewer')}
    >
      <ChatView
        bind:this={viewerRef}
        threadId={selectedThreadId}
        folderId={selectedConversationFolderId}
        folderType={selectedFolderType}
        accountId={selectedConversationAccountId}
        showAccount={selectedAccountId === 'unified' && selectedFolderId === 'inbox'}
        onReply={handleReply}
        onCompose={viewerIsOverlay ? handleCompose : undefined}
        onComposeToAddress={handleComposeToAddress}
        onEditDraft={handleEditDraft}
        onExpandDraft={handleExpandChatDraft}
        onActionComplete={(autoSelectNext) => messageListRef?.handleActionComplete(autoSelectNext)}
        onToggleSenderChat={() => messageListRef?.toggleSelectedSenderChat()}
        isFocused={getFocusedPane() === 'viewer'}
        isFlashing={isPaneFlashing('viewer')}
        showBackButton={isResponsive()}
        onBack={() => { focusMode = 'off'; focusedMessageIdInFocus = null; hideViewer() }}
        onEscape={() => {
          if (isResponsive() && getResponsiveView() === 'viewer') hideViewer()
          messageListRef?.focusList()
        }}
        focusedMessageIdInFocus={focusMode === 'message' ? focusedMessageIdInFocus : null}
      />
    </main>
    </div>
  </div>
</div>

<!-- Toast notifications -->
<ToastContainer />

<!-- Spellcheck right-click suggestion menu (composer) -->
<SpellSuggestionMenu />

<!-- Per-extension settings dialog dispatcher (Settings → Extensions → Edit) -->
<ExtensionSettingsDialog />

<!-- Composer Modal -->
{#if showComposer && composerAccountId}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
    <div class="{getLayoutMode() === 'narrow' ? 'w-full h-full bg-background overflow-hidden' : 'w-full max-w-3xl h-[80vh] bg-background rounded-lg shadow-xl overflow-hidden'}">
      <Composer
        accountId={composerAccountId}
        initialMessage={composerInitialMessage}
        draftId={composerDraftId}
        imagesLoaded={composerImagesLoaded}
        onClose={closeComposer}
        onSent={closeComposer}
        onUndoSend={handleUndoSend}
      />
    </div>
  </div>
{/if}

<!-- Shutdown Overlay -->
{#if isShuttingDown}
  <div class="fixed inset-0 z-100 flex items-center justify-center bg-black/80">
    <p class="text-white/90 text-sm font-medium">{$_('window.shuttingDown')}</p>
  </div>
{/if}

<!-- Certificate TOFU Dialog (for background sync cert errors) -->
<CertificateDialog
  bind:open={showCertDialog}
  certificate={pendingCertificate}
  onAcceptOnce={() => handleBgCertAccept(false)}
  onAcceptPermanently={() => handleBgCertAccept(true)}
  onDecline={handleBgCertDecline}
/>

<!-- Flatpak Filesystem Permission Dialog -->
<AlertDialog.Root bind:open={showFlatpakFsDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('attachment.flatpakOpenTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        <p class="mb-3">{flatpakFsDialogContext === 'saveAll' ? $_('attachment.flatpakSaveAllDescription') : $_('attachment.flatpakOpenDescription')}</p>
        <pre class="mb-3 rounded bg-muted p-2 text-sm overflow-x-auto"><code>flatpak override --user --filesystem=home io.github.caseylabs.Mailven</code></pre>
        <p class="mb-3 text-sm text-destructive">{$_('attachment.flatpakOpenSecurityWarning')}</p>
        <p class="text-sm text-muted-foreground">{flatpakFsDialogContext === 'saveAll' ? $_('attachment.flatpakSaveAllAlternative') : $_('attachment.flatpakOpenAlternative')}</p>
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Action onclick={() => showFlatpakFsDialog = false}>{$_('common.ok')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
