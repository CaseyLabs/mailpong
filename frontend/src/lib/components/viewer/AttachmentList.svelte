<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { Snippet } from 'svelte'
  import { GetAttachments, SaveAttachmentAs, SaveAllAttachments, OpenAttachment, SaveEncryptedAttachmentAs, OpenEncryptedAttachment, SaveAllEncryptedAttachments, OpenFile, OpenFolder } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { message as messageModels } from '../../../../wailsjs/go/models'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'

  // Decrypted attachment metadata (from encrypted messages, not in DB)
  interface DecryptedAttachment {
    filename: string
    contentType: string
    size: number
    isInline: boolean
    contentId: string
  }

  interface Props {
    messageId: string
    encryptedAttachments?: DecryptedAttachment[]
    // Wrapper class and heading, rendered only while there is something to
    // show, so a message whose parts are all embedded in the body (and so
    // filtered out by GetAttachments) gets no empty section
    class?: string
    header?: Snippet
  }

  let { messageId, encryptedAttachments, class: className = '', header }: Props = $props()

  // State
  let attachments = $state<messageModels.Attachment[]>([])
  let downloadingIds = $state<Set<string>>(new Set())
  let savingAll = $state(false)

  // Determine if we're in encrypted mode
  let isEncrypted = $derived(encryptedAttachments != null && encryptedAttachments.length > 0)
  // Stays hidden while loading, so a message whose parts are all embedded
  // doesn't flash an empty heading before GetAttachments filters them out
  let visible = $derived(isEncrypted || attachments.length > 0)

  // Load attachments when messageId changes (only for non-encrypted)
  $effect(() => {
    if (encryptedAttachments && encryptedAttachments.length > 0) {
      attachments = []
      return
    }
    if (messageId) {
      loadAttachments(messageId)
    } else {
      attachments = []
    }
  })

  async function loadAttachments(msgId: string) {
    attachments = []
    try {
      const result = await GetAttachments(msgId)
      // Drop a response for a message the viewer has already moved past
      if (msgId === messageId) attachments = result || []
    } catch (err) {
      console.error('Failed to load attachments:', err)
    }
  }

  async function handleDownload(att: messageModels.Attachment) {
    downloadingIds = new Set([...downloadingIds, att.id])
    try {
      const path = await SaveAttachmentAs(att.id)
      if (path) {
        toasts.success($_('toast.attachmentSaved', { values: { filename: att.filename } }), [
          {
            label: $_('attachment.openFile'),
            onClick: () => OpenFile(path)
          },
          {
            label: $_('attachment.showInFolder'),
            onClick: () => OpenFolder(path)
          }
        ])
      }
    } catch (err) {
      console.error('Failed to save attachment:', err)
      toasts.error($_('toast.failedToSaveAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== att.id))
    }
  }

  async function handleOpen(att: messageModels.Attachment) {
    downloadingIds = new Set([...downloadingIds, att.id])
    try {
      await OpenAttachment(att.id)
    } catch (err) {
      console.error('Failed to open attachment:', err)
      toasts.error($_('toast.failedToOpenAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== att.id))
    }
  }

  async function handleEncryptedDownload(att: DecryptedAttachment) {
    const key = att.filename
    downloadingIds = new Set([...downloadingIds, key])
    try {
      const path = await SaveEncryptedAttachmentAs(messageId, att.filename)
      if (path) {
        toasts.success($_('toast.attachmentSaved', { values: { filename: att.filename } }), [
          {
            label: $_('attachment.openFile'),
            onClick: () => OpenFile(path)
          },
          {
            label: $_('attachment.showInFolder'),
            onClick: () => OpenFolder(path)
          }
        ])
      }
    } catch (err) {
      console.error('Failed to save encrypted attachment:', err)
      toasts.error($_('toast.failedToSaveAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== key))
    }
  }

  async function handleEncryptedOpen(att: DecryptedAttachment) {
    const key = att.filename
    downloadingIds = new Set([...downloadingIds, key])
    try {
      await OpenEncryptedAttachment(messageId, att.filename)
    } catch (err) {
      console.error('Failed to open encrypted attachment:', err)
      toasts.error($_('toast.failedToOpenAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== key))
    }
  }

  async function handleSaveAll() {
    savingAll = true
    try {
      if (isEncrypted) {
        const folder = await SaveAllEncryptedAttachments(messageId)
        if (folder) {
          toasts.success($_('toast.attachmentsSaved', { values: { count: encryptedAttachments!.length } }), [
            {
              label: $_('attachment.openFolder'),
              onClick: () => OpenFolder(folder)
            }
          ])
        }
      } else {
        const folder = await SaveAllAttachments(messageId)
        if (folder) {
          toasts.success($_('toast.attachmentsSaved', { values: { count: attachments.length } }), [
            {
              label: $_('attachment.openFolder'),
              onClick: () => OpenFolder(folder)
            }
          ])
        }
      }
    } catch (err) {
      console.error('Failed to save all attachments:', err)
      toasts.error($_('toast.failedToSaveAttachments'))
    } finally {
      savingAll = false
    }
  }

  function getFileIcon(contentType: string): string {
    if (contentType.startsWith('image/')) return 'mdi:file-image'
    if (contentType.startsWith('video/')) return 'mdi:file-video'
    if (contentType.startsWith('audio/')) return 'mdi:file-music'
    if (contentType === 'application/pdf') return 'mdi:file-pdf-box'
    if (contentType.includes('word') || contentType === 'application/msword') return 'mdi:file-word'
    if (contentType.includes('excel') || contentType === 'application/vnd.ms-excel') return 'mdi:file-excel'
    if (contentType.includes('powerpoint') || contentType === 'application/vnd.ms-powerpoint') return 'mdi:file-powerpoint'
    if (contentType.includes('zip') || contentType.includes('compressed')) return 'mdi:folder-zip'
    if (contentType === 'text/plain') return 'mdi:file-document-outline'
    if (contentType === 'text/html') return 'mdi:language-html5'
    return 'mdi:file'
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }
</script>

{#if visible}
<div class={className}>
{@render header?.()}
{#if isEncrypted && encryptedAttachments && encryptedAttachments.length > 0}
  <div class="space-y-2">
    {#each encryptedAttachments as att (att.filename)}
      {@const isDownloading = downloadingIds.has(att.filename)}
      <div class="flex items-center gap-3 p-2 rounded-md border border-divider bg-muted/30 hover:bg-muted/50 transition-colors group">
        <div class="shrink-0 w-10 h-10 rounded-md bg-primary/10 flex items-center justify-center">
          <Icon icon={getFileIcon(att.contentType)} class="w-5 h-5 text-primary" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="font-medium text-sm text-foreground truncate" title={att.filename}>
            {att.filename}
          </div>
          <div class="text-xs text-muted-foreground">
            {formatSize(att.size)}
          </div>
        </div>
        <div class="flex items-center gap-1 shrink-0">
          {#if isDownloading}
            <div class="p-2">
              <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
            </div>
          {:else}
            <button
              class="p-2 rounded-md hover:bg-muted transition-colors opacity-0 group-hover:opacity-100 focus-visible:opacity-100 group-focus-within:opacity-100"
              title={$_('attachment.open')}
              onclick={() => handleEncryptedOpen(att)}
            >
              <Icon icon="mdi:open-in-new" class="w-4 h-4 text-muted-foreground" />
            </button>
            <button
              class="p-2 rounded-md hover:bg-muted transition-colors opacity-0 group-hover:opacity-100 focus-visible:opacity-100 group-focus-within:opacity-100"
              title={$_('attachment.download')}
              onclick={() => handleEncryptedDownload(att)}
            >
              <Icon icon="mdi:download" class="w-4 h-4 text-muted-foreground" />
            </button>
          {/if}
        </div>
      </div>
    {/each}

    {#if encryptedAttachments.length > 1}
      <button
        class="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors w-full justify-center border border-dashed border-border"
        onclick={handleSaveAll}
        disabled={savingAll}
      >
        {#if savingAll}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {$_('attachment.saving')}
        {:else}
          <Icon icon="mdi:download-multiple" class="w-4 h-4" />
          {$_('attachment.saveAll', { values: { count: encryptedAttachments.length } })}
        {/if}
      </button>
    {/if}
  </div>
{:else if attachments.length > 0}
  <div class="space-y-2">
    {#each attachments as att (att.id)}
      {@const isDownloading = downloadingIds.has(att.id)}
      <div class="flex items-center gap-3 p-2 rounded-md border border-divider bg-muted/30 hover:bg-muted/50 transition-colors group">
        <div class="shrink-0 w-10 h-10 rounded-md bg-primary/10 flex items-center justify-center">
          <Icon icon={getFileIcon(att.contentType)} class="w-5 h-5 text-primary" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="font-medium text-sm text-foreground truncate" title={att.filename}>
            {att.filename}
          </div>
          <div class="text-xs text-muted-foreground">
            {formatSize(att.size)}
          </div>
        </div>
        <div class="flex items-center gap-1 shrink-0">
          {#if isDownloading}
            <div class="p-2">
              <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
            </div>
          {:else}
            <button
              class="p-2 rounded-md hover:bg-muted transition-colors opacity-0 group-hover:opacity-100 focus-visible:opacity-100 group-focus-within:opacity-100"
              title={$_('attachment.open')}
              onclick={() => handleOpen(att)}
            >
              <Icon icon="mdi:open-in-new" class="w-4 h-4 text-muted-foreground" />
            </button>
            <button
              class="p-2 rounded-md hover:bg-muted transition-colors opacity-0 group-hover:opacity-100 focus-visible:opacity-100 group-focus-within:opacity-100"
              title={$_('attachment.download')}
              onclick={() => handleDownload(att)}
            >
              <Icon icon="mdi:download" class="w-4 h-4 text-muted-foreground" />
            </button>
          {/if}
        </div>
      </div>
    {/each}

    {#if attachments.length > 1}
      <button
        class="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors w-full justify-center border border-dashed border-border"
        onclick={handleSaveAll}
        disabled={savingAll}
      >
        {#if savingAll}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {$_('attachment.saving')}
        {:else}
          <Icon icon="mdi:download-multiple" class="w-4 h-4" />
          {$_('attachment.saveAll', { values: { count: attachments.length } })}
        {/if}
      </button>
    {/if}
  </div>
{/if}
</div>
{/if}
