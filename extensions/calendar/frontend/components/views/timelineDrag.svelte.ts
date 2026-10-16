// Drag-to-move / drag-to-resize controller of TimelineView, with the pending
// SendInvitationsDialog state. TimelineView creates one TimelineDrag and
// forwards the pointer events to it.
import { get } from 'svelte/store'
import { _ } from 'svelte-i18n'
import { events } from '$extensions/calendar/frontend/stores/events.svelte'
import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
import { calendarView } from '$extensions/calendar/frontend/stores/calendarView.svelte'
import { toasts } from '$lib/stores/toast'
// @ts-ignore - wailsjs bindings
import { Calendar_MoveEvent } from '$wailsjs/go/app/App.js'
// @ts-ignore - wailsjs bindings
import type { backend } from '$wailsjs/go/models'
import {
  DAY_PX,
  DRAG_THRESHOLD_PX,
  detectMode,
  sourceKindOf,
  type DragState,
  type TimedBlock,
} from './timelineLayout'

/** The view's props and click handler, read lazily so they stay reactive. */
export interface TimelineDragEnv {
  readonly dates: Date[]
  onEventClick(inst: backend.EventInstance): void
}

export class TimelineDrag {
  constructor(private readonly env: TimelineDragEnv) {}

  // Three grip zones per event block: 6px top (resize-start), 6px bottom
  // (resize-end), remaining body (move). Recurring events skip the gesture
  // entirely — the dialog path (RecurrenceScopeDialog → composer) handles
  // them with proper scope semantics.
  //
  // Click vs drag disambiguation via a 4-pixel-or-cross-column movement
  // threshold + a wasDragged flag that suppresses the synthetic click event
  // browsers fire after pointerup. Below the threshold, the existing
  // onclick path runs and opens EventDetail as before.

  dragState = $state<DragState | null>(null)

  // Pending dragState held while SendInvitationsDialog is open — once the
  // user picks Send / Don't send, we fire performDragSave with the
  // chosen sendUpdates value. Cancel just clears dragState (visual snap
  // back to original position via the block.instance fallback).
  pendingDragSave = $state<{ ds: DragState; newStartUnix: number; newEndUnix: number } | null>(null)
  sendInvitationsOpen = $state(false)
  wasDragged = $state(false)
  saving = $state(false)

  onBlockPointerDown(block: TimedBlock, colIdx: number, e: PointerEvent) {
    // Recurring events use the dialog path (RecurrenceScopeDialog →
    // composer). Drag would need to fire a modal mid-gesture, which is
    // a bad UX. Let the click bubble naturally.
    if (block.instance.rruleText) return

    // Only primary button (left mouse / single finger).
    if (e.button !== 0) return

    const btn = e.currentTarget as HTMLButtonElement
    const btnRect = btn.getBoundingClientRect()
    const relY = e.clientY - btnRect.top
    const mode = detectMode(relY, btnRect.height)

    // Column rect for cross-column drift calculations.
    const colEl = btn.closest('[data-day-col]') as HTMLElement | null
    if (!colEl) return
    const colRect = colEl.getBoundingClientRect()

    btn.setPointerCapture(e.pointerId)
    e.preventDefault()

    this.dragState = {
      instanceId: block.instance.id,
      eventId: block.instance.id,
      mode,
      originColIdx: colIdx,
      originStartUnix: block.instance.instanceStartUnix,
      originEndUnix: block.instance.instanceEndUnix,
      startClientX: e.clientX,
      startClientY: e.clientY,
      columnWidth: colRect.width,
      currentColIdx: colIdx,
      currentDeltaMinutes: 0,
      movedPastThreshold: false,
      attendeeCount: block.instance.attendees?.length ?? 0,
      masterSourceKind: sourceKindOf(block.instance.calendarId),
    }
  }

  onWindowPointerMove(e: PointerEvent) {
    const ds = this.dragState
    if (!ds) return
    // Once the save has been claimed (pointerup → persistDrag in flight),
    // the snapped position is locked in. Ignore further pointermove events
    // so the block stops tracking the cursor while we wait for the refetch.
    if (this.saving) return

    const deltaY = e.clientY - ds.startClientY
    const rawMin = (deltaY / DAY_PX) * 1440
    const snapped = Math.round(rawMin / 15) * 15

    // Column drift only for move mode + WeekView (multi-column).
    let newColIdx = ds.originColIdx
    if (ds.mode === 'move' && this.env.dates.length > 1) {
      const colEls = document.querySelectorAll<HTMLElement>('[data-day-col]')
      for (let i = 0; i < colEls.length; i++) {
        const r = colEls[i].getBoundingClientRect()
        if (e.clientX >= r.left && e.clientX <= r.right) {
          const idx = Number(colEls[i].dataset.dayCol)
          if (!Number.isNaN(idx)) newColIdx = idx
          break
        }
      }
    }

    const moved =
      Math.abs(deltaY) > DRAG_THRESHOLD_PX ||
      newColIdx !== ds.originColIdx

    this.dragState = {
      ...ds,
      currentColIdx: newColIdx,
      currentDeltaMinutes: snapped,
      movedPastThreshold: ds.movedPastThreshold || moved,
    }
  }

