<script lang="ts">
  // ChatComposer — the reply box docked under a chat: attachments, autosizing
  // text, a context line ("Reply to:" target | recipient count | "Topic:" subject,
  // sender), Expand
  // (open in the full composer) and Send. State and saving live in the ChatComposer
  // controller.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { getChatSendKey } from '$lib/stores/settings.svelte'
  import type { ChatComposer } from './chatComposerState.svelte'
  import { displayName } from './chatFormat'

  interface Props {
    composer: ChatComposer
    // Subject of the message being answered, and the alias or account the
    // reply goes out from ('' when neither needs naming).
    subject?: string
    from?: string
    disabled?: boolean
    onEscape?: () => void
    onExpand: () => void
  }

  let { composer, subject = '', from = '', disabled = false, onEscape, onExpand }: Props = $props()

  let textarea = $state<HTMLTextAreaElement | null>(null)
  const ctrlSends = $derived(getChatSendKey() === 'ctrl-enter')
  const sendHint = $derived(ctrlSends ? $_('chat.sendHintCtrl') : $_('chat.sendHintEnter'))
  const canSend = $derived(!disabled && !composer.isEmpty)

  // Name who the reply goes to, so the reply mode is never a guess.
  const MAX_NAMES = 3
  const MAX_TITLE_EMAILS = 10
  const listed = (items: string[], max: number) =>
    items.length > max ? `${items.slice(0, max).join(', ')} ${$_('chat.andMore', { values: { count: items.length - max } })}` : items.join(', ')
  const people = $derived(composer.recipients)
  const emails = (list: { email: string }[]) => listed(list.map((p) => p.email), MAX_TITLE_EMAILS)
  // The textarea's title doubles as its accessible description.
  const title = $derived(people.length > 0 ? `${emails(people)}\n${sendHint}` : sendHint)
  // "Reply to:" switches between All and the person (or Reply-To) a plain
  // reply reaches; when both reach the same people there is nothing to choose.
  // Null while unknown or when a plain reply reaches no one.
  const choices = $derived(composer.choices?.reply.length ? composer.choices : null)
  const replyNames = $derived(choices ? listed(choices.reply.map(displayName), MAX_NAMES) : '')
  const sameChoice = $derived.by(() => {
    if (!choices) return true
    const key = (list: { email: string }[]) => list.map((p) => p.email.toLowerCase()).sort().join('\n')
    return key(choices.reply) === key(choices.all)
  })

  // Grow with the text up to the textarea's CSS max-height, then scroll.
  $effect(() => {
    void composer.text
    if (!textarea) return
    textarea.style.height = 'auto'
    // border-box sizing: scrollHeight excludes the border, so add it back.
    const border = textarea.offsetHeight - textarea.clientHeight
    textarea.style.height = `${textarea.scrollHeight + border}px`
  })

  export function focus() {
    textarea?.focus()
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault()
      textarea?.blur()
      onEscape?.()
      return
    }
    if (e.key !== 'Enter' || e.isComposing) return
    const modifier = e.ctrlKey || e.metaKey
    if (ctrlSends ? !modifier : e.shiftKey || modifier) return
    e.preventDefault()
    if (canSend) void composer.send()
  }

  async function attach() {
    try {
      await composer.attach()
    } catch {
      toasts.error($_('chat.attachFailed'))
    }
  }

  function sizeLabel(a: { content_base64?: string; content?: unknown }): string {
    const b64 = a.content_base64 || (typeof a.content === 'string' ? a.content : '')
    const kb = Math.max(1, Math.round((b64.length * 3) / 4 / 1024))
    return kb >= 1024 ? `${(kb / 1024).toFixed(1)} MB` : `${kb} KB`
  }
</script>

