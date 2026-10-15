<script lang="ts">
  // ChatCompactRow — one muted line standing for a run of earlier messages;
  // a button that expands or collapses the run.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    count: number
    open: boolean
    onToggle: (el: HTMLElement) => void
  }

  let { count, open, onToggle }: Props = $props()
</script>

<div class="w-full px-6">
  <button
    type="button"
    data-compact-row
    class="mt-3 flex w-full items-center gap-1.5 rounded-md px-1 py-1 text-left text-meta text-fg-tertiary hover:text-fg-secondary focus-visible:outline-2 focus-visible:outline-ring"
    aria-expanded={open}
    onclick={(e) => onToggle(e.currentTarget)}
  >
    <Icon icon={open ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
    <span class="truncate">{open ? $_('chat.hideEarlier', { values: { count } }) : $_('chat.showEarlier', { values: { count } })}</span>
  </button>
</div>
