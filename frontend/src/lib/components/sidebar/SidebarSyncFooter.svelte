<script lang="ts">
  import Icon from '@iconify/svelte'
  import SidebarFooter from '$lib/components/kit/SidebarFooter.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'
  import type { SyncStatus } from './sidebarNav'

  interface Props {
    syncStatus: SyncStatus
    onSyncButton: () => void
  }

  let { syncStatus, onSyncButton }: Props = $props()
</script>

<!-- Footer: Sync Status. Chrome (padding/border/min-height)
     lives in kit `SidebarFooter` so mail, calendar, and contacts all
     render the same strip height. Mail's progress bar overlay + 2-line
     leading content (account name + status label) pass through
     SidebarFooter's overlay/leading snippets unchanged. -->
<SidebarFooter>
  {#snippet overlay()}
    {#if syncStatus.percentage !== null}
      <div class="absolute top-0 left-0 right-0 h-1 bg-muted overflow-hidden">
        <div
          class="h-full bg-primary transition-all duration-300 ease-out"
          style="width: {syncStatus.percentage}%"
        ></div>
      </div>
    {/if}
  {/snippet}
  {#snippet leading()}
    <button
      class="flex-1 min-w-0 flex items-end gap-2 hover:text-foreground transition-colors text-left"
      onclick={onSyncButton}
      title={$_(accountStore.isAnySyncing ? 'sidebar.clickForSyncStatus' : 'sidebar.syncAllAccounts')}
    >
      <Icon
        icon="mdi:sync"
        class="w-4 h-4 shrink-0 {accountStore.isAnySyncing ? 'animate-spin' : ''}"
      />
      <div class="flex-1 min-w-0">
        {#if syncStatus.accountName}
          <div class="truncate text-foreground font-medium leading-tight mb-0.5">{syncStatus.accountName}</div>
        {/if}
        <span class="block leading-tight">{syncStatus.label}</span>
      </div>
    </button>
  {/snippet}
</SidebarFooter>
