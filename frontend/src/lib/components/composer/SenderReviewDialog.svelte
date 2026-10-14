<script lang="ts">
  import { _ } from '$lib/i18n'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  import type { SubmissionReview } from './submissionReview.svelte'
  let { review }: { review: SubmissionReview } = $props()
</script>

<ConfirmDialog
  bind:open={review.open}
  title={$_('composer.senderReviewTitle')}
  description={review.outcome}
  confirmLabel={$_('composer.senderReviewConfirm')}
  cancelLabel={$_('common.cancel')}
  onConfirm={() => review.finish(true)}
  onCancel={() => review.finish(false)}
/>
