<script lang="ts">
  import { _ } from '$lib/i18n'

  interface Props {
    currentColor: string | null
    onSetColor: (color: string) => void
    onCustomColor: (event: Event) => void
    onReset: () => void
  }

  let { currentColor, onSetColor, onCustomColor, onReset }: Props = $props()

  // Preset colors
  const presetColors = [
    '#000000', '#374151', '#6b7280',
    '#dc2626', '#ea580c', '#ca8a04',
    '#16a34a', '#0891b2', '#2563eb',
    '#7c3aed', '#c026d3', '#e11d48',
  ]
</script>

<div class="absolute top-full left-0 mt-1 p-2 bg-popover border border-border rounded-md shadow-lg z-50">
  <div class="grid grid-cols-4 gap-1 mb-2">
    {#each presetColors as color (color)}
      <button
        onclick={() => onSetColor(color)}
        class="w-6 h-6 rounded border border-border hover:scale-110 transition-transform"
        style="background-color: {color}"
        title={color}
      ></button>
    {/each}
  </div>
  <div class="flex items-center gap-2 pt-2 border-t border-border">
    <input
      type="color"
      value={currentColor || '#000000'}
      onchange={onCustomColor}
      class="w-6 h-6 rounded cursor-pointer"
      title={$_('editor.customColor')}
    />
    <button
      onclick={onReset}
      class="text-xs text-muted-foreground hover:text-foreground"
    >
      {$_('editor.reset')}
    </button>
  </div>
</div>
