// Reactive form state of EventComposerDialog. The dialog creates one
// EventComposerState; the markup binds its fields. Save/init logic lives here.
import { get } from 'svelte/store'
import { _ } from 'svelte-i18n'
import { toasts } from '$lib/stores/toast'
import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
import { calendarSettings } from '$extensions/calendar/frontend/stores/calendarSettings.svelte'
import type { SelectedOccurrence } from '$extensions/calendar/frontend/stores/calendarView.svelte'
import { fromZonedTime, toZonedTime } from 'date-fns-tz'
// @ts-ignore - wailsjs bindings
import { Calendar_CreateEvent, Calendar_UpdateEvent } from '$wailsjs/go/app/App.js'
import { accountStore } from '$lib/stores/accounts.svelte'
// @ts-ignore - wailsjs bindings
import { backend } from '$wailsjs/go/models'
import {
  formatYMD,
  formatHM,
  buildUnix,
  nextDayYMD,
  parseRRuleText,
  sourceKindForCalendar,
  type SourceKind,
} from './eventComposerLogic'

export type ComposerMode = 'create' | 'edit'

/** The dialog's props and close handler, read lazily so they stay reactive. */
export interface EventComposerEnv {
  readonly open: boolean
  readonly mode: ComposerMode
  readonly existing: backend.Event | null
  readonly occurrence: SelectedOccurrence | null
  readonly scope: 'this' | 'this-and-future' | 'all'
  readonly defaultStart: Date | null
  readonly defaultCalendarId: string
  readonly onSaved?: () => void
  close(): void
}

export class EventComposerState {
  private readonly env: EventComposerEnv

  constructor(env: EventComposerEnv) {
    this.env = env
  }

  summary = $state('')
  calendarId = $state('')
  isAllDay = $state(false)
  startDate = $state('')
  startTime = $state('')
  endDate = $state('')
  endTime = $state('')
  location = $state('')
  description = $state('')
  // descriptionHTML carries the rich-text body; description holds its plaintext
  // fallback (from the editor's getText()).
  descriptionHTML = $state('')
  // Seed for the rich-text editor. $derived (not $state) so it's correct
  // synchronously from `existing` BEFORE the editor mounts — the editor reads
  // it once in onMount; no reactive content-syncing.
  initialBody = $derived.by(() =>
    this.env.mode === 'edit' && this.env.existing
      ? this.env.existing.descriptionHTML || this.env.existing.description || ''
      : '',
  )
  transparency = $state('busy') // 'busy' | 'free'
  visibility = $state('public') // 'public' | 'private' | 'confidential'

  recurrenceFreq = $state('')
  recurrenceEnd = $state('never')
  recurrenceUntilDate = $state('')
  recurrenceCount = $state(10)

  reminderChoice = $state('none')
  reminderCustomMinutes = $state(15)

  // Recurrence controls as loaded for an edit. While they're unchanged the
  // backend keeps the event's own RRULE, which may carry parts (BYDAY,
  // INTERVAL) these controls don't show.
  loadedRecurrence = ''
  recurrenceKey = () =>
    [this.recurrenceFreq, this.recurrenceEnd, this.recurrenceUntilDate, this.recurrenceCount].join('|')

  // Phase C: attendees + organizer state. AttendeesSection binds these.
  attendees = $state<backend.AttendeeInput[]>([])
  organizer = $state<backend.OrganizerInput | null>(null)
  // sendUpdates is the value threaded into EventInput at save time. On
  // Create it's always "all" — users add attendees because they want
  // them invited. On Edit it's resolved via SendInvitationsDialog when
  // the event has attendees (Outlook's "Send update?" pattern).
  sendUpdates = $state<string>('all')

  // Edit-only confirmation dialog state.
  sendInvitationsOpen = $state(false)
  // Set per keydown: whether this Esc is closing a Select dropdown.
  escClosesDropdown = false

  submitting = $state(false)
  errorMessage = $state('')
  errorRef = $state<HTMLElement | null>(null)

