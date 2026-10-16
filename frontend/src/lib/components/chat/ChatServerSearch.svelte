<script lang="ts">
  // ChatServerSearch — "search the whole Gmail mailbox" under a chat search.
  // The sync window keeps only recent mail locally; this asks Gmail for
  // matches anywhere and stores them, so the normal search then lists them.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { chatList } from '$lib/stores/chat.svelte'
  import { chatServerSearch as s } from '$lib/stores/chatServerSearch.svelte'
  import { mailApiRoutes } from '$lib/stores/mailApiRoutes.svelte'

  // The collapsed Accounts column does not refresh route state, so do it here.
  $effect(() => {
    if (chatList.isSearch) void mailApiRoutes.refreshAll(accountStore.accounts.map((a) => a.account.id))
  })
</script>

{#if s.available}
  <div class="flex flex-col items-center gap-1 px-4 py-3 text-sm text-muted-foreground">
    <button
      class="inline-flex items-center gap-1.5 text-primary hover:underline disabled:opacity-60 disabled:no-underline"
      disabled={s.busy}
      onclick={() => s.search()}
    >
      <Icon icon={s.busy ? 'mdi:loading' : 'mdi:cloud-search-outline'} class={s.busy ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
      {s.busy ? $_('chat.serverSearching') : $_('chat.serverSearch')}
    </button>
    {#if s.doneQuery === chatList.query.trim() && !s.busy}
      <p role="status">{s.added > 0 ? $_('chat.serverSearchAdded', { values: { count: s.added } }) : $_('chat.serverSearchNone')}</p>
    {/if}
  </div>
{/if}
