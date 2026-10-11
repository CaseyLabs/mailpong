<script lang="ts">
  // ChatStream — the message column of a chat: a centred reading column of
  // day / subject headings, bubbles, and pending replies.
  import ChatBubble from './ChatBubble.svelte'
  import ChatDaySeparator from './ChatDaySeparator.svelte'
  import ChatPendingBubble from './ChatPendingBubble.svelte'
  import type { ChatThread } from './chatThread.svelte'
  import type { ChatComposer } from './chatComposerState.svelte'
  import type { ReplyMode, ThreadItem } from './chatFormat'

  interface Props {
    items: ThreadItem[]
    thread: ChatThread
    composer: ChatComposer
    accountId: string
    folderId: string
    folderType: string
    imagesLoaded: Set<string>
    onFocusChange: (id: string, focused: boolean) => void
    onReply?: (mode: ReplyMode, messageId: string, imagesLoaded?: boolean) => void
    onComposeToAddress?: (to: string) => void
    onEditDraft?: (draftId: string) => void
    onActionComplete: (autoSelectNext?: boolean) => void
    onOpenLink: (url: string) => void
  }

  let {
    items, thread, composer, accountId, folderId, folderType, imagesLoaded,
    onFocusChange, onReply, onComposeToAddress, onEditDraft, onActionComplete, onOpenLink,
  }: Props = $props()
</script>

<div class="mx-auto w-full max-w-chat-stream">
  {#each items as item (item.msg.id)}
    {#if item.newDay || item.subject}
      <div class="mx-auto w-full max-w-chat-column px-4"><ChatDaySeparator date={item.newDay ? item.date : undefined} subject={item.subject} /></div>
    {/if}
    <ChatBubble
      {item}
      {thread}
      {accountId}
      {folderId}
      {folderType}
      onFocusChange={(f) => onFocusChange(item.msg.id, f)}
      {onReply}
      {onComposeToAddress}
      {onEditDraft}
      onImagesLoaded={() => imagesLoaded.add(item.msg.id)}
      imagesLoaded={() => imagesLoaded.has(item.msg.id)}
      {onActionComplete}
      {onOpenLink}
    />
  {/each}
  {#each composer.pendingHere as p (p.id)}
    <ChatPendingBubble pending={p} onRetry={() => composer.retry(p.id)} onEdit={() => composer.edit(p.id)} />
  {/each}
</div>
