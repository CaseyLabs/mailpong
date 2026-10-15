<script lang="ts">
  // ChatViewHeader — the open chat's top bar, a single row. Left: Back (when
  // the chat covers the list) and the title block: every participant, then a
  // subtitle (the subject, or subject and message counts for a sender chat with several subjects).
  // Beside the title: a "⋯" button that opens the shared thread context menu,
  // which holds Snooze, Pin, Reply, Spam, Read/Unread and the sender actions
  // (Reply also sits on each bubble and in the reply box). Forward joins it for
  // multi-message chats, which the menu itself does not offer.
  // Right: Compose (when set; not on narrow panes, where the title needs the
  // room and the list header offers it), the bordered Archive action and Delete.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ComposeButton from '$lib/components/common/ComposeButton.svelte'
  import ToolbarButton from '$lib/components/common/ToolbarButton.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import { ContextMenuItem } from '$lib/components/ui/context-menu'
  import { canToggleSenderChat, chatSender, displayName, type Person, type ReplyMode } from './chatFormat'
  import ChatSnoozeMenu from './ChatSnoozeMenu.svelte'
  import { formatSnoozedUntil } from './chatSnooze'

  interface Props {
    subject: string
    people: Person[]
    // Distinct subjects in a sender chat (1 when it has none).
    subjectCount: number
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
    isTrash: boolean
    showBackButton: boolean
    onBack?: () => void
    // Set only when the chat covers the list, which otherwise holds Compose.
    onCompose?: () => void
    onArchive: () => void
    onForward: () => void
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
    subject, people: allPeople, subjectCount, senderEmail, messageIds, accountId, folderId, folderType, isStarred, allRead, isPinned, snoozedUntil, isLowPriority, canTriage, isTrash,
    showBackButton, onBack, onCompose, onArchive, onForward, onDelete, onSpam, onPin, onSnooze, onUnsnooze, onToggleRead, onToggleSenderLow, onToggleSenderChat, onActionComplete, onReply,
  }: Props = $props()

  // A sender chat shows just its sender.
  const people = $derived(senderEmail ? [chatSender(allPeople, senderEmail)] : allPeople)

  let menuAnchor = $state<HTMLElement | null>(null)
  let snoozeMenu = $state<ChatSnoozeMenu | null>(null)
  const MAX_NAMES = 6
  const COMPACT_WIDTH = 860
  const NARROW_WIDTH = 600
  let width = $state(0)
  // Icon-only toolbar buttons when the pane is too narrow for labels.
  const compact = $derived(width > 0 && width < COMPACT_WIDTH)
  const narrow = $derived(width > 0 && width < NARROW_WIDTH)
  // The shared menu offers Forward only for single-message threads.
  const forwardInMenu = $derived(messageIds.length > 1)
  const senderInMenu = $derived(canTriage && isLowPriority !== null && people.length > 0)

  const title = $derived.by(() => {
    if (people.length === 0) return $_('viewer.unknown')
    const names = people.slice(0, MAX_NAMES).map(displayName).join(', ')
    return people.length > MAX_NAMES ? `${names} ${$_('chat.andMore', { values: { count: people.length - MAX_NAMES } })}` : names
  })
  const subtitle = $derived(
    senderEmail && subjectCount > 1
      ? `${$_('chat.subjectCount', { values: { count: subjectCount } })} · ${$_('chat.messageCount', { values: { count: messageIds.length } })}`
      : subject || $_('viewer.noSubject'),
  )
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
    <Icon {icon} class="w-5 h-5 {active ? 'text-foreground' : 'text-muted-foreground'}" />
  </button>
{/snippet}

{#snippet extraItems()}
  {#if forwardInMenu}
    <ContextMenuItem onSelect={onForward}>
      <Icon icon="mdi:share" class="mr-2 h-4 w-4" />
      {$_('viewer.forward')}
    </ContextMenuItem>
  {/if}
  {#if canTriage}
    <!-- Deferred so the context menu closes before the snooze dropdown opens. -->
    <ContextMenuItem onSelect={() => setTimeout(() => snoozeMenu?.open())}>
      <Icon icon="mdi:alarm-snooze" class="mr-2 h-4 w-4" />
      {$_('chat.snoozeMenu')}
    </ContextMenuItem>
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

<header bind:clientWidth={width} class="flex min-h-header border-b border-divider">
  <!-- Centred like the stream and reply box, so wide panes line up. -->
  <div class="mx-auto w-full max-w-(--chat-column) flex items-center gap-3 px-3 py-2">
    {#if showBackButton}
      {@render iconButton('mdi:arrow-left', $_('responsive.back'), () => onBack?.())}
    {/if}

    <div class="min-w-0 flex-1 flex items-center gap-1">
      <div class="min-w-0 px-1">
        <h2 class="text-body font-semibold text-fg-primary truncate" title={people.map((p) => p.email).join(', ')}>{title}</h2>
        <p class="text-body text-fg-secondary truncate" title={senderEmail ? undefined : subject}>
          {@render snoozed()}{subtitle}
        </p>
      </div>
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
        extraItems={forwardInMenu || canTriage ? extraItems : undefined}
      >
        <span bind:this={menuAnchor} class="inline-flex shrink-0">
          {@render iconButton('mdi:dots-horizontal', $_('chat.moreActions'), openMenu)}
        </span>
      </MessageContextMenu>
      {#if canTriage}
        <!-- Anchors the snooze menu opened from "⋯" or the H key; its state shows in the subtitle. -->
        <ChatSnoozeMenu bind:this={snoozeMenu} hiddenTrigger {snoozedUntil} {onSnooze} {onUnsnooze} />
      {/if}
    </div>

    <div class="flex items-center gap-2 shrink-0">
      {#if onCompose && !narrow}<ComposeButton {compact} onclick={onCompose} />{/if}
      <ToolbarButton icon="mdi:archive-outline" label={$_('viewer.archive')} {compact} onclick={onArchive} />
      <ToolbarButton
        ghost
        icon={isTrash ? 'mdi:delete-forever' : 'mdi:delete-outline'}
        label={$_('viewer.delete')}
        title={$_(isTrash ? 'viewer.deletePermanently' : 'viewer.delete')}
        {compact}
        onclick={onDelete}
      />
    </div>
  </div>
</header>
