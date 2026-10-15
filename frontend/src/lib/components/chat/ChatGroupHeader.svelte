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

<!-- A group without a header (the flat list) is not announced as a group. -->
<div role={onToggle || label ? 'group' : undefined} aria-label={label || undefined}>
  {#if onToggle}
    <button
      class="flex items-center gap-1 w-full px-4 pt-4 pb-1 text-left text-meta font-normal text-fg-tertiary hover:text-fg-primary"
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
      <button class="px-4 py-1 text-xs text-fg-tertiary hover:text-fg-primary hover:underline" onclick={onToggleFewer}>
        {fewerLabel}
      </button>
    {/if}
  {/if}
</div>
