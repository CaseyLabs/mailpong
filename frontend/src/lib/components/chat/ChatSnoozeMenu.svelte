<script lang="ts">
  // ChatSnoozeMenu — snooze button with presets, "Pick date..." and
  // Unsnooze. Shared by the chat header and the chat row's context menu; the H key
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
    // A zero-size, untabbable trigger that only anchors a menu opened through open().
    hiddenTrigger?: boolean
    // Called once the menu and its date dialog have both closed.
    onClose?: () => void
  }

  let { snoozedUntil, onSnooze, onUnsnooze, hiddenTrigger = false, onClose }: Props = $props()

  let menuOpen = $state(false)
  let pickOpen = $state(false)
  let wasActive = false
  $effect(() => {
    const active = menuOpen || pickOpen
    if (wasActive && !active) onClose?.()
    wasActive = active
  })
  // Recomputed each time the menu opens so "Later today" stays current.
  let presets = $state(snoozePresets())

  export function open() {
    presets = snoozePresets()
    menuOpen = true
  }
</script>

<DropdownMenu.Root bind:open={menuOpen} onOpenChange={(o) => { if (o) presets = snoozePresets() }}>
  <DropdownMenu.Trigger
    class={hiddenTrigger ? 'size-0 overflow-hidden' : 'p-2 rounded-md hover:bg-muted transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden'}
    tabindex={hiddenTrigger ? -1 : undefined}
    title={$_('chat.snooze')}
    aria-label={$_('chat.snooze')}
  >
    <Icon icon="mdi:alarm-snooze" class="{hiddenTrigger ? 'size-0' : 'w-5 h-5'} {snoozedUntil ? 'text-primary' : 'text-muted-foreground'}" />
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
