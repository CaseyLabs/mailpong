<script lang="ts">
  import Icon from '@iconify/svelte'
  import PlainTextEditor from './PlainTextEditor.svelte'
  import EditorToolbar from './EditorToolbar.svelte'
  import ComposerAttachmentList from './ComposerAttachmentList.svelte'
  import { getDarkComposerBody } from '$lib/stores/settings.svelte'
  import { getIsDarkActive } from '$lib/stores/theme.svelte'
  import type { ComposerState } from './composerState.svelte'
  import { _ } from '$lib/i18n'
  import { handleAttachFiles, removeAttachment } from './composerAttachments'
  import { scheduleDraftSave } from './composerDraft'
  import { insertImage, loadComposerImages, togglePlainTextMode } from './composerEditorMode'
  import { handleImportRecipientCert, handleImportRecipientPGPKey } from './composerSecurity'

  let { s }: { s: ComposerState } = $props()

  // Keep the composer message body white in dark mode unless the user opted into
  // a dark composer body — mirrors the "Dark mail content" viewer setting.
  const forceLightComposer = $derived(getIsDarkActive() && !getDarkComposerBody())
</script>

<!-- Toolbar - extracted to separate component for performance -->
<!-- Alt+T to focus toolbar, Tab skips it -->
<EditorToolbar
  bind:this={s.toolbarRef}
  editor={s.editor}
  isPlainTextMode={s.isPlainTextMode}
  onTogglePlainText={() => togglePlainTextMode(s)}
  onInsertImage={() => insertImage(s)}
/>

<!-- Remote images blocked bar -->
{#if s.composerImagesBlocked}
  <div class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md bg-yellow-500/10 border border-yellow-500/30 text-sm">
    <Icon icon="mdi:image-off" class="w-4 h-4 text-yellow-600 shrink-0" />
    <span class="text-yellow-700 dark:text-yellow-400">{$_('viewer.remoteImagesBlocked')}</span>
    <button
      class="ml-auto px-2 py-1 text-xs font-medium rounded bg-yellow-600 text-white hover:bg-yellow-700 transition-colors"
      onclick={() => loadComposerImages(s)}
    >
      {$_('viewer.loadImages')}
    </button>
  </div>
{/if}

<!-- Editor -->
<div class="flex-1 overflow-auto {forceLightComposer ? 'composer-body--light' : 'bg-white dark:bg-zinc-900'}">
  <!-- Both surfaces stay mounted; we toggle visibility instead of using
       {#if}/{:else}. Unmounting the editor <div> orphaned the TipTap
       instance, so a later switch back to rich text wrote into a dead
       editor and the body vanished. -->
  <div class="h-full {s.isPlainTextMode ? '' : 'hidden'}">
    <PlainTextEditor
      bind:this={s.plainTextRef}
      bind:value={s.plainTextContent}
      placeholder={$_('composer.writePlaceholder')}
      onInput={() => scheduleDraftSave(s)}
    />
  </div>
  <div bind:this={s.editorElement} class="h-full {s.isPlainTextMode ? 'hidden' : ''}"></div>
</div>

<!-- Attachments List -->
<ComposerAttachmentList attachments={s.attachments} onRemove={(index) => removeAttachment(s, index)} />
<p class="px-4 py-1.5 text-xs text-muted-foreground">
  {$_('composer.messageSizeGuidance')}
</p>

<!-- Missing S/MIME cert warning -->
{#if s.encryptMessage && s.missingCertRecipients.length > 0}
  <div class="flex items-center gap-2 text-xs px-3 py-1.5 bg-amber-50 dark:bg-amber-950/30 border-t border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300">
    <Icon icon="mdi:alert" class="w-3.5 h-3.5 shrink-0" />
    <span class="flex-1">{$_('composer.noCertFor', { values: { emails: s.missingCertRecipients.join(', ') } })}</span>
    <button onclick={() => handleImportRecipientCert(s)} class="px-2 py-0.5 rounded bg-amber-200 dark:bg-amber-800 hover:bg-amber-300 dark:hover:bg-amber-700 font-medium transition-colors">{$_('composer.import')}</button>
    <button onclick={() => s.encryptMessage = false} class="px-2 py-0.5 rounded hover:bg-amber-200 dark:hover:bg-amber-800 font-medium transition-colors">{$_('common.cancel')}</button>
  </div>
{/if}

<!-- Missing PGP key warning -->
{#if s.pgpEncryptMessage && s.missingPGPKeyRecipients.length > 0}
  <div class="flex items-center gap-2 text-xs px-3 py-1.5 bg-amber-50 dark:bg-amber-950/30 border-t border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300">
    <Icon icon="mdi:alert" class="w-3.5 h-3.5 shrink-0" />
    <span class="flex-1">{$_('composer.noPGPKeyFor', { values: { emails: s.missingPGPKeyRecipients.join(', ') } })}</span>
    <button onclick={() => handleImportRecipientPGPKey(s)} class="px-2 py-0.5 rounded bg-amber-200 dark:bg-amber-800 hover:bg-amber-300 dark:hover:bg-amber-700 font-medium transition-colors">{$_('composer.import')}</button>
    <button onclick={() => s.pgpEncryptMessage = false} class="px-2 py-0.5 rounded hover:bg-amber-200 dark:hover:bg-amber-800 font-medium transition-colors">{$_('common.cancel')}</button>
  </div>
{/if}

<!-- Footer -->
<div class="flex items-center gap-2 px-4 py-2 border-t border-border text-sm text-muted-foreground">
  <button
    onclick={() => handleAttachFiles(s)}
    class="flex items-center gap-1 hover:text-foreground transition-colors"
  >
    <Icon icon="mdi:attachment" class="w-4 h-4" />
    {$_('composer.attachFiles')}
  </button>
  {#if s.attachments.length > 0}
    <span class="text-xs">
      {$_('composer.filesAttached', { values: { count: s.attachments.length } })}
    </span>
  {/if}
  <div class="flex-1"></div>
  {#if s.showReadReceiptOption}
    <label class="flex items-center gap-1.5 text-xs cursor-pointer hover:text-foreground transition-colors">
      <input
        type="checkbox"
        bind:checked={s.requestReadReceipt}
        class="w-3.5 h-3.5 rounded border-border accent-primary"
      />
      {$_('composer.requestReadReceipt')}
    </label>
  {/if}
  <span class="text-xs">{$_('composer.ctrlEnterToSend')}</span>
</div>
