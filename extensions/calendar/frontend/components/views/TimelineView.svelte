<script lang="ts">
  // TimelineView — shared N-column hour-grid used by WeekView (N=7) and
  // DayView (N=1). Renders:
  //   - Day header row (sticky)         : weekday + day number, "today" circle
  //   - All-day band (sticky)           : all-day events + multi-day timed events
  //   - Scrollable hour body (24 rows)  : timed single-day events with
  //                                       absolute positioning + lane-packed
  //                                       overlap collision rendering
  //   - Now-line                         : 1px destructive-colored line at the
  //                                       current minute in today's column,
  //                                       refreshed every 60s
  //
  // Click an event → calendarView.selectEvent(instance.id). Click an empty
  // spot in a day column → 15-min slot highlights and the event composer
  // opens with `defaultStart` snapped to that slot.
  //
  // Lane/geometry math: timelineLayout.ts; drag controller: timelineDrag.svelte.ts.

  import { onMount, onDestroy } from 'svelte'
  import { _ } from 'svelte-i18n'
  import EventCard from '$extensions/calendar/frontend/components/EventCard.svelte'
  import { eventChipFill } from '$extensions/calendar/frontend/lib/eventColor'
  import { getIsDarkActive } from '$lib/stores/theme.svelte'
  import { calendarView } from '$extensions/calendar/frontend/stores/calendarView.svelte'
  import { calendarSettings } from '$extensions/calendar/frontend/stores/calendarSettings.svelte'
  import { toTzDate, fromTzDate } from '$extensions/calendar/frontend/lib/tzMath'
  import SendInvitationsDialog from '$extensions/calendar/frontend/components/SendInvitationsDialog.svelte'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'
  import {
    HOUR_PX,
    DAY_PX,
    computeBandBlocks,
    timedBlocksForDay,
    dragOffsetsFor,
  } from './timelineLayout'
  import { TimelineDrag } from './timelineDrag.svelte'

  interface Props {
    /** Local-time dates to render, one per column. Length 1 for DayView, 7 for WeekView. */
    dates: Date[]
  }

  const { dates }: Props = $props()

  // Hour labels: pure 00..23 in the user's chosen tz. Use a UTC reference
  // date and format in UTC so the strings stay DST-safe — we only want the
  // hour-of-day labels for the column gutter, not real timed values.
  const hourLabels = $derived.by(() => {
    const fmt = new Intl.DateTimeFormat(undefined, {
      hour: '2-digit', hour12: false, timeZone: 'UTC',
    })
    const out: string[] = []
    for (let h = 0; h < 24; h++) {
      out.push(fmt.format(new Date(Date.UTC(2020, 0, 1, h, 0, 0))))
    }
    return out
  })

  const weekdayShort = $derived([
    $_('calendar.month.weekdayShort.sun'),
    $_('calendar.month.weekdayShort.mon'),
    $_('calendar.month.weekdayShort.tue'),
    $_('calendar.month.weekdayShort.wed'),
    $_('calendar.month.weekdayShort.thu'),
    $_('calendar.month.weekdayShort.fri'),
    $_('calendar.month.weekdayShort.sat'),
  ])

  function isSameDay(a: Date, b: Date): boolean {
    // Tz-aware: same calendar-day in the user's chosen display tz.
    const za = toTzDate(a)
    const zb = toTzDate(b)
    return za.getFullYear() === zb.getFullYear()
      && za.getMonth() === zb.getMonth()
      && za.getDate() === zb.getDate()
  }

  // Band events laid out across `dates`, packed into vertical lanes.
  const bandBlocks = $derived(computeBandBlocks(dates))

  const bandLaneCount = $derived(bandBlocks.length === 0
    ? 0
    : Math.max(...bandBlocks.map(b => b.laneIdx)) + 1)

  // Per-column timed-event blocks (hour grid), with lane-packed collision.
  const timedByDay = $derived(dates.map(d => timedBlocksForDay(d)))

  // --- Now-line ---------------------------------------------------------------

  let nowDate = $state(new Date())
  let nowTimer: ReturnType<typeof setInterval> | null = null

  onMount(() => {
    nowTimer = setInterval(() => { nowDate = new Date() }, 60_000)
  })
  onDestroy(() => {
    if (nowTimer !== null) clearInterval(nowTimer)
  })

  const nowMinutes = $derived.by(() => {
    const z = toTzDate(nowDate)
    return z.getHours() * 60 + z.getMinutes()
  })
  const nowTopPct = $derived((nowMinutes / 1440) * 100)
  const todayColIdx = $derived.by(() => {
    for (let i = 0; i < dates.length; i++) {
      if (isSameDay(dates[i], nowDate)) return i
    }
    return -1
  })

  // --- Initial scroll ---------------------------------------------------------

  let scrollRef = $state<HTMLDivElement | null>(null)

  onMount(() => {
    if (!scrollRef) return
    const zNow = toTzDate(new Date())
    let targetPx = HOUR_PX * 8 // default 8 AM
    if (todayColIdx >= 0 && zNow.getHours() >= 6) {
      const m = zNow.getHours() * 60 + zNow.getMinutes()
      targetPx = Math.max(0, (m * HOUR_PX / 60) - HOUR_PX)
    }
    scrollRef.scrollTop = targetPx
  })

  // Width of the hour body's vertical scrollbar. The header and all-day band
  // pad their right edge by it so their columns line up with the body's.
  let scrollbarPx = $state(0)

  $effect(() => {
    const el = scrollRef
    if (!el) return
    const measure = () => { scrollbarPx = el.offsetWidth - el.clientWidth }
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  })

  // --- Click handlers ---------------------------------------------------------

  function onEventClick(inst: backend.EventInstance) {
    calendarView.selectEvent(inst.id, inst)
  }

  // --- Click empty timeslot → open composer at 15-min snapped slot ------------
  //
  // The composer dialog itself is now mounted centrally in CalendarPane.
  // We keep the per-cell highlight state local because it's a TimelineView
  // visual artifact (the colored band on the clicked slot); it tears down
  // via a $effect that watches the global composer's open state.

  let highlightedSlot = $state<{ colIdx: number; startMinute: number } | null>(null)

  function onColumnClick(colIdx: number, e: MouseEvent) {
    // Event blocks are absolute-positioned <button> children of the column.
    // Skip if the click hit one — its own onclick handles event selection.
    const target = e.target as HTMLElement
    if (target.closest('button')) return

    const col = e.currentTarget as HTMLElement
    const rect = col.getBoundingClientRect()
    const localY = e.clientY - rect.top
    const minuteOfDay = (localY / DAY_PX) * 1440
    const snapped = Math.max(0, Math.min(1425, Math.floor(minuteOfDay / 15) * 15))

    // dates[colIdx] is midnight-in-tz as a real UTC Date. Round-trip through
    // toTzDate / fromTzDate so DST transitions stay correct.
    const wall = toTzDate(dates[colIdx])
    wall.setHours(Math.floor(snapped / 60), snapped % 60, 0, 0)
    const startUtc = fromTzDate(wall)

    highlightedSlot = { colIdx, startMinute: snapped }
    calendarView.requestNewEvent({ defaultStart: startUtc })
  }

  function clearHighlight() {
    highlightedSlot = null
  }

  // Composer closed (save or cancel) → drop the highlight. Initial pass
  // with composerOpen=false is a harmless no-op clear.
  $effect(() => {
    if (!calendarView.composerOpen) clearHighlight()
  })

  const drag = new TimelineDrag({
    get dates() { return dates },
    onEventClick,
  })
