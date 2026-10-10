<script lang="ts">
  // ChatBubble — one message in the chat thread. New text renders as a
  // bubble (quoted history behind a "⋯" marker after the text); rich mail
  // renders as a card; encrypted mail, mail without chat text, and "Show
  // original" render the full sandboxed body. Right-click keeps the shared message context menu.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { GetMessageSource } from '../../../../wailsjs/go/app/App'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import EmailBody from '$lib/components/viewer/EmailBody.svelte'
  import AttachmentList from '$lib/components/viewer/AttachmentList.svelte'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { getDarkMailContent } from '$lib/stores/settings.svelte'
  import { getIsDarkActive } from '$lib/stores/theme.svelte'
  import { toasts } from '$lib/stores/toast'
  import ChatRichCard from './ChatRichCard.svelte'
  import ChatSecurityBanners from './ChatSecurityBanners.svelte'
  import { securityStatus, TONES } from './chatSecurity'
  import { linkify, parseRecipients, displayName, type ReplyMode, type ThreadItem } from './chatFormat'
  import { formatTime } from '$lib/utils/date'
  import type { ChatThread } from './chatThread.svelte'


  interface Props {
    item: ThreadItem
    thread: ChatThread
    accountId: string
    folderId: string
    folderType: string
    onFocusChange: (focused: boolean) => void
    onReply?: (mode: ReplyMode, messageId: string, imagesLoaded?: boolean) => void
    onComposeToAddress?: (to: string) => void
    onEditDraft?: (draftId: string) => void
    onImagesLoaded: () => void
    imagesLoaded: () => boolean
    onActionComplete: (autoSelectNext?: boolean) => void
    onOpenLink: (url: string) => void
  }

  let {
    item, thread, accountId, folderId, folderType, onFocusChange,
    onReply, onComposeToAddress, onEditDraft, onImagesLoaded, imagesLoaded, onActionComplete, onOpenLink,
  }: Props = $props()

  const msg = $derived(item.msg)
  const smime = $derived(thread.smimeResults[msg.id])
  const pgp = $derived(thread.pgpResults[msg.id])
  const encrypted = $derived(!!(msg.hasSMIME || msg.hasPGP))
  const notFetched = $derived(msg.bodyFetched === false && !msg.bodyHtml && !msg.bodyText)
  const decrypting = $derived((msg.hasSMIME && !smime && thread.smimeLoading.has(msg.id)) || (msg.hasPGP && !pgp && thread.pgpLoading.has(msg.id)))

  let showOriginal = $state(false)
  let showQuoted = $state(false)
  let cardExpanded = $state(false)
  let darkOverride = $state(false)
  let source = $state<string | null>(null)
  let sourceOpen = $state(false)

  type Mode = 'loading' | 'bubble' | 'card' | 'full'
  const mode = $derived.by<Mode>(() => {
    if (notFetched || decrypting) return 'loading'
    if (encrypted || showOriginal || !msg.chat) return 'full'
    return msg.chat.isRich ? 'card' : 'bubble'
  })
  // Attachment-only mail: no empty colored bubble above the attachment list.
  const bare = $derived(mode === 'bubble' && msg.hasAttachments && !msg.chat?.hasQuoted && !msg.chat?.text.trim())

  const bodyHtml = $derived(pgp?.bodyHtml ?? smime?.bodyHtml ?? msg.bodyHtml)
  const bodyText = $derived(pgp?.bodyText ?? smime?.bodyText ?? msg.bodyText)
  const decryptedAttachments = $derived(pgp?.attachments ?? smime?.attachments)
  const hasAttachments = $derived(msg.hasAttachments || (decryptedAttachments?.length ?? 0) > 0)
  // Bubbles keep actions beside them; wide cards and bodies overlay the corner.
  const actionsPos = $derived(mode === 'bubble' ? `top-0 ${item.mine ? 'right-full mr-1' : 'left-full ml-1'}` : 'top-1.5 right-1.5 rounded-md bg-background/90 shadow-sm')
  const canDarken = $derived(getDarkMailContent() && getIsDarkActive())
  const darken = $derived(canDarken && !darkOverride)

  const senderName = $derived(msg.fromName || msg.fromEmail || $_('viewer.unknown'))
  const photo = $derived(contactPhotos.get(msg.fromEmail))
  const time = $derived(formatTime(item.date))
  const security = $derived(securityStatus(msg, thread, $_))
  const badges = $derived(security.filter((l) => l.badge))
  const recipientsTitle = $derived(
    [
      `${$_('viewer.from')} ${msg.fromName ? `${msg.fromName} <${msg.fromEmail}>` : msg.fromEmail}`,
      ...[['viewer.to', msg.toList], ['viewer.cc', msg.ccList], ['viewer.bcc', msg.bccList]]
        .map(([label, raw]) => [label, parseRecipients(raw)] as const)
        .filter(([, list]) => list.length > 0)
        .map(([label, list]) => `${$_(label!)} ${list.map((p) => p.email ? `${displayName(p)} <${p.email}>` : displayName(p)).join(', ')}`),
      item.date.toLocaleString(),
    ].join('\n'),
  )

  async function copy(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text)
      toasts.success($_('viewer.copiedToClipboard', { values: { label } }))
    } catch {
      toasts.error($_('viewer.failedToCopy'))
    }
  }

  async function toggleSource() {
    sourceOpen = !sourceOpen
    if (!sourceOpen || source !== null) return
    try {
      source = await GetMessageSource(msg.id)
    } catch {
      toasts.error($_('viewer.failedToLoadSource'))
      sourceOpen = false
    }
  }

  function reply(mode: ReplyMode) {
    onReply?.(mode, msg.id, imagesLoaded())
  }
