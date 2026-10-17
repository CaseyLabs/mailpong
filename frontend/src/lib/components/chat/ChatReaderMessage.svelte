<script lang="ts">
  // ChatReaderMessage — a newsletter, receipt or other rich message shown as
  // sanitized chat content: the backend's reader blocks (headings, paragraphs,
  // list items, images; http/https/mailto links only). Long messages fold
  // after the first blocks. Remote images follow the same rules as the full
  // body: allowlist, always-load setting, Load images and Ctrl+L. A decrypted
  // message's cid: pictures come from its inline attachments; unknown ones are dropped.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { AddImageAllowlist } from '../../../../wailsjs/go/app/App'
  import type { message } from '../../../../wailsjs/go/models'
  import EmailBodyImageBanner from '$lib/components/viewer/EmailBodyImageBanner.svelte'
  import { extractDomain } from '$lib/components/viewer/emailBodyHtml'
  import { isImageAllowedSync, refreshImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import { getAlwaysLoadImages } from '$lib/stores/settings.svelte'
  import { linkify } from './chatFormat'
  import { resolveReaderCids } from './chatReaderCid'

  interface Props {
    blocks: message.ReaderBlock[]
    // Content ID → data URL of a decrypted message's embedded pictures.
    inlineAttachments?: Record<string, string>
    fromEmail: string
    flat: boolean
    onOpenLink: (url: string) => void
    onImagesLoaded: () => void
  }

  let { blocks: rawBlocks, inlineAttachments, fromEmail, flat, onOpenLink, onImagesLoaded }: Props = $props()

  // Blocks shown before "Show more"; a fold must hide more than a couple.
  const FOLD_AT = 8
  const FOLD_MIN_HIDDEN = 3
  // A lone link this short reads as a call-to-action button.
  const ACTION_CHARS = 40

  let expanded = $state(false)
  let imagesLoaded = $state(false)
  let alwaysLoadOpen = $state(false)

  const blocks = $derived(resolveReaderCids(rawBlocks, inlineAttachments))
  const foldable = $derived(blocks.length >= FOLD_AT + FOLD_MIN_HIDDEN)
  const shown = $derived(foldable && !expanded ? blocks.slice(0, FOLD_AT) : blocks)
  const isRemote = (src = '') => /^https?:/i.test(src)
  const hasRemoteImages = $derived(blocks.some((b) => b.kind === 'image' && isRemote(b.src)))
  const showImages = $derived(imagesLoaded || getAlwaysLoadImages() || isImageAllowedSync(fromEmail))

  const plain = (b: message.ReaderBlock) => (b.spans ?? []).map((s) => s.text).join('')
  const isAction = (b: message.ReaderBlock) =>
    b.kind === 'text' && b.spans?.length === 1 && !!b.spans[0].href && b.spans[0].text.length <= ACTION_CHARS

  function loadImages() {
    imagesLoaded = true
    onImagesLoaded()
  }

  async function allow(kind: 'sender' | 'domain') {
    const value = kind === 'sender' ? fromEmail : extractDomain(fromEmail)
    if (!value) return
    try {
      await AddImageAllowlist(kind, value)
      refreshImageAllowlist()
      loadImages()
    } catch (err) {
      console.error('[ChatReaderMessage] Failed to add to image allowlist:', err)
    }
  }

  // Ctrl+L loads images and Ctrl+Shift+L opens the always-load menu, as in the full body.
  $effect(() => {
    const load = () => { if (hasRemoteImages && !showImages) loadImages() }
    const menu = () => { if (hasRemoteImages && !showImages && fromEmail) alwaysLoadOpen = true }
    window.addEventListener('load-remote-images', load)
    window.addEventListener('open-always-load-dropdown', menu)
    return () => {
      window.removeEventListener('load-remote-images', load)
      window.removeEventListener('open-always-load-dropdown', menu)
    }
  })

  function open(e: MouseEvent, url: string) {
    e.preventDefault()
    onOpenLink(url)
  }
</script>

{#snippet spans(block: message.ReaderBlock)}
  {#each block.spans ?? [] as span, i (i)}
    {#if span.href}
      <a href={span.href} class="underline underline-offset-2 text-primary" onclick={(e) => open(e, span.href!)}>{span.text}</a>
    {:else}
      {#each linkify(span.text) as seg, j (j)}
        {#if seg.url}
          <a href={seg.url} class="underline underline-offset-2 break-all text-primary" onclick={(e) => open(e, seg.url!)}>{seg.text}</a>
        {:else}{seg.text}{/if}
      {/each}
    {/if}
  {/each}
{/snippet}

{#snippet image(block: message.ReaderBlock)}
  {#if showImages || !isRemote(block.src)}
    <img src={block.src} alt={block.alt ?? ''} referrerpolicy="no-referrer" loading="lazy" class="block max-w-full h-auto rounded-lg" />
  {:else if block.alt}
    <span class="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
      <Icon icon="mdi:image-outline" class="w-4 h-4 shrink-0" aria-hidden="true" />{block.alt}
    </span>
  {/if}
{/snippet}

<div class="{flat ? 'px-1 py-0.5' : 'px-3.5 py-2.5'} flex flex-col gap-2 text-[1rem] leading-[1.45] wrap-anywhere" data-chat-reader>
  {#if hasRemoteImages && !showImages}
    <!-- pr-16 keeps the button clear of the message's hover actions in the corner. -->
    <div class="pr-16 -mb-2">
      <EmailBodyImageBanner
        {fromEmail}
        bind:open={alwaysLoadOpen}
        onLoadImages={loadImages}
        onAlwaysLoadDomain={() => allow('domain')}
        onAlwaysLoadSender={() => allow('sender')}
      />
    </div>
  {/if}
  {#each shown as block, i (i)}
    {#if block.kind === 'image'}
      {#if block.href && (showImages || !isRemote(block.src) || block.alt)}
        <a href={block.href} class="self-start max-w-full" onclick={(e) => open(e, block.href!)}>{@render image(block)}</a>
      {:else}
        {@render image(block)}
      {/if}
    {:else if block.kind === 'heading'}
      <p class="text-[1.0625rem] font-semibold leading-snug whitespace-pre-line" role="heading" aria-level={3}>{@render spans(block)}</p>
    {:else if block.kind === 'item'}
      <p class="flex gap-2 whitespace-pre-line"><span aria-hidden="true">•</span><span class="min-w-0">{@render spans(block)}</span></p>
    {:else if isAction(block)}
      <a
        href={block.spans![0].href}
        class="self-start inline-flex items-center gap-1 rounded-full border border-border px-3 py-1 text-sm font-medium text-primary hover:bg-foreground/5 transition-colors"
        onclick={(e) => open(e, block.spans![0].href!)}
      >{plain(block)}<Icon icon="mdi:open-in-new" class="w-3.5 h-3.5" aria-hidden="true" /></a>
    {:else}
      <p class="whitespace-pre-line">{@render spans(block)}</p>
    {/if}
  {/each}
  {#if foldable}
    <button
      class="self-start inline-flex items-center gap-1 text-sm text-primary hover:underline rounded focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
      aria-expanded={expanded}
      onclick={() => (expanded = !expanded)}
    >
      {expanded ? $_('chat.readerShowLess') : $_('chat.readerShowMore')}
      <Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4" aria-hidden="true" />
    </button>
  {/if}
</div>
