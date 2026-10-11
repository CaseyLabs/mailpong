<script lang="ts">
  // ChatGroupHeader — a chat list group: an optional collapse header, its
  // rows, and a quiet "{count} more" / "Show fewer" button when the group has
  // more rows than its trimmed view.
  import type { Snippet } from 'svelte'
  import Icon from '@iconify/svelte'

  interface Props {
    label: string
    open: boolean
    // Absent for a group without a header (it can't collapse).
    onToggle?: () => void
    // The "Show fewer" / "{count} more" button text; absent hides it.
    fewerLabel?: string
    onToggleFewer: () => void
    children: Snippet
  }

  let { label, open, onToggle, fewerLabel, onToggleFewer, children }: Props = $props()
</script>

<div role="group" aria-label={label}>
  {#if onToggle}
    <button
      class="flex items-center gap-1 w-full px-4 pt-3 pb-1 text-left text-xs font-semibold text-muted-foreground hover:text-foreground"
      aria-expanded={open}
      onclick={onToggle}
    >
      <Icon icon={open ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="w-3.5 h-3.5 shrink-0" />
      {label}
    </button>
  {/if}
  {#if open}
    {@render children()}
    {#if fewerLabel}
      <button class="px-4 py-1 text-xs text-muted-foreground hover:text-foreground hover:underline" onclick={onToggleFewer}>
        {fewerLabel}
      </button>
    {/if}
  {/if}
</div>
