<script lang="ts">
  // ChatStream — the messages of a chat: day / subject headings, bubbles and
  // pending replies in a centred --chat-column, all on the left with mine
  // indented a step; email cards stay in a narrower centred --chat-stream column.
  import ChatBubble from './ChatBubble.svelte'
  import ChatDaySeparator from './ChatDaySeparator.svelte'
  import ChatPendingBubble from './ChatPendingBubble.svelte'
  import type { ChatThread } from './chatThread.svelte'
  import type { ChatComposer } from './chatComposerState.svelte'
  import { pendingGroupStart, type Person, type ReplyMode, type ThreadItem } from './chatFormat'

  interface Props {
    items: ThreadItem[]
    thread: ChatThread
    composer: ChatComposer
    me: Person
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
    items, thread, composer, me, accountId, folderId, folderType, imagesLoaded,
    onFocusChange, onReply, onComposeToAddress, onEditDraft, onActionComplete, onOpenLink,
  }: Props = $props()
</script>

<div class="mx-auto w-full max-w-(--chat-column)">
  {#each items as item (item.msg.id)}
    {#if item.newDay || item.subject}
      <div class="w-full px-6"><ChatDaySeparator date={item.newDay ? item.date : undefined} subject={item.subject} /></div>
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
  {#each composer.pendingHere as p, i (p.id)}
    <ChatPendingBubble pending={p} groupStart={i === 0 && pendingGroupStart(items.at(-1), me, p.sentAt)} onRetry={() => composer.retry(p.id)} onEdit={() => composer.edit(p.id)} onUndo={() => void composer.undo(p.id)} />
  {/each}
</div>
