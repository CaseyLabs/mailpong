<script lang="ts">
  // ChatStream — the messages of a chat: day / subject headings, bubbles and
  // pending replies in a centred --chat-column, all on the left with mine
  // indented a step; email cards stay in a narrower centred --chat-stream column.
  import ChatBubble from './ChatBubble.svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import ChatCompactRow from './ChatCompactRow.svelte'
  import ChatDaySeparator from './ChatDaySeparator.svelte'
  import ChatPendingBubble from './ChatPendingBubble.svelte'
  import type { ChatThread } from './chatThread.svelte'
  import type { ChatComposer } from './chatComposerState.svelte'
  import { compactItems } from './chatCompact'
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
    // Changes when another chat opens; folded runs reset then.
    resetKey?: string | null
    // Runs a change that resizes content around `el` without moving `el`.
    holdPosition?: (el: HTMLElement, change: () => void) => void
    onFocusChange: (id: string, focused: boolean) => void
    onReply?: (mode: ReplyMode, messageId: string, imagesLoaded?: boolean) => void
    onComposeToAddress?: (to: string) => void
    onEditDraft?: (draftId: string) => void
    onActionComplete: (autoSelectNext?: boolean) => void
    onOpenLink: (url: string) => void
  }

  let {
    items, thread, composer, me, accountId, folderId, folderType, imagesLoaded, resetKey = null, holdPosition,
    onFocusChange, onReply, onComposeToAddress, onEditDraft, onActionComplete, onOpenLink,
  }: Props = $props()

  // Opened runs, by the ID of their first message.
  const expanded = new SvelteSet<string>()
  // Messages that were unread when the chat opened stay visible once read.
  const keep = new SvelteSet<string>()
  let lastKey: string | null | undefined
  $effect.pre(() => {
    if (resetKey !== lastKey) {
      lastKey = resetKey
      expanded.clear()
      keep.clear()
    }
    for (const it of items) if (!it.msg.isRead) keep.add(it.msg.id)
  })
  const entries = $derived(compactItems(items, expanded, keep))

  function toggle(id: string, el: HTMLElement) {
    const change = () => void (expanded.delete(id) || expanded.add(id))
    if (holdPosition) holdPosition(el, change)
    else change()
  }
</script>

<div class="mx-auto w-full max-w-(--chat-column)">
  {#each entries as entry (entry.kind === 'run' ? 'run-' + entry.id : entry.item.msg.id)}
    {#if entry.kind === 'run'}
      <ChatCompactRow count={entry.count} open={entry.open} onToggle={(el) => toggle(entry.id, el)} />
    {:else}
      {@const item = entry.item}
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
    {/if}
  {/each}
  {#each composer.pendingHere as p, i (p.id)}
    <ChatPendingBubble pending={p} groupStart={i === 0 && pendingGroupStart(items.at(-1), me, p.sentAt)} onRetry={() => composer.retry(p.id)} onEdit={() => composer.edit(p.id)} onUndo={() => void composer.undo(p.id)} />
  {/each}
</div>
