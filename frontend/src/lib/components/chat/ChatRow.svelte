<script lang="ts">
  // ChatRow — one chat in the chat list: people first (avatar + names), then
  // subject, then the latest line unless Settings hides previews. Right-click
  // opens the shared message context menu for the whole thread, with the
  // triage actions (Archive, Pin, Snooze, sender moves) on top.
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { formatListDate } from '$lib/utils/date'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import { ContextMenuItem } from '$lib/components/ui/context-menu'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { getLayoutMode } from '$lib/stores/layout.svelte'
  import { getMessageListDensity, getShowMessagePreview, type MessageListDensity } from '$lib/stores/settings.svelte'
  import type { ChatItem, ChatPerson } from '$lib/stores/chat.svelte'
  import ChatSnoozeMenu from './ChatSnoozeMenu.svelte'
  import { canToggleSenderChat, displayName, displaySubject, rowPeople } from './chatFormat'
  import type { ChatRowActions } from './chatTriage'
  import { formatSnoozedUntil } from './chatSnooze'
  import { isSendingChat } from './chatComposerState.svelte'
  import { rowStatus } from './chatStatus'

  interface Props {
    chat: ChatItem
    id: string
    selected: boolean
    folderType: string
    showAccount: boolean
    // Rendered inside the Pinned group, which already says so.
    inPinned?: boolean
    myEmails: Set<string>
    onSelect: () => void
    onActionComplete?: (autoSelectNext?: boolean) => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
    actions: ChatRowActions
    onFocusList: () => void
  }

  let { chat, id, selected, folderType, showAccount, inPinned = false, myEmails, onSelect, onActionComplete, onReply, actions, onFocusList }: Props = $props()

  let contextMenuRef: MessageContextMenu | null = null
  let snoozeMenu = $state<ChatSnoozeMenu | null>(null)
  // The snooze menu mounts on request (context menu or H key) and unmounts
  // once it and its date dialog have closed.
  let snoozeMounted = $state(false)

  export async function openSnooze() {
    snoozeMounted = true
    await tick()
    snoozeMenu?.open()
  }

  // Unmounting removes the trigger the menu hands focus back to, so return
  // focus to the list instead of leaving it on the page body.
  async function handleSnoozeClose() {
    snoozeMounted = false
    await tick()
    if (document.activeElement === document.body) onFocusList()
  }

  export function isFolderPickerOpen(): boolean {
    return contextMenuRef?.isFolderPickerOpen() ?? false
  }

  export function toggleFolderPicker(mode: 'move' | 'copy') {
    contextMenuRef?.toggleFolderPicker(mode)
  }

  const people = $derived(rowPeople(chat, myEmails, folderType === 'sent'))

  const title = $derived.by(() => {
    if (people.length === 0) return $_('viewer.unknown')
    const names = people.slice(0, 2).map(displayName).join(', ')
    const extra = people.length - 2
    return extra > 0 ? `${names} ${$_('chat.andMore', { values: { count: extra } })}` : names
  })

  // Per density: avatar px, vertical padding class, and the size class and
  // line height (px) of the lines under the 22 px name line. Without previews,
  // rows come out at 32 (micro), 52 (compact), 72 (standard) and 80 (large) px.
  interface DensityStyle { avatar: number; pad: string; line: string; lineHeight: number }
  const DENSITY: Record<MessageListDensity, DensityStyle> = {
    micro: { avatar: 24, pad: 'py-1', line: 'text-body', lineHeight: 22 },
    compact: { avatar: 32, pad: 'py-1.5', line: 'text-meta', lineHeight: 18 },
    standard: { avatar: 44, pad: 'py-3.5', line: 'text-compact', lineHeight: 20 },
    large: { avatar: 48, pad: 'py-4', line: 'text-body', lineHeight: 22 },
  }
  const NAME_LINE_HEIGHT = 22
  const LINE_COUNT = { single: 0, double: 1, triple: 2 }

  const hasUnread = $derived(chat.unreadCount > 0)
  const density = $derived(getMessageListDensity())
  const ds = $derived(DENSITY[density])
  // Search results always show the matching text.
  const showPreview = $derived(getShowMessagePreview() || !!chat.highlightedSnippet)
  // single: name, subject and time on one line (micro, no search match).
  // double: name line, then subject (with the preview inline at micro and
  // compact, where search matches land).
  // triple: name, subject, preview on their own lines.
  const layout = $derived<'single' | 'double' | 'triple'>(
    density === 'micro' && !chat.highlightedSnippet ? 'single'
      : showPreview && (density === 'standard' || density === 'large') ? 'triple' : 'double'
  )
  // The content box: the taller of the avatar and the text lines.
  const intrinsicHeight = $derived(Math.max(ds.avatar, NAME_LINE_HEIGHT + LINE_COUNT[layout] * ds.lineHeight))
  const time = $derived(formatListDate(chat.latestDate))
  const snoozeTime = $derived(chat.snoozedUntil ? formatSnoozedUntil(chat.snoozedUntil) : '')

  // The highest-priority state, shown as one glyph at the row's right edge.
  const status = $derived(rowStatus(chat, isSendingChat(chat.accountId, chat.threadKey)))

  const subject = $derived(displaySubject(chat.subject) || $_('viewer.noSubject'))
  // Screen-reader summary of the row's visual signals.
  const ariaLabel = $derived(
    [
      title,
      subject,
      // The row shows only a dot for unread; screen readers still hear the count.
      status === 'sending' ? $_('chat.sending') : '',
      hasUnread ? $_('chat.unreadCount', { values: { count: chat.unreadCount } }) : '',
      chat.isPinned ? $_('chat.pinnedLabel') : '',
      snoozeTime ? $_('chat.snoozedUntil', { values: { time: snoozeTime } }) : '',
      chat.hasAttachments ? $_('chat.hasAttachments') : '',
      chat.isEncrypted ? $_('chat.encrypted') : '',
      showAccount ? chat.accountName : '',
      time,
    ].filter(Boolean).join(', ')
  )

  function handleDragStart(e: DragEvent) {
    if (!e.dataTransfer || chat.messageIds.length === 0) return
    const payload = JSON.stringify({ messageIds: chat.messageIds, sourceAccountId: chat.accountId })
    e.dataTransfer.setData('application/x-mailven-messages', payload)
    e.dataTransfer.effectAllowed = 'move'
  }