</script>

<svelte:window
  onpointermove={(e) => drag.onWindowPointerMove(e)}
  onpointerup={(e) => drag.onWindowPointerUp(e)}
  onpointercancel={(e) => drag.onWindowPointerUp(e)}
/>

<div class="flex-1 flex flex-col min-h-0 bg-surface-content">
  <!-- Day header row: weekday + day number per column -->
  <div
    class="grid border-b border-border bg-muted/20 shrink-0"
    style:grid-template-columns="60px repeat({dates.length}, 1fr)"
    style:padding-right={`${scrollbarPx}px`}
  >
    <div></div>
    {#each dates as date, i (i)}
      {@const isToday = isSameDay(date, new Date())}
      {@const zd = toTzDate(date)}
      <div class="px-2 py-1 text-center border-l border-border">
        <div class="text-caption font-medium text-muted-foreground uppercase tracking-wide">
          {weekdayShort[zd.getDay()]}
        </div>
        <div class="inline-flex items-center justify-center w-6 h-6 mt-0.5 text-sm
                    {isToday ? 'rounded-full bg-primary text-primary-foreground' : 'text-foreground'}">
          {zd.getDate()}
        </div>
      </div>
    {/each}
  </div>

  <!-- All-day band: row per lane, columns aligned with day columns -->
  {#if bandLaneCount > 0}
    <div
      class="grid border-b border-border bg-surface-content shrink-0 py-1 gap-y-0.5"
      style:grid-template-columns="60px repeat({dates.length}, 1fr)"
      style:padding-right={`${scrollbarPx}px`}
      style:grid-template-rows={`repeat(${bandLaneCount}, minmax(20px, auto))`}
    >
      <!-- Left gutter label -->
      <div
        class="text-2xs text-muted-foreground text-right pr-1 self-center"
        style:grid-row={`1 / span ${bandLaneCount}`}
      >
        {$_('calendar.timeline.allDay')}
      </div>
      {#each bandBlocks as block (block.instance.id)}
        <div
          class="px-0.5"
          style:grid-row={`${block.laneIdx + 1}`}
          style:grid-column={`${block.startColIdx + 2} / ${block.endColIdx + 3}`}
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
    </div>
  {/if}

  <!-- Scrollable hour body -->
  <div bind:this={scrollRef} class="flex-1 overflow-y-auto">
    <div
      class="grid relative"
      style:grid-template-columns="60px repeat({dates.length}, 1fr)"
      style:height={`${DAY_PX}px`}
    >
      <!-- Hour label column -->
      <div class="border-r border-border">
        {#each hourLabels as label, h (h)}
          <div
            class="flex items-start justify-end pr-1 text-2xs text-muted-foreground border-b border-border/40"
            style:height={`${HOUR_PX}px`}
          >
            <span class="-translate-y-1/2">{label}</span>
          </div>
        {/each}
      </div>

      <!-- Day columns -->
      {#each dates as date, colIdx (colIdx)}
        <div
          class="relative border-l border-border cursor-pointer"
          data-day-col={colIdx}
          onclick={(e) => onColumnClick(colIdx, e)}
          onkeydown={() => {}}
          role="button"
          tabindex="-1"
        >
          <!-- Hour gridlines -->
          {#each hourLabels as _label, h (h)}
            <div
              class="border-b border-border/40"
              style:height={`${HOUR_PX}px`}
            ></div>
          {/each}

          <!-- Click-to-create highlight: 15-min band at clicked slot -->
          {#if highlightedSlot?.colIdx === colIdx}
            <div
              class="pointer-events-none absolute left-0 right-0 bg-primary/15 border border-primary/40 rounded-sm"
              style:top={`${(highlightedSlot.startMinute / 1440) * 100}%`}
              style:height={`${(15 / 1440) * 100}%`}
            ></div>
          {/if}

          <!-- Timed event blocks (absolute, %-based vertical positioning).
               During drag, the targeted block switches to px-based top/height
               + an X translate so it can cross columns in WeekView. -->
          {#each timedByDay[colIdx] as block (block.instance.id)}
            {@const dragOff = dragOffsetsFor(drag.dragState, block)}
            {@const isRecurring = !!block.instance.rruleText}
            <!-- Under ~32px there is no room for two lines, so time and title share one. -->
            {@const oneLine = (dragOff ? dragOff.height : (block.heightPct / 100) * DAY_PX) < 32}
            <button
              type="button"
              class="absolute rounded text-2xs text-foreground text-left
                     px-1 overflow-hidden flex gap-1
                     hover:brightness-110 transition-[filter]"
              class:flex-col={!oneLine}
              class:items-center={oneLine}
              class:py-0.5={!oneLine}
              class:gap-0={!oneLine}
              class:cursor-pointer={isRecurring}
              class:cursor-grab={!isRecurring && !dragOff}
              class:cursor-grabbing={!isRecurring && dragOff && drag.dragState?.mode === 'move'}
              class:cursor-ns-resize={!isRecurring && dragOff && (drag.dragState?.mode === 'resize-start' || drag.dragState?.mode === 'resize-end')}
              style:top={dragOff ? `${dragOff.top}px` : `${block.topPct}%`}
              style:height={dragOff ? `${dragOff.height}px` : `${block.heightPct}%`}
              style:left={`calc(${block.leftPct}% + 2px)`}
              style:width={`calc(${block.widthPct}% - 4px)`}
              style:background-color={eventChipFill(block.color, getIsDarkActive())}
              style:border-left={`3px solid ${block.color}`}
              style:transform={dragOff ? `translateX(${dragOff.xPx}px)` : undefined}
              style:opacity={dragOff ? 0.85 : undefined}
              style:z-index={dragOff ? 50 : undefined}
              style:touch-action={isRecurring ? undefined : 'none'}
              title={isRecurring
                ? `${block.instance.summary} — ${$_('calendar.drag.recurringHint')}`
                : block.instance.summary}
              onpointerdown={(e) => drag.onBlockPointerDown(block, colIdx, e)}
              onpointermove={(e) => drag.onBlockHoverMove(block, e)}
              onclick={(e) => drag.onBlockClick(block.instance, e)}
            >
              <div class="shrink-0 font-mono text-2xs text-muted-foreground {oneLine ? 'leading-[13px]' : 'leading-tight'}">
                {new Intl.DateTimeFormat(undefined, {
                  hour: '2-digit', minute: '2-digit', hour12: false,
                  timeZone: calendarSettings.effectiveTimezone,
                }).format(new Date(block.instance.instanceStartUnix * 1000))}
              </div>
              <div class="min-w-0 max-w-full truncate {oneLine ? 'leading-[13px]' : 'leading-tight'}">
                {block.instance.summary || $_('calendar.detail.noTitle')}
              </div>
            </button>
          {/each}

          <!-- Now-line, only in today's column -->
          {#if colIdx === todayColIdx}
            <div
              class="absolute left-0 right-0 z-10 pointer-events-none"
              style:top={`${nowTopPct}%`}
              aria-label={$_('calendar.timeline.nowLabel')}
            >
              <div class="h-px bg-destructive"></div>
              <div class="absolute -left-1 -top-1 w-2 h-2 rounded-full bg-destructive"></div>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>

<SendInvitationsDialog
  bind:open={drag.sendInvitationsOpen}
  attendeeCount={drag.pendingDragSave?.ds.attendeeCount ?? 0}
  sourceKind={drag.pendingDragSave?.ds.masterSourceKind ?? ''}
  onConfirm={drag.onSendInvitationsConfirm}
  onCancel={drag.onSendInvitationsCancel}
/>