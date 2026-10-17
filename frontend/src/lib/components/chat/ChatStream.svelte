<script lang="ts">
  // ChatStream — the messages of a chat: day / subject headings, bubbles and
  // pending replies in a centred --chat-column, all on the left with mine
  // indented a step; email cards stay in a narrower centred --chat-stream column.
  // Each subject run is a section whose heading sticks while it scrolls past.
  import ChatBubble from './ChatBubble.svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import ChatCompactRow from './ChatCompactRow.svelte'
  import ChatDaySeparator from './ChatDaySeparator.svelte'
  import ChatPendingBubble from './ChatPendingBubble.svelte'
  import ChatSubjectHeading from './ChatSubjectHeading.svelte'
  import type { ChatThread } from './chatThread.svelte'
  import type { ChatComposer } from './chatComposerState.svelte'
  import { compactItems, subjectSections } from './chatCompact'
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
  const sections = $derived(subjectSections(compactItems(items, expanded, keep)))

  function toggle(id: string, el: HTMLElement) {
    const change = () => void (expanded.delete(id) || expanded.add(id))
    if (holdPosition) holdPosition(el, change)
    else change()
  }
</script>

{#snippet pending()}
  {#each composer.pendingHere as p, i (p.id)}
    <ChatPendingBubble pending={p} name={me.name || me.email} groupStart={i === 0 && pendingGroupStart(items.at(-1), me, p.sentAt)} onRetry={() => composer.retry(p.id)} onEdit={() => composer.edit(p.id)} onUndo={() => void composer.undo(p.id)} />
  {/each}
{/snippet}

<div class="mx-auto w-full max-w-(--chat-column)">
  {#each sections as section, si (section.id ?? 'lead')}
    <section>
      {#if section.subject !== undefined}<ChatSubjectHeading subject={section.subject} />{/if}
      {#each section.entries as entry (entry.kind === 'run' ? 'run-' + entry.id : entry.item.msg.id)}
        {#if entry.kind === 'run'}
          <ChatCompactRow count={entry.count} open={entry.open} onToggle={(el) => toggle(entry.id, el)} />
        {:else}
          {@const item = entry.item}
          {#if item.newDay}
            <div class="w-full px-6"><ChatDaySeparator date={item.date} tight={item.subject !== undefined} /></div>
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
      <!-- Pending replies join the last section, under its heading. -->
      {#if si === sections.length - 1}
        {@render pending()}
      {/if}
    </section>
  {:else}
    {@render pending()}
  {/each}
</div>
