<script lang="ts">
  // ChatDaySeparator — full-width row above a message that starts a new day:
  // the day, then a hairline. Tight sits it just under a subject heading.
  import { formatDay } from './chatFormat'

  let { date, tight = false }: { date: Date; tight?: boolean } = $props()
</script>

<div class="flex items-center gap-2 {tight ? 'pt-1' : 'pt-4'} pb-1.5 text-xs text-fg-tertiary" role="separator">
  <span class="shrink-0">{formatDay(date)}</span>
  <div class="flex-1 h-px bg-divider"></div>
</div>
