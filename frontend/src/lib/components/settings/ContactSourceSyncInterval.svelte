<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import { Label } from '$lib/components/ui/label'
  import { _ } from '$lib/i18n'
  import type { ContactSourceState } from './contactSourceState.svelte'

  let { s }: { s: ContactSourceState } = $props()

  // Sync interval options (value is string for Select component)
  const syncIntervalOptions = $derived([
    { value: '0', label: $_('contactSource.manualOnly') },
    { value: '15', label: $_('contactSource.every15Min') },
    { value: '30', label: $_('contactSource.every30Min') },
    { value: '60', label: $_('contactSource.everyHour') },
    { value: '120', label: $_('contactSource.every2Hours') },
    { value: '360', label: $_('contactSource.every6Hours') },
    { value: '1440', label: $_('contactSource.daily') },
  ])

  function getSyncIntervalLabel(value: number): string {
    return syncIntervalOptions.find(opt => opt.value === String(value))?.label || $_('contactSource.minutesFallback', { values: { value } })
  }
</script>

<div class="space-y-2">
  <Label>{$_('contactSource.syncInterval')}</Label>
  <Select.Root value={s.syncIntervalStr} onValueChange={s.handleSyncIntervalChange}>
    <Select.Trigger>
      <Select.Value placeholder={$_('contactSource.selectInterval')}>
        {getSyncIntervalLabel(s.syncInterval)}
      </Select.Value>
    </Select.Trigger>
    <Select.Content>
      {#each syncIntervalOptions as opt (opt.value)}
        <Select.Item value={opt.value} label={opt.label} />
      {/each}
    </Select.Content>
  </Select.Root>
</div>
