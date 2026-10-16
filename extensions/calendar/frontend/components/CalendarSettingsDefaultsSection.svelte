<script lang="ts">
  // Defaults-for-new-events section of CalendarSettingsDialog.
  import { _ } from 'svelte-i18n'
  import * as Select from '$lib/components/ui/select'
  import Icon from '@iconify/svelte'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
  import { calendarSettings } from '$extensions/calendar/frontend/stores/calendarSettings.svelte'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'

  interface Props {
    expanded: boolean
    ontoggle: () => void
  }

  let { expanded, ontoggle }: Props = $props()

  // Defaults section. We expose:
  //   - One global Select listing all writable calendars (grouped by source).
  //   - One per-source Select for each writable source — provider default.
  // Both bind into calendarSettings; stale entries return '' via the store's
  // pruning so the trigger label gracefully falls back to "not set".
  const writableSources = $derived(
    calendarSources.sources.filter(s => s.writable)
  )

  interface GlobalDefaultOption {
    value: string         // calendar ID
    label: string         // "Personal · Murena"
  }

  const globalDefaultOptions = $derived.by<GlobalDefaultOption[]>(() => {
    const out: GlobalDefaultOption[] = []
    for (const src of writableSources) {
      const cals = (calendarSources.calendarsBySource[src.id] || []).filter(c => c.writable !== false)
      for (const cal of cals) {
        out.push({ value: cal.id, label: `${cal.displayName} · ${src.name}` })
      }
    }
    return out
  })

  function globalDefaultLabel(): string {
    const id = calendarSettings.globalDefaultCalendarId
    if (!id) return $_('calendar.add.globalDefaultUnset')
    const match = globalDefaultOptions.find(o => o.value === id)
    if (match) return match.label
    return $_('calendar.add.globalDefaultUnset')
  }

  function providerDefaultLabelFor(src: backend.Source): string {
    const id = calendarSettings.providerDefaultFor(src.id)
    if (!id) return $_('calendar.add.globalDefaultUnset')
    const cal = (calendarSources.calendarsBySource[src.id] || []).find(c => c.id === id)
    return cal?.displayName ?? $_('calendar.add.globalDefaultUnset')
  }

  function writableCalsFor(sourceId: string) {
    return (calendarSources.calendarsBySource[sourceId] || []).filter(c => c.writable !== false)
  }

  function calendarCountLabel(src: backend.Source): string {
    const n = calendarSources.calendarsBySource[src.id]?.length ?? 0
    if (n === 1) return $_('calendar.settings.calendarCountOne')
    return $_('calendar.settings.calendarCount', { values: { count: n } })
  }
</script>

  <!-- Defaults for new events — both global + per-source. Stale entries
       (the source/calendar got deleted) return '' from the store getters
       so the trigger label gracefully shows "not set" until the user
       picks again. -->
  <section class="border border-border rounded-md">
    <button
      type="button"
      class="w-full flex items-center justify-between p-3 text-left hover:bg-muted/30 rounded-md"
      onclick={() => ontoggle()}
      aria-expanded={expanded}
    >
      <h3 class="text-sm font-semibold text-foreground">
        {$_('calendar.settings.defaultsSection')}
      </h3>
      <Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4 text-muted-foreground" />
    </button>
    {#if expanded}
    <div class="space-y-2 px-3 pb-3 pt-1">

    {#if writableSources.length === 0}
      <p class="text-xs text-muted-foreground">
        {$_('calendar.settings.noDefaultsHint')}
      </p>
    {/if}

    {#if writableSources.length > 0}
      <div class="flex items-center justify-between gap-3 p-3 border border-border rounded-md">
        <span class="text-sm text-foreground shrink-0">{$_('calendar.settings.globalDefaultLabel')}</span>
        <Select.Root
          value={calendarSettings.globalDefaultCalendarId}
          onValueChange={(v) => { if (v !== undefined) calendarSettings.setGlobalDefaultCalendarId(v) }}
        >
          <Select.Trigger class="h-8 max-w-xs text-xs">
            {globalDefaultLabel()}
          </Select.Trigger>
          <Select.Content>
            {#each globalDefaultOptions as opt (opt.value)}
              <Select.Item value={opt.value} label={opt.label} />
            {/each}
          </Select.Content>
        </Select.Root>
      </div>

      <div class="text-xs text-muted-foreground mt-3">
        {$_('calendar.settings.providerDefaultsLabel')}
      </div>
      {#each writableSources as src (src.id)}
        {@const cals = writableCalsFor(src.id)}
        {#if cals.length > 0}
          <div class="flex items-center justify-between gap-3 p-3 border border-border rounded-md">
            <div class="min-w-0">
              <div class="text-sm font-medium text-foreground truncate">{src.name}</div>
              <div class="text-xs text-muted-foreground mt-0.5">{calendarCountLabel(src)}</div>
            </div>
            <Select.Root
              value={calendarSettings.providerDefaultFor(src.id)}
              onValueChange={(v) => { if (v !== undefined) calendarSettings.setProviderDefaultFor(src.id, v) }}
            >
              <Select.Trigger class="h-8 max-w-xs text-xs">
                {providerDefaultLabelFor(src)}
              </Select.Trigger>
              <Select.Content>
                {#each cals as cal (cal.id)}
                  <Select.Item value={cal.id} label={cal.displayName} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>
        {/if}
      {/each}
    {/if}
    </div>
    {/if}
  </section>
