import type { Readable } from 'svelte/store'
import { formatDistanceToNow } from 'date-fns'
import { accountStore } from '$lib/stores/accounts.svelte'
import { isUnifiedInboxExpanded } from '$lib/stores/uiState.svelte'
import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'
import type { _ } from '$lib/i18n'
// @ts-ignore - wailsjs path
import type { folder } from '../../../../wailsjs/go/models'

// Folder item type for flat navigation list
export interface FolderNavItem {
  type: 'unified' | 'unified-account' | 'account-header' | 'folder'
  accountId?: string
  folderId?: string
  folderPath?: string
  folderName: string
  folderType?: string
}

export interface SidebarNavState {
  expandedAccounts: Record<string, boolean>
  collapsedFolders: Record<string, boolean>
}

export interface SidebarCurrentSelection {
  focusedAccountId: string | null
  selectedAccountId: string | null
  selectedFolderId: string | null
  selectionSource: 'unified' | 'account' | null
}

// Build flat list of all navigable folders including Unified Inbox
// The list matches the exact visual order in the sidebar, respecting expanded/collapsed state
export function buildFolderNavList({ expandedAccounts, collapsedFolders }: SidebarNavState): FolderNavItem[] {
  const items: FolderNavItem[] = []

  // Add Unified Inbox section items if more than 1 account
  if (accountStore.accounts.length > 1) {
    // 1. Add "All Inboxes"
    items.push({
      type: 'unified',
      folderName: 'Unified Inbox',
      folderType: 'unified',
    })

    // 2. Add each account's inbox (under unified section) - only if unified section is expanded
    if (isUnifiedInboxExpanded()) {
      for (const accWithFolders of accountStore.accounts) {
        // Skip if account is not fully loaded yet (can happen during reauth)
        if (!accWithFolders.account) continue

        const inbox = accountStore.getInbox(accWithFolders.account.id)
        if (inbox) {
          items.push({
            type: 'unified-account',
            accountId: accWithFolders.account.id,
            folderId: inbox.id,
            folderPath: inbox.path,
            folderName: inbox.name,
            folderType: 'inbox',
          })
        }
      }
    }
  }

  // 3. Add account headers and their folders
  for (const accWithFolders of accountStore.accounts) {
    // Skip if account is not fully loaded yet (can happen during reauth)
    if (!accWithFolders.account) continue

    // Always add the account header (so user can navigate to it and expand)
    items.push({
      type: 'account-header',
      accountId: accWithFolders.account.id,
      folderName: accWithFolders.account.name,
    })

    // Only add folders if the account is expanded
    if (expandedAccounts[accWithFolders.account.id]) {
      const flattenFolders = (trees: folder.FolderTree[]) => {
        for (const tree of trees) {
          if (tree.folder) {
            items.push({
              type: 'folder',
              accountId: accWithFolders.account.id,
              folderId: tree.folder.id,
              folderPath: tree.folder.path,
              folderName: tree.folder.name,
              folderType: tree.folder.type,
            })
          }
          // Skip children of collapsed folders
          if (tree.children && tree.children.length > 0 && tree.folder && collapsedFolders[tree.folder.id] === false) {
            flattenFolders(tree.children)
          }
        }
      }
      flattenFolders(accWithFolders.folders || [])
    }
  }

  return items
}

// Get current folder index in navigation list
export function getCurrentFolderIndex(
  navList: FolderNavItem[],
  { focusedAccountId, selectedAccountId, selectedFolderId, selectionSource }: SidebarCurrentSelection,
): number {
  // Check if an account header is focused
  if (focusedAccountId) {
    return navList.findIndex(item =>
      item.type === 'account-header' && item.accountId === focusedAccountId
    )
  }

  // Check if Unified Inbox is selected (All Inboxes)
  if (selectedAccountId === 'unified') {
    return navList.findIndex(item => item.type === 'unified')
  }

  // Check selectionSource to find the correct item
  if (selectionSource === 'unified') {
    // Looking for unified-account item
    return navList.findIndex(item =>
      item.type === 'unified-account' && item.folderId === selectedFolderId
    )
  } else {
    // Looking for regular folder item
    return navList.findIndex(item =>
      item.type === 'folder' && item.folderId === selectedFolderId
    )
  }
}

