<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { ComposerState } from './composerState.svelte'
  import { _ } from '$lib/i18n'
  import { handleClose } from './composerClose'
  import { getDisplayMode } from './composerContent'
  import { handlePopOut, handleSend } from './composerSend'

  let { s }: { s: ComposerState } = $props()

  // Computed draft status indicator
  let draftStatusIcon = $derived.by(() => {
    if (s.saveStatus === 'saving') return 'mdi:loading'
    if (s.saveStatus === 'error') return 'mdi:alert-circle'
    if (s.saveStatus !== 'saved' || !s.lastSavedAt) return ''
    if (s.encryptMessage || s.pgpEncryptMessage) {
      return s.syncStatus === 'synced' ? 'mdi:lock-check' : 'mdi:lock'
    }
    switch (s.syncStatus) {
      case 'synced': return 'mdi:cloud-check'
      case 'pending': return 'mdi:cloud-upload'
      case 'failed': return 'mdi:cloud-off-outline'
      default: return ''
    }
  })
  let draftStatusColor = $derived.by(() => {
    if (s.saveStatus === 'saving') return ''
    if (s.saveStatus === 'error') return 'text-red-500'
    if (s.saveStatus !== 'saved' || !s.lastSavedAt) return ''
    switch (s.syncStatus) {
      case 'synced': return 'text-green-500'
      case 'pending': return 'text-blue-500'
      case 'failed': return 'text-yellow-500'
      default: return ''
    }
  })
  let draftStatusLabel = $derived.by(() => {
    if (s.saveStatus === 'saving') return (s.encryptMessage || s.pgpEncryptMessage) ? $_('composer.encrypting') : $_('composer.saving')
    if (s.saveStatus === 'error') return $_('composer.saveFailed')
    if (s.saveStatus !== 'saved' || !s.lastSavedAt) return ''
    if (s.encryptMessage || s.pgpEncryptMessage) {
      switch (s.syncStatus) {
        case 'synced': return $_('composer.encryptedSynced')
        case 'pending': return $_('composer.encryptedDraft')
        case 'failed': return $_('composer.encryptedOffline')
        default: return ''
      }
    }
    switch (s.syncStatus) {
      case 'synced': return $_('composer.synced')
      case 'pending': return $_('composer.savedLocally')
      case 'failed': return $_('composer.savedLocallyOffline')
      default: return ''
    }
  })
</script>

<!-- Header -->
<div class="flex items-center justify-between px-4 py-3 border-b border-border">
  <div class="flex items-center gap-3">
    <h2 class="text-lg font-semibold">
      {#if getDisplayMode(s) === 'new'}
        {$_('composer.newMessage')}
      {:else if getDisplayMode(s) === 'reply'}
        {$_('composer.reply')}
      {:else if getDisplayMode(s) === 'reply-all'}
        {$_('composer.replyAll')}
      {:else if getDisplayMode(s) === 'forward'}
        {$_('composer.forward')}
      {/if}
    </h2>
    <!-- Draft status indicator -->
    {#if draftStatusLabel}
      <span class="text-xs text-muted-foreground flex items-center gap-1">
        <Icon icon={draftStatusIcon} class="w-3 h-3 {draftStatusColor} {s.saveStatus === 'saving' ? 'animate-spin' : ''}" />
        {draftStatusLabel}
      </span>
    {/if}
  </div>
  <div class="flex items-center gap-2">
    <!-- Pop-out button (only shown in main window, not detached) -->
    {#if !s.isDetached && s.api.openComposerWindow}
      <button
        onclick={() => handlePopOut(s)}
        disabled={s.poppingOut || s.sending}
        class="p-1.5 text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        title={$_('composer.openInNewWindow')}
      >
        {#if s.poppingOut}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
        {:else}
          <Icon icon="mdi:open-in-new" class="w-4 h-4" />
        {/if}
      </button>
    {/if}
    {#if s.countdown.seconds > 0}
      <span class="text-sm text-muted-foreground" role="status">{$_('composer.sendingIn', { values: { seconds: s.countdown.seconds } })}</span>
      <button
        onclick={() => s.countdown.undo()}
        class="px-3 py-1.5 text-sm font-medium text-primary hover:bg-muted rounded-md transition-colors"
      >
        {$_('composer.undo')}
      </button>
      <button
        onclick={() => s.countdown.sendNow()}
        class="px-4 py-1.5 text-sm font-medium text-primary-foreground bg-primary hover:bg-primary/90 rounded-md transition-colors flex items-center gap-2"
      >
        <Icon icon="mdi:send" class="w-4 h-4" />
        {$_('composer.sendNow')}
      </button>
    {:else}
    <button
      onclick={() => handleClose(s)}
      disabled={s.poppingOut}
      class="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors disabled:opacity-50"
    >
      {$_('composer.close')}
    </button>
    <button
      onclick={() => handleSend(s)}
      disabled={s.sending || s.poppingOut || s.toRecipients.length === 0}
      class="px-4 py-1.5 text-sm font-medium text-primary-foreground bg-primary hover:bg-primary/90 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
    >
      {#if s.sending}
        <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
        {$_('composer.sending')}
      {:else if s.poppingOut}
        <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
        {$_('composer.opening')}
      {:else}
        <Icon icon="mdi:send" class="w-4 h-4" />
        {$_('composer.send')}
      {/if}
    </button>
    {/if}
  </div>
</div>
