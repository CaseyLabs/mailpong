<script lang="ts">
  import { _ } from '$lib/i18n'

  interface Props {
    open: boolean
    currentFontSize: string
    fontSizes: string[]
    onToggle: () => void
    onSetSize: (size: string) => void
  }

  let { open, currentFontSize, fontSizes, onToggle, onSetSize }: Props = $props()
</script>

<!-- Font Size Picker -->
<div class="relative font-size-picker-container" role="presentation" onclick={(e) => e.stopPropagation()}>
  <button
    type="button"
    onclick={onToggle}
    class="p-1.5 rounded hover:bg-muted transition-colors flex items-center gap-0.5 text-xs min-w-[40px] justify-center"
    class:bg-muted={open}
    title={$_('editor.fontSize')}
  >
    {currentFontSize || '15px'}
  </button>

  {#if open}
    <div class="absolute top-full left-0 mt-1 py-1 bg-popover border border-border rounded-md shadow-lg z-50 min-w-[60px]">
      {#each fontSizes as size (size)}
        <button
          type="button"
          onclick={() => onSetSize(size)}
          class="w-full px-3 py-1 text-left text-sm hover:bg-muted transition-colors"
          class:bg-muted={currentFontSize === size}
        >
          {size}
        </button>
      {/each}
    </div>
  {/if}
</div>
