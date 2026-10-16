<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { providers } from '$lib/config/providers'
  import { selectProvider } from './accountFormLoad'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()
</script>
<!-- Step 1: Provider Selection -->
<div class="space-y-4">
  <div class="text-center">
    <h3 class="text-lg font-medium">{$_('account.chooseProvider')}</h3>
    <p class="text-sm text-muted-foreground mt-1">
      {$_('account.chooseProviderHelp')}
    </p>
  </div>

  <div class="grid grid-cols-3 gap-3">
    {#each providers as provider (provider.id)}
      <button
        type="button"
        class="flex flex-col items-center gap-2 p-4 rounded-lg border border-input bg-background hover:bg-accent hover:text-accent-foreground transition-colors"
        onclick={() => selectProvider(s, provider)}
      >
        {#if provider.iconSrc}
          <img src={provider.iconSrc} alt={provider.name} class="w-8 h-8" />
        {:else}
          <Icon icon={provider.icon} class="w-8 h-8" />
        {/if}
        <span class="text-sm font-medium text-center">{provider.name}</span>
      </button>
    {/each}
  </div>

  <div class="flex justify-end pt-2">
    <Button variant="ghost" onclick={() => s.onCancel?.()}>
      {$_('common.cancel')}
    </Button>
  </div>
</div>
