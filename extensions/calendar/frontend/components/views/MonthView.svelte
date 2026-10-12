<script lang="ts">
  // MonthView — 6-row × 7-col grid showing the anchored month plus spill
  // days from adjacent months.
  //
  // Layout per week-row (a CSS grid with explicit rows):
  //   - Row 1: day-number header per cell (muted for other-month, circled today).
  //   - Rows 2..(1+laneCount): multi-day "band" bars that SPAN columns — a
  //     multi-day event renders as one continuous bar across its days (lane-
  //     packed, with ◀/▶ continuation markers where it crosses a week-row).
  //   - Last row (1fr): per-cell single-day event pills + "+N more" overflow.
  //     A cell's pills start right below the lanes its own column uses, so
  //     they also fill lanes that are empty on that day.
  // A background <button> per cell (spanning all rows, behind everything) keeps
  // the empty-area click → DayView navigation; EventCards stopPropagation so a
  // bar/pill click selects the event.

  import { _ } from 'svelte-i18n'
  import EventCard from '$extensions/calendar/frontend/components/EventCard.svelte'
  import { calendarView } from '$extensions/calendar/frontend/stores/calendarView.svelte'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
  import { events } from '$extensions/calendar/frontend/stores/events.svelte'
  import { toTzDate } from '$extensions/calendar/frontend/lib/tzMath'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'

  // Per-cell event count tracks the available cell height (#323). Sizes match
  // the rendered rows: PILL_PX = one EventCard (22px: an 18px text-xs line plus
  // py-0.5) plus the gap-0.5; MORE_PX = the "+N more" line; BAND_PX = one
  // multi-day lane; DAY_NUM_PX = the day-number header row; CELL_PAD_PX = the
  // pill column's pb-1 plus the row's bottom border.
  const PILL_PX = 24
  const PILL_GAP_PX = 2
  const MORE_PX = 15
  const BAND_PX = 22
  const DAY_NUM_PX = 28
  const CELL_PAD_PX = 5

  // Measured height of the 6-row grid (layout-driven, so no measure→render loop).
  // Rows are equal height, so each row gets gridHeight / 6.
  let gridHeight = $state(0)

  // Height left for single-day pills below a row's day numbers and bands.
  function pillSpace(laneCount: number): number {
    return gridHeight / 6 - DAY_NUM_PX - CELL_PAD_PX - laneCount * BAND_PX
  }

  // Pills to show in a cell: all of them when they fit, otherwise as many as
  // leave room for the "+N more" line.
  function visibleCount(total: number, space: number): number {
    if (total * PILL_PX - PILL_GAP_PX <= space) return total
    return Math.max(0, Math.floor((space - MORE_PX) / PILL_PX))
  }

  type Cell = { date: Date; isOtherMonth: boolean; isToday: boolean }

  type BandBlock = {
    instance: backend.EventInstance
    color: string
    startColIdx: number // first overlapped column in this week-row (0..6)
    endColIdx: number // last overlapped column (inclusive)
    laneIdx: number // vertical lane within the band
    continuesLeft: boolean // event started before this week-row
    continuesRight: boolean // event continues past this week-row
  }

  // laneCount = lanes in the whole week-row; cellLanes[i] = lanes that column i
  // actually uses, so its pills can move up into lanes left empty there.
  type BandRow = { blocks: BandBlock[]; laneCount: number; cellLanes: number[] }

  // 6 rows × 7 cols = 42 cells starting from the grid-start (in tz).
  const gridStart = $derived(calendarView.monthGridStart(calendarView.anchorDate))
  const anchorMonth = $derived(toTzDate(calendarView.anchorDate).getMonth())
  const today = $derived(calendarView.startOfDay(new Date()).getTime())

  const cells = $derived.by(() => {
    const out: Cell[] = []
    for (let i = 0; i < 42; i++) {
      const date = calendarView.addDays(gridStart, i)
      out.push({
        date,
        isOtherMonth: toTzDate(date).getMonth() !== anchorMonth,
        isToday: date.getTime() === today,
      })
    }
    return out
  })

  // An event is "multi-day" when it touches 2+ distinct grid days. The −1s on
  // the end makes an all-day event [1st 00:00, 2nd 00:00) count as single-day.
  function isMultiDay(inst: backend.EventInstance): boolean {
    const startDay = calendarView.startOfDay(new Date(inst.instanceStartUnix * 1000)).getTime()
    const endDay = calendarView.startOfDay(new Date((inst.instanceEndUnix - 1) * 1000)).getTime()
    return endDay > startDay
  }

  // Slice the 42 cells into 6 week-rows, each with precomputed day-start unix
  // timestamps + the row's end (exclusive).
  const weekRows = $derived.by(() => {
    const rows: { cells: Cell[]; dayStartsUnix: number[]; rowEndUnix: number }[] = []
    for (let w = 0; w < 6; w++) {
      const rowCells = cells.slice(w * 7, w * 7 + 7)
      const dayStartsUnix = rowCells.map(c => Math.floor(c.date.getTime() / 1000))
      rows.push({ cells: rowCells, dayStartsUnix, rowEndUnix: dayStartsUnix[6] + 86400 })
    }
    return rows
  })

  // Per week-row: multi-day events laid out as column-spanning bars, lane-packed.
  // Direct port of TimelineView's bandBlocks, scoped to each row's 7 columns —
  // so a week-crossing event segments into one bar per row automatically.
  const bandRows = $derived.by<BandRow[]>(() => {
    return weekRows.map(({ dayStartsUnix, rowEndUnix }) => {
      const candidates: BandBlock[] = []
      for (const inst of events.instances) {
        if (!isMultiDay(inst)) continue
        if (inst.instanceEndUnix <= dayStartsUnix[0]) continue
        if (inst.instanceStartUnix >= rowEndUnix) continue

        let startColIdx = 0
        let endColIdx = 6
        let continuesLeft = false
        let continuesRight = false
        for (let i = 0; i < 7; i++) {
          if (inst.instanceStartUnix < dayStartsUnix[i] + 86400) {
            startColIdx = i
            continuesLeft = inst.instanceStartUnix < dayStartsUnix[0]
            break
          }
        }
        for (let i = 6; i >= 0; i--) {
          if (inst.instanceEndUnix > dayStartsUnix[i]) {
            endColIdx = i
            continuesRight = inst.instanceEndUnix > rowEndUnix
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

      candidates.sort((a, b) => {
        if (a.startColIdx !== b.startColIdx) return a.startColIdx - b.startColIdx
        return a.instance.instanceStartUnix - b.instance.instanceStartUnix
      })

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

      const laneCount = candidates.length === 0 ? 0 : Math.max(...candidates.map(b => b.laneIdx)) + 1
      const cellLanes = Array.from({ length: 7 }, () => 0)
      for (const block of candidates) {
        for (let i = block.startColIdx; i <= block.endColIdx; i++) {
          cellLanes[i] = Math.max(cellLanes[i], block.laneIdx + 1)
        }
      }
      return { blocks: candidates, laneCount, cellLanes }
    })
  })

  // Single-day events only, bucketed per cell (multi-day events render as bands).
  const pillsByCell = $derived.by(() => {
    const out: backend.EventInstance[][] = Array.from({ length: 42 }, () => [])
    if (events.instances.length === 0) return out
    for (let i = 0; i < 42; i++) {
      const dayStart = cells[i].date.getTime() / 1000
      const dayEnd = dayStart + 86400
      for (const inst of events.instances) {
        if (isMultiDay(inst)) continue
        if (inst.instanceEndUnix > dayStart && inst.instanceStartUnix < dayEnd) {
          out[i].push(inst)
        }
      }
      out[i].sort((a, b) => a.instanceStartUnix - b.instanceStartUnix)
    }
    return out
  })

  const weekdayLabels = $derived([
    $_('calendar.month.weekdayShort.sun'),
    $_('calendar.month.weekdayShort.mon'),
    $_('calendar.month.weekdayShort.tue'),
    $_('calendar.month.weekdayShort.wed'),
    $_('calendar.month.weekdayShort.thu'),
    $_('calendar.month.weekdayShort.fri'),
    $_('calendar.month.weekdayShort.sat'),
  ])

  function onCellClick(date: Date) {
    calendarView.setViewKind('day')
    calendarView.setAnchorDate(date)
  }

  function onEventClick(inst: backend.EventInstance) {
    calendarView.selectEvent(inst.id, inst)
  }

  const noSources = $derived(calendarSources.sources.length === 0)
</script>

<div class="flex-1 flex flex-col min-h-0 bg-background">
  {#if noSources}
    <div class="flex-1 flex items-center justify-center text-muted-foreground text-sm px-6 text-center">
      {$_('calendar.month.emptyState')}
    </div>
  {/if}

  {#if !noSources}
    <!-- Weekday header row -->
    <div class="grid grid-cols-7 border-b border-border bg-muted/20">
      {#each weekdayLabels as label, i (i)}
        <div class="px-2 py-1 text-caption font-medium text-muted-foreground uppercase tracking-wide text-center">
          {label}
        </div>
      {/each}
    </div>

    <!-- 6 week-rows -->
    <div class="flex-1 grid grid-rows-6 min-h-0" bind:clientHeight={gridHeight}>
      {#each weekRows as row, w (w)}
        {@const band = bandRows[w]}
        <div
          class="relative grid grid-cols-7 border-b border-border min-h-0 overflow-hidden"
          style:grid-template-rows={`auto ${band.laneCount > 0 ? `repeat(${band.laneCount}, minmax(0, auto)) ` : ''}1fr`}
        >
          <!-- Background cell layer: borders, hover, empty-area click → DayView -->
          {#each row.cells as cell, i (i)}
            <button
              type="button"
              aria-label={String(toTzDate(cell.date).getDate())}
              class="border-r border-border min-h-0 hover:bg-muted/30 transition-colors
                     {cell.isOtherMonth ? 'bg-muted/10' : 'bg-background'}"
              style:grid-column={`${i + 1}`}
              style:grid-row="1 / -1"
              onclick={() => onCellClick(cell.date)}
            ></button>
          {/each}

          <!-- Day-number header layer (visual only — clicks pass through) -->
          {#each row.cells as cell, i (i)}
            <div
              class="pointer-events-none px-1 pt-1 pb-1 flex items-start justify-start"
              style:grid-column={`${i + 1}`}
              style:grid-row="1"
            >
              <span
                class="inline-flex items-center justify-center text-xs leading-none w-5 h-5 rounded-full
                       {cell.isToday ? 'bg-primary text-primary-foreground font-semibold' : ''}
                       {cell.isOtherMonth && !cell.isToday ? 'text-muted-foreground/50' : 'text-foreground'}"
              >
                {toTzDate(cell.date).getDate()}
              </span>
            </div>
          {/each}

          <!-- Multi-day spanning bars -->
          {#each band.blocks as block (block.instance.id + ':' + block.instance.instanceStartUnix)}
            <div
              class="pointer-events-auto px-1 min-w-0"
              style:grid-column={`${block.startColIdx + 1} / ${block.endColIdx + 2}`}
              style:grid-row={`${block.laneIdx + 2}`}
            >
              <EventCard
                instance={block.instance}
                color={block.color}
                continuesLeft={block.continuesLeft}
                continuesRight={block.continuesRight}
                onclick={() => onEventClick(block.instance)}
              />
            </div>
          {/each}

          <!-- Single-day pills + overflow, per cell -->
          {#each row.cells as cell, i (i)}
            {@const cellPills = pillsByCell[w * 7 + i]}
            {@const lanes = band.cellLanes[i]}
            {@const space = pillSpace(lanes)}
            {@const visible = cellPills.slice(0, visibleCount(cellPills.length, space))}
            {@const overflow = cellPills.length - visible.length}
            <div
              class="pointer-events-none flex flex-col gap-0.5 px-1 pb-1 min-h-0 overflow-hidden"
              style:grid-column={`${i + 1}`}
              style:grid-row={`${2 + lanes} / -1`}
            >
              {#each visible as inst (inst.id + ':' + inst.instanceStartUnix)}
                <div class="pointer-events-auto min-w-0">
                  <EventCard
                    instance={inst}
                    color={calendarSources.colorOf(inst.calendarId)}
                    onclick={() => onEventClick(inst)}
                  />
                </div>
              {/each}
              {#if overflow > 0}
                <span class="pointer-events-none text-2xs text-muted-foreground px-1">
                  {$_('calendar.month.moreEventsHidden', { values: { n: overflow } })}
                </span>
              {/if}
            </div>
          {/each}
        </div>
      {/each}
    </div>
  {/if}
</div>
