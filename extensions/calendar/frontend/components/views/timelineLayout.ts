// Layout math and shared types of TimelineView: all-day band lanes, hour-grid
// lane packing, drag geometry. Reads the events and calendarSources stores.
import { events } from '$extensions/calendar/frontend/stores/events.svelte'
import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
// @ts-ignore - wailsjs bindings
import type { backend } from '$wailsjs/go/models'

export const HOUR_PX = 48
export const DAY_PX = HOUR_PX * 24

export type TimedBlock = {
  instance: backend.EventInstance
  color: string
  topPct: number      // 0..100
  heightPct: number   // 0..100
  leftPct: number     // 0..100 (within day column)
  widthPct: number    // 0..100
}

export type BandBlock = {
  instance: backend.EventInstance
  color: string
  startColIdx: number   // first visible day
  endColIdx: number     // last visible day (inclusive)
  laneIdx: number       // vertical row in band
  continuesLeft: boolean
  continuesRight: boolean
}

/**
 * Only all-day events go in the all-day band. Timed events — including
 * multi-day timed events — go in the hour grid, where timedBlocksForDay
 * clips each instance to each day's [dayStart, dayEnd) window, so a
 * multi-day timed event renders as proper time blocks across days
 * (e.g. 2pm→midnight, full day, midnight→10am) instead of a flat all-day bar.
 */
export function isBandEvent(inst: backend.EventInstance): boolean {
  return inst.isAllDay
}

// Band events laid out across `dates`, packed into vertical lanes.
export function computeBandBlocks(dates: Date[]): BandBlock[] {
  if (dates.length === 0) return []
  const dayStartsUnix = dates.map(d => Math.floor(d.getTime() / 1000))
  const lastDayEndUnix = dayStartsUnix[dayStartsUnix.length - 1] + 86400

  // Collect overlapping band events for the visible window.
  const candidates: BandBlock[] = []
  for (const inst of events.instances) {
    if (!isBandEvent(inst)) continue
    if (inst.instanceEndUnix <= dayStartsUnix[0]) continue
    if (inst.instanceStartUnix >= lastDayEndUnix) continue

    let startColIdx = 0
    let endColIdx = dates.length - 1
    let continuesLeft = false
    let continuesRight = false

    // Find first visible day this event overlaps.
    for (let i = 0; i < dates.length; i++) {
      const dayEnd = dayStartsUnix[i] + 86400
      if (inst.instanceStartUnix < dayEnd) {
        startColIdx = i
        continuesLeft = inst.instanceStartUnix < dayStartsUnix[0]
        break
      }
    }
    // Find last visible day this event overlaps.
    for (let i = dates.length - 1; i >= 0; i--) {
      if (inst.instanceEndUnix > dayStartsUnix[i]) {
        endColIdx = i
        continuesRight = inst.instanceEndUnix > lastDayEndUnix
        break
      }
    }

    candidates.push({
      instance: inst,
      color: calendarSources.colorOf(inst.calendarId),
      startColIdx,
      endColIdx,
      laneIdx: 0,
      continuesLeft,
      continuesRight,
    })
  }

  // Sort by start col then by start time for stable lane assignment.
  candidates.sort((a, b) => {
    if (a.startColIdx !== b.startColIdx) return a.startColIdx - b.startColIdx
    return a.instance.instanceStartUnix - b.instance.instanceStartUnix
  })

  // Greedy lane assignment: per lane, track the rightmost-occupied col.
  const laneRightmostCol: number[] = []
  for (const block of candidates) {
    let assigned = -1
    for (let lane = 0; lane < laneRightmostCol.length; lane++) {
      if (laneRightmostCol[lane] < block.startColIdx) {
        assigned = lane
        break
      }
    }
    if (assigned < 0) {
      assigned = laneRightmostCol.length
      laneRightmostCol.push(-1)
    }
    block.laneIdx = assigned
    laneRightmostCol[assigned] = block.endColIdx
  }
  return candidates
}

