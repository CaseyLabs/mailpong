<script lang="ts">
  // Settings → Chat: the chat-style mail list, thread view and composer.
  import Icon from '@iconify/svelte'
  import * as Select from '$lib/components/ui/select'
  import { Label } from '$lib/components/ui/label'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    sendKey: string
    includeQuote: boolean
    autoAdvance: string
    notifyPriorityOnly: boolean
    showLowGroup: boolean
    combineSenders: boolean
  }

  let {
    sendKey = $bindable(),
    includeQuote = $bindable(),
    autoAdvance = $bindable(),
    notifyPriorityOnly = $bindable(),
    showLowGroup = $bindable(),
    combineSenders = $bindable(),
  }: Props = $props()

  const sendKeyOptions = $derived([
    { value: 'enter', label: $_('settingsChat.sendKeyEnter') },
    { value: 'ctrl-enter', label: $_('settingsChat.sendKeyCtrlEnter') },
  ])

  const autoAdvanceOptions = $derived([
    { value: 'next', label: $_('settingsChat.autoAdvanceNext') },
    { value: 'previous', label: $_('settingsChat.autoAdvancePrevious') },
  ])

  const label = (options: { value: string; label: string }[], value: string) =>
    options.find((o) => o.value === value)?.label ?? value
</script>

<div class="space-y-6 p-1">
  <div class="space-y-2">
    <Label>{$_('settingsChat.sendKey')}</Label>
    <Select.Root value={sendKey} onValueChange={(v) => { if (v) sendKey = v }}>
      <Select.Trigger>
        <Select.Value placeholder={$_('settingsChat.sendKey')}>{label(sendKeyOptions, sendKey)}</Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each sendKeyOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
    <p class="text-xs text-muted-foreground">{$_('settingsChat.sendKeyDescription')}</p>
  </div>

  <div class="flex items-center justify-between gap-4">
    <div>
      <Label for="chat-include-quote">{$_('settingsChat.includeQuote')}</Label>
      <p class="text-xs text-muted-foreground">{$_('settingsChat.includeQuoteDescription')}</p>
    </div>
    <Switch id="chat-include-quote" checked={includeQuote} onCheckedChange={(v) => (includeQuote = v)} />
  </div>

  <div class="border-t border-border"></div>

  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:inbox-arrow-down-outline" class="w-4 h-4" />
      {$_('settingsChat.triage')}
    </h3>

    <div class="space-y-2">
      <Label>{$_('settingsChat.autoAdvance')}</Label>
      <Select.Root value={autoAdvance} onValueChange={(v) => { if (v) autoAdvance = v }}>
        <Select.Trigger>
          <Select.Value placeholder={$_('settingsChat.autoAdvance')}>{label(autoAdvanceOptions, autoAdvance)}</Select.Value>
        </Select.Trigger>
        <Select.Content>
          {#each autoAdvanceOptions as opt (opt.value)}
            <Select.Item value={opt.value} label={opt.label} />
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">{$_('settingsChat.autoAdvanceDescription')}</p>
    </div>

    <div class="flex items-center justify-between gap-4">
      <div>
        <Label for="chat-combine-senders">{$_('settingsChat.combineSenders')}</Label>
        <p class="text-xs text-muted-foreground">{$_('settingsChat.combineSendersDescription')}</p>
      </div>
      <Switch id="chat-combine-senders" checked={combineSenders} onCheckedChange={(v) => (combineSenders = v)} />
    </div>

    <div class="flex items-center justify-between gap-4">
      <div>
        <Label for="chat-show-low-group">{$_('settingsChat.showLowGroup')}</Label>
        <p class="text-xs text-muted-foreground">{$_('settingsChat.showLowGroupDescription')}</p>
      </div>
      <Switch id="chat-show-low-group" checked={showLowGroup} onCheckedChange={(v) => (showLowGroup = v)} />
    </div>

    <div class="flex items-center justify-between gap-4">
      <div>
        <Label for="chat-notify-priority-only">{$_('settingsChat.notifyPriorityOnly')}</Label>
        <p class="text-xs text-muted-foreground">{$_('settingsChat.notifyPriorityOnlyDescription')}</p>
      </div>
      <Switch id="chat-notify-priority-only" checked={notifyPriorityOnly} onCheckedChange={(v) => (notifyPriorityOnly = v)} />
    </div>
  </div>
</div>
