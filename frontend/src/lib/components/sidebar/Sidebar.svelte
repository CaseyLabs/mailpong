<script lang="ts">
  import SyncStatusDialog from '../list/SyncStatusDialog.svelte'
  import Icon from '@iconify/svelte'
  import { onMount } from 'svelte'
  import AccountSection from './AccountSection.svelte'
  import UnifiedInboxSection from './UnifiedInboxSection.svelte'
  import SidebarActions from './SidebarActions.svelte'
  import SidebarSyncFooter from './SidebarSyncFooter.svelte'
  import SidebarEmptyState from './SidebarEmptyState.svelte'
  import {
    buildFolderNavList as buildNavList,
    getCurrentFolderIndex as findCurrentIndex,
    navItemSelector,
    pruneCollapsedFolders,
    computeSyncStatus,
    type FolderNavItem,
  } from './sidebarNav'
  import AccountDialog from '$lib/components/settings/AccountDialog.svelte'
  import DeleteAccountDialog from '$lib/components/settings/DeleteAccountDialog.svelte'
  import { accountStore, findFolderNode } from '$lib/stores/accounts.svelte'
  import { contactSourcesStore } from '$lib/stores/contactSources.svelte'
  import { isAccountExpanded, setAccountExpanded, setFolderCollapsed, getUIState, getUIStateVersion, saveUIState } from '$lib/stores/uiState.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetUnifiedInboxUnreadCount } from '../../../../wailsjs/go/app/App'
  import { EventsOn } from '../../../../wailsjs/runtime/runtime'

  // Track focused account header for keyboard navigation
  let focusedAccountId = $state<string | null>(null)

  // Ref to scrollable container for auto-scroll
  let scrollContainer: HTMLDivElement | null = null

  // Track expanded state for each account (reactive, synced with persisted state)
  let expandedAccounts = $state<Record<string, boolean>>({})

  // Initialize expanded state from persisted storage
  // Depends on both accounts list AND UI state version (so it re-runs when persisted state loads)
  $effect(() => {
    // Read version to create dependency - effect re-runs when UI state finishes loading
    const _version = getUIStateVersion()

    const newExpanded: Record<string, boolean> = {}
    for (const acc of accountStore.accounts) {
      newExpanded[acc.account.id] = isAccountExpanded(acc.account.id)
    }
    expandedAccounts = newExpanded
  })

  // Toggle account expansion
  function toggleAccountExpanded(accountId: string) {
    const newValue = !expandedAccounts[accountId]
    expandedAccounts[accountId] = newValue
    setAccountExpanded(accountId, newValue)
  }

  // Track collapsed state for folders with children (reactive, synced with persisted state)
  let collapsedFolders = $state<Record<string, boolean>>({})

  // Initialize collapsed state from persisted storage and prune stale entries
  $effect(() => {
    const _version = getUIStateVersion()

    // Read persisted collapsed state, keep only entries for existing folders
    const { newCollapsed, hasStale } = pruneCollapsedFolders(getUIState().collapsedFolders)
    collapsedFolders = newCollapsed

    // Persist cleaned state if stale entries were pruned
    if (hasStale) {
      saveUIState({ collapsedFolders: newCollapsed })
    }
  })

  // Toggle folder collapse
  function toggleFolderCollapsed(folderId: string) {
    const isCurrentlyCollapsed = collapsedFolders[folderId] !== false
    const newValue = !isCurrentlyCollapsed
    collapsedFolders = { ...collapsedFolders, [folderId]: newValue }
    setFolderCollapsed(folderId, newValue)
  }

  interface Props {
    onFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onUnifiedFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onUnifiedInboxSelect?: () => void
    onMessagesMoved?: () => void
    onCompose?: () => void
    onSearch?: () => void
    selectedAccountId?: string | null
    selectedFolderId?: string | null
    selectionSource?: 'unified' | 'account' | null
    isFocused?: boolean
    isFlashing?: boolean
    showBackButton?: boolean
    onBack?: () => void
  }

  let {
    onFolderSelect,
    onUnifiedFolderSelect,
    onUnifiedInboxSelect,
    onMessagesMoved,
    onCompose,
    onSearch,
    selectedAccountId = null,
    selectedFolderId = null,
    selectionSource = null,
    isFocused: _isFocused = false,
    isFlashing = false,
    showBackButton = false,
    onBack,
  }: Props = $props()

  // Unified inbox state
  let unifiedUnreadCount = $state(0)

  // Dialog state
  let showAccountDialog = $state(false)
  let showDeleteDialog = $state(false)
  let editingAccount = $state<account.Account | null>(null)
  let deletingAccount = $state<account.Account | null>(null)

  // Load accounts and contact sources on mount
  onMount(() => {
    // Load accounts, then trigger comprehensive sync on launch
    accountStore.load().then(async () => {
      try {
        await accountStore.syncAllComplete()
      } catch (err) {
        console.error('Failed to sync on launch:', err)
      }
    })

    contactSourcesStore.load()
    loadUnifiedInboxCount()

    // Listen for folder count changes to update unified inbox count
    const unsubscribe = EventsOn('folders:countsChanged', (_data: Record<string, number>) => {
      loadUnifiedInboxCount()
    })

    return () => {
      unsubscribe()
    }
  })

  // Load unified inbox unread count
  async function loadUnifiedInboxCount() {
    try {
      const count = await GetUnifiedInboxUnreadCount()
      unifiedUnreadCount = count
    } catch (err) {
      console.error('Failed to load unified inbox count:', err)
    }
  }

  // Get accounts with their inbox folders for unified inbox section
  function getAccountsWithInbox() {
    return accountStore.accounts.map(acc => {
      return {
        account: acc.account,
        inbox: accountStore.getInbox(acc.account.id) ?? null
      }
    })
  }

  // Handle unified inbox selection (All Inboxes)
  function handleUnifiedInboxSelect() {
    onUnifiedInboxSelect?.()
  }

  // Handle individual account inbox selection from unified section
  function handleAccountInboxSelect(accountId: string, folderId: string, folderPath: string) {
    onUnifiedFolderSelect?.(accountId, folderId, folderPath, 'Inbox', 'inbox')
  }

  // Sync status: { accountName, label, percentage } — accountName + percentage are populated only when a sync is active
  let syncStatus = $derived.by(() => computeSyncStatus($_))


  // Handle folder selection
  function handleFolderSelect(accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) {
    accountStore.selectFolder(accountId, folderId, folderPath, folderName)
    onFolderSelect?.(accountId, folderId, folderPath, folderName, folderType)
  }

  // Open add account dialog
  function openAddAccount() {
    editingAccount = null
    showAccountDialog = true
  }

  // Open edit account dialog
  function openEditAccount(acc: account.Account) {
    editingAccount = acc
    showAccountDialog = true
  }

  // Open delete confirmation
  function openDeleteAccount(acc: account.Account) {
    deletingAccount = acc
    showDeleteDialog = true
  }

  // Sync all accounts (comprehensive sync)
  export async function syncAllAccounts() {
    try {
      await accountStore.syncAllComplete()
    } catch (err) {
      console.error('Sync failed:', err)
      // Error is already stored in account store
    }
  }

  let syncStatusOpen = $state(false)

  // The Sync button: start a sync, or while one runs open its status dialog,
  // where it can be cancelled.
  function onSyncButton() {
    if (accountStore.isAnySyncing) syncStatusOpen = true
    else void syncAllAccounts()
  }

  // Cancel all running syncs
  async function cancelSync() {
    try {
      await accountStore.cancelAllSyncs()
    } catch (err) {
      console.error('Failed to cancel sync:', err)
    }
  }

  // Keyboard shortcut: same as the Sync button
  export function toggleSync() {
    onSyncButton()
  }

  // Build flat list of all navigable folders including Unified Inbox
  function buildFolderNavList(): FolderNavItem[] {
    return buildNavList({ expandedAccounts, collapsedFolders })
  }

  // Get current folder index in navigation list
  function getCurrentFolderIndex(): number {
    return findCurrentIndex(buildFolderNavList(), { focusedAccountId, selectedAccountId, selectedFolderId, selectionSource })
  }

  // Navigate to previous folder (exposed for keyboard navigation)
  export function selectPreviousFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex <= 0 ? 0 : currentIndex - 1

    selectFolderByIndex(navList, newIndex)
  }

  // Navigate to next folder (exposed for keyboard navigation)
  export function selectNextFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex >= navList.length - 1 ? navList.length - 1 : currentIndex + 1

    selectFolderByIndex(navList, newIndex)
  }

  // Jump to the top sidebar item — All Inboxes when present (Alt+G)
  export function selectFirstFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return
    selectFolderByIndex(navList, 0)
  }

  // Jump to the last visible item: last folder of the last expanded account,
  // or the last account header when all trees are collapsed (Alt+Shift+G)
  export function selectLastFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return
    selectFolderByIndex(navList, navList.length - 1)
  }

  // Scroll an item into view
  function scrollItemIntoView(item: FolderNavItem) {
    if (!scrollContainer) return

    const selector = navItemSelector(item)

    if (selector) {
      const element = scrollContainer.querySelector(selector)
      element?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }

  // Select folder by index in nav list
  function selectFolderByIndex(navList: FolderNavItem[], index: number) {
    const item = navList[index]
    if (!item) return

    // Clear account header focus when selecting a folder
    if (item.type !== 'account-header') {
      focusedAccountId = null
    }

    if (item.type === 'unified') {
      onUnifiedInboxSelect?.()
    } else if (item.type === 'unified-account' && item.accountId && item.folderId && item.folderPath) {
      // Select from unified section - uses onUnifiedFolderSelect
      onUnifiedFolderSelect?.(item.accountId, item.folderId, item.folderPath, item.folderName, item.folderType || 'inbox')
    } else if (item.type === 'account-header' && item.accountId) {
      // Focus on account header (Enter/Space will toggle expand)
      focusedAccountId = item.accountId
    } else if (item.type === 'folder' && item.accountId && item.folderId && item.folderPath) {
      // Select from account tree - uses handleFolderSelect
      handleFolderSelect(item.accountId, item.folderId, item.folderPath, item.folderName, item.folderType || 'folder')
    }

    // Scroll the selected item into view
    scrollItemIntoView(item)
  }

  // Toggle expand/collapse for the focused account (called on Enter/Space/Alt+Enter)
  export function toggleFocusedAccount() {
    if (focusedAccountId) {
      toggleAccountExpanded(focusedAccountId)
    }
  }

  // Check if an account header is focused
  export function hasFocusedAccount(): boolean {
    return focusedAccountId !== null
  }

  // Check if the currently selected folder has children
  export function hasSelectedFolderWithChildren(): boolean {
    if (!selectedFolderId || selectionSource !== 'account') return false
    return folderHasChildren(selectedFolderId)
  }

  // Toggle collapse for the currently selected folder
  export function toggleSelectedFolderCollapse(): void {
    if (!selectedFolderId || selectionSource !== 'account') return
    if (!folderHasChildren(selectedFolderId)) return
    toggleFolderCollapsed(selectedFolderId)
  }

  // Check if a folder has children by searching the account folder trees
  function folderHasChildren(folderId: string): boolean {
    for (const acc of accountStore.accounts) {
      const found = findFolderNode(acc.folders || [], (f) => f.id === folderId)
      if (found) return (found.children && found.children.length > 0) || false
    }
    return false
  }
</script>
<div class="flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <!-- Header: title and close button for the narrow-layout overlay -->
  {#if showBackButton}
    <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-border">
      <h2 class="text-body font-semibold text-fg-primary">{$_('sidebar.accountsTitle')}</h2>
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors shrink-0"
        title={$_('responsive.back')}
        aria-label={$_('aria.closeSidebar')}
        onclick={onBack}
      >
        <Icon icon="mdi:close" class="w-5 h-5 text-muted-foreground" />
      </button>
    </div>
  {/if}

  <!-- Account List -->
  <div class="flex-1 overflow-y-auto scrollbar-thin py-2" bind:this={scrollContainer}>
    {#if accountStore.loading}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else if accountStore.accounts.length === 0}
      <!-- Empty State -->
      <SidebarEmptyState onAddAccount={openAddAccount} />
    {:else}
      {#if onCompose && onSearch}<SidebarActions {onCompose} {onSearch} />{/if}

      <!-- Unified Inbox Section (only show if more than 1 account) -->
      {#if accountStore.accounts.length > 1}
        <UnifiedInboxSection
          accounts={getAccountsWithInbox()}
          {unifiedUnreadCount}
          {selectedAccountId}
          {selectedFolderId}
          {selectionSource}
          onSelectUnified={handleUnifiedInboxSelect}
          onSelectAccountInbox={handleAccountInboxSelect}
        />
        <div class="border-b border-border mx-3 my-1"></div>
      {/if}

      {#each accountStore.accounts as accWithFolders (accWithFolders.account.id)}
        <AccountSection
          account={accWithFolders.account}
          folders={accWithFolders.folders}
          loading={accWithFolders.loading}
          syncing={accWithFolders.syncing}
          error={accWithFolders.error}
          selectedFolderId={selectedFolderId ?? ''}
          {selectionSource}
          isHeaderFocused={focusedAccountId === accWithFolders.account.id}
          isExpanded={expandedAccounts[accWithFolders.account.id] ?? true}
          syncError={accountStore.getSyncError(accWithFolders.account.id)}
          {collapsedFolders}
          {onMessagesMoved}
          onFolderSelect={handleFolderSelect}
          onToggleExpanded={() => toggleAccountExpanded(accWithFolders.account.id)}
          onToggleFolderCollapse={toggleFolderCollapsed}
          onEdit={() => openEditAccount(accWithFolders.account)}
          onDelete={() => openDeleteAccount(accWithFolders.account)}
          onShowSyncError={() => (syncStatusOpen = true)}
          onSync={() => {
            // Clear any sync error before retrying
            accountStore.clearSyncError(accWithFolders.account.id)
            accountStore.syncAccount(accWithFolders.account.id)
          }}
        />
      {/each}

      <!-- Add Account Button -->
      <div class="px-3 py-2">
        <button
          class="w-full flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground hover:text-foreground hover:bg-muted/50 rounded-md transition-colors"
          onclick={openAddAccount}
        >
          <Icon icon="mdi:plus" class="w-4 h-4" />
          <span>{$_('sidebar.addAccount')}</span>
        </button>
      </div>
    {/if}
  </div>

  <SidebarSyncFooter {syncStatus} {onSyncButton} />
</div>

<!-- Account Dialog -->
<AccountDialog
  bind:open={showAccountDialog}
  editAccount={editingAccount}
  onClose={() => {
    showAccountDialog = false
    editingAccount = null
    setFocusedPane('messageList')
  }}
/>

<!-- Delete Confirmation Dialog -->
<DeleteAccountDialog
  bind:open={showDeleteDialog}
  account={deletingAccount}
  onClose={() => {
    showDeleteDialog = false
    deletingAccount = null
    setFocusedPane('messageList')
  }}
/>

<SyncStatusDialog
  bind:open={syncStatusOpen}
  busy={accountStore.isAnySyncing}
  onCancelSync={cancelSync}
  onFixAccount={(id) => {
    const acc = accountStore.accounts.find((a) => a.account.id === id)
    if (!acc) return
    syncStatusOpen = false
    openEditAccount(acc.account)
  }}
/>
