// ChatScroll — scrolling and message focus for the open chat: the anchor that
// keeps the first unread message in view while bodies render, and Tab
// navigation between messages and folded-run rows.

import { tick } from 'svelte'

// After a load, keep the scroll anchored while bodies render and grow.
const ANCHOR_MS = 2000
const SCROLL_AMOUNT = 100
const BOTTOM_SLOP_PX = 40
// Space kept above the anchored message, below the sticky subject heading.
const ANCHOR_GAP_PX = 16
const SCROLL_GAP_PX = 8

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
  // An element held at a fixed offset while content around it resizes.
  private keep: { el: HTMLElement; top: number; until: number } | null = null

  // Back to a clean slate for a newly opened thread.
  reset() {
    this.focusedId = null
    this.anchor = null
    this.keep = null
  }

  // Stop following the anchor (the user is scrolling).
  release = () => {
    this.anchor = null
    this.keep = null
  }

  // Run `change`, which grows or shrinks the content around `el`, and keep
  // `el` where it is on screen, also while bodies render afterwards.
  async holdPosition(el: HTMLElement, change: () => void) {
    this.anchor = null
    this.keep = { el, top: el.getBoundingClientRect().top, until: Date.now() + ANCHOR_MS }
    change()
    await tick()
    this.applyKeep()
  }

  private applyKeep(): boolean {
    const { keep, scroller } = this
    if (!keep || !scroller || !keep.el.isConnected || Date.now() > keep.until) {
      this.keep = null
      return false
    }
    scroller.scrollTop += keep.el.getBoundingClientRect().top - keep.top
    return true
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
    const msg = anchor.id ? this.messageEl(anchor.id) : null
    if (!msg) return toBottom(scroller)
    // A message that starts a day has its day label just above it; keep that in view too.
    const el = this.dayLabelOf(msg) ?? msg
    scroller.scrollTop = el.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - this.headingHeight() - ANCHOR_GAP_PX
  }

  // The day label row directly above a message, if it starts a day. The bubble
  // may sit inside wrapper elements, so climb to the level that has a sibling.
  private dayLabelOf(msg: HTMLElement): Element | null {
    let node: HTMLElement = msg
    while (!node.previousElementSibling && node.parentElement && node.parentElement !== this.content) node = node.parentElement
    const prev = node.previousElementSibling
    return prev?.querySelector('[role="separator"]') ? prev : null
  }

  // Height of the sticky subject heading covering the top of the scroller (0 without one).
  private headingHeight(): number {
    return this.scroller?.querySelector<HTMLElement>('[data-subject-heading]')?.offsetHeight ?? 0
  }

  // Focusing a message scrolls it clear of the sticky heading.
  private syncScrollPadding() {
    this.scroller?.style.setProperty('scroll-padding-top', `${this.headingHeight() + SCROLL_GAP_PX}px`)
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
      this.syncScrollPadding()
      if (this.applyKeep()) return
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
    this.release()
    await tick()
    if (this.scroller) toBottom(this.scroller)
  }

  by(direction: 1 | -1) {
    this.release()
    this.scroller?.scrollBy({ top: direction * SCROLL_AMOUNT, behavior: 'smooth' })
  }

  messageEl(id: string): HTMLElement | null {
    return this.scroller?.querySelector<HTMLElement>(`[data-message-id="${id}"]`) ?? null
  }

  focus(id: string) {
    this.focusedId = id
    this.messageEl(id)?.focus()
  }

  // Tab moves between messages and folded-run rows, in stream order; at
  // either end native Tab leaves the view.
  handleTab(e: KeyboardEvent) {
    if (e.key !== 'Tab' || !this.scroller) return
    const stops = [...this.scroller.querySelectorAll<HTMLElement>('[role="article"][data-message-id], [data-compact-row]')]
    const row = (e.target as HTMLElement | null)?.closest<HTMLElement>('[data-compact-row]')
    const idx = row ? stops.indexOf(row) : stops.findIndex((el) => el.dataset.messageId === this.focusedId)
    if (stops.length === 0 || (e.shiftKey ? idx <= 0 : idx >= stops.length - 1)) return
    e.preventDefault()
    const next = stops[e.shiftKey ? idx - 1 : idx + 1]
    if (next.dataset.messageId) this.focus(next.dataset.messageId)
    else next.focus()
  }
}
