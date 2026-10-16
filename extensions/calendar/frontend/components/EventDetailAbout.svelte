<script lang="ts">
  // HTML "About" body of EventDetail: sanitized rich-text description with
  // link clicks routed through the hardened opener.

  import { _ } from 'svelte-i18n'
  import { logger } from '$extensions/calendar/frontend/lib/logger'
  // @ts-ignore - wailsjs bindings
  import { Calendar_OpenURL } from '$wailsjs/go/app/App.js'

  interface Props {
    html: string
  }

  let { html }: Props = $props()

  // Container for the sanitized HTML About body ({@html}). Its anchors are raw
  // DOM nodes (not Linkified-generated), so we intercept their clicks here to
  // route through the same hardened opener every other calendar link uses —
  // Wails' default webview navigation doesn't reach the host browser (Flatpak).
  let aboutEl = $state<HTMLElement | null>(null)
  $effect(() => {
    const el = aboutEl
    if (!el) return
    const onClick = (e: MouseEvent) => {
      const anchor = (e.target as HTMLElement | null)?.closest('a')
      const href = anchor?.getAttribute('href')
      if (!href) return
      e.preventDefault()
      e.stopPropagation()
      Calendar_OpenURL(href).catch((err: unknown) => logger.warn(`openURL failed: ${err}`))
    }
    el.addEventListener('click', onClick)
    return () => el.removeEventListener('click', onClick)
  })
</script>

<div>
  <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
    {$_('calendar.detail.aboutLabel')}
  </div>
  <div class="cal-about-html text-foreground wrap-break-word text-sm" bind:this={aboutEl}>
    {@html html}
  </div>
</div>

<style>
  .cal-about-html :global(p) {
    margin: 0 0 0.5rem;
  }
  .cal-about-html :global(p:last-child) {
    margin-bottom: 0;
  }
  .cal-about-html :global(ul),
  .cal-about-html :global(ol) {
    margin: 0 0 0.5rem;
    padding-left: 1.25rem;
  }
  .cal-about-html :global(ul) {
    list-style: disc;
  }
  .cal-about-html :global(ol) {
    list-style: decimal;
  }
  .cal-about-html :global(a) {
    color: hsl(var(--primary));
    text-decoration: underline;
  }
  .cal-about-html :global(b),
  .cal-about-html :global(strong) {
    font-weight: 600;
  }
</style>
