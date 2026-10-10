<script lang="ts">
  // Labelled icon button for primary toolbar actions (Compose, Reply, Archive…).
  // `primary` uses the accent colours; otherwise a bordered neutral style.
  // `compact` shows only the icon, keeping the label for screen readers.
  import Icon from '@iconify/svelte'

  interface Props {
    icon: string
    label: string
    title?: string
    primary?: boolean
    compact?: boolean
    disabled?: boolean
    onclick: () => void
  }

  let { icon, label, title = label, primary = false, compact = false, disabled = false, onclick }: Props = $props()
</script>

<button
  class="flex items-center gap-2 {compact ? 'px-2' : 'px-3'} py-2 rounded-md text-sm font-medium whitespace-nowrap shrink-0 transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:opacity-50 disabled:pointer-events-none {primary
    ? 'border border-transparent bg-primary text-primary-foreground hover:bg-primary/90'
    : 'border border-border text-foreground hover:bg-muted'}"
  {title}
  {disabled}
  {onclick}
>
  <Icon {icon} class="w-4 h-4" />
  <span class:sr-only={compact}>{label}</span>
</button>