{#snippet sep()}
  <span class="shrink-0" aria-hidden="true">|</span>
{/snippet}

<div class="mx-auto w-full max-w-(--chat-column) border-t border-divider px-3 py-2 bg-background">
  <div class="flex items-baseline gap-1.5 mb-1.5 px-1 text-xs text-fg-tertiary min-w-0">
    {#if choices}
      <span class="shrink-0">{$_('chat.replyTo')}</span>
      {#if sameChoice}
        <span class="truncate min-w-[4rem] shrink-[3] font-medium text-foreground" title={emails(choices.reply)}>{replyNames}</span>
      {:else}
        <!-- Shows the selected target; a click switches to the other one. -->
        <button
          class="flex items-baseline gap-0.5 {composer.replyAll ? 'shrink-0' : 'min-w-[4rem] shrink-[3]'} font-medium text-foreground hover:underline disabled:opacity-50 disabled:pointer-events-none"
          title={$_('chat.replySwitchTo', { values: { target: composer.replyAll ? replyNames : $_('chat.replyToAll') } })}
          {disabled}
          onclick={() => composer.toggleReplyAll()}
        >
          <span class="truncate">{composer.replyAll ? $_('chat.replyToAll') : replyNames}</span>
          <Icon icon="mdi:chevron-down" class="w-3.5 h-3.5 shrink-0 self-center" aria-hidden="true" />
        </button>
        {#if composer.replyAll}
          {@render sep()}
          <span class="shrink-0" title={emails(choices.all)}>{$_('chat.replyRecipientCount', { values: { count: choices.all.length } })}</span>
        {/if}
      {/if}
    {/if}
    {#if subject}
      {#if choices}{@render sep()}{/if}
      <span class="truncate min-w-0 shrink max-w-[24rem]" title={subject}>{$_('chat.replyTopic', { values: { subject } })}</span>
    {/if}
    {#if from}
      <span class="ml-auto shrink-0 max-w-[40%] truncate">{$_('chat.replyLineFrom', { values: { sender: from } })}</span>
    {/if}
  </div>

  {#if composer.attachments.length > 0}
    <ul class="flex flex-wrap gap-1.5 mb-2" aria-label={$_('chat.attachments')}>
      {#each composer.attachments as att, i (i)}
        <li class="flex items-center gap-1 pl-2 pr-1 py-0.5 rounded-full bg-muted text-xs max-w-60">
          <Icon icon="mdi:paperclip" class="w-3.5 h-3.5 shrink-0 text-muted-foreground" />
          <span class="truncate">{att.filename}</span>
          <span class="text-muted-foreground shrink-0">{sizeLabel(att)}</span>
          <button
            class="p-0.5 rounded-full hover:bg-foreground/10"
            title={$_('chat.removeAttachment')}
            aria-label={$_('chat.removeAttachmentNamed', { values: { name: att.filename } })}
            onclick={() => composer.removeAttachment(i)}
          >
            <Icon icon="mdi:close" class="w-3.5 h-3.5" />
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="flex items-end gap-1 min-h-control rounded-xl border border-divider px-1.5 py-1 focus-within:ring-2 focus-within:ring-ring">
    <button class="shrink-0 p-1.5 rounded-full hover:bg-muted" title={$_('chat.attach')} aria-label={$_('chat.attach')} {disabled} onclick={attach}>
      <Icon icon="mdi:paperclip" class="w-5 h-5 text-muted-foreground" />
    </button>

    <textarea
      bind:this={textarea}
      class="flex-1 min-w-0 max-h-[40vh] resize-none bg-transparent px-1.5 py-1.5 text-sm leading-5 focus:outline-none scrollbar-thin"
      rows="1"
      placeholder={$_('chat.typeReply')}
      aria-label={$_('viewer.reply')}
      {title}
      spellcheck="true"
      {disabled}
      value={composer.text}
      oninput={(e) => composer.setText(e.currentTarget.value)}
      onkeydown={handleKeyDown}
      onblur={() => void composer.flush()}
    ></textarea>

    <button class="shrink-0 p-1.5 rounded-full hover:bg-muted" title={$_('chat.expand')} aria-label={$_('chat.expand')} {disabled} onclick={onExpand}>
      <Icon icon="mdi:arrow-expand" class="w-5 h-5 text-muted-foreground" />
    </button>

    <!-- The wrapper carries the tooltip, which a disabled button may not show. -->
    <span class="shrink-0" title={`${$_('chat.send')} (${sendHint})`}>
      <button
        class="h-8 w-8 rounded-full border flex items-center justify-center transition-colors border-primary bg-primary text-primary-foreground enabled:hover:bg-primary/90 disabled:border-border disabled:bg-transparent disabled:text-muted-foreground disabled:cursor-not-allowed"
        aria-label={$_('chat.send')}
        disabled={!canSend}
        onclick={() => void composer.send()}
      >
        <Icon icon="mdi:send" class="w-4 h-4" />
      </button>
    </span>
  </div>
</div>
