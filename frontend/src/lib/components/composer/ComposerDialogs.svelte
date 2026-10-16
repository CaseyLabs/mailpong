<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { ThreeOptionDialog } from '$lib/components/ui/confirm-dialog'
  import SubmissionReviewDialog from './SubmissionReviewDialog.svelte'
  import SenderReviewDialog from './SenderReviewDialog.svelte'
  import type { ComposerState } from './composerState.svelte'
  import { _ } from '$lib/i18n'
  import { handleDiscardAndClose, handleKeepEditing, handleSaveAndClose } from './composerClose'
  import { handleConfirmEmptySubject, handleConfirmMissingAttachment } from './composerSend'

  let { s }: { s: ComposerState } = $props()

</script>

<!-- Empty Subject Confirmation Dialog -->
<AlertDialog.Root bind:open={s.showEmptySubjectDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.emptySubjectTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.emptySubjectDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={() => handleConfirmEmptySubject(s)}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Missing Attachment Confirmation Dialog -->
<AlertDialog.Root bind:open={s.showMissingAttachmentDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.missingAttachmentTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.missingAttachmentDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={() => handleConfirmMissingAttachment(s)}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Flatpak Drag-and-Drop Info Dialog -->
<AlertDialog.Root bind:open={s.showFlatpakDndDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.flatpakDndTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        <p class="mb-3">{$_('composer.flatpakDndDescription')}</p>
        <p class="mb-2">{$_('composer.flatpakDndGrantExample')}</p>
        <code class="block bg-muted px-3 py-2 rounded text-sm font-mono mb-3 select-all overflow-x-auto">flatpak override --user --filesystem=home io.github.caseylabs.Mailven</code>
        <p class="mb-3 text-sm text-destructive">{$_('composer.flatpakDndSecurityWarning')}</p>
        <p class="text-sm text-muted-foreground">{$_('composer.flatpakDndAlternative')}</p>
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Action onclick={() => s.showFlatpakDndDialog = false}>{$_('common.ok')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Close Confirmation Dialog -->
<ThreeOptionDialog
  bind:open={s.showCloseConfirm}
  title={$_('composer.closeTitle')}
  description={$_('composer.closeDescription')}
  option1Label={$_('composer.discardDraft')}
  option2Label={$_('composer.saveAndClose')}
  option3Label={$_('composer.keepEditing')}
  option1Variant="destructive"
  option2Variant="default"
  loading={s.closeLoading === 'discard' ? 'option1' : s.closeLoading === 'save' ? 'option2' : null}
  onOption1={() => handleDiscardAndClose(s)}
  onOption2={() => handleSaveAndClose(s)}
  onOption3={() => handleKeepEditing(s)}
/>
<SubmissionReviewDialog review={s.submissionReview} />
<SenderReviewDialog review={s.senderReview} />
