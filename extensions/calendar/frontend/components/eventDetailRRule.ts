// Recurrence-rule humanizing helpers for EventDetail. Recognizes the common
// RRULE shapes; unknown shapes fall through to the raw text.

export type Translate = (
  id: string,
  options?: { values?: Record<string, string | number> },
) => string

export function humanizeRRule(
rruleText: string,
t: Translate,
dateFmt: Intl.DateTimeFormat,
): { human: string; raw: string } {
  if (rruleText === '') return { human: '', raw: '' }
  const parts = parseRRule(rruleText)
  const freq = parts.FREQ
  let base: string
  if (freq === 'DAILY') {
    base = t('calendar.rrule.daily')
  } else if (freq === 'WEEKLY') {
    const days = parts.BYDAY ? humanizeByDay(parts.BYDAY) : ''
    base = days !== ''
      ? t('calendar.rrule.weeklyOn', { values: { days } })
      : t('calendar.rrule.weekly')
  } else if (freq === 'MONTHLY') {
    base = t('calendar.rrule.monthly')
  } else if (freq === 'YEARLY') {
    base = t('calendar.rrule.yearly')
  } else {
    return { human: '', raw: rruleText }
  }
  if (parts.UNTIL) {
    const untilDate = parseICSDate(parts.UNTIL)
    if (untilDate) {
      base += ' ' + t('calendar.rrule.until', { values: { date: dateFmt.format(untilDate) } })
    }
  }
  if (parts.COUNT) {
    base += ' ' + t('calendar.rrule.count', { values: { count: parts.COUNT } })
  }
  return { human: base, raw: rruleText }
}

function parseRRule(rrule: string): Record<string, string> {
  const out: Record<string, string> = {}
  // Strip optional "RRULE:" prefix; split on semicolons.
  const body = rrule.startsWith('RRULE:') ? rrule.slice(6) : rrule
  for (const segment of body.split(';')) {
    const eq = segment.indexOf('=')
    if (eq <= 0) continue
    out[segment.slice(0, eq).toUpperCase().trim()] = segment.slice(eq + 1).trim()
  }
  return out
}

function humanizeByDay(byDay: string): string {
  const map: Record<string, string> = {
    MO: 'Monday', TU: 'Tuesday', WE: 'Wednesday', TH: 'Thursday',
    FR: 'Friday', SA: 'Saturday', SU: 'Sunday',
  }
  const days = byDay.split(',')
    .map(d => d.replace(/^[+-]?\d+/, '').toUpperCase())
    .map(d => map[d] || d)
  return days.join(', ')
}

function parseICSDate(s: string): Date | null {
  // RFC 5545 DATE-TIME-UTC: 20251215T140000Z
  // Or DATE: 20251215
  const m = s.match(/^(\d{4})(\d{2})(\d{2})(T(\d{2})(\d{2})(\d{2})Z?)?$/)
  if (!m) return null
  const y = Number(m[1]), mo = Number(m[2]) - 1, d = Number(m[3])
  if (m[4] === undefined) return new Date(Date.UTC(y, mo, d))
  return new Date(Date.UTC(y, mo, d, Number(m[5]), Number(m[6]), Number(m[7])))
}
