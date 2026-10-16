<script lang="ts">
  // EventDetail — the read-only body content rendered inside DetailOverlay
  // when an event is selected. Fetches the event via Calendar_GetEvent
  // whenever eventId changes; renders a labeled vertical key/value layout
  // sized for the ~340px sidebar overlay.

  import { _, locale } from 'svelte-i18n'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
  import { calendarSettings } from '$extensions/calendar/frontend/stores/calendarSettings.svelte'
  import { toTzDate } from '$extensions/calendar/frontend/lib/tzMath'
  import { calendarView, type SelectedOccurrence } from '$extensions/calendar/frontend/stores/calendarView.svelte'
  import { events } from '$extensions/calendar/frontend/stores/events.svelte'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'
  import ConfirmDialog from '$lib/components/kit/ConfirmDialog.svelte'
  import EventComposerDialog from './EventComposerDialog.svelte'
  import RecurrenceScopeDialog from './RecurrenceScopeDialog.svelte'
  import Linkified from './Linkified.svelte'
  import AttendeeListDisplay from './AttendeeListDisplay.svelte'
  import RSVPControls from './RSVPControls.svelte'
  import EventDetailAbout from './EventDetailAbout.svelte'
  import { humanizeRRule } from './eventDetailRRule'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { Calendar_DeleteEvent } from '$wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs bindings
  import { Calendar_GetEvent } from '$wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'

  interface Props {
    eventId: string | null
    occurrence?: SelectedOccurrence | null
  }

  let { eventId, occurrence = null }: Props = $props()

  let event = $state<backend.Event | null>(null)
  let loading = $state(false)
  let loadError = $state<string | null>(null)

  // Refetch when eventId changes. Null id → clear local state.
  $effect(() => {
    const id = eventId
    if (id === null || id === '') {
      event = null
      loadError = null
      loading = false
      return
    }
    loading = true
    loadError = null
    Calendar_GetEvent(id)
      .then((ev: backend.Event) => {
        // Drop result if a newer fetch superseded us mid-flight.
        if (eventId !== id) return
        event = ev ?? null
      })
      .catch((err: unknown) => {
        if (eventId !== id) return
        loadError = err instanceof Error ? err.message : String(err)
      })
      .finally(() => {
        if (eventId === id) loading = false
      })
  })

  // Calendar + source labels for the header. Sources store is already loaded
  // by CalendarPane on mount — no need to refetch here.
  const calendarInfo = $derived.by(() => {
    if (!event) return null
    for (const src of calendarSources.sources) {
      const cals = calendarSources.calendarsBySource[src.id] || []
      for (const cal of cals) {
        if (cal.id === event.calendarId) {
          return { source: src, calendar: cal }
        }
      }
    }
    return null
  })

  const color = $derived(event ? calendarSources.colorOf(event.calendarId) : '#999999')

  // Self-emails for AttendeeListDisplay "(you)" suffix + RSVP self-match.
  // Same predicate the backend's UpdateMyAttendeeStatus uses on its side.
  //
  // Union of two sources:
  //   - Every configured Mailven mail account's primary email.
  //   - Every calendar source's organizer identities (from
  //     organizerIdentities populated at source-add time).
  //
  // The second part is critical for users whose CalDAV principal email
  // isn't also a configured mail account (e.g., Nextcloud-only setups,
  // or the user runs Mail under one address but the calendar under
  // another). Without it, an event imported via the email-client's
  // "Add to calendar" lands with the user as an attendee that the
  // composer doesn't recognize → no RSVP options surface.
  //
  // Account aliases (Identity rows on AccountIdentityGroup) still
  // require a separate GetAllAccountIdentities call; deferred — the
  // calendar-source coverage closes the user-reported gap.
  const selfEmails = $derived.by(() => {
    const seen = new Set<string>()
    const out: string[] = []
    const push = (raw: string | undefined) => {
      const v = (raw || '').toLowerCase().trim()
      if (v === '' || seen.has(v)) return
      seen.add(v)
      out.push(v)
    }
    for (const aw of accountStore.accounts) {
      push(aw.account?.email)
    }
    for (const src of calendarSources.sources) {
      for (const email of src.organizerIdentities ?? []) {
        push(email)
      }
    }
    return out
  })

  const selfAttendee = $derived.by(() => {
    if (!event?.attendees) return null
    const lowerSet = new Set(selfEmails)
    for (const a of event.attendees) {
      if (lowerSet.has((a.email || '').toLowerCase())) return a
    }
    return null
  })

  // Refresh the detail pane after a successful RSVP so the user sees their
  // new PartStat in AttendeeListDisplay without close+reopen.
  function onRSVPed() {
    if (!eventId) return
    Calendar_GetEvent(eventId)
      .then((ev: backend.Event) => {
        event = ev ?? null
      })
      .catch((err: unknown) => {
        loadError = err instanceof Error ? err.message : String(err)
      })
  }

  // Locale-aware AND tz-aware formatters: locale via svelte-i18n's $locale,
  // timezone via the user's chosen display timezone.
  const dateFmt = $derived(new Intl.DateTimeFormat($locale || undefined, {
    weekday: 'short', year: 'numeric', month: 'short', day: 'numeric',
    timeZone: calendarSettings.effectiveTimezone,
  }))
  const timeFmt = $derived(new Intl.DateTimeFormat($locale || undefined, {
    hour: '2-digit', minute: '2-digit',
    timeZone: calendarSettings.effectiveTimezone,
  }))

  const whenLabel = $derived.by(() => {
    if (!event) return ''
    const start = new Date(event.dtstartUnix * 1000)
    const end = new Date(event.dtendUnix * 1000)
    if (event.isAllDay) {
      return `${dateFmt.format(start)} (${$_('calendar.detail.allDay')})`
    }
    // Sameness check is tz-aware: same local-day in the user's chosen tz.
    const sameDay = toTzDate(start).toDateString() === toTzDate(end).toDateString()
    if (sameDay) {
      return `${dateFmt.format(start)} · ${timeFmt.format(start)} – ${timeFmt.format(end)}`
    }
    return `${dateFmt.format(start)} ${timeFmt.format(start)} → ${dateFmt.format(end)} ${timeFmt.format(end)}`
  })

  // Recurrence humanizer. Recognizes the common shapes; unknown shapes
  // fall through to the raw RRULE text in mono.
  const repeatsLabel = $derived.by(() => humanizeRRule(event?.rruleText ?? '', $_, dateFmt))

  // Last-sync relative label for the calendar.
  const lastSyncLabel = $derived.by(() => {
    const last = calendarInfo?.calendar?.lastSyncedAt ?? 0
    if (last === 0) return $_('calendar.detail.lastSyncNever')
    const elapsed = Math.floor(Date.now() / 1000) - last
    if (elapsed < 60) return $_('calendar.detail.lastSync', { values: { time: 'just now' } })
    if (elapsed < 3600) return $_('calendar.detail.lastSync', { values: { time: `${Math.floor(elapsed / 60)}m ago` } })
    if (elapsed < 86400) return $_('calendar.detail.lastSync', { values: { time: `${Math.floor(elapsed / 3600)}h ago` } })
    return $_('calendar.detail.lastSync', { values: { time: `${Math.floor(elapsed / 86400)}d ago` } })
  })

  // --- Edit / Delete (writable sources only) ---------------------------------
  // Local sources are always writable. CalDAV flips to writable after first
  // sync (or at add time for new sources). Google/Microsoft providers in
  // future chunks set the flag per accessRole / canEdit.

  const isWritable = $derived.by(() => {
    if (!event) return false
    return calendarSources.isWritable(event.calendarId)
  })

  const isRecurring = $derived(!!event?.rruleText && event.rruleText !== '')

  let showComposer = $state(false)
  let composerScope = $state<'this' | 'this-and-future' | 'all'>('all')
  let showConfirmDelete = $state(false)
  let deleting = $state(false)
  let showScopeDialog = $state(false)
  let scopeAction = $state<'edit' | 'delete'>('edit')

  function startEdit() {
    if (!event) return
    if (isRecurring) {
      scopeAction = 'edit'
      showScopeDialog = true
      return
    }
    composerScope = 'all'
    showComposer = true
  }

  function startDelete() {
    if (!event) return
    if (isRecurring) {
      scopeAction = 'delete'
      showScopeDialog = true
      return
    }
    showConfirmDelete = true
  }

  function onScopePicked(scope: 'this' | 'this-and-future' | 'all') {
    showScopeDialog = false
    if (scopeAction === 'edit') {
      composerScope = scope
      showComposer = true
      return
    }
    composerScope = scope
    showConfirmDelete = true
  }

  async function performDelete() {
    if (!event) return
    deleting = true
    try {
      await Calendar_DeleteEvent(event.id, composerScope, occurrence?.recurrenceIdUnix ?? 0)
      toasts.success($_('calendar.composer.toastDeleted'))
      // Refresh and close overlay.
      void events.fetchRange(
        calendarSources.visibleCalendarIDs,
        calendarView.visibleRange.fromUnix,
        calendarView.visibleRange.toUnix,
      )
      calendarView.selectEvent(null)
    } catch (err) {
      toasts.error((err as Error)?.message ?? String(err))
    } finally {
      deleting = false
      showConfirmDelete = false
    }
  }

  function onComposerSaved() {
    // Refresh the visible window so the edit is reflected in the list view.
    void events.fetchRange(
      calendarSources.visibleCalendarIDs,
      calendarView.visibleRange.fromUnix,
      calendarView.visibleRange.toUnix,
    )
    // Refresh THIS pane too. The load $effect above only re-fetches when
    // eventId changes — editing the same event keeps eventId constant, so
    // the displayed `event` state would stay pre-edit unless we explicitly
    // re-query here.
    if (eventId) {
      Calendar_GetEvent(eventId)
        .then((ev: backend.Event) => {
          event = ev ?? null
        })
        .catch((err: unknown) => {
          loadError = err instanceof Error ? err.message : String(err)
        })
    }
  }
