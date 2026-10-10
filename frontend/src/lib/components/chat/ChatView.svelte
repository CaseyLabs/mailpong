<script lang="ts">
  // ChatView — the open chat: header with triage actions, then the thread as
  // bubbles with day separators. Data and side effects live in ChatThread.
  // Keeps ConversationViewer's exported ref API for App's keyboard wiring.
  import { onMount, tick, untrack } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { GetIdentities, OpenURL } from '../../../../wailsjs/go/app/App'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  import ComposeButton from '$lib/components/common/ComposeButton.svelte'
  import { toasts } from '$lib/stores/toast'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { chatList, senderEmailOf } from '$lib/stores/chat.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import ChatViewHeader from './ChatViewHeader.svelte'
  import ChatBubble from './ChatBubble.svelte'
  import ChatDaySeparator from './ChatDaySeparator.svelte'
  import ChatComposer from './ChatComposer.svelte'
  import ChatPendingBubble from './ChatPendingBubble.svelte'
  import { ChatThread } from './chatThread.svelte'
  import { ChatComposer as ComposerState } from './chatComposer.svelte'
  import { buildThreadItems, myAddresses, replyRecipients, replyTarget, threadPeople, type ReplyMode } from './chatFormat'
  import { ChatScroll } from './chatScroll.svelte'
  import { ChatActions } from './chatActions.svelte'

  interface Props {
    threadId?: string | null
    folderId?: string | null
    folderType?: string | null
    accountId?: string | null
    onReply?: (mode: ReplyMode, messageId: string, imagesLoaded?: boolean) => void
    onCompose?: () => void
    onComposeToAddress?: (toAddress: string) => void
    onEditDraft?: (draftId: string) => void
    // Opens a released chat draft in the full composer.
    onExpandDraft?: (draftId: string) => void
    onActionComplete?: (autoSelectNext?: boolean) => void
    onToggleSenderChat?: () => void
    isFocused?: boolean
    isFlashing?: boolean
    showBackButton?: boolean
    onBack?: () => void
    // Escape in the docked composer: return to the chat list.
    onEscape?: () => void
    // Message focus mode (App keyboard shortcut) narrows to one message.
    focusedMessageIdInFocus?: string | null
  }

  let {
    threadId = null, folderId = null, folderType = null, accountId = null,
    onReply, onCompose, onComposeToAddress, onEditDraft, onExpandDraft, onActionComplete, onToggleSenderChat,
    isFocused = false, isFlashing = false, showBackButton = false, onBack, onEscape, focusedMessageIdInFocus = null,
  }: Props = $props()

  const FOCUS_WAIT_MS = 3000

  let header = $state<ChatViewHeader | null>(null)
  let composerRef = $state<ChatComposer | null>(null)
  let showDeleteConfirm = $state(false)
  // Set by focusComposer() until the composer can take focus (after a load).
  // Expires so a request that couldn't apply doesn't steal focus later.
  let pendingFocus = $state<{ replyAll?: boolean; until: number } | null>(null)
  const imagesLoaded = new Set<string>()

  const scroll = new ChatScroll(() => messageIds)
  const thread = new ChatThread({
    onGone: (next) => onActionComplete?.(next),
    onLoaded: () => void scroll.toStart(thread.messages.find((m) => !m.isRead)?.id ?? null),
    scroller: () => scroll.scroller,
  })

  const myEmails = $derived(myAddresses(accountStore.accounts))
  const target = $derived(replyTarget(thread.messages, myEmails))
  // The account's identity addresses, which PrepareReply leaves out of a
  // reply; null until loaded.
  let identityEmails = $state<Set<string> | null>(null)
  const recipients = $derived(target ? replyRecipients(thread.messages, target.messageId, identityEmails ?? myEmails) : null)
  const composer = new ComposerState({ target: () => target, recipients: () => recipients })
  // The docked composer shows its own draft; hide draft copies while it has one.
  const shown = $derived(composer.draftId ? thread.messages.filter((m) => !m.isDraft) : thread.messages)
  const items = $derived(
    buildThreadItems(focusedMessageIdInFocus ? shown.filter((m) => m.id === focusedMessageIdInFocus) : shown, !!senderEmailOf(threadId)),
  )
  const people = $derived(threadPeople(thread.messages, myEmails))
  const messageIds: string[] = $derived(thread.messages.map((m) => m.id))
  const effectiveFolderType = $derived(folderType || 'inbox')
  const allRead = $derived(thread.messages.every((m) => m.isRead))
  const isStarred = $derived(thread.messages.length > 0 && thread.messages.every((m) => m.isStarred))
  const isTrash = $derived(folderType === 'trash')
  const isSpam = $derived(folderType === 'spam')
  const chatItem = $derived(chatList.items.find((c) => c.accountId === accountId && c.threadId === threadId) ?? null)
  const threadKey = $derived(chatItem?.threadKey ?? (threadId ?? '').replace(/[<>]/g, ''))
  const actions = new ChatActions({
    thread, scroll, onActionComplete: (next) => onActionComplete?.(next),
    accountId: () => accountId, threadKey: () => threadKey, messageIds: () => messageIds,
    chatItem: () => chatItem, people: () => people, allRead: () => allRead, isTrash: () => isTrash, isSpam: () => isSpam,
  })

  onMount(() => {
    const stop = thread.start()
    window.addEventListener('keydown', handleKeyDown)
    return () => {
      stop()
      composer.dispose()
      window.removeEventListener('keydown', handleKeyDown)
    }
  })

  $effect(() => {
    const key = threadId ? threadKey : null
    const account = accountId
    untrack(() => void composer.open(account, key))
  })

  $effect(() => {
    const account = accountId
    identityEmails = null
    if (!account) return
    let stale = false
    GetIdentities(account)
      .then((ids) => {
        if (!stale) identityEmails = new Set(ids.map((id) => id.email.trim().toLowerCase()))
      })
      .catch((err) => {
        if (!stale) console.error('Failed to load identities:', err)
      })
    return () => {
      stale = true
    }
  })

  const canReply = $derived(!!target && !!accountId && !!threadKey)

  $effect(() => {
    if (!pendingFocus || !canReply || !composerRef || thread.loading) return
    const { replyAll, until } = pendingFocus
    if (Date.now() > until) {
      pendingFocus = null
      return
    }
    untrack(() => {
      pendingFocus = null
      if (replyAll !== undefined && replyAll !== composer.replyAll) composer.toggleReplyAll()
      void tick().then(() => composerRef?.focus())
    })
  })

  // Drop sent bubbles once the synced thread has my reply.
  $effect(() => {
    let latestMineAt = 0
    for (const m of thread.messages) {
      if (!m.isDraft && m.mine) latestMineAt = Math.max(latestMineAt, new Date(m.date).getTime())
    }
    untrack(() => composer.prune(latestMineAt))
  })

  // Keep a new pending bubble in view.
  $effect(() => {
    if (composer.pendingHere.length === 0) return
    void scroll.toEnd()
  })

  // Bodies (iframes) grow after render; re-apply the load anchor as they do.
  $effect(() => scroll.observe())

  $effect(() => {
    imagesLoaded.clear()
    scroll.reset()
    thread.open(threadId, folderId, accountId)
  })

  $effect(() => {
    void contactPhotos.ensure(people.map((p) => p.email).concat(thread.messages.map((m) => m.fromEmail)))
  })

  $effect(() => {
    if (thread.messages.length > 0) thread.autoSendReadReceipts()
  })

  function handleKeyDown(e: KeyboardEvent) {
    if (!isFocused) return
    const el = e.target as HTMLElement | null
    if (el?.closest('textarea, input, [contenteditable="true"]')) return
    scroll.handleTab(e)
    // Delete/Backspace on a focused message is App's shortcut (it calls trash()).
  }

  function openLink(url: string) {
    if (url.startsWith('mailto:') && onComposeToAddress) {
      let addr = url.slice(7).split('?')[0]
      try {
        addr = decodeURIComponent(addr)
      } catch {
        // malformed escape: use it as written
      }
      onComposeToAddress(addr)
      return
    }
    OpenURL(url).catch((err: unknown) => console.error('Failed to open URL:', err))
  }

  async function handleMenuActionComplete(next?: boolean) {
    await thread.load({ markRead: false })
    if (next || thread.messages.length === 0) onActionComplete?.(true)
  }

  // Expand hands the text to the full composer: as a draft when there is
  // text, else as a fresh reply.
  async function expand() {
    const t = target
    if (!t) return
    const mode: ReplyMode = composer.replyAll ? 'reply-all' : 'reply'
    try {
      const draftId = await composer.release()
      if (draftId) onExpandDraft?.(draftId)
      else onReply?.(mode, t.messageId, imagesLoaded.has(t.messageId))
    } catch (err) {
      console.error('Expand failed:', err)
      toasts.error($_('chat.expandFailed'))
    }
  }

  // --- Ref API used by App.svelte keyboard shortcuts ---
  // focusComposer focuses the reply box once the chat has loaded; replyAll
  // switches the reply mode first when given.
  export function focusComposer(replyAll?: boolean) { pendingFocus = { replyAll, until: Date.now() + FOCUS_WAIT_MS } }
  export function openSnooze() { header?.openSnooze() }
  export function hasFocusedMessage(): boolean { return scroll.focusedId !== null }
  export function getFocusedMessageId(): string | null { return scroll.focusedId }
  export function getLastMessageId(): string | null { return messageIds.at(-1) ?? null }
  export function refreshFlags() { return thread.refreshFlags() }
  export function scrollUp() { scroll.by(-1) }
  export function scrollDown() { scroll.by(1) }
  export function isImagesLoaded(id: string): boolean { return imagesLoaded.has(id) }
  export function loadImages() { window.dispatchEvent(new CustomEvent('load-remote-images')) }
  export function openAlwaysLoadDropdown() { window.dispatchEvent(new CustomEvent('open-always-load-dropdown')) }
  export function markRead() { void actions.toggleRead() }

  export const archive = () => actions.archive()
  export function reply() { replyAs('reply') }
  export function replyAll() { replyAs('reply-all') }
  export function forward() { replyAs('forward') }
  function replyAs(mode: ReplyMode) {
    const id = scroll.focusedId ?? getLastMessageId()
    if (id) onReply?.(mode, id, imagesLoaded.has(id))
  }

  export function trash() {
    if (scroll.focusedId) {
      void actions.deleteMessages([scroll.focusedId], false)
      return
    }
    deleteChat()
  }

  // The header's Delete always acts on the whole chat, whatever bubble has
  // focus; in Trash it asks before deleting permanently.
  function deleteChat() {
    if (isTrash) showDeleteConfirm = true
    else void actions.deleteMessages(messageIds, true)
  }

  export function deletePermanently() {
    if (scroll.focusedId) {
      void actions.deleteMessages([scroll.focusedId], false)
      return
    }
    showDeleteConfirm = true
  }

  export const spam = () => actions.spam()

  // Select-all inside the focused (or last) message's rendered body, if any.
  export function selectAllText() {
    const id = scroll.focusedId ?? getLastMessageId()
    const iframe = id ? scroll.scroller?.querySelector<HTMLIFrameElement>(`[data-message-id="${id}"] iframe`) : null
    iframe?.contentWindow?.postMessage({ type: 'select-all' }, '*')
  }

  // Context menu for the focused message, else the thread menu.
  export function openContextMenu() {
    const el = scroll.focusedId ? scroll.messageEl(scroll.focusedId) : null
    if (!el) {
      header?.openMenu()
      return
    }
    const r = el.getBoundingClientRect()
    el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: r.left + r.width / 2, clientY: r.top + 16 }))
  }
