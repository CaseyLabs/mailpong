<script lang="ts">
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import { syncPeriodOptions, syncIntervalOptions } from '$lib/config/providers'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()

  // Read receipt request policy options
  const readReceiptRequestOptions = $derived([
    { value: 'never', label: $_('account.neverRequest') },
    { value: 'ask', label: $_('account.askEachTime') },
    { value: 'always', label: $_('account.alwaysRequest') },
  ])

  function getSyncPeriodLabel(value: string): string {
    const numValue = Number(value)
    const option = syncPeriodOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }

  function getSyncIntervalLabel(value: string): string {
    const numValue = Number(value)
    const option = syncIntervalOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} min`
  }

  function getReadReceiptLabel(value: string): string {
    return readReceiptRequestOptions.find(opt => opt.value === value)?.label || value
  }
</script>
<!-- Sync Settings -->
<div class="space-y-2">
  <Label>{$_('account.syncPeriod')}</Label>
  <Select.Root bind:value={s.syncPeriodDays}>
    <Select.Trigger>
      <Select.Value placeholder="Select">
        {getSyncPeriodLabel(s.syncPeriodDays)}
      </Select.Value>
    </Select.Trigger>
    <Select.Content>
      {#each syncPeriodOptions as opt (opt.value)}
        <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
      {/each}
    </Select.Content>
  </Select.Root>
  <p class="text-xs text-muted-foreground">
    {$_('account.syncPeriodHelp')}
  </p>
</div>

<!-- Check Interval Settings -->
<div class="space-y-2">
  <Label>{$_('account.checkNewMail')}</Label>
  <Select.Root bind:value={s.syncInterval}>
    <Select.Trigger>
      <Select.Value placeholder="Select">
        {getSyncIntervalLabel(s.syncInterval)}
      </Select.Value>
    </Select.Trigger>
    <Select.Content>
      {#each syncIntervalOptions as opt (opt.value)}
        <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
      {/each}
    </Select.Content>
  </Select.Root>
  <p class="text-xs text-muted-foreground">
    {$_('account.checkNewMailHelp')}
  </p>
</div>

<!-- Read Receipt Settings -->
<div class="space-y-2">
  <Label>{$_('account.requestReadReceipts')}</Label>
  <Select.Root bind:value={s.readReceiptRequestPolicy}>
    <Select.Trigger>
      <Select.Value placeholder="Select">
        {getReadReceiptLabel(s.readReceiptRequestPolicy)}
      </Select.Value>
    </Select.Trigger>
    <Select.Content>
      {#each readReceiptRequestOptions as opt (opt.value)}
        <Select.Item value={opt.value} label={opt.label} />
      {/each}
    </Select.Content>
  </Select.Root>
  <p class="text-xs text-muted-foreground">
    {$_('account.requestReadReceiptsHelp')}
  </p>
</div>