</script>

{#snippet extraItems()}
  <ContextMenuItem onSelect={actions.onArchive}><Icon icon="mdi:archive-outline" class="mr-2 h-4 w-4" />{$_('viewer.archive')}</ContextMenuItem>
  <ContextMenuItem onSelect={actions.onPin}>
    <Icon icon={chat.isPinned ? 'mdi:pin-off-outline' : 'mdi:pin-outline'} class="mr-2 h-4 w-4" />{chat.isPinned ? $_('chat.unpin') : $_('chat.pin')}
  </ContextMenuItem>
  <!-- Opens the row's snooze menu once the context menu has closed. -->
  <ContextMenuItem onSelect={() => setTimeout(() => void openSnooze())}><Icon icon="mdi:alarm-snooze" class="mr-2 h-4 w-4" />{$_('chat.snoozeMenu')}</ContextMenuItem>
  {#if chat.snoozedUntil}
    <ContextMenuItem onSelect={actions.onUnsnooze}><Icon icon="mdi:alarm-off" class="mr-2 h-4 w-4" />{$_('chat.unsnooze')}</ContextMenuItem>
  {/if}
  <ContextMenuItem onSelect={actions.onToggleSenderLow}>
    <Icon icon={chat.isLowPriority ? 'mdi:account-arrow-up-outline' : 'mdi:newspaper-variant-outline'} class="mr-2 h-4 w-4" />
    {chat.isLowPriority ? $_('chat.moveSenderToPriority') : $_('chat.moveSenderToLow')}
  </ContextMenuItem>
  {#if canToggleSenderChat(chat.senderEmail, folderType)}
    <ContextMenuItem onSelect={actions.onToggleSenderChat}>
      <Icon icon={chat.senderEmail ? 'mdi:call-split' : 'mdi:account-multiple-outline'} class="mr-2 h-4 w-4" />
      {chat.senderEmail ? $_('chat.splitSender') : $_('chat.combineSender')}
    </ContextMenuItem>
  {/if}
{/snippet}

{#snippet statusIcons()}
  <span class="flex items-center gap-1 shrink-0 text-muted-foreground" aria-hidden="true">
    {#if chat.isEncrypted}<Icon icon="mdi:lock" class="w-3.5 h-3.5" />{/if}
    {#if chat.hasAttachments}<Icon icon="mdi:paperclip" class="w-3.5 h-3.5" />{/if}
    {#if chat.isPinned && !inPinned}<Icon icon="mdi:pin" class="w-3.5 h-3.5" />{/if}
  </span>
{/snippet}

{#snippet subjectLine()}
  <span class="flex-1 min-w-0 truncate {ds.line} text-fg-secondary {hasUnread ? 'font-medium' : ''}">
    <!-- Chat search highlights the subject with its prefixes already dropped. -->
    <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
    {#if chat.highlightedSubject}{@html chat.highlightedSubject}{:else}{subject}{/if}
    {#if layout === 'double' && showPreview}<span class="text-fg-tertiary"> — {@render previewText('')}</span>{/if}
  </span>
{/snippet}

{#snippet previewText(cls: string)}
  <span class="{cls} {ds.line} text-fg-tertiary">
    <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
    {#if chat.highlightedSnippet}{@html chat.highlightedSnippet}{:else}{chat.snippet}{/if}
  </span>
{/snippet}

{#snippet avatar(p: ChatPerson | undefined, size: number)}
  {@const photo = p ? contactPhotos.get(p.email) : undefined}
  <Avatar email={p?.email || chat.threadId} name={p?.name} {size} photoData={photo?.data} photoMediaType={photo?.mediaType} />
{/snippet}

<MessageContextMenu
  bind:this={contextMenuRef}
  messageIds={chat.messageIds}
  accountId={chat.accountId}
  currentFolderId={chat.folderId}
  {folderType}
  isStarred={chat.isStarred}
  isRead={!hasUnread}
  {onActionComplete}
  {onReply}
  {extraItems}
>
  <div
    {id}
    role="option"
    aria-selected={selected}
    aria-label={ariaLabel}
    tabindex="-1"
    data-chat-row
    draggable={getLayoutMode() !== 'narrow'}
    data-density={density}
    style:contain-intrinsic-size="auto {intrinsicHeight}px"
    class="chat-row relative flex items-center {density === 'micro' ? 'gap-2' : 'gap-3'} px-3 {ds.pad} mx-1.5 my-1.5 rounded-lg cursor-pointer outline-hidden transition-colors {selected ? 'bg-selected text-foreground group-data-[focused]/list:ring-2 group-data-[focused]/list:ring-inset group-data-[focused]/list:ring-ring' : 'hover:bg-muted/50'}"
    onclick={onSelect}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect() } }}
    ondragstart={handleDragStart}
  >
    <!-- Avatar: one person, or two stacked for a group -->
    <div class="relative shrink-0" style:width="{ds.avatar}px" style:height="{ds.avatar}px" aria-hidden="true">
      {#if people.length > 1}
        <div class="absolute top-0 left-0">{@render avatar(people[0], Math.round(ds.avatar * 0.6))}</div>
        <div class="absolute bottom-0 right-0 rounded-full ring-2 ring-background">{@render avatar(people[1], Math.round(ds.avatar * 0.6))}</div>
      {:else}
        {@render avatar(people[0], ds.avatar)}
      {/if}
      {#if showAccount && chat.accountColor}
        <span
          class="absolute -bottom-0.5 -left-0.5 rounded-full ring-2 ring-background {ds.avatar < 36 ? 'w-2 h-2' : 'w-3 h-3'}"
          style="background-color: {chat.accountColor}"
          title={chat.accountName}
        ></span>
      {/if}
    </div>

    <div class="flex-1 min-w-0">
      <!-- People + time (+ subject on one line when single) -->
      <div class="flex items-baseline gap-2">
        <span class="min-w-0 truncate text-body text-fg-primary {layout === 'single' ? 'max-w-[45%] shrink-0' : 'flex-1'} {hasUnread ? 'font-semibold' : 'font-medium'}">{title}</span>
        {#if layout === 'single'}
          {@render subjectLine()}
          {@render statusIcons()}
        {/if}
        <span class="shrink-0 text-meta tabular-nums text-fg-tertiary">{time}</span>
      </div>
      {#if layout !== 'single'}
        <!-- Subject (one line), with the status icons when the latest line is hidden -->
        <div class="flex items-center gap-1.5">
          {@render subjectLine()}
          {#if layout !== 'triple'}{@render statusIcons()}{/if}
        </div>
      {/if}
      {#if layout === 'triple'}
        <!-- Latest line + status icons -->
        <div class="flex items-center gap-1.5">
          {@render previewText('flex-1 min-w-0 truncate')}
          {@render statusIcons()}
        </div>
      {/if}
    </div>

    <!-- Status glyph: one slot at the right edge, kept even when empty so rows align. -->
    <span class="flex items-center justify-center w-4 shrink-0" data-row-status={status ?? undefined} aria-hidden="true">
      {#if status === 'sending'}
        <span title={$_('chat.sending')}><Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-fg-tertiary" /></span>
      {:else if status === 'snoozed'}
        <span title={$_('chat.snoozedUntil', { values: { time: snoozeTime } })}><Icon icon="mdi:alarm-snooze" class="w-4 h-4 text-fg-tertiary" /></span>
      {:else if status === 'unread'}
        <span class="w-2 h-2 rounded-full bg-primary"></span>
      {/if}
    </span>

    {#if snoozeMounted}
      <!-- Hidden anchor so the snooze menu opens at the row's top-right. -->
      <div class="absolute right-2 top-1.5" aria-hidden="true">
        <ChatSnoozeMenu
          bind:this={snoozeMenu}
          hiddenTrigger
          onClose={handleSnoozeClose}
          snoozedUntil={chat.snoozedUntil}
          onSnooze={actions.onSnooze}
          onUnsnooze={actions.onUnsnooze}
        />
      </div>
    {/if}
  </div>
</MessageContextMenu>

<style>
  /* Skip layout and paint for off-screen rows in long lists. The intrinsic
     size (the content box; padding is added) is set inline per density. */
  .chat-row {
    content-visibility: auto;
  }
</style>
