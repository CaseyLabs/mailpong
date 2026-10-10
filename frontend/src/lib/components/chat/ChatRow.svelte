<script lang="ts">
  // ChatRow — one chat in the chat list: people first (avatar + names), then
  // subject, then the latest line unless Settings hides previews. Hovered or
  // selected rows show triage actions (Archive, Pin, Snooze, Unread);
  // right-click opens the shared message context menu for the whole thread,
  // with the triage actions on top.
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { formatListDate } from '$lib/utils/date'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import { ContextMenuItem } from '$lib/components/ui/context-menu'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { getLayoutMode } from '$lib/stores/layout.svelte'
  import { getShowMessagePreview } from '$lib/stores/settings.svelte'
  import type { ChatItem, ChatPerson } from '$lib/stores/chat.svelte'
  import ChatSnoozeMenu from './ChatSnoozeMenu.svelte'
  import { canToggleSenderChat, displayName, rowPeople } from './chatFormat'
  import type { ChatRowActions } from './chatTriage'
  import { formatSnoozedUntil } from './chatSnooze'

  interface Props {
    chat: ChatItem
    id: string
    selected: boolean
    folderType: string
    showAccount: boolean
    myEmails: Set<string>
    onSelect: () => void
    onActionComplete?: (autoSelectNext?: boolean) => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
    actions: ChatRowActions
  }

  let { chat, id, selected, folderType, showAccount, myEmails, onSelect, onActionComplete, onReply, actions }: Props = $props()

  let contextMenuRef: MessageContextMenu | null = null
  let snoozeMenu = $state<ChatSnoozeMenu | null>(null)
  let hovered = $state(false)
  let snoozeActive = $state(false)
  // Stay rendered while the snooze menu or date dialog is open.
  const showActions = $derived(hovered || selected || snoozeActive)
  // Secondary text and avatar rings on the solid selection of a focused list.
  const selMuted = $derived(selected ? 'group-data-[focused]/list:text-selection-foreground/80' : '')
  const selRing = $derived(selected ? 'group-data-[focused]/list:ring-selection' : '')

  // The actions (and so the snooze menu) render on hover or selection;
  // count as hovered so a context-menu or keyboard request can open it.
  export async function openSnooze() {
    hovered = true
    await tick()
    snoozeMenu?.open()
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

  const hasUnread = $derived(chat.unreadCount > 0)
  // Search results always show the matching text.
  const showPreview = $derived(getShowMessagePreview() || !!chat.highlightedSnippet)
  const time = $derived(formatListDate(chat.latestDate))
  const snoozeTime = $derived(chat.snoozedUntil ? formatSnoozedUntil(chat.snoozedUntil) : '')

  const subject = $derived(chat.subject || $_('viewer.noSubject'))
  // Screen-reader summary of the row's visual signals.
  const ariaLabel = $derived(
    [
      title,
      subject,
      hasUnread ? $_('chat.unreadCount', { values: { count: chat.unreadCount } }) : '',
      chat.isPinned ? $_('chat.pinnedLabel') : '',
      snoozeTime ? $_('chat.snoozedUntil', { values: { time: snoozeTime } }) : '',
      chat.lastFromMe ? $_('chat.youRepliedLast') : '',
      chat.hasAttachments ? $_('chat.hasAttachments') : '',
      chat.isEncrypted ? $_('chat.encrypted') : '',
      showAccount ? chat.accountName : '',
      time,
    ].filter(Boolean).join(', ')
  )

  function handleDragStart(e: DragEvent) {
    if (!e.dataTransfer || chat.messageIds.length === 0) return
    const payload = JSON.stringify({ messageIds: chat.messageIds, sourceAccountId: chat.accountId })
    e.dataTransfer.setData('application/x-aerion-messages', payload)
    e.dataTransfer.effectAllowed = 'move'
  }
</script>

{#snippet action(icon: string, label: string, onclick: () => void)}
  <button class="p-1 rounded hover:bg-muted" tabindex="-1" title={label} aria-label={label} {onclick}>
    <Icon {icon} class="w-4 h-4 text-muted-foreground" />
  </button>
{/snippet}

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

{#snippet repliedIcon()}
  <span class="text-muted-foreground {selMuted}" title={$_('chat.youRepliedLast')}><Icon icon="mdi:reply" class="inline w-3.5 h-3.5 -mt-0.5" /></span>
{/snippet}

{#snippet statusIcons()}
  <span class="flex items-center gap-1 shrink-0 text-muted-foreground {selMuted}" aria-hidden="true">
    {#if chat.isEncrypted}<Icon icon="mdi:lock" class="w-3.5 h-3.5" />{/if}
    {#if chat.hasAttachments}<Icon icon="mdi:paperclip" class="w-3.5 h-3.5" />{/if}
    {#if chat.snoozedUntil}<span title={$_('chat.snoozedUntil', { values: { time: snoozeTime } })}><Icon icon="mdi:alarm-snooze" class="w-3.5 h-3.5" /></span>{/if}
    {#if chat.isPinned}<Icon icon="mdi:pin" class="w-3.5 h-3.5" />{/if}
    {#if hasUnread}
      <span class="min-w-5 h-5 px-1.5 rounded-full bg-primary text-primary-foreground text-[11px] font-semibold leading-5 text-center {selected ? 'group-data-[focused]/list:bg-selection-foreground group-data-[focused]/list:text-selection' : ''}">{chat.unreadCount}</span>
    {/if}
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
    class="chat-row relative flex items-center gap-3 px-3 py-2.5 mx-1.5 my-0.5 rounded-lg cursor-pointer outline-hidden transition-colors {selected ? 'bg-muted group-data-[focused]/list:bg-selection group-data-[focused]/list:text-selection-foreground' : 'hover:bg-muted/50'} {showPreview ? 'chat-row-tall' : 'chat-row-short'}"
    onclick={onSelect}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect() } }}
    ondragstart={handleDragStart}
    onpointerenter={() => (hovered = true)}
    onpointerleave={() => (hovered = false)}
  >
    <!-- Avatar: one person, or two stacked for a group -->
    <div class="relative shrink-0 w-11 h-11" aria-hidden="true">
      {#if people.length > 1}
        <div class="absolute top-0 left-0">{@render avatar(people[0], 26)}</div>
        <div class="absolute bottom-0 right-0 rounded-full ring-2 ring-background {selRing}">{@render avatar(people[1], 26)}</div>
      {:else}
        {@render avatar(people[0], 44)}
      {/if}
      {#if showAccount && chat.accountColor}
        <span
          class="absolute -bottom-0.5 -left-0.5 w-3 h-3 rounded-full ring-2 ring-background {selRing}"
          style="background-color: {chat.accountColor}"
          title={chat.accountName}
        ></span>
      {/if}
    </div>

    <div class="flex-1 min-w-0">
      <!-- People + time -->
      <div class="flex items-baseline gap-2">
        <span class="flex-1 min-w-0 truncate text-sm {hasUnread ? 'font-semibold text-foreground' : 'font-medium text-foreground/90'} {selected ? 'group-data-[focused]/list:text-selection-foreground' : ''}">{title}</span>
        <span class="shrink-0 text-xs {hasUnread ? 'text-primary font-medium' : 'text-muted-foreground'} {selMuted}">{time}</span>
      </div>
      <!-- Subject (one line), with the status icons when the latest line is hidden -->
      <div class="flex items-center gap-1.5">
        <span class="flex-1 min-w-0 truncate text-sm leading-5 {hasUnread ? 'text-foreground' : 'text-foreground/80'} {selMuted}">
          {#if !showPreview && chat.lastFromMe}{@render repliedIcon()}{/if}
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
          {#if chat.highlightedSubject}{@html chat.highlightedSubject}{:else}{subject}{/if}
        </span>
        {#if !showPreview}{@render statusIcons()}{/if}
      </div>
      {#if showPreview}
        <!-- Latest line + status icons -->
        <div class="flex items-center gap-1.5">
          <span class="flex-1 min-w-0 truncate text-sm {hasUnread ? 'text-foreground/90' : 'text-muted-foreground'} {selMuted}">
            {#if chat.lastFromMe}{@render repliedIcon()}{/if}
            <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
            {#if chat.highlightedSnippet}{@html chat.highlightedSnippet}{:else}{chat.snippet}{/if}
          </span>
          {@render statusIcons()}
        </div>
      {/if}
    </div>

    {#if showActions}
      <!-- Hover actions; keyboard users have the E/P/H/Shift+U shortcuts. -->
      <div
        class="absolute right-2 top-1.5 flex items-center gap-0.5 rounded-md border border-border bg-background/95 shadow-sm"
        role="toolbar"
        tabindex="-1"
        aria-label={$_('chat.chatActions')}
        onclick={(e) => e.stopPropagation()}
        onkeydown={(e) => e.stopPropagation()}
      >
        {@render action('mdi:archive-outline', $_('viewer.archive'), actions.onArchive)}
        {@render action(chat.isPinned ? 'mdi:pin-off-outline' : 'mdi:pin-outline', chat.isPinned ? $_('chat.unpin') : $_('chat.pin'), actions.onPin)}
        <ChatSnoozeMenu
          bind:this={snoozeMenu}
          onActiveChange={(a) => (snoozeActive = a)}
          snoozedUntil={chat.snoozedUntil}
          onSnooze={actions.onSnooze}
          onUnsnooze={actions.onUnsnooze}
          triggerClass="p-1 rounded hover:bg-muted"
          iconClass="w-4 h-4"
        />
        {#if !hasUnread}
          {@render action('mdi:email-mark-as-unread', $_('chat.markUnread'), actions.onMarkUnread)}
        {/if}
      </div>
    {/if}
  </div>
</MessageContextMenu>

<style>
  /* Skip layout and paint for off-screen rows in long lists. */
  .chat-row {
    content-visibility: auto;
  }
  .chat-row-tall {
    contain-intrinsic-size: auto 80px;
  }
  .chat-row-short {
    contain-intrinsic-size: auto 60px;
  }
</style>
