<script lang="ts">
  // ChatDaySeparator — full-width heading row above a message that starts a
  // new day and/or a new subject: subject, day, then a hairline.
  import { formatDay } from './chatFormat'

  let { date, subject }: { date?: Date; subject?: string } = $props()
</script>

<div class="flex items-center gap-2 pt-4 pb-1.5 text-xs text-muted-foreground" role="separator">
  {#if subject}<span class="min-w-0 truncate font-semibold text-foreground" title={subject}>{subject}</span>{/if}
  {#if date}<span class="shrink-0">{formatDay(date)}</span>{/if}
  <div class="flex-1 h-px bg-divider"></div>
</div>