</script>

<div class="flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  {#if !threadId || (!thread.conversation && !thread.loading && !thread.error)}
    {#if onCompose}
      <div class="flex items-center px-4 py-3 border-b border-border"><ComposeButton onclick={onCompose} /></div>
    {/if}
    <div class="flex flex-col items-center justify-center flex-1 text-muted-foreground">
      <Icon icon="mdi:chat-outline" class="w-16 h-16 mb-4" />
      <p class="text-lg">{$_('chat.selectChat')}</p>
    </div>
  {:else if thread.loading && !thread.conversation}
    <div class="flex items-center justify-center h-full">
      <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-muted-foreground" />
    </div>
  {:else if thread.error}
    <div class="flex flex-col items-center justify-center h-full text-center px-4">
      <Icon icon="mdi:alert-circle-outline" class="w-12 h-12 text-destructive mb-3" />
      <p class="text-destructive mb-2">{thread.error}</p>
      <button class="mt-2 text-sm text-primary hover:underline" onclick={() => thread.reload()}>{$_('viewer.tryAgain')}</button>
    </div>
  {:else if thread.conversation}
    <ChatViewHeader
      bind:this={header}
      subject={thread.conversation.subject}
      senderEmail={senderEmailOf(threadId)}
      {people}
      {messageIds}
      accountId={accountId ?? ''}
      folderId={folderId ?? ''}
      folderType={effectiveFolderType}
      {isStarred}
      {allRead}
      isPinned={!!chatItem?.isPinned}
      snoozedUntil={chatItem?.snoozedUntil ?? null}
      isLowPriority={chatItem ? chatItem.isLowPriority : null}
      canTriage={!!accountId && !!threadKey}
      {canReply}
      {isTrash}
      {isSpam}
      {showBackButton}
      {onBack}
      {onCompose}
      onArchive={() => void actions.archive()}
      onReplyChat={focusComposer}
      onForward={forward}
      onExpand={() => void expand()}
      onDelete={deleteChat}
      onSpam={() => void actions.spam()}
      onPin={() => actions.togglePin()}
      onSnooze={(d) => void actions.snooze(d)}
      onUnsnooze={() => actions.unsnooze()}
      onToggleRead={() => void actions.toggleRead()}
      onToggleSenderLow={() => actions.toggleSenderLow()}
      {onToggleSenderChat}
      onActionComplete={handleMenuActionComplete}
      {onReply}
    />

    <div
      bind:this={scroll.scroller}
      class="flex-1 min-h-0 overflow-y-auto scrollbar-thin"
      role="log"
      aria-label={$_('chat.threadLabel')}
      onfocusin={() => setFocusedPane('viewer')}
      onwheel={scroll.release}
      onpointerdown={scroll.release}
    >
      <div bind:this={scroll.content} class="pb-6">
        {#each items as item (item.msg.id)}
          {#if item.newDay}<div class="px-4"><ChatDaySeparator date={item.date} /></div>{/if}
          <ChatBubble
            {item}
            {thread}
            accountId={accountId ?? ''}
            folderId={folderId ?? ''}
            folderType={effectiveFolderType}
            onFocusChange={(f) => { if (f) scroll.focusedId = item.msg.id; else if (scroll.focusedId === item.msg.id) scroll.focusedId = null }}
            {onReply}
            {onComposeToAddress}
            {onEditDraft}
            onImagesLoaded={() => imagesLoaded.add(item.msg.id)}
            imagesLoaded={() => imagesLoaded.has(item.msg.id)}
            onActionComplete={handleMenuActionComplete}
            onOpenLink={openLink}
          />
        {/each}
        {#each composer.pendingHere as p (p.id)}
          <ChatPendingBubble pending={p} onRetry={() => composer.retry(p.id)} onEdit={() => composer.edit(p.id)} />
        {/each}
      </div>
    </div>

    <ChatComposer bind:this={composerRef} {composer} disabled={!canReply} {onEscape} />
  {/if}
</div>

<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('viewer.deleteConversationTitle')}
  description={$_('viewer.deleteConversationDescription')}
  confirmLabel={$_('viewer.deletePermanently')}
  variant="destructive"
  onConfirm={() => { showDeleteConfirm = false; void actions.deleteMessages(messageIds, true) }}
  onCancel={() => (showDeleteConfirm = false)}
/>
