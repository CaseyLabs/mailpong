<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { handleTestConnection } from './accountFormSubmit'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()
</script>
<!-- Test Result -->
{#if s.testResult}
  <div
    class="flex items-start gap-2 p-3 rounded-lg {s.testResult.success
      ? 'bg-green-500/10 border border-green-500/20'
      : 'bg-destructive/10 border border-destructive/20'}"
  >
    <Icon
      icon={s.testResult.success ? 'mdi:check-circle' : 'mdi:alert-circle'}
      class="w-5 h-5 shrink-0 mt-0.5 {s.testResult.success
        ? 'text-green-500'
        : 'text-destructive'}"
    />
    <p
      class="text-sm {s.testResult.success
        ? 'text-green-600 dark:text-green-400'
        : 'text-destructive'}"
    >
      {s.testResult.message}
    </p>
  </div>
{/if}

<!-- Actions -->
<div class="flex items-center justify-between pt-4 border-t border-border">
  <Button
    type="button"
    variant="outline"
    onclick={() => handleTestConnection(s)}
    disabled={s.testing || s.submitting}
  >
    {#if s.testing}
      <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
    {:else}
      <Icon icon="mdi:connection" class="w-4 h-4 mr-2" />
    {/if}
    {$_('account.testConnection')}
  </Button>

  <div class="flex gap-2">
    <Button type="button" variant="ghost" onclick={s.onCancel} disabled={s.submitting}>
      {$_('common.cancel')}
    </Button>
    <Button type="submit" disabled={s.submitting || s.testing}>
      {#if s.submitting}
        <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
      {/if}
      {s.editAccount ? $_('common.saveChanges') : $_('account.addAccount')}
    </Button>
  </div>
</div>