</script>

{#if loading}
  <div class="p-4 text-sm text-muted-foreground">
    {$_('calendar.common.loading')}
  </div>
{/if}

{#if loadError !== null}
  <div class="p-4 text-sm text-destructive">{loadError}</div>
{/if}

{#if event && !loading && loadError === null}
  <div class="p-4 space-y-4">
    <!-- Header: summary + calendar color tag -->
    <div>
      <h1 class="text-base font-semibold text-foreground wrap-break-word">
        {#if event.summary}
          <Linkified text={event.summary} />
        {/if}
        {#if !event.summary}
          {$_('calendar.detail.noTitle')}
        {/if}
      </h1>
      <div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
        <span
          class="inline-block w-2.5 h-2.5 rounded-full shrink-0"
          style:background-color={color}
          aria-hidden="true"
        ></span>
        <span class="truncate flex-1">
          {calendarInfo?.source.name ?? ''} / {calendarInfo?.calendar.displayName ?? ''}
        </span>
      </div>

      {#if isWritable}
        <div class="flex items-center gap-2 mt-3">
          <Button variant="outline" size="sm" onclick={startEdit}>
            <Icon icon="mdi:pencil" class="w-3.5 h-3.5 mr-1" />
            {$_('calendar.detail.editButton')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="text-destructive hover:text-destructive"
            onclick={startDelete}
          >
            <Icon icon="mdi:delete-outline" class="w-3.5 h-3.5 mr-1" />
            {$_('calendar.detail.deleteButton')}
          </Button>
        </div>
      {/if}
    </div>

    <div class="space-y-3 text-sm">
      <!-- When -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.detail.whenLabel')}
        </div>
        <div class="text-foreground wrap-break-word">{whenLabel}</div>
        {#if !event.isAllDay && event.tzName && event.tzName !== calendarSettings.effectiveTimezone}
          <div class="text-xs text-muted-foreground mt-0.5">{calendarSettings.effectiveTimezone}</div>
        {/if}
      </div>

      <!-- Where (skip if empty) -->
      {#if event.location && event.location !== ''}
        <div>
          <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
            {$_('calendar.detail.whereLabel')}
          </div>
          <div class="text-foreground wrap-break-word">
            <Linkified text={event.location} />
          </div>
        </div>
      {/if}

      <!-- Availability (Busy/Free) — always shown. -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.composer.availabilityLabel')}
        </div>
        <div class="text-foreground">
          {$_('calendar.composer.availability.' + (event.transparency === 'free' ? 'free' : 'busy'))}
        </div>
      </div>

      <!-- Visibility (Public/Private/Confidential) — always shown. -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.composer.visibilityLabel')}
        </div>
        <div class="text-foreground">
          {$_('calendar.composer.visibility.' + (event.visibility || 'public'))}
        </div>
      </div>

      <!-- Repeats (skip if non-recurring) -->
      {#if event.rruleText && event.rruleText !== ''}
        <div>
          <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
            {$_('calendar.detail.repeatsLabel')}
          </div>
          {#if repeatsLabel.human !== ''}
            <div class="text-foreground wrap-break-word">{repeatsLabel.human}</div>
          {/if}
          {#if repeatsLabel.human === '' && repeatsLabel.raw !== ''}
            <div class="text-foreground break-all text-xs font-mono">{repeatsLabel.raw}</div>
          {/if}
        </div>
      {/if}

      <!-- About — one field, two render modes. HTML bodies (Exchange/Graph
           rich text) are sanitized host-side and rendered as HTML. Plaintext
           bodies render via Linkified (clickable URLs through the hardened
           Calendar_OpenURL resolver) inside a whitespace-pre-wrap wrapper so
           newlines survive. Guard blocks are mutually exclusive. -->
      {#if event.descriptionHTML && event.descriptionHTML !== ''}
        <EventDetailAbout html={event.descriptionHTML} />
      {/if}
      {#if (!event.descriptionHTML || event.descriptionHTML === '') && event.description}
        <div>
          <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
            {$_('calendar.detail.aboutLabel')}
          </div>
          <div class="text-foreground wrap-break-word text-sm whitespace-pre-wrap">
            <Linkified text={event.description} />
          </div>
        </div>
      {/if}

      <!-- Attendees + organizer (Phase D) -->
      {#if (event.attendees && event.attendees.length > 0) || event.organizer}
        <div>
          <AttendeeListDisplay
            attendees={event.attendees ?? []}
            organizer={event.organizer ?? null}
            selfEmails={selfEmails}
          />
          {#if selfAttendee}
            <div class="mt-3">
              <div class="text-xs uppercase tracking-wide text-muted-foreground mb-1.5">
                {$_('calendar.attendees.yourResponse')}
              </div>
              <RSVPControls
                eventId={event.id}
                currentPartStat={selfAttendee.partStat}
                selfEmails={selfEmails}
                onUpdated={onRSVPed}
              />
            </div>
          {/if}
        </div>
      {/if}

      <!-- Calendar -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.detail.calendarLabel')}
        </div>
        <div class="text-foreground wrap-break-word">
          {calendarInfo?.source.name ?? ''} / {calendarInfo?.calendar.displayName ?? ''}
        </div>
      </div>

      <!-- UID (debug-y; small mono) -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.detail.uidLabel')}
        </div>
        <div class="text-xs text-muted-foreground font-mono break-all">{event.uid}</div>
      </div>

      <!-- Last sync -->
      <div>
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-0.5">
          {$_('calendar.detail.lastSyncLabel')}
        </div>
        <div class="text-xs text-muted-foreground">{lastSyncLabel}</div>
      </div>
    </div>
  </div>
{/if}

<EventComposerDialog
  bind:open={showComposer}
  mode="edit"
  existing={event}
  {occurrence}
  scope={composerScope}
  onSaved={onComposerSaved}
/>

<RecurrenceScopeDialog
  bind:open={showScopeDialog}
  action={scopeAction}
  onPicked={onScopePicked}
/>

<ConfirmDialog
  bind:open={showConfirmDelete}
  title={$_('calendar.composer.deleteConfirmTitle')}
  description={$_('calendar.composer.deleteConfirmDescription')}
  confirmLabel={$_('calendar.common.delete')}
  cancelLabel={$_('calendar.common.cancel')}
  variant="destructive"
  loading={deleting}
  onConfirm={performDelete}
/>
