<script lang="ts">
  // ChatViewHeader — the open chat's top bar, a single row. Centre: the title,
  // every participant (the subject heads the stream instead), which opens the
  // participant list, with a "Snoozed until" chip when snoozed. Left: Back
  // (when the chat covers the list), then the actions, ending with a "⋯"
  // button that opens the shared thread context menu, which holds Snooze, Pin, Reply, Spam, Read/Unread and the sender actions
  // (Reply also sits on each bubble and in the reply box). Forward joins it for
  // multi-message chats, which the menu itself does not offer.
  // The actions: Compose (when set; not on narrow panes, where the title needs
  // the room and the list header offers it), the bordered Archive action, Delete
  // and "⋯".
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ComposeButton from '$lib/components/common/ComposeButton.svelte'
  import ToolbarButton from '$lib/components/common/ToolbarButton.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import { ContextMenuItem } from '$lib/components/ui/context-menu'
  import { canToggleSenderChat, chatSender, displayName, type Person, type ReplyMode } from './chatFormat'
  import ChatPeopleMenu from './ChatPeopleMenu.svelte'
  import ChatSnoozeMenu from './ChatSnoozeMenu.svelte'
  import { formatSnoozedUntil } from './chatSnooze'

  interface Props {
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
    isTrash: boolean
    showBackButton: boolean
    onBack?: () => void
    // Set only when the chat covers the list, which otherwise holds Compose.
    onCompose?: () => void
    // Starts a message to one participant, from the participant list.
    onComposeTo?: (to: string) => void
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
    people: allPeople, senderEmail, messageIds, accountId, folderId, folderType, isStarred, allRead, isPinned, snoozedUntil, isLowPriority, canTriage, isTrash,
    showBackButton, onBack, onCompose, onComposeTo, onArchive, onForward, onDelete, onSpam, onPin, onSnooze, onUnsnooze, onToggleRead, onToggleSenderLow, onToggleSenderChat, onActionComplete, onReply,
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
  const snoozeLabel = $derived(snoozedUntil ? formatSnoozedUntil(snoozedUntil) : '')

  export function openSnooze() { snoozeMenu?.open() }

  // Open the shared context menu (bits-ui ContextMenu) under the "⋯" button.
  export function openMenu() {
    if (!menuAnchor) return
    const r = menuAnchor.getBoundingClientRect()
    menuAnchor.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: r.right, clientY: r.bottom }))
  }
</script>

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
  <!-- Centred like the stream and reply box, so wide panes line up. The equal
       outer columns keep the title centred; the actions sit to its left. -->
  <div class="mx-auto w-full max-w-(--chat-column) grid grid-cols-[1fr_minmax(0,auto)_1fr] items-center gap-3 px-3 py-2">
    <div class="flex items-center gap-2">
      {#if showBackButton}
        {@render iconButton('mdi:arrow-left', $_('responsive.back'), () => onBack?.())}
      {/if}
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
        <!-- Anchors the snooze menu opened from "⋯" or the H key; its state shows beside the title. -->
        <ChatSnoozeMenu bind:this={snoozeMenu} hiddenTrigger {snoozedUntil} {onSnooze} {onUnsnooze} />
      {/if}
    </div>

    <div class="min-w-0 flex items-center justify-center gap-1">
      <h2 class="min-w-0 flex"><ChatPeopleMenu {people} {title} {onComposeTo} /></h2>
      {#if snoozeLabel}
        <span class="shrink-0 inline-flex items-center gap-1 text-xs text-fg-secondary">
          <Icon icon="mdi:alarm-snooze" class="w-3.5 h-3.5" />{$_('chat.snoozedUntil', { values: { time: snoozeLabel } })}
        </span>
      {/if}
    </div>

    <div aria-hidden="true"></div>
  </div>
</header>
