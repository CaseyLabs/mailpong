<script lang="ts">
  // ChatListHeader — scope menu (all inboxes, an account inbox, or the folder
  // sidebar), sync/compose buttons, search field, and filter chips.
  import Icon from '@iconify/svelte'
  import { DropdownMenu } from 'bits-ui'
  import { _ } from '$lib/i18n'
  import { cn } from '$lib/utils'
  import ResponsiveSidebarToggle from '$lib/components/kit/ResponsiveSidebarToggle.svelte'
  import ComposeButton from '$lib/components/common/ComposeButton.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { searchSupports, type ChatFilter } from '$lib/stores/chat.svelte'
  // @ts-ignore - wailsjs path
  import type { folder } from '../../../../wailsjs/go/models'

  interface Props {
    scopeLabel: string
    isUnified: boolean
    folderId: string | null
    filter: ChatFilter
    isSearch: boolean
    query: string
    syncBusy: boolean
    onUnifiedSelect: () => void
    onInboxSelect: (accountId: string, inbox: folder.Folder) => void
    onBrowseFolders: () => void
    onFilter: (filter: ChatFilter) => void
    onQuery: (query: string) => void
    onSync: () => void
    onCompose?: () => void
  }

  let {
    scopeLabel, isUnified, folderId, filter, isSearch, query, syncBusy,
    onUnifiedSelect, onInboxSelect, onBrowseFolders, onFilter, onQuery, onSync, onCompose,
  }: Props = $props()

  let searchInputRef = $state<HTMLInputElement | null>(null)

  export function focusSearch() {
    searchInputRef?.focus()
    searchInputRef?.select()
  }

  export function isSearchFocused(): boolean {
    return !!searchInputRef && document.activeElement === searchInputRef
  }

  const inboxes = $derived(
    accountStore.accounts
      .map((a) => ({
        account: a.account,
        inbox: accountStore.getInbox(a.account.id),
      }))
      .filter((x): x is { account: typeof x.account; inbox: folder.Folder } => !!x.inbox)
  )

  const filters: { value: ChatFilter; label: string }[] = $derived([
    { value: 'all', label: $_('chat.filterAll') },
    { value: 'unread', label: $_('chat.filterUnread') },
    { value: 'low', label: $_('chat.filterLow') },
    { value: 'snoozed', label: $_('chat.filterSnoozed') },
  ])

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.preventDefault()
    e.stopPropagation()
    if (query) {
      onQuery('')
      return
    }
    searchInputRef?.blur()
  }

  const ring = 'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden'
  const itemClass = 'relative flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden focus:bg-accent focus:text-accent-foreground'
</script>

<div class="flex flex-col gap-2 px-3 pt-3 pb-2 border-b border-border">
  <div class="flex items-center gap-1 min-w-0">
    <ResponsiveSidebarToggle />
    <DropdownMenu.Root>
      <DropdownMenu.Trigger
        class="flex items-center gap-1 min-w-0 px-2 py-1 rounded-md hover:bg-muted transition-colors {ring}"
        aria-label={`${$_('chat.scope')}: ${scopeLabel}`}
      >
        <span class="font-semibold text-foreground truncate">{scopeLabel}</span>
        <Icon icon="mdi:chevron-down" class="w-4 h-4 shrink-0 text-muted-foreground" />
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          side="bottom"
          align="start"
          sideOffset={4}
          class={cn(
            'z-50 min-w-[220px] rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
            'data-[state=open]:animate-in data-[state=closed]:animate-out',
            'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0'
          )}
        >
          <DropdownMenu.Item class={itemClass} onSelect={onUnifiedSelect}>
            <Icon icon="mdi:check" class="w-4 h-4 {isUnified ? '' : 'invisible'}" />
            <Icon icon="mdi:inbox-multiple" class="w-4 h-4 text-muted-foreground" />
            {$_('chat.allInboxes')}
          </DropdownMenu.Item>
          {#each inboxes as { account, inbox } (account.id)}
            <DropdownMenu.Item class={itemClass} onSelect={() => onInboxSelect(account.id, inbox)}>
              <Icon icon="mdi:check" class="w-4 h-4 {!isUnified && folderId === inbox.id ? '' : 'invisible'}" />
              <span class="w-4 h-4 flex items-center justify-center" aria-hidden="true">
                <span class="w-2.5 h-2.5 rounded-full" style="background-color: {account.color || 'var(--color-muted-foreground)'}"></span>
              </span>
              <span class="truncate">{account.name}</span>
            </DropdownMenu.Item>
          {/each}
          <DropdownMenu.Separator class="-mx-1 my-1 h-px bg-border" />
          <DropdownMenu.Item class={itemClass} onSelect={onBrowseFolders}>
            <span class="w-4 h-4"></span>
            <Icon icon="mdi:folder-outline" class="w-4 h-4 text-muted-foreground" />
            {$_('chat.allFolders')}
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>

    <div class="flex-1"></div>
    {#if onCompose}
      <div class="mr-1"><ComposeButton onclick={onCompose} /></div>
    {/if}
    <button
      class="p-2 rounded-md hover:bg-muted transition-colors {ring}"
      title={syncBusy ? `${$_('sidebar.syncing')} ${$_('sidebar.clickToCancel')}` : $_('sidebar.syncAllAccounts')}
      aria-label={$_('sidebar.syncAllAccounts')}
      onclick={onSync}
    >
      <Icon icon="mdi:refresh" class="w-5 h-5 text-muted-foreground {syncBusy ? 'animate-spin' : ''}" />
    </button>
  </div>

  <div class="flex items-center gap-1 bg-muted rounded-md px-2 focus-within:ring-2 focus-within:ring-ring">
    <Icon icon="mdi:magnify" class="w-4 h-4 text-muted-foreground shrink-0" />
    <input
      bind:this={searchInputRef}
      type="search"
      placeholder={$_('chat.searchChats')}
      aria-label={$_('chat.searchChats')}
      class="bg-transparent border-none outline-hidden text-sm py-1.5 w-full min-w-0"
      value={query}
      oninput={(e) => onQuery(e.currentTarget.value)}
      onkeydown={handleSearchKeydown}
    />
    {#if query}
      <button
        class="p-0.5 rounded hover:bg-background/60 {ring}"
        aria-label={$_('messageList.clearSearch')}
        onclick={() => { onQuery(''); searchInputRef?.focus() }}
      >
        <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
      </button>
    {/if}
  </div>

  <div class="flex items-center gap-1.5 overflow-x-auto" role="group" aria-label={$_('chat.filters')}>
    {#each filters as f (f.value)}
      <button
        disabled={isSearch && !searchSupports(f.value)}
        class="px-2.5 py-0.5 rounded-full text-xs whitespace-nowrap transition-colors {filter === f.value ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/70'} {ring} disabled:opacity-50 disabled:pointer-events-none"
        aria-pressed={filter === f.value}
        onclick={() => onFilter(f.value)}
      >
        {f.label}
      </button>
    {/each}
  </div>
</div>
