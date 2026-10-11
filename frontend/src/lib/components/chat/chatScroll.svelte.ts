// ChatScroll — scrolling and message focus for the open chat: the anchor that
// keeps the first unread message in view while bodies render, and Tab
// navigation between messages.

import { tick } from 'svelte'

// After a load, keep the scroll anchored while bodies render and grow.
const ANCHOR_MS = 2000
const SCROLL_AMOUNT = 100
const BOTTOM_SLOP_PX = 40

// Whether the scroller is at (or within a few pixels of) the bottom.
export function nearBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight < BOTTOM_SLOP_PX
}

function toBottom(el: HTMLElement) {
  el.scrollTop = el.scrollHeight
}

export class ChatScroll {
  scroller = $state<HTMLDivElement | null>(null)
  content = $state<HTMLDivElement | null>(null)
  focusedId = $state<string | null>(null)
  private anchor: { id: string | null; until: number } | null = null

  constructor(private messageIds: () => string[]) {}

  // Back to a clean slate for a newly opened thread.
  reset() {
    this.focusedId = null
    this.anchor = null
  }

  // Stop following the anchor (the user is scrolling).
  release = () => {
    this.anchor = null
  }

  // Scroll to the first unread message, else to the bottom.
  async toStart(firstUnreadId: string | null) {
    this.anchor = { id: firstUnreadId, until: Date.now() + ANCHOR_MS }
    await tick()
    this.apply()
  }

  private apply() {
    const { anchor, scroller } = this
    if (!anchor || !scroller || Date.now() > anchor.until) {
      this.anchor = null
      return
    }
    const el = anchor.id ? this.messageEl(anchor.id) : null
    if (!el) return toBottom(scroller)
    scroller.scrollTop = el.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - 48
  }

  // Bodies (iframes) grow after render; re-apply the anchor as they do.
  // Otherwise, when the content or the scroller resizes (the composer grows),
  // stay at the bottom if the view was there.
  observe(): (() => void) | undefined {
    const { content, scroller } = this
    if (!content || !scroller) return
    let atBottom = true
    const onScroll = () => {
      atBottom = nearBottom(scroller)
    }
    const observer = new ResizeObserver(() => {
      if (this.anchor) this.apply()
      else if (atBottom) toBottom(scroller)
    })
    observer.observe(content)
    observer.observe(scroller)
    scroller.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      observer.disconnect()
      scroller.removeEventListener('scroll', onScroll)
    }
  }

  // Keep a new pending bubble in view.
  async toEnd() {
    this.anchor = null
    await tick()
    if (this.scroller) toBottom(this.scroller)
  }

  by(direction: 1 | -1) {
    this.anchor = null
    this.scroller?.scrollBy({ top: direction * SCROLL_AMOUNT, behavior: 'smooth' })
  }

  messageEl(id: string): HTMLElement | null {
    return this.scroller?.querySelector<HTMLElement>(`[data-message-id="${id}"]`) ?? null
  }

  focus(id: string) {
    this.focusedId = id
    this.messageEl(id)?.focus()
  }

  // Tab moves between messages; at either end native Tab leaves the view.
  handleTab(e: KeyboardEvent) {
    const ids = this.messageIds()
    if (e.key !== 'Tab' || ids.length === 0) return
    const idx = this.focusedId ? ids.indexOf(this.focusedId) : -1
    if (e.shiftKey ? idx <= 0 : idx >= ids.length - 1) return
    e.preventDefault()
    this.focus(ids[e.shiftKey ? idx - 1 : idx + 1])
  }
}