  // Writable target calendars for create + edit. Gates by BOTH source-level
  // writability (e.g., a Google account with calendars.readonly scope is
  // false-source) AND per-calendar writability (Google Contacts Birthdays,
  // Nextcloud read-only shares, Microsoft canEdit=false). The per-calendar
  // check uses `cal.writable !== false` to stay permissive — older rows
  // from before migration v6 default to writable=true at the DB level.
  writableCalendars = $derived.by(() => {
    const out: { id: string; name: string }[] = []
    for (const src of calendarSources.sources) {
      if (!src.writable) continue
      for (const cal of calendarSources.calendarsBySource[src.id] || []) {
        if (cal.writable === false) continue
        out.push({ id: cal.id, name: cal.displayName })
      }
    }
    return out
  })

  // Self-emails for AttendeesSection's self-suggestion suppression +
  // downstream RSVP self-match. Union of mail account primaries and
  // every calendar source's organizer identities — so users whose
  // CalDAV identity isn't also a configured mail account still get
  // recognized as themselves (and the picker doesn't suggest them as
  // an invitee). Account aliases (Identity rows) require a separate
  // GetAllAccountIdentities call; deferred.
  selfEmails = $derived.by(() => {
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

  // Identity list for the AttendeesSection organizer picker. Sourced from
  // the picked calendar's owning source (NOT from the global mail account
  // list — see Phase I of the v0.3.0 plan). source.organizerIdentities is
  // populated at source-add time from provider discovery:
  //   - Google / Microsoft: the bound account's email (1 entry).
  //   - CalDAV: PROPFIND <C:calendar-user-address-set> or the user's
  //     manual entry from the setup / settings dialog (1+ entries).
  //   - Local: empty (attendees section is hidden entirely).
  //
  // Legacy fallback: pre-migration-v9 Google/Microsoft sources have
  // empty organizerIdentities. We derive the identity live by looking
  // up the bound account in accountStore so the composer keeps working
  // without forcing the user to re-add every source.
  identityOptions = $derived.by(() => {
    if (!this.calendarId) return [] as { email: string; commonName: string }[]
    for (const src of calendarSources.sources) {
      const cals = calendarSources.calendarsBySource[src.id] || []
      const owns = cals.some((c) => c.id === this.calendarId)
      if (!owns) continue
      const list = (src.organizerIdentities ?? []).filter((e) => e !== '')
      if (list.length > 0) {
        return list.map((email) => ({
          email,
          commonName: this.legacyCommonNameFor(src.accountId, email),
        }))
      }
      // Legacy fallback (no stored identities yet): live lookup against
      // accountStore. Only applies to Google/Microsoft — Local has no
      // identity by design, CalDAV without a populated list signals the
      // user must set one via CalendarSettingsDialog.
      if (src.type === 'google' || src.type === 'microsoft') {
        const acc = accountStore.accounts.find((aw) => aw.account?.id === src.accountId)
        const email = acc?.account?.email
        if (email) {
          return [{ email: email.toLowerCase(), commonName: acc?.account?.name || '' }]
        }
      }
      return []
    }
    return []
  })

  // Helper for the legacy fallback path: when the source's stored
  // identity list is populated, the email itself is authoritative — but
  // we still try to label it with the bound account's display name when
  // they match, so the dropdown reads "Alice <alice@…>" rather than
  // bare "<alice@…>".
  legacyCommonNameFor(accountId: string | undefined, email: string): string {
    if (!accountId) return ''
    const acc = accountStore.accounts.find((aw) => aw.account?.id === accountId)
    if (!acc?.account?.email) return ''
    if (acc.account.email.toLowerCase() !== email.toLowerCase()) return ''
    return acc.account.name || ''
  }

  // hasAttendeesCapability gates the entire attendees section: hidden
  // when there's no organizer identity to attribute the event to (Local
  // sources by design, or empty-CalDAV in the recovery window before
  // the user fills in an organizer email via CalendarSettingsDialog).
  hasAttendeesCapability = $derived(this.identityOptions.length > 0)

  // Phase H: day + duration derivations for "Find a time".
  // dayUnixForFreeBusy is midnight (user's tz) on startDate; the picker
  // covers 8-20 local; pickedDurationMinutes is the current event
  // duration so the picked slot calculates DTEND correctly.
  dayUnixForFreeBusy = $derived.by(() => {
    if (!this.startDate) return 0
    // startDate is "YYYY-MM-DD". Parse in the user's tz.
    const parts = this.startDate.split('-')
    if (parts.length !== 3) return 0
    const tz = calendarSettings.effectiveTimezone
    // Use fromZonedTime to anchor midnight local to the user's tz.
    const naive = new Date(
      parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]), 0, 0, 0, 0,
    )
    try {
      const utc = fromZonedTime(naive, tz)
      return Math.floor(utc.getTime() / 1000)
    } catch {
      return Math.floor(naive.getTime() / 1000)
    }
  })

