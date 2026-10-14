<script lang="ts">
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import { account, folder } from '../../../../wailsjs/go/models'
  import FolderTreeItem from './FolderTreeItem.svelte'
  import { _ } from '$lib/i18n'
  import { mailApiRoutes, mailApiRouteName } from '$lib/stores/mailApiRoutes.svelte'

  interface Props {
    account: account.Account
    folders: folder.FolderTree[]
    loading: boolean
    syncing: boolean
    error: string | null
    selectedFolderId: string
    selectionSource: 'unified' | 'account' | null
    isHeaderFocused?: boolean
    isExpanded?: boolean
    syncError?: { folderId: string; error: string } | null
    onFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onToggleExpanded?: () => void
    onEdit?: () => void
    onDelete?: () => void
    onSync?: () => void
    onShowSyncError?: () => void
    collapsedFolders?: Record<string, boolean>
    onToggleFolderCollapse?: (folderId: string) => void
    onMessagesMoved?: () => void
  }

  let {
    account: acc,
    folders,
    loading,
    syncing,
    error,
    selectedFolderId,
    selectionSource,
    isHeaderFocused = false,
    isExpanded = true,
    syncError = null,
    onFolderSelect,
    onToggleExpanded,
    onEdit,
    onDelete,
    onSync,
    onShowSyncError,
    collapsedFolders = {},
    onToggleFolderCollapse,
    onMessagesMoved,
  }: Props = $props()

  let showMenu = $state(false)
  let menuButton = $state<HTMLButtonElement>()

  // Toggle expand/collapse via callback
  function toggleExpanded() {
    onToggleExpanded?.()
  }

  function selectFolder(f: folder.Folder) {
    onFolderSelect?.(acc.id, f.id, f.path, f.name, f.type)
  }

  function toggleMenu(e: MouseEvent) {
    e.stopPropagation()
    showMenu = !showMenu
  }

  function handleEdit() {
    showMenu = false
    onEdit?.()
  }

  function handleDelete() {
    showMenu = false
    onDelete?.()
  }

  function handleSync() {
    showMenu = false
    onSync?.()
  }

  // Escape closes the menu and returns focus to its button. Stop the event
  // here so the window-level shortcut handler doesn't also act on it.
  function handleMenuKeydown(e: KeyboardEvent) {
    if (!showMenu || e.key !== 'Escape') return
    e.preventDefault()
    e.stopPropagation()
    showMenu = false
    menuButton?.focus()
  }

  // Close menu when clicking outside
  function handleClickOutside() {
    showMenu = false
  }

  $effect(() => { void mailApiRoutes.refresh(acc.id) })
</script>

<svelte:window onclick={handleClickOutside} />

<div class="mb-2">
  <!-- Account Header -->
  <div class="relative group">
    <button
      class="w-full flex items-center gap-2 px-3 min-h-control text-sm font-medium text-fg-primary hover:bg-muted/50 transition-colors {syncing || error ? 'pr-10' : ''} {isHeaderFocused ? 'bg-muted ring-1 ring-primary/50' : ''}"
      data-sidebar-item="account-header"
      data-account-id={acc.id}
      onclick={toggleExpanded}
    >
      <Icon
        icon={isExpanded ? 'mdi:chevron-down' : 'mdi:chevron-right'}
        class="w-4 h-4 text-muted-foreground"
      />
      <Icon icon="mdi:email-outline" class="w-4 h-4" />
      <span class="truncate flex-1 text-left">{acc.name}</span>
      {#if mailApiRoutes.active[acc.id]}
        <span class="text-xs font-normal text-muted-foreground shrink-0" data-testid="route-badge">{$_(`mailApi.${mailApiRouteName(mailApiRoutes.active[acc.id])}`)}</span>
      {/if}

      <!-- pr-10 keeps these clear of the menu button, so the error tooltip stays hoverable. -->
      {#if syncing}
        <Icon icon="mdi:sync" class="w-4 h-4 animate-spin text-muted-foreground" />
      {:else if error}
        <span title={error}>
          <Icon icon="mdi:alert-circle" class="w-4 h-4 text-destructive" />
        </span>
      {/if}
    </button>

    <!-- Account Menu Button -->
    <button
      class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center justify-center size-7 rounded hover:bg-muted transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100"
      bind:this={menuButton}
      onclick={toggleMenu}
      onkeydown={handleMenuKeydown}
      aria-label={$_('sidebar.accountOptions')}
      title={$_('sidebar.accountOptions')}
      aria-haspopup="menu"
      aria-expanded={showMenu}
    >
      <Icon icon="mdi:dots-vertical" class="w-4 h-4 text-muted-foreground" />
    </button>

    <!-- Dropdown Menu -->
    {#if showMenu}
      <div
        class="absolute right-2 top-full mt-1 z-50 min-w-[160px] bg-popover border border-border rounded-md shadow-md py-1"
        role="menu"
        tabindex="-1"
        onclick={(e) => e.stopPropagation()}
        onkeydown={handleMenuKeydown}
      >
        <button
          role="menuitem"
          class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-muted transition-colors"
          onclick={handleSync}
        >
          <Icon icon="mdi:sync" class="w-4 h-4" />
          <span>{$_('sidebar.syncNow')}</span>
        </button>
        <button
          role="menuitem"
          class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-muted transition-colors"
          onclick={handleEdit}
        >
          <Icon icon="mdi:pencil-outline" class="w-4 h-4" />
          <span>{$_('sidebar.editAccount')}</span>
        </button>
        <div class="my-1 border-t border-border"></div>
        <button
          role="menuitem"
          class="w-full flex items-center gap-2 px-3 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors"
          onclick={handleDelete}
        >
          <Icon icon="mdi:delete-outline" class="w-4 h-4" />
          <span>{$_('sidebar.deleteAccount')}</span>
        </button>
      </div>
    {/if}
  </div>

  <!-- Sync Error — bar removed; spinner in header + phase label in bottom status convey sync state. -->
  {#if syncError}
    <div class="px-3 py-1.5">
      <button
        type="button"
        class="flex w-full items-center gap-2 text-left text-destructive hover:underline"
        title={syncError.error}
        aria-label={$_('sidebar.syncErrorShowDetails')}
        onclick={onShowSyncError}
      >
        <Icon icon="mdi:alert-circle" class="w-4 h-4 shrink-0" />
        <span class="text-xs">{$_('sidebar.syncError')}</span>
      </button>
    </div>
  {/if}

  <!-- Folder List -->
  {#if isExpanded}
    <div class="ml-4">
      {#if loading}
        <div class="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground">
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          <span>{$_('sidebar.loadingFolders')}</span>
        </div>
      {:else if folders.length === 0}
        <div class="px-3 py-2 text-sm text-muted-foreground">
          {$_('sidebar.noFoldersSynced')}
        </div>
      {:else}
        {#each folders as tree (tree.folder?.id ?? 'unknown')}
          <FolderTreeItem
            {tree}
            accountId={acc.id}
            {selectedFolderId}
            {selectionSource}
            {collapsedFolders}
            {onMessagesMoved}
            onFolderSelect={(f) => selectFolder(f)}
            onToggleCollapse={(folderId) => onToggleFolderCollapse?.(folderId)}
          />
        {/each}
      {/if}
    </div>
  {/if}
</div>
