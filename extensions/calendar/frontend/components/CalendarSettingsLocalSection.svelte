<script lang="ts">
  // Local calendars section of CalendarSettingsDialog.
  import { _ } from 'svelte-i18n'
  import { Button } from '$lib/components/ui/button'
  import ColorPicker from '$lib/components/kit/ColorPicker.svelte'
  import Icon from '@iconify/svelte'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'

  interface Props {
    expanded: boolean
    ontoggle: () => void
    onDelete: (cal: { id: string; displayName: string }) => void
    onAdd: () => void
  }

  let { expanded, ontoggle, onDelete, onAdd }: Props = $props()

  // Local calendars surface separately from CalDAV sources — they don't
  // have sync intervals or Sync Now buttons. Derived for clean rendering.
  const localCalendars = $derived.by(() => {
    const out: { sourceId: string; id: string; displayName: string }[] = []
    for (const src of calendarSources.sources) {
      if (src.type !== 'local') continue
      for (const cal of calendarSources.calendarsBySource[src.id] || []) {
        out.push({ sourceId: src.id, id: cal.id, displayName: cal.displayName })
      }
    }
    return out
  })
</script>

  <section class="border border-border rounded-md">
    <button
      type="button"
      class="w-full flex items-center justify-between p-3 text-left hover:bg-muted/30 rounded-md"
      onclick={() => ontoggle()}
      aria-expanded={expanded}
    >
      <h3 class="text-sm font-semibold text-foreground">
        {$_('calendar.settings.localCalendarsSection')}
      </h3>
      <Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4 text-muted-foreground" />
    </button>
    {#if expanded}
    <div class="space-y-2 px-3 pb-3 pt-1">

    {#if localCalendars.length === 0}
      <p class="text-sm text-muted-foreground py-2">
        {$_('calendar.settings.noLocalCalendars')}
      </p>
    {/if}

    {#each localCalendars as cal (cal.id)}
      <div class="flex items-center gap-3 p-3 border border-border rounded-md">
        <ColorPicker
          value={calendarSources.colorOfHex(cal.id)}
          onchange={(hex) => { void calendarSources.setColor(cal.id, hex) }}
        />
        <span class="flex-1 min-w-0 truncate text-sm text-foreground">
          {cal.displayName}
        </span>
        <Button
          variant="ghost"
          size="sm"
          class="h-8 text-destructive hover:text-destructive shrink-0"
          onclick={() => { onDelete(cal) }}
          aria-label={$_('calendar.common.delete')}
        >
          <Icon icon="mdi:delete-outline" class="w-3.5 h-3.5" />
        </Button>
      </div>
    {/each}

    <Button
      variant="outline"
      size="sm"
      class="mt-2"
      onclick={() => { onAdd() }}
    >
      <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
      {$_('calendar.settings.addLocalCalendar')}
    </Button>
    </div>
    {/if}
  </section>
