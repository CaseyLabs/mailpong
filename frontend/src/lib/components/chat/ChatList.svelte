<script lang="ts">
  // ChatList — the mail section's list pane: chats for the current scope
  // (unified inbox, an account inbox, or any folder), grouped into Pinned,
  // Chats and Low priority (each collapsible, and trimmable to its first few
  // rows with "Show fewer", both remembered across sessions; Low starts
  // collapsed), with paged loading.
  //
  // Exposes the same ref API App.svelte uses on MessageList (selection,
  // open, delete, move, sync) so keyboard shortcuts keep working, plus the
  // triage actions on the selected chat. The chat list has no multi-select.
  import { onMount, untrack } from 'svelte'
  import { _ } from '$lib/i18n'
  import type ChatRow from './ChatRow.svelte'
  import ChatListSection from './ChatListSection.svelte'
  import ChatListHeader from './ChatListHeader.svelte'
  import ChatLowGroupHeader from './ChatLowGroupHeader.svelte'
  import ChatListEmpty from './ChatListEmpty.svelte'
  import ChatServerSearch from './ChatServerSearch.svelte'
  import SyncStatusDialog from '../list/SyncStatusDialog.svelte'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  import { chatList, type ChatItem } from '$lib/stores/chat.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { getLayoutMode, hideViewer, showSidebar, isSidebarHidden, toggleActiveSidebar } from '$lib/stores/layout.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { getChatAutoAdvance, getChatShowLowGroup } from '$lib/stores/settings.svelte'
  import { type ChatGroup, isChatGroupExpanded, isChatGroupShowingFewer, toggleChatGroupExpanded, toggleChatGroupShowFewer } from '$lib/stores/uiState.svelte'
  import { SyncFolder } from '../../../../wailsjs/go/app/App'
  import { canToggleSenderChat, GROUP_FEWER_CAP, rowPeople, trimGroup } from './chatFormat'
  import { archiveChat, deleteMessagesPermanently, markChatUnread, type ChatRowActions, pinChat, setSenderChat, setSenderLow, snoozeChat, trashMessages, unsnoozeChat } from './chatTriage'
  import type { folder } from '../../../../wailsjs/go/models'

  interface Props {
    accountId: string | null
    folderId: string | null
    folderName: string
    folderType: string
    isFocused?: boolean
    isFlashing?: boolean
    // The conversation open in the viewer; its row is selected when none is.
    openThreadId?: string | null
    openAccountId?: string | null
    onConversationSelect?: (threadId: string, folderId: string, accountId: string) => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
    onRowActionComplete?: () => void
    onCompose: () => void
    onUnifiedInboxSelect: () => void
    onFolderSelect: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
  }

  let {
    accountId, folderId, folderName, folderType, isFocused = false, isFlashing = false, openThreadId = null, openAccountId = null,
    onConversationSelect, onReply, onRowActionComplete, onCompose, onUnifiedInboxSelect, onFolderSelect,
  }: Props = $props()

  let listRef = $state<HTMLDivElement | null>(null)
  let headerRef = $state<ChatListHeader | null>(null)
  let selectedKey = $state<string | null>(null)
  let rowRefs = $state<Record<string, ChatRow | null>>({})

  $effect(() => {
    const acct = accountId
    const fld = folderId
    untrack(() => {
      chatList.setScope(acct, fld)
      selectedKey = null
    })
  })

  $effect(() => {
    const show = getChatShowLowGroup()
    untrack(() => chatList.setShowLowGroup(show))
  })

  onMount(() => chatList.start())

  // Load avatars for everyone visible in the loaded rows.
  $effect(() => {
    const emails = chatList.items.flatMap((c) => c.participants.map((p) => p.email))
    if (emails.length > 0) void contactPhotos.ensure(emails)
  })

  // Keyed on a string so sync-time account updates don't rebuild every row.
  const myEmailsKey = $derived(accountStore.accounts.map((a) => (a.account.email || '').toLowerCase()).join('\n'))
  const myEmails = $derived(new Set(myEmailsKey.split('\n')))

  const scopeLabel = $derived.by(() => {
    if (chatList.isUnified) return $_('chat.allInboxes')
    if (folderType !== 'inbox') return folderName
    return accountStore.accounts.find((a) => a.account.id === accountId)?.account.name || folderName
  })

  // Rows by group. Only the All filter groups; other filters are flat.
  const grouped = $derived(chatList.filter === 'all' && !chatList.isSearch)
  const groups = $derived.by(() => {
    if (!grouped) return { pinned: [], main: chatList.items, low: [] }
    const g: Record<'pinned' | 'main' | 'low', ChatItem[]> = { pinned: [], main: [], low: [] }
    for (const c of chatList.items) g[c.isPinned ? 'pinned' : c.isLowPriority ? 'low' : 'main'].push(c)
    return g
  })
  const { pinned, main, low } = $derived(groups)
  const showLowGroup = $derived(grouped && chatList.lowCount > 0)
  // Chats gets a header (and so can collapse) only beside another group.
  const chatsHeader = $derived(pinned.length > 0 || showLowGroup)
  const pinnedOpen = $derived(isChatGroupExpanded('pinned'))
  const chatsOpen = $derived(!chatsHeader || isChatGroupExpanded('chats'))
  const lowOpen = $derived(isChatGroupExpanded('low'))

  // A group set to "Show fewer" lists its first few rows (see trimGroup).
  // Chats trims only beside another group.
  const trimmed = (group: ChatGroup, items: ChatItem[]) => trimGroup(items, isChatGroupShowingFewer(group), selectedKey)
  const pinnedShown = $derived(trimmed('pinned', pinned))
  const mainShown = $derived(chatsHeader ? trimmed('chats', main) : main)
  const lowShown = $derived(trimmed('low', low))

  // The group's "Show fewer" / hidden-rows button text, or undefined when
  // there is nothing to trim. hidden formats the count of rows (of total)
  // not shown.
  function fewerLabel(group: ChatGroup, items: ChatItem[], shown: ChatItem[], hidden: (count: number) => string, total = items.length): string | undefined {
    if (items.length <= GROUP_FEWER_CAP) return undefined
    if (!isChatGroupShowingFewer(group)) return $_('chat.showFewer')
    return shown.length < items.length ? hidden(total - shown.length) : undefined
  }
  const moreInGroup = (count: number) => $_('chat.moreInGroup', { values: { count } })
  const pinnedFewer = $derived(fewerLabel('pinned', pinned, pinnedShown, (count) => $_('chat.morePinned', { values: { count } })))
  // Chats may have unloaded rows, so its count is open-ended.
  const chatsFewer = $derived(
    chatsHeader ? fewerLabel('chats', main, mainShown, (count) => (chatList.hasMore ? $_('chat.showMore') : moreInGroup(count))) : undefined,
  )
  const lowFewer = $derived(fewerLabel('low', low, lowShown, moreInGroup, Math.max(chatList.lowCount, low.length)))

  // Keyboard navigation order: what's on screen, top to bottom.
  const visible = $derived([...(pinnedOpen ? pinnedShown : []), ...(chatsOpen ? mainShown : []), ...(lowOpen ? lowShown : [])])
  // Whether another page could add rows on screen: some open group shows
  // all of its rows and could still get more. Pinned chats sort first, so
  // once any other row is loaded no page adds pinned rows; Low stops growing
  // once all lowCount rows are loaded.
  const canGrow = $derived(
    (pinnedOpen && main.length === 0 && low.length === 0) ||
      (chatsOpen && mainShown.length === main.length) ||
      (showLowGroup && lowOpen && lowShown.length === low.length && low.length < chatList.lowCount),
  )

  // Load the next page when the list is scrolled near its end, and keep
  // loading while it doesn't fill the view (e.g. a page of mostly collapsed
  // low-priority rows). Runs after each render, so new rows are measured.
  function nearEnd(): boolean {
    if (!listRef) return false
    return listRef.scrollHeight - listRef.scrollTop - listRef.clientHeight < 400
  }

  function maybeLoadMore() {
    // With every group collapsed or trimmed, more rows wouldn't show, so
    // don't page.
    if (!canGrow) return
    if (chatList.hasMore && !chatList.loading && nearEnd()) void chatList.loadMore()
  }

  $effect(() => {
    void chatList.items.length
    void chatList.loading
    void visible.length
    maybeLoadMore()
  })

  function rowId(c: ChatItem): string {
    return `chat-row-${c.key.replace(/[^A-Za-z0-9_-]/g, '_')}`
  }

  const selected = $derived(chatList.items.find((c) => c.key === selectedKey) ?? null)

  // Adopt the open conversation's row (e.g. the one restored at launch) so
  // it is highlighted and arrow keys start from it.
  $effect(() => {
    if (selectedKey !== null || !openThreadId) return
    const c = chatList.items.find((r) => r.threadId === openThreadId && (!openAccountId || r.accountId === openAccountId))
    if (c) selectedKey = c.key
  })

  function open(c: ChatItem) {
    selectedKey = c.key
    onConversationSelect?.(c.threadId, c.folderId, c.accountId)
  }

  // moveTo selects the row at index; openIt also opens it in the viewer.
  function moveTo(index: number, openIt = false) {
    const c = visible[index]
    if (!c) return
    if (openIt && c.key !== selectedKey) open(c)
    selectedKey = c.key
    document.getElementById(rowId(c))?.scrollIntoView({ block: 'nearest' })
    // Keep focus off buttons so Enter reaches openSelected().
    if (document.activeElement !== listRef) (document.activeElement as HTMLElement)?.blur?.()
  }

  function selectedIndex(): number {
    return visible.findIndex((c) => c.key === selectedKey)
  }

  export function selectPrevious(openIt = false) { moveTo(Math.max(0, selectedIndex() - 1), openIt) }
  export function selectNext(openIt = false) {
    const i = selectedIndex()
    if (i >= visible.length - 1 && chatList.hasMore) void chatList.loadMore()
    moveTo(Math.min(visible.length - 1, i + 1), openIt)
  }
  export function selectFirst() { moveTo(0) }
  export function selectLast() { moveTo(visible.length - 1) }
  export function openSelected() { if (selected) open(selected) }
  export function selectThread(threadId: string) {
    const i = visible.findIndex((c) => c.threadId === threadId)
    if (i >= 0) moveTo(i)
  }
  export function getSelectedThreadId(): string | null { return selected?.threadId ?? null }
  export function hasSelection(): boolean { return selected !== null }
  export function focusList() {
    listRef?.focus({ preventScroll: true })
    setFocusedPane('messageList')
  }
  export function getSelectedMessageIds(): string[] { return selected?.messageIds ?? [] }
  export function isSelectedStarred(): boolean { return selected?.isStarred ?? false }
  export function toggleSearchFocus() {
    if (headerRef?.isSearchFocused()) {
      chatList.setQuery('')
      listRef?.focus()
      return
    }
    headerRef?.focusSearch()
  }

  // No multi-select in the chat list.
  export function selectAll() {}
  export function selectNextWithCheck() { selectNext() }
  export function selectPreviousWithCheck() { selectPrevious() }

  export function openContextMenu() {
    if (!selected) return
    const row = document.getElementById(rowId(selected))
    if (!row) return
    const rect = row.getBoundingClientRect()
    row.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: rect.right, clientY: rect.top + rect.height / 2 }))
  }

  export function isFolderPickerOpen(): boolean {
    return (selected && rowRefs[selected.key]?.isFolderPickerOpen()) ?? false
  }
  export function toggleMoveToDialog() { if (selected) rowRefs[selected.key]?.toggleFolderPicker('move') }
  export function toggleCopyToDialog() { if (selected) rowRefs[selected.key]?.toggleFolderPicker('copy') }

  // --- Triage (row actions, context menu, keyboard on the selected chat) ---

  // afterRemoval advances past a chat that left the list (Done, Snooze) when
  // it is the selected one; otherwise the list just refreshes.
  function afterRemoval(c: ChatItem) {
    if (c.key === selectedKey) handleActionComplete(true)
    else void chatList.reload(true)
  }

  const reloadAfterUndo = () => {
    onRowActionComplete?.()
    void chatList.reload(true)
  }

  async function archive(c: ChatItem) {
    if (await archiveChat(c.messageIds, reloadAfterUndo)) afterRemoval(c)
  }

  async function snooze(c: ChatItem, until: Date) {
    if (await snoozeChat(c, until, reloadAfterUndo)) afterRemoval(c)
  }

  function chatSenderOf(c: ChatItem) {
    return rowPeople(c, myEmails, folderType === 'sent')[0]
  }

  function toggleSenderLow(c: ChatItem) {
    const sender = chatSenderOf(c)
    if (sender) void setSenderLow(c.accountId, sender, !c.isLowPriority)
  }

  // toggleSenderChat combines the row's sender into one sender chat, or
  // splits a sender chat; a selected row's selection follows the change.
  async function toggleSenderChat(c: ChatItem) {
    const sender = chatSenderOf(c)
    if (!sender || !canToggleSenderChat(c.senderEmail, folderType)) return
    const combine = !c.senderEmail
    const ids = new Set(c.messageIds)
    let followed = c.key === selectedKey ? c.key : ''
    // follow reloads, then keeps the selection on the chat: the sender chat
    // when combined, the newest of its threads when split.
    const follow = async (combined: boolean) => {
      await chatList.reload(true)
      if (!followed || followed !== selectedKey) return
      const next = combined
        ? chatList.items.find((r) => r.accountId === c.accountId && r.senderEmail === sender.email.toLowerCase())
        : chatList.items.find((r) => r.accountId === c.accountId && !r.senderEmail && r.messageIds.some((id) => ids.has(id)))
      if (!next) return
      followed = next.key
      open(next)
    }
    const afterUndo = () => {
      onRowActionComplete?.()
      void follow(!combine)
    }
    if (await setSenderChat(c.accountId, sender, combine, afterUndo)) await follow(combine)
  }

  function rowActions(c: ChatItem): ChatRowActions {
    return {
      onArchive: () => void archive(c),
      onPin: () => void pinChat(c, !c.isPinned, reloadAfterUndo),
      onSnooze: (until) => void snooze(c, until),
      onUnsnooze: () => void unsnoozeChat(c, reloadAfterUndo),
      onToggleSenderLow: () => toggleSenderLow(c),
      onToggleSenderChat: () => void toggleSenderChat(c),
    }
  }

  export function archiveSelected() { if (selected) void archive(selected) }
  export function pinSelected() { if (selected) void pinChat(selected, !selected.isPinned, reloadAfterUndo) }
  export function markSelectedUnread() { if (selected) void markChatUnread(selected.messageIds) }
  export function toggleSelectedSenderLow() { if (selected) toggleSenderLow(selected) }
  export function toggleSelectedSenderChat() { if (selected) void toggleSenderChat(selected) }
  export function openSnoozeForSelected() { if (selected) void rowRefs[selected.key]?.openSnooze() }

  // After an action, refresh in place; autoSelectNext moves the selection
  // to the row that took the acted-on row's place ("next") or the row above
  // it ("previous"), per Settings → Chat.
  export function handleActionComplete(autoSelectNext = false) {
    onRowActionComplete?.()
    const index = selectedIndex()
    const target = getChatAutoAdvance() === 'previous' ? Math.max(index - 1, 0) : index
    void chatList.reload(true).then(() => {
      if (!autoSelectNext) return
      // Keyboard triage continues from the list, whichever pane acted.
      focusList()
      const c = index >= 0 ? visible[Math.min(target, visible.length - 1)] : undefined
      if (getLayoutMode() === 'narrow') {
        hideViewer()
        if (c) selectedKey = c.key
        return
      }
      if (c) open(c)
    })
  }

  // Sync: a folder scope syncs that folder; the unified inbox syncs all.
  const folderSyncing = $derived(
    !chatList.isUnified && !!(accountId && folderId && accountStore.syncProgress[accountId]?.[folderId] !== undefined)
  )
  const syncBusy = $derived(accountStore.isAnySyncing || folderSyncing)

  let syncStatusOpen = $state(false)
  const cancelRunningSyncs = () => Promise.all([
    folderSyncing && accountStore.cancelFolderSync(accountId!, folderId!),
    accountStore.isAnySyncing && accountStore.cancelAllSyncs(),
  ]).then(() => {}, (err) => console.error('Cancel sync failed:', err))

  const onSyncButton = () => (syncBusy ? (syncStatusOpen = true) : toggleFolderSync())

  export async function toggleFolderSync() {
    try {
      if (syncBusy) return await cancelRunningSyncs()
      if (chatList.isUnified || !accountId || !folderId) {
        await accountStore.syncAllComplete()
        return
      }
      await SyncFolder(accountId, folderId)
      await Promise.all([chatList.reload(true), neverSynced && accountStore.loadFolders(accountId)])
    } catch (err) {
      console.error('Sync failed:', err)
    }
  }

  // Delete: move to Trash with undo, or confirm a permanent delete.
  let showDeleteConfirm = $state(false)
  let pendingDeleteIds = $state<string[]>([])

  export function requestDelete(messageIds: string[], permanent = false) {
    if (permanent || folderType === 'trash') {
      pendingDeleteIds = messageIds
      showDeleteConfirm = true
      return
    }
    void trashMessages(messageIds, reloadAfterUndo).then((ok) => {
      if (ok) handleActionComplete(true)
    })
  }

  async function confirmPermanentDelete() {
    if (await deleteMessagesPermanently(pendingDeleteIds)) handleActionComplete(true)
    showDeleteConfirm = false
    pendingDeleteIds = []
  }

  const handleInboxSelect = (acctId: string, inbox: folder.Folder) => onFolderSelect(acctId, inbox.id, inbox.path, inbox.name, inbox.type)

  function handleBrowseFolders() {
    if (getLayoutMode() === 'narrow') showSidebar()
    else if (isSidebarHidden()) toggleActiveSidebar()
  }

  // Only core folders auto-sync, so other folders stay empty until synced once.
  const neverSynced = $derived.by(() => {
    if (chatList.isUnified || chatList.isSearch || !accountId || !folderId) return false
    const f = accountStore.getFolder(accountId, folderId)
    return !!f && !f.lastSync
  })
