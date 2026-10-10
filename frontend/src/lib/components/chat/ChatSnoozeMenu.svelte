<script lang="ts">
  // ChatSnoozeMenu — snooze button with presets, "Pick date..." and
  // Unsnooze. Shared by the chat header and the row hover actions; the H key
  // opens it through open().
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import ChatSnoozeDateDialog from './ChatSnoozeDateDialog.svelte'
  import { formatSnoozePreset, snoozePresets } from './chatSnooze'

  interface Props {
    snoozedUntil: Date | null
    onSnooze: (until: Date) => void
    onUnsnooze: () => void
    triggerClass?: string
    iconClass?: string
    // True while the menu or its date dialog is open.
    onActiveChange?: (active: boolean) => void
  }

  let { snoozedUntil, onSnooze, onUnsnooze, triggerClass = 'p-2 rounded-md hover:bg-muted transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden', iconClass = 'w-5 h-5', onActiveChange }: Props = $props()

  let menuOpen = $state(false)
  let pickOpen = $state(false)
  $effect(() => onActiveChange?.(menuOpen || pickOpen))
  // Recomputed each time the menu opens so "Later today" stays current.
  let presets = $state(snoozePresets())

  export function open() {
    presets = snoozePresets()
    menuOpen = true
  }
</script>

<DropdownMenu.Root bind:open={menuOpen} onOpenChange={(o) => { if (o) presets = snoozePresets() }}>
  <DropdownMenu.Trigger class={triggerClass} title={$_('chat.snooze')} aria-label={$_('chat.snooze')}>
    <Icon icon="mdi:alarm-snooze" class="{iconClass} {snoozedUntil ? 'text-primary' : 'text-muted-foreground'}" />
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end">
    {#each presets as p (p.id)}
      <DropdownMenu.Item onSelect={() => onSnooze(p.until)}>
        <span class="flex-1">{$_(p.labelKey)}</span>
        <span class="ml-4 text-xs text-muted-foreground">{formatSnoozePreset(p.until)}</span>
      </DropdownMenu.Item>
    {/each}
    <DropdownMenu.Item onSelect={() => (pickOpen = true)}>{$_('chat.snoozePickDate')}</DropdownMenu.Item>
    {#if snoozedUntil}
      <DropdownMenu.Separator />
      <DropdownMenu.Item onSelect={onUnsnooze}>{$_('chat.unsnooze')}</DropdownMenu.Item>
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>

<ChatSnoozeDateDialog bind:open={pickOpen} onPick={onSnooze} />
