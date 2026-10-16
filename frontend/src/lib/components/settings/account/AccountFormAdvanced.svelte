<script lang="ts">
  import type { AccountFormState } from './accountFormState.svelte'
  import AccountFormServerSettings from './AccountFormServerSettings.svelte'
  import AccountFormSyncSettings from './AccountFormSyncSettings.svelte'
  import AccountFormFolderMapping from './AccountFormFolderMapping.svelte'

  let { s }: { s: AccountFormState } = $props()
</script>

{#if s.showAdvanced}
  <div class="space-y-4 pt-2 border-t border-border">
    <AccountFormServerSettings {s} />

    <AccountFormSyncSettings {s} />

    <AccountFormFolderMapping {s} />
  </div>
{/if}