</script>

{#snippet group(label: string, items: ChatItem[], id?: ChatGroup, fewerLabel?: string, header = false)}
  <ChatListSection
    {label}
    {items}
    open={!id || !header || isChatGroupExpanded(id)}
    onToggle={id && header ? () => toggleChatGroupExpanded(id) : undefined}
    {fewerLabel}
    onToggleFewer={() => id && toggleChatGroupShowFewer(id)}
    inPinned={id === 'pinned'}
    {selectedKey}
    folderType={chatList.isUnified ? 'inbox' : folderType}
    showAccount={chatList.isUnified}
    {myEmails}
    bind:rowRefs
    {rowId}
    {rowActions}
    onOpen={open}
    onFocusList={focusList}
    onActionComplete={handleActionComplete}
    {onReply}
  />
{/snippet}

<div class="flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <ChatListHeader
    bind:this={headerRef}
    {scopeLabel}
    isUnified={chatList.isUnified}
    {folderId}
    filter={chatList.effectiveFilter}
    isSearch={chatList.isSearch}
    query={chatList.query}
    {syncBusy}
    onUnifiedSelect={onUnifiedInboxSelect}
    onInboxSelect={handleInboxSelect}
    onBrowseFolders={handleBrowseFolders}
    onFilter={(f) => chatList.setFilter(f)}
    onQuery={(q) => chatList.setQuery(q)}
    onSync={onSyncButton}
    {onCompose}
  />

  <div
    bind:this={listRef}
    class="group/list flex-1 overflow-y-auto py-1 outline-hidden"
    role="listbox"
    tabindex="0"
    aria-label={`${$_('chat.listLabel')}: ${scopeLabel}`}
    aria-activedescendant={selected ? rowId(selected) : undefined}
    data-focused={isFocused || undefined}
    onscroll={maybeLoadMore}
  >
    {#if chatList.items.length === 0 && (chatList.error || !chatList.loading)}
      <ChatListEmpty failed={!!chatList.error} {folderType} {neverSynced} {syncBusy} onSyncNow={toggleFolderSync} />
    {:else}
      {#if pinned.length > 0}{@render group($_('chat.pinned'), pinnedShown, 'pinned', pinnedFewer, true)}{/if}
      {#if grouped}
        {#if main.length > 0}{@render group($_('chat.chats'), mainShown, 'chats', chatsFewer, chatsHeader)}{/if}
      {:else}
        {@render group('', main)}
      {/if}
      {#if showLowGroup}
        <ChatLowGroupHeader onArchived={reloadAfterUndo} />
        {#if lowOpen}{@render group($_('chat.filterLow'), lowShown, 'low', lowFewer, false)}{/if}
      {/if}
    {/if}
    {#if chatList.loading && chatList.items.length > 0}
      <div class="flex justify-center py-3 text-xs text-muted-foreground">{$_('common.loading')}</div>
    {/if}
    {#if chatList.items.length > 0 && !chatList.hasMore}<ChatServerSearch />{/if}
  </div>
</div>

<SyncStatusDialog bind:open={syncStatusOpen} busy={syncBusy} onCancelSync={cancelRunningSyncs} />
<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('dialog.deletePermanently')}
  description={$_('dialog.deleteDescription')}
  confirmLabel={$_('dialog.confirmDeletePermanently')}
  variant="destructive"
  onConfirm={confirmPermanentDelete}
  onCancel={() => { showDeleteConfirm = false; pendingDeleteIds = [] }}
/>
