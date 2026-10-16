// Pure helpers of EventComposerDialog: date/time formatting, unix conversion,
// and RRULE parsing. No reactive state.
import { fromZonedTime } from 'date-fns-tz'
import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
import { calendarSettings } from '$extensions/calendar/frontend/stores/calendarSettings.svelte'

export function formatYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function formatHM(d: Date): string {
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function buildUnix(dateStr: string, timeStr: string, allDay: boolean): number {
  const [y, m, d] = dateStr.split('-').map(Number)
  let hh = 0
  let mm = 0
  if (!allDay) {
    const tparts = (timeStr || '00:00').split(':').map(Number)
    hh = tparts[0] || 0
    mm = tparts[1] || 0
  }
  const wall = new Date(y, (m || 1) - 1, d || 1, hh, mm, 0, 0)
  const utc = fromZonedTime(wall, calendarSettings.effectiveTimezone)
  return Math.floor(utc.getTime() / 1000)
}

// nextDayYMD bumps a YYYY-MM-DD string by one calendar day, with Date
// doing the month/year-rollover arithmetic. Used by performSave to write
// RFC 5545 §3.6.1-correct DTEND for all-day events (exclusive next-day
// midnight). Calendar-date math only — no seconds, no DST involvement.
export function nextDayYMD(ymd: string): string {
  const [y, m, d] = ymd.split('-').map(Number)
  return formatYMD(new Date(y, (m || 1) - 1, (d || 1) + 1))
}

/** Recurrence fields an RRULE sets; absent keys leave the form value untouched. */
export interface ParsedRRule {
  freq: string
  end?: string
  untilDate?: string
  count?: number
}

export function parseRRuleText(text: string): ParsedRRule {
  if (!text) return { freq: '' }
  const body = text.startsWith('RRULE:') ? text.slice(6) : text
  const parts: Record<string, string> = {}
  for (const seg of body.split(';')) {
    const eq = seg.indexOf('=')
    if (eq <= 0) continue
    parts[seg.slice(0, eq).toUpperCase()] = seg.slice(eq + 1)
  }
  const out: ParsedRRule = { freq: parts.FREQ || '' }
  if (parts.UNTIL) {
    out.end = 'date'
    const m = parts.UNTIL.match(/^(\d{4})(\d{2})(\d{2})/)
    if (m) out.untilDate = `${m[1]}-${m[2]}-${m[3]}`
    return out
  }
  if (parts.COUNT) {
    out.end = 'count'
    const n = Number(parts.COUNT)
    if (Number.isFinite(n) && n > 0) out.count = n
    return out
  }
  out.end = 'never'
  return out
}

export type SourceKind = 'google' | 'microsoft' | 'caldav-server' | 'caldav-none' | 'local' | ''

// sourceKind of the picked calendar's owning source.
// Routes through calendarSources's existing lookups. For CalDAV we
// refine by the source's `itipMode` so the AttendeesSection toggle's
// gating reflects probe results from Phase E.
export function sourceKindForCalendar(calendarId: string): SourceKind {
  if (!calendarId) return ''
  for (const src of calendarSources.sources) {
    const cals = calendarSources.calendarsBySource[src.id] || []
    for (const cal of cals) {
      if (cal.id !== calendarId) continue
      switch (src.type) {
        case 'google':
          return 'google'
        case 'microsoft':
          return 'microsoft'
        case 'local':
          return 'local'
        case 'caldav':
          return src.itipMode === 'none' ? 'caldav-none' : 'caldav-server'
      }
      return ''
    }
  }
  return ''
}
