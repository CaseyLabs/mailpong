<script lang="ts">
  // Load offline icon data before anything else
  import './lib/iconify-offline'

  import { onMount, untrack } from 'svelte'
  import TitleBar from './lib/components/common/TitleBar.svelte'
  import SnapshotRestoreBanner from './lib/components/common/SnapshotRestoreBanner.svelte'
  import AppOverlays from './lib/components/common/AppOverlays.svelte'
  import Sidebar from './lib/components/sidebar/Sidebar.svelte'
  import PaneResizeHandle from '$lib/components/kit/PaneResizeHandle.svelte'
  import ChatList from './lib/components/chat/ChatList.svelte'
  import ChatView from './lib/components/chat/ChatView.svelte'
  import Composer from './lib/components/composer/Composer.svelte'
  import ToastContainer from './lib/components/ui/toast/ToastContainer.svelte'
  import SpellSuggestionMenu from './lib/spellcheck/SpellSuggestionMenu.svelte'
  import ExtensionSettingsDialog from './lib/components/settings/ExtensionSettingsDialog.svelte'
  import ExtensionRail from './lib/components/rail/ExtensionRail.svelte'
  import ContactsPane from '$extensions/contacts/frontend/components/ContactsPane.svelte'
  import CalendarPane from '$extensions/calendar/frontend/components/CalendarPane.svelte'
  import { getShowTitleBar, getNativeTitleBar, getThemeMode } from '$lib/stores/settings.svelte'
  import { applyThemeFromMode } from '$lib/stores/theme.svelte'
  import { saveUIState, getActiveExtension } from '$lib/stores/uiState.svelte'
  import { type FocusablePane, getFocusedPane, setFocusedPane, isPaneFlashing, setComposerOpen } from '$lib/stores/keyboard.svelte'
  import { registerViewerSelection, getLayoutMode, getResponsiveView, hideViewer, hideSidebar, isResponsive, isSidebarHidden } from '$lib/stores/layout.svelte'
  import {
    mailSelection,
    selectFolder,
    selectUnifiedInbox,
    selectConversation,
    exitFocusMode,
  } from '$lib/stores/mailSelection.svelte'
  import {
    composer,
    closeComposer,
    openNewMessage,
    openReply,
    editDraft,
    undoSend,
    expandChatDraft,
    composeToAddress,
  } from '$lib/stores/appComposer.svelte'
  import { shutdown } from '$lib/stores/shutdown.svelte'
  import { createBulkActions } from '$lib/keyboard/bulkActions'
  import { createGlobalShortcuts } from '$lib/keyboard/globalShortcuts'
  import { startApp } from '$lib/appStartup'
  // @ts-ignore - wailsjs path
  import { CloseWindow, QuitApp } from '../wailsjs/go/app/App.js'
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
  let viewerContainerRef: HTMLElement | null = null

  // Pane sizing state
  let sidebarWidth = $state(240)
  let listWidth = $state(420)
  // Mail's own collapse flag — the mail tree stays mounted under other rail
  // views, so it must not follow the active extension's flag.
  const mailSidebarHidden = $derived(isSidebarHidden('mail'))

  function focusListSearch() {
    messageListRef?.toggleSearchFocus()
    setFocusedPane('messageList')
  }

  // Ctrl+Q always quits, regardless of background mode.
  function quit() {
    shutdown.active = true
    setTimeout(() => QuitApp(), 100)
  }

  const shortcuts = createGlobalShortcuts({
    sidebar: () => sidebarRef,
    list: () => messageListRef,
    viewer: () => viewerRef,
    bulk: createBulkActions(() => messageListRef, () => mailSelection.folderType),
    quit,
    focusListSearch,
  })

  // React to theme mode changes from settings store
  $effect(() => {
    applyThemeFromMode(getThemeMode())
  })

  // Mirror composer visibility into the keyboard store so the viewer can
  // suppress its Delete/Backspace shortcut during the composer's mount→focus race.
  $effect(() => {
    setComposerOpen(composer.show)
  })

  // A selected conversation (restored at startup or picked in full mode or
  // before switching views) reopens as the viewer overlay in responsive
  // layouts; see registerViewerSelection.
  registerViewerSelection('mail', () => mailSelection.threadId !== null)

  const viewerIsOverlay = $derived(isResponsive() || mailSelection.focusMode !== 'off')
  const viewerIsVisible = $derived(getResponsiveView() === 'viewer' || mailSelection.focusMode !== 'off')
  // A closed viewer overlay is inert so its controls cannot take keyboard focus.
  const viewerIsClosed = $derived(viewerIsOverlay && !viewerIsVisible)

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

  // Auto-reset focus mode when the conversation changes (or is closed).
  // Prevents focus state from leaking across navigation.
  $effect(() => {
    void mailSelection.threadId
    exitFocusMode()
  })

  // Route keyboard pane focus to the viewer while in focus mode so j/k/arrow
  // shortcuts scroll the viewer, then restore the prior pane on exit.
  let focusedPaneBeforeFocusMode: FocusablePane | null = null
  $effect(() => {
    const mode = mailSelection.focusMode
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

  // A hidden sidebar can't hold pane focus — covers collapsing, switching to a
  // view whose sidebar is collapsed, and leaving narrow mode.
  $effect(() => {
    if (isSidebarHidden() && getFocusedPane() === 'sidebar') {
      setFocusedPane('messageList')
    }
  })

  onMount(() => {
    void startApp({
      selectThreadInList: (threadId) => messageListRef?.selectThread(threadId),
      focusMessageList: () => messageListContainerRef?.focus(),
      applyUIState: (uiState) => {
        sidebarWidth = uiState.sidebarWidth
        listWidth = uiState.listWidth
      },
    })
  })

  // Folder selection from the account tree or the unified section. The list
  // supplies the path too, which the selection does not need.
  function handleFolderSelect(accountId: string, folderId: string, _path: string, folderName: string, folderType: string) {
    selectFolder(accountId, folderId, folderName, folderType, 'account')
  }

  function handleUnifiedFolderSelect(accountId: string, folderId: string, _path: string, folderName: string, folderType: string) {
    selectFolder(accountId, folderId, folderName, folderType, 'unified')
  }

  // Handle click on pane to set focus
  function handlePaneClick(pane: FocusablePane) {
    setFocusedPane(pane)
  }
</script>

<svelte:window onkeydown={shortcuts.onKeyDown} onkeyup={shortcuts.onKeyUp} />

<div class="flex flex-col h-full w-full overflow-hidden bg-background">
  <!-- Custom Title Bar -->
  {#if getShowTitleBar() && !getNativeTitleBar()}
    <TitleBar onClose={() => CloseWindow()} />
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
        onUnifiedInboxSelect={selectUnifiedInbox}
        onMessagesMoved={() => messageListRef?.handleActionComplete(false)}
        onCompose={openNewMessage}
        onSearch={focusListSearch}
        selectedAccountId={mailSelection.accountId}
        selectedFolderId={mailSelection.folderId}
        selectionSource={mailSelection.source}
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
        accountId={mailSelection.accountId}
        folderId={mailSelection.folderId}
        folderName={mailSelection.folderName}
        folderType={mailSelection.folderType || 'inbox'}
        onConversationSelect={selectConversation}
        onReply={openReply}
        onRowActionComplete={() => viewerRef?.refreshFlags()}
        onCompose={openNewMessage}
        onAddAccount={() => sidebarRef?.openAddAccount()}
        onUnifiedInboxSelect={selectUnifiedInbox}
        onFolderSelect={handleFolderSelect}
        isFocused={getFocusedPane() === 'messageList'}
        isFlashing={isPaneFlashing('messageList')}
        openThreadId={mailSelection.threadId}
        openAccountId={mailSelection.conversationAccountId}
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
        threadId={mailSelection.threadId}
        folderId={mailSelection.conversationFolderId}
        folderType={mailSelection.folderType}
        accountId={mailSelection.conversationAccountId}
        showAccount={mailSelection.accountId === 'unified' && mailSelection.folderId === 'inbox'}
        onReply={openReply}
        onCompose={viewerIsOverlay ? openNewMessage : undefined}
        onComposeToAddress={composeToAddress}
        onEditDraft={editDraft}
        onExpandDraft={expandChatDraft}
        onActionComplete={(autoSelectNext) => messageListRef?.handleActionComplete(autoSelectNext)}
        onToggleSenderChat={() => messageListRef?.toggleSelectedSenderChat()}
        isFocused={getFocusedPane() === 'viewer'}
        isFlashing={isPaneFlashing('viewer')}
        showBackButton={isResponsive()}
        onBack={() => { exitFocusMode(); hideViewer() }}
        onEscape={() => {
          if (isResponsive() && getResponsiveView() === 'viewer') hideViewer()
          messageListRef?.focusList()
        }}
        focusedMessageIdInFocus={mailSelection.focusMode === 'message' ? mailSelection.focusedMessageId : null}
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
{#if composer.show && composer.accountId}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
    <div class="{getLayoutMode() === 'narrow' ? 'w-full h-full bg-background overflow-hidden' : 'w-full max-w-3xl h-[80vh] bg-background rounded-lg shadow-xl overflow-hidden'}">
      <Composer
        accountId={composer.accountId}
        initialMessage={composer.initialMessage}
        draftId={composer.draftId}
        imagesLoaded={composer.imagesLoaded}
        onClose={closeComposer}
        onSent={closeComposer}
        onUndoSend={undoSend}
      />
    </div>
  </div>
{/if}


<!-- Shutdown cover, certificate prompts and the Flatpak permission notice -->
<AppOverlays />
