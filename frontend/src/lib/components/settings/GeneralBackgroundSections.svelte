<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    runBackground: boolean
    startHidden: boolean
    autostart: boolean
    onRunBackgroundChange: (value: boolean) => void
    onStartHiddenChange: (value: boolean) => void
    onAutostartChange: (value: boolean) => void
  }

  let {
    runBackground = $bindable(),
    startHidden = $bindable(),
    autostart = $bindable(),
    onRunBackgroundChange,
    onStartHiddenChange,
    onAutostartChange,
  }: Props = $props()

  function handleRunBackgroundChange(value: boolean) {
    runBackground = value
    if (!value) {
      startHidden = false
    }
    onRunBackgroundChange?.(value)
  }

  function handleStartHiddenChange(value: boolean) {
    startHidden = value
    if (value && !runBackground) {
      runBackground = true
      onRunBackgroundChange?.(true)
    }
    onStartHiddenChange?.(value)
  }

  function handleAutostartChange(value: boolean) {
    autostart = value
    onAutostartChange?.(value)
  }
</script>

  <!-- Divider -->
  <div class="border-t border-border"></div>

  <!-- Background Section -->
  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:application-cog-outline" class="w-4 h-4" />
      {$_('settingsGeneral.background')}
    </h3>

    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="space-y-0.5">
          <Label for="run-background">{$_('settingsGeneral.runInBackground')}</Label>
          <p class="text-xs text-muted-foreground">
            {$_('settingsGeneral.runInBackgroundHelp')}
          </p>
        </div>
        <Switch
          id="run-background"
          bind:checked={runBackground}
          onCheckedChange={handleRunBackgroundChange}
        />
      </div>
    </div>

    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="space-y-0.5">
          <Label for="start-hidden" class={!runBackground ? 'text-muted-foreground' : ''}>{$_('settingsGeneral.startHidden')}</Label>
          <p class="text-xs text-muted-foreground">
            {$_('settingsGeneral.startHiddenHelp')}
          </p>
        </div>
        <Switch
          id="start-hidden"
          bind:checked={startHidden}
          onCheckedChange={handleStartHiddenChange}
          disabled={!runBackground}
        />
      </div>
    </div>
  </div>

  <!-- Divider -->
  <div class="border-t border-border"></div>

  <!-- Startup Section -->
  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:power" class="w-4 h-4" />
      {$_('settingsGeneral.startup')}
    </h3>

    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="space-y-0.5">
          <Label for="autostart">{$_('settingsGeneral.autostartOnLogin')}</Label>
          <p class="text-xs text-muted-foreground">
            {$_('settingsGeneral.autostartHelp')}
          </p>
        </div>
        <Switch
          id="autostart"
          bind:checked={autostart}
          onCheckedChange={handleAutostartChange}
        />
      </div>
    </div>
  </div>
