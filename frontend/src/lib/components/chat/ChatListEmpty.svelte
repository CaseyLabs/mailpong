<script lang="ts">
  // ChatListEmpty — the chat list's load-error and empty states: failed load
  // with retry, no accounts yet, inbox zero, a folder that was never synced, or no matches.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { chatList, type ChatFilter } from '$lib/stores/chat.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { Button } from '$lib/components/ui/button'
  import ChatServerSearch from './ChatServerSearch.svelte'

  interface Props {
    // True when loading failed and there is nothing to show.
    failed: boolean
    folderType: string
    neverSynced: boolean
    syncBusy: boolean
    onSyncNow: () => void
    onAddAccount?: () => void
  }

  let { failed, folderType, neverSynced, syncBusy, onSyncNow, onAddAccount }: Props = $props()

  // Without accounts an empty inbox is not "inbox zero": point to Add Account.
  const noAccounts = $derived(!accountStore.loading && accountStore.accounts.length === 0)

  // Inbox zero: nothing left in an inbox under the All filter.
  const inboxZero = $derived(
    chatList.filter === 'all' && !chatList.isSearch && (chatList.isUnified || folderType === 'inbox'),
  )

  const emptyText = $derived.by(() => {
    if (chatList.isSearch) return $_('chat.noResults', { values: { query: chatList.query.trim() } })
    const byFilter: Record<ChatFilter, string> = {
      all: $_('chat.noChats'),
      unread: $_('chat.noUnread'),
      low: $_('chat.noLow'),
      snoozed: $_('chat.noSnoozed'),
    }
    return byFilter[chatList.filter]
  })
</script>

<div class="flex flex-col items-center justify-center h-full gap-2 text-muted-foreground">
  {#if failed}
    <Icon icon="mdi:alert-circle-outline" class="w-10 h-10" />
    <p>{$_('chat.loadFailed')}</p>
    <button class="text-sm text-primary hover:underline" onclick={() => chatList.reload()}>{$_('messageList.tryAgain')}</button>
  {:else if noAccounts}
    <Icon icon="mdi:email-plus-outline" class="w-12 h-12" />
    <p class="text-base font-medium text-foreground">{$_('sidebar.noAccountsYet')}</p>
    <p class="text-sm">{$_('sidebar.addFirstAccount')}</p>
    {#if onAddAccount}
      <Button size="sm" class="mt-2" onclick={onAddAccount}>
        <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
        {$_('sidebar.addAccount')}
      </Button>
    {/if}
  {:else if inboxZero}
    <Icon icon="mdi:check-circle-outline" class="w-12 h-12 text-primary" />
    <p class="text-base font-medium text-foreground">{$_('chat.inboxZero')}</p>
    <p class="text-sm">{$_('chat.inboxZeroHint')}</p>
  {:else if neverSynced && syncBusy}
    <Icon icon="mdi:loading" class="w-10 h-10 animate-spin" />
    <p>{$_('sidebar.syncing')}</p>
  {:else if neverSynced}
    <Icon icon="mdi:cloud-sync-outline" class="w-10 h-10" />
    <p>{$_('chat.notSynced')}</p>
    <button class="text-sm text-primary hover:underline" onclick={onSyncNow}>{$_('chat.syncNow')}</button>
  {:else}
    <Icon icon="mdi:chat-outline" class="w-10 h-10" />
    <p>{emptyText}</p>
    <ChatServerSearch />
    {#if chatList.filter === 'snoozed' && !chatList.isSearch}<p class="text-sm text-center px-6">{$_('chat.noSnoozedHint')}</p>{/if}
  {/if}
</div>
