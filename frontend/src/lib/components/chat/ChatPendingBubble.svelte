<script lang="ts">
  // ChatPendingBubble — a reply sent from the docked composer, shown until
  // the synced thread holds it. Failed sends offer Retry and Edit.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import type { PendingSend } from './chatComposer.svelte'

  interface Props {
    pending: PendingSend
    onRetry: () => void
    onEdit: () => void
  }

  let { pending, onRetry, onEdit }: Props = $props()
</script>

<div class="flex flex-row-reverse px-4 mt-3" role="status">
  <div class="flex flex-col items-end gap-1 max-w-[min(75%,36rem)]">
    <div
      class="px-3.5 py-2 text-sm whitespace-pre-wrap wrap-anywhere rounded-2xl rounded-br-md bg-bubble-own text-foreground {pending.status === 'sending' ? 'opacity-70' : ''} {pending.status === 'failed' ? 'ring-2 ring-destructive' : ''}"
    >
      {pending.text}
      {#each pending.attachmentNames as name (name)}
        <div class="flex items-center gap-1 mt-1 text-xs opacity-90"><Icon icon="mdi:paperclip" class="w-3.5 h-3.5" />{name}</div>
      {/each}
    </div>
    <div class="flex items-center gap-2 px-1 text-xs text-muted-foreground">
      {#if pending.status === 'sending'}
        <Icon icon="mdi:loading" class="w-3.5 h-3.5 animate-spin" />{$_('chat.sending')}
      {:else if pending.status === 'sent'}
        <Icon icon="mdi:check" class="w-3.5 h-3.5" />{$_('chat.sent')}
      {:else}
        <span class="text-destructive" title={pending.error}>{$_('chat.sendFailed')}</span>
        <button class="text-primary hover:underline" onclick={onRetry}>{$_('chat.retry')}</button>
        <button class="text-primary hover:underline" onclick={onEdit}>{$_('chat.editReply')}</button>
      {/if}
    </div>
  </div>
</div>
