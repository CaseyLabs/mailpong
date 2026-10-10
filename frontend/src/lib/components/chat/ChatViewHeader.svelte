<script lang="ts">
  // ChatViewHeader — the open chat's top bar. Left: Compose, Archive, Delete,
  // Spam, icon-only Reply, Reply All and Forward, then Expand (open in the
  // full composer). Right: Snooze, Pin, Unread and a "⋯" button that opens the
  // shared thread context menu. People and subject sit centered on a second
  // row. On narrow panes Spam, Forward, Pin and Unread move into the "⋯" menu
  // so the buttons keep to one row; the menu's Spam and Read/Unread call the
  // same handlers as the buttons.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import ComposeButton from '$lib/components/common/ComposeButton.svelte'
  import ToolbarButton from '$lib/components/common/ToolbarButton.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import { ContextMenuItem } from '$lib/components/ui/context-menu'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { canToggleSenderChat, chatSender, displayName, type Person, type ReplyMode } from './chatFormat'
  import ChatSnoozeMenu from './ChatSnoozeMenu.svelte'
  import { formatSnoozedUntil } from './chatSnooze'

  interface Props {
    subject: string
    people: Person[]
    // Set for a sender chat: the header names the sender, not a subject.
    senderEmail: string
    messageIds: string[]
    accountId: string
    folderId: string
    folderType: string
    isStarred: boolean
    allRead: boolean
    isPinned: boolean
    snoozedUntil: Date | null
    // null when the chat's priority is unknown (not in the loaded list).
    isLowPriority: boolean | null
    canTriage: boolean
    // False when the chat has no reply target yet; disables Reply buttons.
    canReply: boolean
    isTrash: boolean
    isSpam: boolean
    showBackButton: boolean
    onBack?: () => void
    onCompose?: () => void
    onArchive: () => void
    onReplyChat: (replyAll: boolean) => void
    onForward: () => void
    onExpand: () => void
    onDelete: () => void
    onSpam: () => void
    onPin: () => void
    onSnooze: (until: Date) => void
    onUnsnooze: () => void
    onToggleRead: () => void
    onToggleSenderLow: () => void
    onToggleSenderChat?: () => void
    onActionComplete: (autoSelectNext?: boolean) => void
    onReply?: (mode: ReplyMode, messageId: string) => void
  }

  let {
    subject, people: allPeople, senderEmail, messageIds, accountId, folderId, folderType, isStarred, allRead, isPinned, snoozedUntil, isLowPriority, canTriage, canReply, isTrash, isSpam,
    showBackButton, onBack, onCompose, onArchive, onReplyChat, onForward, onExpand, onDelete, onSpam, onPin, onSnooze, onUnsnooze, onToggleRead, onToggleSenderLow, onToggleSenderChat, onActionComplete, onReply,
  }: Props = $props()

  // A sender chat shows just its sender.
  const people = $derived(senderEmail ? [chatSender(allPeople, senderEmail)] : allPeople)

  let menuAnchor = $state<HTMLElement | null>(null)
  let snoozeMenu = $state<ChatSnoozeMenu | null>(null)
  const COMPACT_WIDTH = 860
  const NARROW_WIDTH = 600
  let width = $state(0)
  // Icon-only toolbar buttons when the pane is too narrow for labels.
  const compact = $derived(width > 0 && width < COMPACT_WIDTH)
  const narrow = $derived(width > 0 && width < NARROW_WIDTH)
  const pinInMenu = $derived(narrow && canTriage)
  // The shared menu offers Forward only for single-message threads.
  const forwardInMenu = $derived(narrow && messageIds.length > 1)
  const senderInMenu = $derived(canTriage && isLowPriority !== null && people.length > 0)

  const MAX_AVATARS = 3
  const title = $derived.by(() => {
    if (people.length === 0) return $_('viewer.unknown')
    const names = people.slice(0, 3).map(displayName).join(', ')
    return people.length > 3 ? `${names} ${$_('chat.andMore', { values: { count: people.length - 3 } })}` : names
  })
  const snoozeLabel = $derived(snoozedUntil ? formatSnoozedUntil(snoozedUntil) : '')

  export function openSnooze() { snoozeMenu?.open() }

  // Open the shared context menu (bits-ui ContextMenu) under the "⋯" button.
  export function openMenu() {
    if (!menuAnchor) return
    const r = menuAnchor.getBoundingClientRect()
    menuAnchor.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: r.right, clientY: r.bottom }))
  }
</script>

