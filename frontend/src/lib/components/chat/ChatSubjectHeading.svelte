<script lang="ts">
  // ChatSubjectHeading — the subject above the messages of one subject run.
  // It sticks to the top of the stream while its run scrolls past, and the
  // next run's heading pushes it away (each run is its own section).
  let { subject }: { subject: string } = $props()
</script>

<h3 class="sticky top-0 z-10 mt-2 flex items-center gap-2 bg-surface-content px-6 pt-2 pb-1.5 text-xs" data-subject-heading>
  <span class="min-w-0 truncate font-semibold text-fg-secondary" title={subject}>{subject}</span>
  <span class="flex-1 h-px bg-divider" aria-hidden="true"></span>
</h3>
