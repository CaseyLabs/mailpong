<script lang="ts">
  interface Props {
    currentFontSize: string
    onSetFontSize: (size: string) => void
  }

  let { currentFontSize, onSetFontSize }: Props = $props()

  // Font sizes
  const fontSizes = ['10px', '12px', '14px', '15px', '16px', '18px', '20px', '24px', '28px', '32px']
</script>

<div class="absolute top-full left-0 mt-1 py-1 bg-popover border border-border rounded-md shadow-lg z-50 min-w-[60px]">
  {#each fontSizes as size (size)}
    <button
      onclick={() => onSetFontSize(size)}
      class="w-full px-3 py-1 text-left text-sm hover:bg-muted transition-colors"
      class:bg-muted={currentFontSize === size}
    >
      {size}
    </button>
  {/each}
</div>
