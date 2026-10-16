<script lang="ts">
  // ChatLoadOlder — "Load older mail" at the end of a chat list.
  // The sync window keeps only recent mail locally; this asks the server for the next
  // page before everything loaded so far and stores it, without widening the
  // sync period. Reconciles keep what it loads.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { chatList } from '$lib/stores/chat.svelte'
  import { chatServerSearch as s } from '$lib/stores/chatServerSearch.svelte'
  import { mailApiRoutes } from '$lib/stores/mailApiRoutes.svelte'

  // The collapsed Accounts column does not refresh route state, so do it here.
  $effect(() => {
    if (!chatList.isSearch) void mailApiRoutes.refreshAll(accountStore.accounts.map((a) => a.account.id))
  })
</script>

{#if s.olderAvailable}
  <div class="flex flex-col items-center gap-1 px-4 py-3 text-sm text-muted-foreground">
    <button
      class="inline-flex items-center gap-1.5 text-primary hover:underline disabled:opacity-60 disabled:no-underline"
      disabled={s.olderBusy}
      onclick={() => s.loadOlder()}
    >
      <Icon icon={s.olderBusy ? 'mdi:loading' : 'mdi:history'} class={s.olderBusy ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
      {s.olderBusy ? $_('chat.loadingOlder') : $_('chat.loadOlder')}
    </button>
    {#if s.olderAdded === 0 && !s.olderBusy}
      <p role="status">{$_('chat.loadOlderNone')}</p>
    {/if}
  </div>
{:else if s.olderExhausted}
  <p class="px-4 py-3 text-center text-sm text-muted-foreground" role="status">{$_('chat.loadOlderEnd')}</p>
{/if}
