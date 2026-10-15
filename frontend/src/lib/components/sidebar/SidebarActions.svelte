<script lang="ts">
  // SidebarActions — the labeled primary actions at the top of the Accounts
  // sidebar. The chat list header keeps its icon-only Compose and Search for
  // when this sidebar is collapsed.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    onCompose: () => void
    onSearch: () => void
  }

  let { onCompose, onSearch }: Props = $props()
</script>

<div class="flex flex-col gap-0.5 px-3 pb-2">
  {#each [
    { icon: 'mdi:pencil-outline', label: $_('sidebar.newMessage'), onclick: onCompose },
    { icon: 'mdi:magnify', label: $_('sidebar.searchChats'), onclick: onSearch },
  ] as action (action.icon)}
    <button
      class="flex items-center gap-2 px-3 py-2 text-left text-body text-fg-secondary hover:bg-muted/50 hover:text-foreground rounded-md transition-colors"
      onclick={action.onclick}
    >
      <Icon icon={action.icon} class="w-4 h-4 shrink-0" />
      <span class="truncate">{action.label}</span>
    </button>
  {/each}
</div>