{#snippet snoozed()}
  {#if snoozeLabel}<Icon icon="mdi:alarm-snooze" class="inline w-3.5 h-3.5 -mt-0.5" /> {$_('chat.snoozedUntil', { values: { time: snoozeLabel } })} · {/if}
{/snippet}

{#snippet iconButton(icon: string, label: string, onclick: () => void, active?: boolean, disabled = false)}
  <!-- aria-pressed only for toggles (active passed); plain actions omit it. -->
  <button
    class="p-2 rounded-md hover:bg-muted transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden disabled:opacity-50 disabled:pointer-events-none"
    title={label}
    aria-label={label}
    aria-pressed={active}
    {disabled}
    {onclick}
  >
    <Icon {icon} class="w-5 h-5 {active ? 'text-primary' : 'text-muted-foreground'}" />
  </button>
{/snippet}

{#snippet extraItems()}
  {#if forwardInMenu}
    <ContextMenuItem onSelect={onForward}>
      <Icon icon="mdi:share" class="mr-2 h-4 w-4" />
      {$_('viewer.forward')}
    </ContextMenuItem>
  {/if}
  {#if pinInMenu}
    <ContextMenuItem onSelect={onPin}>
      <Icon icon={isPinned ? 'mdi:pin-off-outline' : 'mdi:pin-outline'} class="mr-2 h-4 w-4" />
      {isPinned ? $_('chat.unpin') : $_('chat.pin')}
    </ContextMenuItem>
  {/if}
  {#if senderInMenu}
    <ContextMenuItem onSelect={onToggleSenderLow}>
      <Icon icon={isLowPriority ? 'mdi:account-arrow-up-outline' : 'mdi:newspaper-variant-outline'} class="mr-2 h-4 w-4" />
      {isLowPriority ? $_('chat.moveSenderToPriority') : $_('chat.moveSenderToLow')}
    </ContextMenuItem>
    {#if onToggleSenderChat && canToggleSenderChat(senderEmail, folderType)}
      <ContextMenuItem onSelect={onToggleSenderChat}>
        <Icon icon={senderEmail ? 'mdi:call-split' : 'mdi:account-multiple-outline'} class="mr-2 h-4 w-4" />
        {senderEmail ? $_('chat.splitSender') : $_('chat.combineSender')}
      </ContextMenuItem>
    {/if}
  {/if}
{/snippet}

<header bind:clientWidth={width}>
  <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 px-3 py-2 border-b border-border">
    <div class="flex items-center gap-2">
      {#if showBackButton}
        {@render iconButton('mdi:arrow-left', $_('responsive.back'), () => onBack?.())}
        <div class="w-px h-5 bg-border"></div>
      {/if}
      {#if onCompose}<ComposeButton {compact} onclick={onCompose} />{/if}
      <ToolbarButton icon="mdi:archive-outline" label={$_('viewer.archive')} {compact} onclick={onArchive} />
      <ToolbarButton
        icon={isTrash ? 'mdi:delete-forever' : 'mdi:delete-outline'}
        label={$_('viewer.delete')}
        title={$_(isTrash ? 'viewer.deletePermanently' : 'viewer.delete')}
        {compact}
        onclick={onDelete}
      />
      {#if !narrow}
        <ToolbarButton
          icon={isSpam ? 'mdi:email-check-outline' : 'mdi:alert-octagon-outline'}
          label={$_(isSpam ? 'viewer.notSpam' : 'viewer.spam')}
          title={$_(isSpam ? 'viewer.markAsNotSpam' : 'viewer.markAsSpam')}
          {compact}
          onclick={onSpam}
        />
      {/if}
      <div class="w-px h-5 bg-border"></div>
      <div class="flex items-center gap-0.5">
        {@render iconButton('mdi:reply', $_('viewer.reply'), () => onReplyChat(false), undefined, !canReply)}
        {@render iconButton('mdi:reply-all', $_('viewer.replyAll'), () => onReplyChat(true), undefined, !canReply)}
        {#if !narrow}{@render iconButton('mdi:share', $_('viewer.forward'), onForward)}{/if}
      </div>
      <div class="w-px h-5 bg-border"></div>
      {@render iconButton('mdi:arrow-expand', $_('chat.expand'), onExpand, undefined, !canReply)}
    </div>

    <div class="ml-auto flex items-center justify-end gap-0.5">
      {#if canTriage}
        <ChatSnoozeMenu bind:this={snoozeMenu} {snoozedUntil} {onSnooze} {onUnsnooze} />
        {#if !narrow}{@render iconButton(isPinned ? 'mdi:pin' : 'mdi:pin-outline', isPinned ? $_('chat.unpin') : $_('chat.pin'), onPin, isPinned)}{/if}
      {/if}
      {#if !narrow}
        {@render iconButton(allRead ? 'mdi:email-outline' : 'mdi:email-open-outline', allRead ? $_('viewer.markAsUnread') : $_('viewer.markAsRead'), onToggleRead)}
      {/if}

      <MessageContextMenu
        {messageIds}
        {accountId}
        currentFolderId={folderId}
        {folderType}
        {isStarred}
        isRead={allRead}
        {onActionComplete}
        {onReply}
        {onSpam}
        {onToggleRead}
        extraItems={forwardInMenu || pinInMenu || senderInMenu ? extraItems : undefined}
      >
        <span bind:this={menuAnchor} class="inline-flex">
          {@render iconButton('mdi:dots-vertical', $_('chat.moreActions'), openMenu)}
        </span>
      </MessageContextMenu>
    </div>
  </div>

  <div class="flex items-center gap-3 min-w-0 px-4 py-3 border-b border-border">
    <div class="flex -space-x-1 shrink-0" aria-hidden="true">
      {#each people.slice(0, MAX_AVATARS) as p (p.email)}
        {@const photo = contactPhotos.get(p.email)}
        <div class="rounded-full ring-2 ring-background">
          <Avatar email={p.email} name={p.name} size={32} photoData={photo?.data} photoMediaType={photo?.mediaType} />
        </div>
      {/each}
    </div>

    <div class="min-w-0">
      {#if senderEmail || !subject}
        <h2 class="text-sm font-semibold text-foreground truncate" title={people.map((p) => p.email).join(', ')}>{title}</h2>
        <p class="text-sm text-muted-foreground truncate" title={senderEmail || subject}>
          {@render snoozed()}{senderEmail || $_('viewer.noSubject')}
        </p>
      {:else}
        <h2 class="text-base font-semibold text-foreground truncate" title={subject}>{subject}</h2>
        <p class="text-sm text-muted-foreground truncate" title={people.map((p) => p.email).join(', ')}>
          {@render snoozed()}{title}
        </p>
      {/if}
    </div>
  </div>
</header>