  pickedDurationMinutes = $derived.by(() => {
    if (this.isAllDay) return 60
    if (!this.startTime || !this.endTime) return 60
    const [sh, sm] = this.startTime.split(':').map(Number)
    const [eh, em] = this.endTime.split(':').map(Number)
    if (isNaN(sh) || isNaN(eh)) return 60
    const mins = (eh * 60 + em) - (sh * 60 + sm)
    return mins > 0 ? mins : 60
  })

  applyPickedSlot = (startUnix: number, endUnix: number) => {
    const tz = calendarSettings.effectiveTimezone
    const startInTz = toZonedTime(new Date(startUnix * 1000), tz)
    const endInTz = toZonedTime(new Date(endUnix * 1000), tz)
    this.startDate = formatYMD(startInTz)
    this.startTime = formatHM(startInTz)
    this.endDate = formatYMD(endInTz)
    this.endTime = formatHM(endInTz)
    this.isAllDay = false
  }

  // sourceKind derived from the picked calendarId (see sourceKindForCalendar).
  sourceKind: SourceKind = $derived(sourceKindForCalendar(this.calendarId))

  initForm() {
    if (this.env.mode === 'edit' && this.env.existing) {
      this.initFromExisting(this.env.existing)
      return
    }
    this.initCreateDefaults()
  }

  initFromExisting(ev: backend.Event) {
    const tz = calendarSettings.effectiveTimezone
    this.calendarId = ev.calendarId
    this.summary = ev.summary || ''
    this.location = ev.location || ''
    this.description = ev.description || ''
    this.descriptionHTML = ev.descriptionHTML || ''
    this.isAllDay = !!ev.isAllDay
    const occurrence = this.env.occurrence
    const single = this.env.scope !== 'all' && occurrence
    const startInTz = toZonedTime(new Date((single ? occurrence.startUnix : ev.dtstartUnix) * 1000), tz)
    const endInTz = toZonedTime(new Date((single ? occurrence.endUnix : ev.dtendUnix) * 1000), tz)
    // Wire DTEND is exclusive (next-day midnight) for all-day events per
    // RFC 5545 §3.6.1 — subtract a day so the picker shows the inclusive
    // last day. If a legacy/zero-duration record snuck in (DTEND == DTSTART),
    // the subtract would render endDate before startDate; clamp to start.
    if (this.isAllDay) {
      endInTz.setDate(endInTz.getDate() - 1)
      if (endInTz.getTime() < startInTz.getTime()) {
        endInTz.setTime(startInTz.getTime())
      }
    }
    this.startDate = formatYMD(startInTz)
    this.startTime = formatHM(startInTz)
    this.endDate = formatYMD(endInTz)
    this.endTime = formatHM(endInTz)
    this.parseRRule(ev.rruleText || '')
    this.loadedRecurrence = this.recurrenceKey()
    this.transparency = ev.transparency || 'busy'
    this.visibility = ev.visibility || 'public'
    const rm = ev.reminderMinutes
    this.reminderChoice = rm == null ? 'none'
      : ['0', '5', '15', '30', '60', '1440'].includes(String(rm)) ? String(rm) : 'custom'
    if (this.reminderChoice === 'custom') this.reminderCustomMinutes = rm ?? 15
    // Attendees + organizer. backend.Attendee → backend.AttendeeInput
    // (shape-compatible; createFrom safely cherry-picks fields).
    this.attendees = (ev.attendees ?? []).map((a) =>
      backend.AttendeeInput.createFrom({
        email: a.email,
        cn: a.cn,
        partStat: a.partStat,
        role: a.role,
        rsvp: a.rsvp,
        cuType: a.cuType,
        delegate: a.delegate,
      }),
    )
    this.organizer = ev.organizer
      ? { email: ev.organizer.email, cn: ev.organizer.cn ?? '' }
      : null
  }