// Build selector based on item type
export function navItemSelector(item: FolderNavItem): string | null {
  let selector: string | null = null
  if (item.type === 'unified') {
    selector = '[data-sidebar-item="unified"]'
  } else if (item.type === 'unified-account' && item.folderId) {
    selector = `[data-sidebar-item="unified-account"][data-folder-id="${item.folderId}"]`
  } else if (item.type === 'account-header' && item.accountId) {
    selector = `[data-sidebar-item="account-header"][data-account-id="${item.accountId}"]`
  } else if (item.type === 'folder' && item.folderId) {
    selector = `[data-sidebar-item="folder"][data-folder-id="${item.folderId}"]`
  }
  return selector
}

// Collapsed-folder state from persisted storage, pruned to folders that still exist
export function pruneCollapsedFolders(
  persisted: Record<string, boolean>,
): { newCollapsed: Record<string, boolean>; hasStale: boolean } {
  // Collect all folder IDs that exist in current accounts
  const allFolderIds = new Set<string>()
  const collectIds = (trees: folder.FolderTree[]) => {
    for (const tree of trees) {
      if (tree.folder) allFolderIds.add(tree.folder.id)
      if (tree.children) collectIds(tree.children)
    }
  }
  for (const acc of accountStore.accounts) {
    collectIds(acc.folders || [])
  }

  // Keep only entries for existing folders
  const newCollapsed: Record<string, boolean> = {}
  let hasStale = false
  for (const folderId of Object.keys(persisted)) {
    if (allFolderIds.has(folderId)) {
      newCollapsed[folderId] = persisted[folderId]
    } else {
      hasStale = true
    }
  }
  return { newCollapsed, hasStale }
}

type Translate = typeof _ extends Readable<infer F> ? F : never

export interface SyncStatus {
  accountName: string | null
  label: string
  percentage: number | null
}

// Sync status: { accountName, label, percentage } — accountName + percentage are populated only when a sync is active.
// `t` is the reactive `$_` passed in by the caller so locale changes keep re-deriving it.
export function computeSyncStatus(t: Translate): SyncStatus {
  if (accountStore.isAnySyncing) {
    const syncingAcc = accountStore.accounts.find((a) => a.syncing)
    if (syncingAcc) {
      const accountName = syncingAcc.account.name
      const progress = accountStore.getSyncProgress(syncingAcc.account.id)
      if (progress) {
        if (progress.phase === 'folders') return { accountName, label: t('sidebar.syncingFolders'), percentage: null }
        if (progress.phase === 'messages') return { accountName, label: t('sidebar.fetchingMessageList'), percentage: null }
        if (progress.phase === 'headers') return { accountName, label: t('sidebar.fetchingHeaders', { values: { percentage: progress.percentage } }), percentage: progress.percentage }
        return { accountName, label: t('sidebar.syncingContent', { values: { percentage: progress.percentage } }), percentage: progress.percentage }
      }
      return { accountName, label: t('sidebar.syncing'), percentage: null }
    }
    return { accountName: null, label: t('sidebar.syncing'), percentage: null }
  }
  if (!accountStore.isOnline) return { accountName: null, label: t('sidebar.offline'), percentage: null }
  if (!accountStore.lastSyncTime) return { accountName: null, label: t('sidebar.notSynced'), percentage: null }
  return { accountName: null, label: t('sidebar.synced', { values: { time: formatDistanceToNow(accountStore.lastSyncTime, { addSuffix: true, locale: getCurrentDateFnsLocale() }) } }), percentage: null }
}
