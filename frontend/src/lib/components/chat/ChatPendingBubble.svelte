<script lang="ts">
  // ChatPendingBubble — a reply sent from the docked composer, shown until
  // the synced thread holds it. Queued sends count down with Undo; failed
  // sends offer Retry and Edit.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import type { PendingSend } from './chatComposerState.svelte'
  import { MINE_BUBBLE_GUTTER, type Person } from './chatFormat'

  interface Props {
    pending: PendingSend
    // The sender, for the avatar on the left.
    me: Person
    // First bubble of a run of mine: show the avatar and a gap above.
    groupStart: boolean
    onRetry: () => void
    onEdit: () => void
    onUndo: () => void
  }

  let { pending, me, groupStart, onRetry, onEdit, onUndo }: Props = $props()

  const photo = $derived(contactPhotos.get(me.email))

  let now = $state(Date.now())
  $effect(() => {
    if (pending.status !== 'queued') return
    const t = setInterval(() => (now = Date.now()), 250)
    return () => clearInterval(t)
  })
  // Seconds left before a queued send goes out; 0 once it is sending.
  const secondsLeft = $derived(pending.status === 'queued' ? Math.max(0, Math.ceil((pending.sentAt - now) / 1000)) : 0)
</script>

<!-- Laid out like ChatBubble's own messages: indented, avatar then bubble -->
<div class="flex gap-2 w-full {MINE_BUBBLE_GUTTER} {groupStart ? 'mt-3' : 'mt-0.5'}" role="status">
  <div class="w-8 shrink-0" aria-hidden="true">
    {#if groupStart}
      <Avatar email={me.email} name={me.name} size={32} photoData={photo?.data} photoMediaType={photo?.mediaType} />
    {/if}
  </div>
  <div class="flex flex-col items-start gap-1 min-w-0 max-w-[min(75%,var(--chat-bubble))]">
    <div
      class="px-3.5 py-2 text-[1rem] leading-[1.45] whitespace-pre-wrap wrap-anywhere rounded-2xl rounded-bl-md bg-bubble-own text-foreground {pending.status === 'queued' ? 'opacity-70' : ''} {pending.status === 'failed' ? 'ring-2 ring-destructive' : ''}"
    >
      {pending.text}
      {#each pending.attachmentNames as name (name)}
        <div class="flex items-center gap-1 mt-1 text-xs opacity-90"><Icon icon="mdi:paperclip" class="w-3.5 h-3.5" />{name}</div>
      {/each}
    </div>
    <div class="flex items-center gap-2 px-1 text-xs text-muted-foreground">
      {#if pending.status === 'queued' && secondsLeft > 0 && pending.sendId}
        <Icon icon="mdi:timer-sand" class="w-3.5 h-3.5" />{$_('chat.sendingIn', { values: { seconds: secondsLeft } })}
        <button class="text-primary hover:underline" onclick={onUndo}>{$_('chat.undo')}</button>
      {:else if pending.status === 'queued'}
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
