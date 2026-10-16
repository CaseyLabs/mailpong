<script lang="ts">
  import Icon from '@iconify/svelte'
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
  import { GetInlineAttachments, AddImageAllowlist, OpenURL } from '../../../../wailsjs/go/app/App'
  import { getCached, setCache } from '../../stores/inlineAttachmentCache'
  import { isImageAllowedSync, refreshImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import { setFocusedPane, focusPreviousPane, focusNextPane } from '$lib/stores/keyboard.svelte'
  import EmailBodyImageBanner from './EmailBodyImageBanner.svelte'
  import { checkForRemoteImages, extractDomain, getChromeBgHsl, linkifyText } from './emailBodyHtml'
  import { buildIframeContent } from './emailBodyIframe'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { getAlwaysLoadImages, getThemeMode } from '$lib/stores/settings.svelte'

  interface Props {
    messageId: string
    accountId?: string
    bodyHtml?: string
    bodyText?: string
    fromEmail?: string
    onCompose?: (to: string) => void
    onImagesLoaded?: () => void
    encryptedInlineAttachments?: Record<string, string>
    darken?: boolean
  }

  let { messageId, accountId: _accountId, bodyHtml = '', bodyText = '', fromEmail = '', onCompose, onImagesLoaded, encryptedInlineAttachments, darken = false }: Props = $props()

  // State for remote image handling
  let imagesBlocked = $state(true)
  let iframeElement = $state<HTMLIFrameElement | null>(null)
  let iframeReady = $state(false)

  // Inline attachment state
  let inlineAttachments = $state<Record<string, string>>({})
  let lastSentMessageId = $state<string | null>(null)

  // Link tooltip state
  let tooltipVisible = $state(false)
  let tooltipUrl = $state('')
  let tooltipX = $state(0)
  let tooltipY = $state(0)

  // Context menu state (unified for text selection and links)
  let ctxMenuVisible = $state(false)
  let ctxMenuText = $state('')
  let ctxMenuUrl = $state('')
  let ctxMenuX = $state(0)
  let ctxMenuY = $state(0)

  // Derived state
  let hasRemoteImages = $derived(checkForRemoteImages(bodyHtml))

  // Outer iframe element bg: matches the dark-mail surface color (derived from
  // active theme) when darken=true, otherwise white. Reactive to theme changes
  // via getThemeMode() so the iframe outer color updates when user switches.
  let iframeOuterBg = $derived.by(() => {
    void getThemeMode()
    return darken ? getChromeBgHsl() : 'white'
  })

  // Handle a clicked link URL — routes mailto: to composer, others to system browser.
  // Used by both the iframe message handler and the plain text div click handler.
  function handleLinkClick(url: string) {
    if (url.startsWith('mailto:')) {
      const emailAddress = url.replace('mailto:', '').split('?')[0]
      if (onCompose) {
        onCompose(emailAddress)
        return
      }
    }
    safeOpenURL(url)
  }

  // Helper function to safely open URLs
  // Uses our custom OpenURL backend function which properly handles shell escaping
  async function safeOpenURL(url: string) {
    console.log('[EmailBody] Opening URL:', url)

    // Validate URL format first
    try {
      new URL(url) // Validate it's a proper URL

      // Use our backend OpenURL function which properly handles shell escaping
      try {
        await OpenURL(url)
        toasts.info($_('toast.linkOpened'))
      } catch (err) {
        console.error('[EmailBody] OpenURL failed:', err)
        // Fallback to direct BrowserOpenURL
        try {
          BrowserOpenURL(url)
          toasts.info($_('toast.linkOpened'))
        } catch (err2) {
          console.error('[EmailBody] BrowserOpenURL also failed:', err2)
        }
      }
    } catch (e) {
      console.error('[EmailBody] Invalid URL:', url, e)
    }
  }

  function handleIframeMessage(event: MessageEvent) {
    // Only handle messages from this component's iframe
    if (event.source !== iframeElement?.contentWindow) return

    if (event.data?.type === 'iframe-height' && iframeElement) {
      iframeElement.style.height = `${event.data.height + 20}px`
    } else if (event.data?.type === 'iframe-ready') {
      iframeReady = true
    } else if (event.data?.type === 'open-link') {
      handleLinkClick(event.data.url as string)
    } else if (event.data?.type === 'iframe-keydown') {
      // Handle Alt+arrow/hjkl directly for pane navigation
      if (event.data.altKey) {
        const key = event.data.key
        if (key === 'ArrowLeft' || key === 'h') {
          focusPreviousPane()
          // Dispatch event to let App.svelte handle focus
          window.dispatchEvent(new CustomEvent('escape-iframe-focus'))
          return
        } else if (key === 'ArrowRight' || key === 'l') {
          focusNextPane()
          window.dispatchEvent(new CustomEvent('escape-iframe-focus'))
          return
        }
      }
      // For other shortcuts (Ctrl+, Escape), dispatch to window
      const syntheticEvent = new KeyboardEvent('keydown', {
        key: event.data.key,
        code: event.data.code,
        altKey: event.data.altKey,
        ctrlKey: event.data.ctrlKey,
        metaKey: event.data.metaKey,
        shiftKey: event.data.shiftKey,
        bubbles: true,
        cancelable: true
      })
      window.dispatchEvent(syntheticEvent)
    } else if (event.data?.type === 'iframe-focus') {
      // Set focus to viewer pane when iframe is clicked/focused
      setFocusedPane('viewer')
    } else if (event.data?.type === 'link-hover') {
      // Show tooltip with link URL - adjust coordinates relative to iframe position
      if (iframeElement) {
        const iframeRect = iframeElement.getBoundingClientRect()
        tooltipUrl = event.data.url
        tooltipX = iframeRect.left + event.data.x
        tooltipY = iframeRect.top + event.data.y
        tooltipVisible = true
      }
    } else if (event.data?.type === 'link-hover-end') {
      // Hide tooltip
      tooltipVisible = false
    } else if (event.data?.type === 'contextmenu') {
      // Show unified context menu for text selection and/or links
      if (iframeElement) {
        const iframeRect = iframeElement.getBoundingClientRect()
        ctxMenuText = event.data.text || ''
        ctxMenuUrl = event.data.url || ''
        ctxMenuX = iframeRect.left + event.data.x
        ctxMenuY = iframeRect.top + event.data.y
        ctxMenuVisible = true
      }
    }
  }

  function sendInlineImagesToIframe(images: Record<string, string>) {
    if (iframeElement?.contentWindow && Object.keys(images).length > 0) {
      // Use spread operator to create plain object from Svelte 5 $state proxy
      // This is needed because postMessage uses structured clone which can't handle proxies
      iframeElement.contentWindow.postMessage({
        type: 'inline-images',
        images: { ...images }
      }, '*')
    }
  }

  // Returns the message body as printable HTML. The body iframe is sandboxed
  // WITHOUT allow-same-origin, so its DOM isn't readable from here — instead we
  // ask the iframe's own script (over the existing postMessage channel) to hand
  // back its rendered body, with images exactly as displayed (inline resolved,
  // remote in their loaded/blocked state, so printing fires no trackers). Falls
  // back to the plain-text render when there's no iframe.
  export function getPrintableHtml(): Promise<string> {
    const win = iframeElement?.contentWindow
    if (!win) {
      return Promise.resolve(bodyText ? `<div style="white-space:pre-wrap">${linkifyText(bodyText)}</div>` : '')
    }
    return new Promise<string>((resolve) => {
      let settled = false
      const onMsg = (ev: MessageEvent) => {
        if (ev.source !== win || ev.data?.type !== 'print-html') return
        settled = true
        window.removeEventListener('message', onMsg)
        resolve(ev.data.html || '')
      }
      window.addEventListener('message', onMsg)
      win.postMessage({ type: 'request-print-html' }, '*')
      setTimeout(() => {
        if (settled) return
        window.removeEventListener('message', onMsg)
        resolve('')
      }, 1500)
    })
  }

  function loadImages() {
    imagesBlocked = false
    onImagesLoaded?.()
  }

  // Handle "Always load for this sender" action
  async function handleAlwaysLoadSender() {
    if (!fromEmail) return
    try {
      await AddImageAllowlist('sender', fromEmail)
      refreshImageAllowlist()
      loadImages()
    } catch (err) {
      console.error('[EmailBody] Failed to add sender to allowlist:', err)
    }
  }

  // Handle "Always load for this domain" action
  async function handleAlwaysLoadDomain() {
    const domain = extractDomain(fromEmail)
    if (!domain) return
    try {
      await AddImageAllowlist('domain', domain)
      refreshImageAllowlist()
      loadImages()
    } catch (err) {
      console.error('[EmailBody] Failed to add domain to allowlist:', err)
    }
  }

  // Check allowlist and reset state on message change (single effect to avoid race conditions)
  // Uses synchronous frontend cache instead of async Wails call to avoid bridge saturation.
  $effect(() => {
    void messageId // dependency only
    const email = fromEmail
    const hasImages = hasRemoteImages

    // Reset state (was in separate effect — merged to avoid race)
    iframeReady = false
    lastSentMessageId = null
    inlineAttachments = {}
    imagesBlocked = true

    if (!hasImages) return

    if (getAlwaysLoadImages()) {
      imagesBlocked = false
      return
    }

    if (email && isImageAllowedSync(email)) {
      imagesBlocked = false
    }
  })

  // Fetch inline attachments when we have cid references
  $effect(() => {
    const id = messageId
    const html = bodyHtml
    const hasCid = html ? /src=["']cid:([^"']+)["']/i.test(html) : false
    const encInline = encryptedInlineAttachments

    if (!id || !hasCid) {
      return
    }

    // For encrypted messages, use the in-memory inline attachments from decryption
    if (encInline && Object.keys(encInline).length > 0) {
      inlineAttachments = encInline
      return
    }

    // Check memory cache first
    const cached = getCached(id)
    if (cached && Object.keys(cached).length > 0) {
      inlineAttachments = cached
      return
    }

    GetInlineAttachments(id)
      .then((result: Record<string, string>) => {
        const data = result || {}
        inlineAttachments = data
        if (Object.keys(data).length > 0) {
          setCache(id, data)
        }
      })
      .catch((err: Error) => {
        console.error('[EmailBody] Fetch error:', err)
      })
  })

  // Build iframe content
  $effect(() => {
    const html = bodyHtml
    void imagesBlocked // dependency only
    void getThemeMode() // rebuild when theme changes so dark-mail invert re-derives
    const applyDarken = darken

    if (iframeElement && html) {
      const content = buildIframeContent(html, applyDarken, imagesBlocked)
      iframeElement.srcdoc = content
      iframeReady = false
      lastSentMessageId = null
    }
  })

  // Send inline images when ready
  $effect(() => {
    const ready = iframeReady
    const images = inlineAttachments
    const id = messageId
    const alreadySent = lastSentMessageId === id

    if (ready && Object.keys(images).length > 0 && !alreadySent) {
      sendInlineImagesToIframe(images)
      lastSentMessageId = id
    }
  })

  // Message listener
  $effect(() => {
    window.addEventListener('message', handleIframeMessage)
    return () => window.removeEventListener('message', handleIframeMessage)
  })

  // State for controlling the Always Load dropdown
  let alwaysLoadDropdownOpen = $state(false)

  // Listen for Ctrl-L load images event
  $effect(() => {
    function handleLoadImagesEvent() {
      if (hasRemoteImages && imagesBlocked) {
        loadImages()
      }
    }
    window.addEventListener('load-remote-images', handleLoadImagesEvent)
    return () => window.removeEventListener('load-remote-images', handleLoadImagesEvent)
  })

  // Listen for Ctrl-Shift-L always load dropdown event
  $effect(() => {
    function handleAlwaysLoadDropdownEvent() {
      if (hasRemoteImages && imagesBlocked && fromEmail) {
        alwaysLoadDropdownOpen = true
      }
    }
    window.addEventListener('open-always-load-dropdown', handleAlwaysLoadDropdownEvent)
    return () => window.removeEventListener('open-always-load-dropdown', handleAlwaysLoadDropdownEvent)
  })

  // Copy selected text to clipboard
  async function copyTextToClipboard() {
    if (!ctxMenuText) return
    try {
      await navigator.clipboard.writeText(ctxMenuText)
      ctxMenuVisible = false
    } catch (err) {
      console.error('[EmailBody] Failed to copy text:', err)
    }
  }

  // Copy link to clipboard
  async function copyLinkToClipboard() {
    if (!ctxMenuUrl) return
    try {
      await navigator.clipboard.writeText(ctxMenuUrl)
      ctxMenuVisible = false
    } catch (err) {
      console.error('[EmailBody] Failed to copy link:', err)
    }
  }

  // Select all text in iframe
  function selectAllInIframe() {
    iframeElement?.contentWindow?.postMessage({ type: 'select-all' }, '*')
    ctxMenuVisible = false
  }
</script>

<div class="email-body relative">
  {#if bodyHtml}
    {#if hasRemoteImages && imagesBlocked}
      <!-- Compact split button instead of a banner: load once, or always via the chevron. -->
      <EmailBodyImageBanner
        {fromEmail}
        bind:open={alwaysLoadDropdownOpen}
        onLoadImages={loadImages}
        onAlwaysLoadDomain={handleAlwaysLoadDomain}
        onAlwaysLoadSender={handleAlwaysLoadSender}
      />
    {/if}

    <iframe
      bind:this={iframeElement}
      title={$_('aria.emailContent')}
      sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox"
      class="w-full border-0 rounded-md min-h-[100px]"
      style="height: 200px; background-color: {iframeOuterBg};"
    ></iframe>
  {:else if bodyText}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="whitespace-pre-wrap font-sans text-sm text-foreground bg-muted/30 rounded-md p-4"
      onkeydown={(e) => { if (e.key === 'Enter') { const link = (e.target as HTMLElement).closest('a'); if (link?.href) { e.preventDefault(); handleLinkClick(link.href) } } }}
      onclick={(e) => {
        const link = (e.target as HTMLElement).closest('a')
        if (!link?.href) return
        e.preventDefault()
        handleLinkClick(link.href)
      }}
    >
      <!-- eslint-disable-next-line svelte/no-at-html-tags -- linkifyText escapes user content; only injects safe <a> tags around URLs -->
      {@html linkifyText(bodyText)}
    </div>
  {:else}
    <p class="text-muted-foreground italic">{$_('viewer.noContent')}</p>
  {/if}

  <!-- Link hover tooltip -->
  {#if tooltipVisible && tooltipUrl}
    <div
      class="fixed z-50 px-3 py-1.5 text-xs bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 rounded shadow-lg max-w-md truncate pointer-events-none border border-gray-700 dark:border-gray-300"
      style="left: {tooltipX}px; top: {tooltipY + 5}px;"
    >
      {tooltipUrl}
    </div>
  {/if}

  <!-- Context menu (text copy and/or link copy) -->
  {#if ctxMenuVisible}
    <div
      class="fixed z-50 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1 min-w-[160px]"
      style="left: {ctxMenuX}px; top: {ctxMenuY}px;"
      role="menu"
    >
      {#if ctxMenuText}
        <button
          class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
          onclick={copyTextToClipboard}
        >
          <Icon icon="mdi:content-copy" class="w-4 h-4" />
          {$_('viewer.copy')}
        </button>
      {/if}
      {#if ctxMenuUrl}
        <button
          class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
          onclick={copyLinkToClipboard}
        >
          <Icon icon="mdi:link-variant" class="w-4 h-4" />
          {$_('viewer.copyLink')}
        </button>
      {/if}
      <button
        class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        onclick={selectAllInIframe}
      >
        <Icon icon="mdi:select-all" class="w-4 h-4" />
        {$_('viewer.selectAll')}
      </button>
    </div>
  {/if}
</div>

<!-- Click outside to close context menu -->
{#if ctxMenuVisible}
  <button
    type="button"
    class="fixed inset-0 z-40 cursor-default"
    aria-label={$_('aria.closeContextMenu')}
    onclick={() => ctxMenuVisible = false}
    onkeydown={(e) => { if (e.key === 'Escape') ctxMenuVisible = false }}
  ></button>
{/if}