  initCreateDefaults() {
    const tz = calendarSettings.effectiveTimezone
    // Resolution order: caller-passed defaultCalendarId → stored global
    // default (validated writable by the store's stale-pruning getter) →
    // first writable calendar in the list. The user's manual Select choice
    // inside the composer overrides this on subsequent saves.
    this.calendarId =
      this.env.defaultCalendarId ||
      calendarSettings.globalDefaultCalendarId ||
      this.writableCalendars[0]?.id || ''
    const defaultStart = this.env.defaultStart
    const ref = defaultStart ?? new Date()
    const refInTz = toZonedTime(ref, tz)
    const isDefaultNow = defaultStart === null
    if (isDefaultNow) {
      const min = refInTz.getMinutes()
      if (min < 30) refInTz.setMinutes(30, 0, 0)
      if (min >= 30) {
        refInTz.setMinutes(0, 0, 0)
        refInTz.setHours(refInTz.getHours() + 1)
      }
    }
    const endRef = new Date(refInTz)
    endRef.setHours(endRef.getHours() + 1)
    this.summary = ''
    this.location = ''
    this.description = ''
    this.descriptionHTML = ''
    this.transparency = 'busy'
    this.visibility = 'public'
    this.isAllDay = false
    this.startDate = formatYMD(refInTz)
    this.startTime = formatHM(refInTz)
    this.endDate = formatYMD(endRef)
    this.endTime = formatHM(endRef)
    this.recurrenceFreq = ''
    this.recurrenceEnd = 'never'
    this.recurrenceUntilDate = ''
    this.recurrenceCount = 10
    this.loadedRecurrence = ''
    this.reminderChoice = 'none'
    this.reminderCustomMinutes = 15
    this.attendees = []
    this.sendUpdates = 'all'
    // Organizer is re-derived reactively from the calendar's source-bound
    // identity list by the dialog's organizer $effect — leave it null here
    // and let that effect settle it after calendarId is in place.
    this.organizer = null
  }

  parseRRule(text: string) {
    const r = parseRRuleText(text)
    this.recurrenceFreq = r.freq
    if (r.end !== undefined) this.recurrenceEnd = r.end
    if (r.untilDate !== undefined) this.recurrenceUntilDate = r.untilDate
    if (r.count !== undefined) this.recurrenceCount = r.count
  }

  reminderMinutes(): number {
    if (this.reminderChoice === 'custom') return this.reminderCustomMinutes
    if (this.reminderChoice === 'none') return -1
    const n = Number(this.reminderChoice)
    return Number.isFinite(n) ? n : -1
  }

  async handleSave() {
    const t = get(_)
    if (this.submitting) return
    this.errorMessage = ''
    if (!this.summary.trim()) {
      this.errorMessage = t('calendar.composer.errorSummaryRequired')
      return
    }
    if (!this.calendarId) {
      this.errorMessage = t('calendar.composer.errorCalendarRequired')
      return
    }
    if (!this.startDate) {
      this.errorMessage = t('calendar.composer.errorStartRequired')
      return
    }

    // Edit-flow: when the event has attendees, intercept the save with
    // the "Send update?" dialog. The dialog calls performSave(choice)
    // after the user picks; Cancel just returns to the composer.
    // Create-flow skips the dialog entirely — adding attendees on a new
    // event implies you want them invited.
    if (this.env.mode === 'edit' && this.env.existing && this.attendees.length > 0) {
      this.sendInvitationsOpen = true
      return
    }
    this.sendUpdates = 'all'
    await this.performSave()
  }