  onWindowPointerUp(_e: PointerEvent) {
    const ds = this.dragState
    if (!ds) return

    // Click without drag — clear and let the native click → onclick path fire.
    if (!ds.movedPastThreshold) {
      this.dragState = null
      return
    }

    // Drag occurred. Claim the save synchronously via the `saving` flag so a
    // duplicate pointerup or pointercancel can't re-fire persistDrag while
    // we're mid-save. dragState stays set until the save resolves — the
    // block continues to render at its snapped target position throughout.
    if (this.saving) return
    this.saving = true
    this.wasDragged = true
    void this.persistDrag(ds)
  }

  async persistDrag(ds: DragState) {
    let newStartUnix = ds.originStartUnix
    let newEndUnix = ds.originEndUnix

    switch (ds.mode) {
      case 'move': {
        const minuteShift = ds.currentDeltaMinutes * 60
        newStartUnix += minuteShift
        newEndUnix += minuteShift
        const dayDelta = ds.currentColIdx - ds.originColIdx
        if (dayDelta !== 0) {
          newStartUnix += dayDelta * 86400
          newEndUnix += dayDelta * 86400
        }
        break
      }
      case 'resize-start': {
        newStartUnix += ds.currentDeltaMinutes * 60
        if (newStartUnix > newEndUnix - 15 * 60) {
          newStartUnix = newEndUnix - 15 * 60
        }
        break
      }
      case 'resize-end': {
        newEndUnix += ds.currentDeltaMinutes * 60
        if (newEndUnix < newStartUnix + 15 * 60) {
          newEndUnix = newStartUnix + 15 * 60
        }
        break
      }
    }

    // If the event has attendees, intercept with SendInvitationsDialog
    // (same UX as Edit-via-composer). Cancel reverts the visual snap.
    // Otherwise save directly.
    if (ds.attendeeCount > 0) {
      this.pendingDragSave = { ds, newStartUnix, newEndUnix }
      this.sendInvitationsOpen = true
      return
    }
    await this.performDragSave(ds, newStartUnix, newEndUnix, 'all')
  }

  async performDragSave(
    ds: DragState,
    newStartUnix: number,
    newEndUnix: number,
    sendUpdates: string,
  ) {
    try {
      // The backend keeps every other field as stored (body, reminder,
      // transparency, visibility, attendees, time zone).
      await Calendar_MoveEvent(
        ds.eventId,
        newStartUnix,
        newEndUnix,
        ds.attendeeCount > 0 ? sendUpdates : '',
      )
      // Wait for the events store to reflect the new state BEFORE clearing
      // dragState — otherwise the block would briefly snap back to its old
      // position (from the stale block.instance) before the refetch lands.
      await this.refreshEventsAsync()
    } catch (err) {
      // On failure: dragState clears (in finally) → block uses
      // block.instance position which is the ORIGINAL (since no commit
      // happened) → natural snap-back to where the user grabbed it.
      toasts.error(get(_)('calendar.drag.errorSave', { values: { message: String(err) } }))
    } finally {
      this.saving = false
      this.dragState = null
      this.pendingDragSave = null
    }
  }

  onSendInvitationsConfirm = async (sendUpdates: string) => {
    const pending = this.pendingDragSave
    if (!pending) return
    await this.performDragSave(pending.ds, pending.newStartUnix, pending.newEndUnix, sendUpdates)
  }

  onSendInvitationsCancel = () => {
    // User backed out of the move — revert the visual snap. Clearing
    // dragState makes the block render at block.instance's original
    // position (no commit happened, so the master event still has the
    // old times).
    this.saving = false
    this.dragState = null
    this.pendingDragSave = null
  }

  // Hover-cursor handler: switches between grab (body) and ns-resize
  // (top/bottom 6px zones) so the user can see which gesture a click+drag
  // will produce. Runs only when not actively dragging.
  onBlockHoverMove(block: TimedBlock, e: PointerEvent) {
    if (this.dragState) return
    if (block.instance.rruleText) return
    const btn = e.currentTarget as HTMLButtonElement
    const rect = btn.getBoundingClientRect()
    const relY = e.clientY - rect.top
    const mode = detectMode(relY, rect.height)
    const cursor = mode === 'move' ? 'grab' : 'ns-resize'
    if (btn.style.cursor !== cursor) {
      btn.style.cursor = cursor
    }
  }

  onBlockClick(inst: backend.EventInstance, e: MouseEvent) {
    // Drag-occurred click: synthetic click fires after pointerup; suppress
    // it so we don't accidentally open the detail view after a successful
    // drag. Reset the flag for the next interaction.
    if (this.wasDragged) {
      e.preventDefault()
      e.stopPropagation()
      this.wasDragged = false
      return
    }
    this.env.onEventClick(inst)
  }

  async refreshEventsAsync() {
    await events.fetchRange(
      calendarSources.visibleCalendarIDs,
      calendarView.visibleRange.fromUnix,
      calendarView.visibleRange.toUnix,
    )
  }
}
