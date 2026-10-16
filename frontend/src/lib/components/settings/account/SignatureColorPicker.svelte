<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    open: boolean
    currentColor: string | null
    presetColors: string[]
    onToggle: () => void
    onSetColor: (color: string) => void
    onCustomColor: (event: Event) => void
    onRemoveColor: () => void
  }

  let { open, currentColor, presetColors, onToggle, onSetColor, onCustomColor, onRemoveColor }: Props = $props()
</script>

<!-- Color Picker -->
<div class="relative color-picker-container" role="presentation" onclick={(e) => e.stopPropagation()}>
  <button
    type="button"
    onclick={onToggle}
    class="p-1.5 rounded hover:bg-muted transition-colors flex items-center gap-0.5"
    class:bg-muted={open}
    title={$_('editor.textColor')}
  >
    <Icon icon="mdi:format-color-text" class="w-4 h-4" />
    <div
      class="w-3 h-1 rounded-sm"
      style="background-color: {currentColor || '#000000'}"
    ></div>
  </button>

  {#if open}
    <div class="absolute top-full left-0 mt-1 p-2 bg-popover border border-border rounded-md shadow-lg z-50">
      <div class="grid grid-cols-4 gap-1 mb-2">
        {#each presetColors as color (color)}
          <button
            type="button"
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
          type="button"
          onclick={onRemoveColor}
          class="text-xs text-muted-foreground hover:text-foreground"
        >
          {$_('editor.reset')}
        </button>
      </div>
    </div>
  {/if}
</div>