</script>

{#snippet text(value: string)}
  {#each linkify(value) as seg, i (i)}
    {#if seg.url}
      <a
        href={seg.url}
        class="underline underline-offset-2 break-all {item.mine ? 'text-primary-foreground' : 'text-primary'}"
        onclick={(e) => { e.preventDefault(); onOpenLink(seg.url!) }}
      >{seg.text}</a>
    {:else}{seg.text}{/if}
  {/each}
{/snippet}

{#snippet emailBody()}
  <div class="p-3 bg-background text-foreground">
    <EmailBody
      messageId={msg.id}
      accountId={msg.accountId}
      {bodyHtml}
      {bodyText}
      fromEmail={msg.fromEmail}
      onCompose={onComposeToAddress}
      {onImagesLoaded}
      encryptedInlineAttachments={pgp?.inlineAttachments ?? smime?.inlineAttachments}
      {darken}
    />
  </div>
{/snippet}

<MessageContextMenu
  messageIds={[msg.id]}
  {accountId}
  currentFolderId={folderId}
  {folderType}
  isStarred={msg.isStarred}
  isRead={msg.isRead}
  {onActionComplete}
  {onReply}
>
  <div
    class="group flex gap-2 px-4 {item.groupStart ? (mode === 'full' && !item.newDay ? 'mt-3 pt-3 border-t border-border' : 'mt-3') : 'mt-0.5'} {item.mine ? 'flex-row-reverse' : ''}"
    data-message-id={msg.id}
    tabindex="-1"
    role="article"
    aria-label={`${senderName}, ${time}`}
    onfocus={() => onFocusChange(true)}
    onblur={() => onFocusChange(false)}
  >
    <!-- Avatar column (others only); spacer keeps grouped bubbles aligned -->
    {#if !item.mine}
      <div class="w-8 shrink-0" aria-hidden="true">
        {#if item.groupStart}
          <Avatar email={msg.fromEmail} name={msg.fromName} size={32} photoData={photo?.data} photoMediaType={photo?.mediaType} />
        {/if}
      </div>
    {/if}

    <div class="flex flex-col min-w-0 {mode === 'bubble' ? 'max-w-[75%]' : 'flex-1 max-w-[min(100%,52rem)]'} {item.mine ? 'items-end' : 'items-start'}">
      {#if item.subject}
        <div class="flex items-center gap-1 max-w-full px-1 mb-0.5 text-xs font-medium text-foreground/70" title={item.subject}>
          <Icon icon="mdi:email-outline" class="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
          <span class="truncate">{item.subject}</span>
        </div>
      {/if}
      {#if item.groupStart || badges.length > 0}
        <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 px-1 mb-0.5 text-xs text-muted-foreground" title={item.groupStart ? recipientsTitle : undefined}>
          {#if item.groupStart}
            {#if !item.mine}<span class="font-medium text-foreground/80">{senderName}</span>{/if}
            {#if msg.isDraft}<span class="text-destructive font-medium">{$_('chat.draft')}</span>{/if}
            <span>{time}</span>
          {/if}
          {#each badges as b (b.icon + b.text)}
            <span class="inline-flex items-center gap-1 px-1.5 py-px rounded-full border text-[11px] leading-4 {TONES[b.tone]}" title={b.text}>
              <Icon icon={b.icon} class="w-3 h-3 shrink-0" aria-hidden="true" />
              <span class="max-w-48 truncate">{b.text}</span>
            </span>
          {/each}
        </div>
      {/if}

      <div class="flex flex-col gap-1.5 w-full {item.mine ? 'items-end' : 'items-start'}">
        <ChatSecurityBanners {msg} {thread} lines={security} />

        <div
          class="relative w-full rounded-2xl group-focus-visible:ring-2 group-focus-visible:ring-primary/40 {mode === 'bubble' && !bare ? (item.mine ? 'w-auto bg-primary text-primary-foreground rounded-br-md' : 'w-auto bg-muted text-foreground rounded-bl-md') : ''}"
        >
          {#if notFetched && thread.bodyErrors.has(msg.id)}
            <div class="flex items-center gap-2 px-3.5 py-2 text-sm text-muted-foreground bg-muted rounded-2xl">
              <Icon icon="mdi:alert-circle-outline" class="w-4 h-4 text-destructive" />
              {$_('viewer.failedToLoadContent')}
              <button class="text-primary hover:underline" onclick={() => thread.retryBody(msg.id)}>{$_('viewer.tryAgain')}</button>
            </div>
          {:else if mode === 'loading'}
            <div class="flex items-center gap-2 px-3.5 py-2 text-sm italic text-muted-foreground bg-muted rounded-2xl">
              <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
              {notFetched ? $_('viewer.downloadingContent') : $_('viewer.decryptingMessage')}
            </div>
          {:else if bare}
            <!-- Attachment-only mail: the attachment list below is the message -->
          {:else if mode === 'bubble'}
            <div class="px-3.5 py-2 text-sm whitespace-pre-wrap wrap-anywhere">
              {@render text(msg.chat!.text)}
              {#if msg.chat!.hasQuoted}
                <!-- Quiet marker at the end of the text, not a pill below it -->
                <button
                  class="inline-flex align-middle ml-1.5 -my-1 px-1 rounded opacity-50 group-hover:opacity-70 hover:opacity-100! focus-visible:opacity-100! transition-opacity {item.mine ? 'hover:bg-primary-foreground/20' : 'hover:bg-foreground/10'}"
                  title={showQuoted ? $_('chat.hideQuoted') : $_('chat.showQuoted')}
                  aria-label={showQuoted ? $_('chat.hideQuoted') : $_('chat.showQuoted')}
                  aria-expanded={showQuoted}
                  onclick={() => (showQuoted = !showQuoted)}
                ><Icon icon="mdi:format-quote-close" class="w-4 h-4" /></button>
                {#if showQuoted}
                  <div class="mt-2 pl-2 border-l-2 text-[13px] opacity-80 {item.mine ? 'border-primary-foreground/40' : 'border-foreground/20'}">
                    {@render text(msg.chat!.quoted ?? '')}
                  </div>
                {/if}
              {/if}
            </div>
          {:else if mode === 'card'}
            <ChatRichCard subject={msg.subject} preview={msg.chat?.text ?? ''} expanded={cardExpanded} onToggle={() => (cardExpanded = !cardExpanded)} body={emailBody} />
          {:else}
            <div class="overflow-hidden">{@render emailBody()}</div>
          {/if}

          <!-- Hover / focus actions -->
          <div class="absolute {actionsPos} flex items-center gap-0.5 opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100 group-has-focus-visible:opacity-100 transition-opacity">
            {#if msg.isDraft && onEditDraft}
              <button class="p-1 rounded hover:bg-muted" title={$_('viewer.editDraft')} onclick={() => onEditDraft(msg.id)}>
                <Icon icon="mdi:pencil" class="w-4 h-4 text-muted-foreground" />
              </button>
            {/if}
            <button class="p-1 rounded hover:bg-muted" title={$_('viewer.reply')} onclick={() => reply('reply')}>
              <Icon icon="mdi:reply" class="w-4 h-4 text-muted-foreground" />
            </button>
            <DropdownMenu.Root>
              <DropdownMenu.Trigger class="p-1 rounded hover:bg-muted" title={$_('chat.messageActions')} aria-label={$_('chat.messageActions')}>
                <Icon icon="mdi:dots-horizontal" class="w-4 h-4 text-muted-foreground" />
              </DropdownMenu.Trigger>
              <DropdownMenu.Content align={item.mine ? 'end' : 'start'}>
                <DropdownMenu.Item onSelect={() => reply('reply')}><Icon icon="mdi:reply" class="w-4 h-4 mr-2" />{$_('viewer.reply')}</DropdownMenu.Item>
                <DropdownMenu.Item onSelect={() => reply('reply-all')}><Icon icon="mdi:reply-all" class="w-4 h-4 mr-2" />{$_('viewer.replyAll')}</DropdownMenu.Item>
                <DropdownMenu.Item onSelect={() => reply('forward')}><Icon icon="mdi:share" class="w-4 h-4 mr-2" />{$_('viewer.forward')}</DropdownMenu.Item>
                <DropdownMenu.Separator />
                {#if !encrypted && msg.chat}
                  <DropdownMenu.Item onSelect={() => (showOriginal = !showOriginal)}>
                    <Icon icon={showOriginal ? 'mdi:chat-outline' : 'mdi:email-outline'} class="w-4 h-4 mr-2" />{showOriginal ? $_('chat.showChatView') : $_('chat.showOriginal')}
                  </DropdownMenu.Item>
                {/if}
                {#if canDarken && mode !== 'bubble'}
                  <DropdownMenu.Item onSelect={() => (darkOverride = !darkOverride)}>
                    <Icon icon={darken ? 'mdi:white-balance-sunny' : 'mdi:weather-night'} class="w-4 h-4 mr-2" />{darken ? $_('viewer.darkMailToLight') : $_('viewer.darkMailToDark')}
                  </DropdownMenu.Item>
                {/if}
                <DropdownMenu.Item onSelect={toggleSource}><Icon icon="mdi:code-tags" class="w-4 h-4 mr-2" />{sourceOpen ? $_('viewer.hideSource') : $_('viewer.viewSource')}</DropdownMenu.Item>
                <DropdownMenu.Item onSelect={() => copy(msg.chat?.text || bodyText || '', $_('chat.messageText'))}><Icon icon="mdi:content-copy" class="w-4 h-4 mr-2" />{$_('chat.copyText')}</DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          </div>
        </div>

        {#if hasAttachments}
          <AttachmentList class="w-full max-w-md" messageId={msg.id} encryptedAttachments={decryptedAttachments} />
        {/if}

        {#if sourceOpen}
          <div class="relative w-full">
            {#if source === null}
              <div class="flex items-center gap-2 text-sm text-muted-foreground">
                <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />{$_('viewer.loadingSource')}
              </div>
            {:else}
              <button
                class="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80"
                title={$_('viewer.copySource')}
                onclick={() => copy(source ?? '', $_('viewer.viewSource'))}
              >
                <Icon icon="mdi:content-copy" class="w-4 h-4" />
              </button>
              <pre class="text-xs bg-muted/50 p-4 rounded-md max-h-96 overflow-auto whitespace-pre-wrap break-all font-mono">{source}</pre>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
</MessageContextMenu>