  async performSave() {
    const t = get(_)
    const dtstartUnix = buildUnix(this.startDate, this.startTime, this.isAllDay)
    // For all-day events, DTEND is exclusive (next-day midnight) per
    // RFC 5545 §3.6.1 — so a single-day event on June 5 has DTSTART:20260605
    // DTEND:20260606. The picker shows the *inclusive* last day; convert by
    // bumping by one calendar day at write time. Without this, single-day
    // all-day events were stored zero-duration, which several views (Month,
    // Day) silently filtered out via the strict-greater overlap check.
    const dtendUnix = this.isAllDay
      ? buildUnix(nextDayYMD(this.endDate || this.startDate), '', true)
      : buildUnix(this.endDate || this.startDate, this.endTime || this.startTime, false)
    if (dtendUnix < dtstartUnix) {
      this.errorMessage = t('calendar.composer.errorEndBeforeStart')
      return
    }

    const attendees = this.attendees
    const input = {
      calendarId: this.calendarId,
      summary: this.summary.trim(),
      description: this.description.trim() || undefined,
      descriptionHTML: this.descriptionHTML || undefined,
      location: this.location.trim() || undefined,
      dtstartUnix,
      dtendUnix,
      isAllDay: this.isAllDay || undefined,
      tz: this.isAllDay ? undefined : calendarSettings.effectiveTimezone,
      transparency: this.transparency,
      visibility: this.visibility,
      recurrence: this.buildRecurrenceSpec(),
      reminder: this.buildReminderSpec(),
      attendees: attendees.length > 0 ? attendees : undefined,
      organizer: attendees.length > 0 && this.organizer ? this.organizer : undefined,
      sendUpdates: attendees.length > 0 ? this.sendUpdates : undefined,
    } as backend.EventInput

    const { mode, existing, occurrence, scope } = this.env
    this.submitting = true
    try {
      if (mode === 'edit' && existing) {
        await Calendar_UpdateEvent(
          { eventId: existing.id, instanceUnix: occurrence?.recurrenceIdUnix, ...input } as backend.EventUpdateInput,
          scope,
        )
        toasts.success(t('calendar.composer.toastUpdated'))
      }
      if (mode !== 'edit' || !existing) {
        await Calendar_CreateEvent(input)
        toasts.success(t('calendar.composer.toastCreated'))
      }
      // Clear submitting before close() so the guard inside close()
      // (which blocks user-initiated cancels during a request) doesn't
      // short-circuit the auto-close.
      this.submitting = false
      this.env.onSaved?.()
      this.env.close()
    } catch (err) {
      this.errorMessage = (err as Error)?.message ?? String(err)
    } finally {
      this.submitting = false
    }
  }

  buildRecurrenceSpec(): backend.RecurrenceSpec | undefined {
    if (!this.recurrenceFreq) return undefined
    const spec = { freq: this.recurrenceFreq, keep: this.recurrenceKey() === this.loadedRecurrence } as backend.RecurrenceSpec
    if (this.recurrenceEnd === 'date' && this.recurrenceUntilDate) {
      spec.untilUnix = buildUnix(this.recurrenceUntilDate, '23:59', true)
    }
    if (this.recurrenceEnd === 'count' && this.recurrenceCount > 0) {
      spec.count = this.recurrenceCount
    }
    return spec
  }

  buildReminderSpec(): backend.ReminderSpec | undefined {
    const m = this.reminderMinutes()
    if (m < 0) return undefined
    return { offsetMinutes: m } as backend.ReminderSpec
  }
}