// Per-column timed-event blocks (hour grid), with lane-packed collision.
export function timedBlocksForDay(date: Date): TimedBlock[] {
  const dayStartUnix = Math.floor(date.getTime() / 1000)
  const dayEndUnix = dayStartUnix + 86400

  const list: backend.EventInstance[] = []
  for (const inst of events.instances) {
    if (isBandEvent(inst)) continue
    if (inst.instanceEndUnix <= dayStartUnix) continue
    if (inst.instanceStartUnix >= dayEndUnix) continue
    list.push(inst)
  }
  if (list.length === 0) return []

  // Sort by start (ties: longer first → fatter events get earlier lanes).
  list.sort((a, b) => {
    if (a.instanceStartUnix !== b.instanceStartUnix) {
      return a.instanceStartUnix - b.instanceStartUnix
    }
    return b.instanceEndUnix - a.instanceEndUnix
  })

  // Greedy lane assignment by overlap.
  type LanedEvent = { inst: backend.EventInstance; lane: number }
  const laneEndUnix: number[] = []
  const laned: LanedEvent[] = []
  for (const inst of list) {
    let assigned = -1
    for (let lane = 0; lane < laneEndUnix.length; lane++) {
      if (laneEndUnix[lane] <= inst.instanceStartUnix) {
        assigned = lane
        break
      }
    }
    if (assigned < 0) {
      assigned = laneEndUnix.length
      laneEndUnix.push(0)
    }
    laned.push({ inst, lane: assigned })
    laneEndUnix[assigned] = Math.max(laneEndUnix[assigned], inst.instanceEndUnix)
  }

  // Compute laneCount per overlap-group (transitively-overlapping events).
  // Sweep through laned (which is start-sorted): when current event starts
  // after the running-max-end-of-group, close the group.
  const groupIdx: number[] = new Array(laned.length).fill(0)
  let currentGroup = 0
  let groupMaxEnd = 0
  for (let i = 0; i < laned.length; i++) {
    if (i === 0 || laned[i].inst.instanceStartUnix >= groupMaxEnd) {
      currentGroup = i === 0 ? 0 : currentGroup + 1
      groupMaxEnd = laned[i].inst.instanceEndUnix
    }
    groupIdx[i] = currentGroup
    if (laned[i].inst.instanceEndUnix > groupMaxEnd) {
      groupMaxEnd = laned[i].inst.instanceEndUnix
    }
  }
  const groupLaneCount: Record<number, number> = {}
  for (let i = 0; i < laned.length; i++) {
    const g = groupIdx[i]
    groupLaneCount[g] = Math.max(groupLaneCount[g] ?? 0, laned[i].lane + 1)
  }

  // Build TimedBlock array with %-based positioning.
  const out: TimedBlock[] = []
  for (let i = 0; i < laned.length; i++) {
    const { inst, lane } = laned[i]
    const startSec = Math.max(inst.instanceStartUnix, dayStartUnix)
    const endSec = Math.min(inst.instanceEndUnix, dayEndUnix)
    const startMin = (startSec - dayStartUnix) / 60
    const endMin = (endSec - dayStartUnix) / 60
    const lanes = groupLaneCount[groupIdx[i]]
    out.push({
      instance: inst,
      color: calendarSources.colorOf(inst.calendarId),
      topPct: (startMin / 1440) * 100,
      heightPct: Math.max(((endMin - startMin) / 1440) * 100, 1.2),
      leftPct: (lane / lanes) * 100,
      widthPct: (1 / lanes) * 100,
    })
  }
  return out
}

export type DragMode = 'move' | 'resize-start' | 'resize-end'

export type DragState = {
  instanceId: string
  eventId: string
  mode: DragMode
  originColIdx: number
  originStartUnix: number
  originEndUnix: number
  startClientX: number
  startClientY: number
  columnWidth: number
  currentColIdx: number
  currentDeltaMinutes: number
  movedPastThreshold: boolean
  // Attendee count decides whether the drop asks about sending updates.
  attendeeCount: number
  // sourceKind for the SendInvitationsDialog provider note.
  masterSourceKind: 'google' | 'microsoft' | 'caldav-server' | 'caldav-none' | 'local' | ''
}

// sourceKind for the dragged event's calendar — used by the dialog's
// provider note (matches EventComposerDialog's derivation logic).
export function sourceKindOf(calendarId: string): DragState['masterSourceKind'] {
  for (const src of calendarSources.sources) {
    const cals = calendarSources.calendarsBySource[src.id] || []
    for (const cal of cals) {
      if (cal.id !== calendarId) continue
      switch (src.type) {
        case 'google':    return 'google'
        case 'microsoft': return 'microsoft'
        case 'local':     return 'local'
        case 'caldav':    return src.itipMode === 'none' ? 'caldav-none' : 'caldav-server'
      }
      return ''
    }
  }
  return ''
}

export const RESIZE_GRIP_PX = 6
export const DRAG_THRESHOLD_PX = 4

export function detectMode(relY: number, height: number): DragMode {
  if (relY < RESIZE_GRIP_PX) return 'resize-start'
  if (relY > height - RESIZE_GRIP_PX) return 'resize-end'
  return 'move'
}

// Derived visual offsets for the block currently being dragged. Returns
// null when the block isn't the drag target OR the user hasn't moved
// past the threshold yet (no visual feedback below the click/drag
// disambiguation threshold).
export function dragOffsetsFor(
  ds: DragState | null,
  block: TimedBlock,
): { top: number; height: number; xPx: number } | null {
  if (!ds) return null
  if (ds.instanceId !== block.instance.id) return null
  if (!ds.movedPastThreshold) return null

  const minutePx = HOUR_PX / 60
  const deltaPx = ds.currentDeltaMinutes * minutePx

  // Base top/height from the block's current pct (relative to DAY_PX).
  const baseTopPx = (block.topPct / 100) * DAY_PX
  const baseHeightPx = (block.heightPct / 100) * DAY_PX

  let topPx = baseTopPx
  let heightPx = baseHeightPx
  let xPx = 0

  switch (ds.mode) {
    case 'move':
      topPx = baseTopPx + deltaPx
      xPx = (ds.currentColIdx - ds.originColIdx) * ds.columnWidth
      break
    case 'resize-start':
      topPx = baseTopPx + deltaPx
      heightPx = baseHeightPx - deltaPx
      if (heightPx < 14) {
        heightPx = 14
        topPx = baseTopPx + baseHeightPx - 14
      }
      break
    case 'resize-end':
      heightPx = baseHeightPx + deltaPx
      if (heightPx < 14) heightPx = 14
      break
  }

  return { top: topPx, height: heightPx, xPx }
}
