import { differenceInCalendarDays, format, isToday, isYesterday, isThisWeek, isThisYear } from 'date-fns'
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'

/**
 * Format a date relative to now for message list display
 * - < 1 minute: "just now"
 * - < 1 hour: "Xm"
 * - < 24 hours: "Xh"
 * - Yesterday: "Yesterday"
 * - This week: "Monday", "Tuesday", etc.
 * - This year: "Dec 15"
 * - Older: "Dec 15, 2023"
 */
export function formatRelativeDate(date: Date): string {
  const t = get(_)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMinutes = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))

  if (diffMinutes < 1) {
    return t('date.justNow')
  }

  if (diffMinutes < 60) {
    return `${diffMinutes}m`
  }

  if (diffHours < 24 && isToday(date)) {
    return `${diffHours}h`
  }

  if (isYesterday(date)) {
    return t('date.yesterday')
  }

  if (isThisWeek(date)) {
    return format(date, 'EEEE')
  }

  if (isThisYear(date)) {
    return format(date, 'MMM d')
  }

  return format(date, 'MMM d, yyyy')
}

const timeFormat = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' })

/** Clock time per locale ("14:05" or "02:05 PM"), shared by chat list rows and bubbles. */
export function formatTime(date: Date): string {
  return timeFormat.format(date)
}

/**
 * Format a date for chat list rows
 * - Today: clock time (formatTime)
 * - Yesterday: "Yesterday"
 * - Within the last 7 days: weekday ("Monday")
 * - This year: "Dec 15"
 * - Older: "Dec 15, 2023"
 */
export function formatListDate(date: Date): string {
  if (isToday(date)) {
    return formatTime(date)
  }

  if (isYesterday(date)) {
    return get(_)('date.yesterday')
  }

  const daysAgo = differenceInCalendarDays(new Date(), date)
  if (daysAgo > 0 && daysAgo < 7) {
    return format(date, 'EEEE')
  }

  if (isThisYear(date)) {
    return format(date, 'MMM d')
  }

  return format(date, 'MMM d, yyyy')
}

/**
 * Format a date for message header display
 * Shows full date and time
 */
export function formatMessageDate(date: Date): string {
  const t = get(_)

  if (isToday(date)) {
    return t('date.todayAt', { values: { time: format(date, 'h:mm a') } })
  }

  if (isYesterday(date)) {
    return t('date.yesterdayAt', { values: { time: format(date, 'h:mm a') } })
  }

  if (isThisYear(date)) {
    return format(date, 'MMM d \'at\' h:mm a')
  }

  return format(date, 'MMM d, yyyy \'at\' h:mm a')
}

/**
 * Format a date for full display (tooltips, etc.)
 */
export function formatFullDate(date: Date): string {
  return format(date, 'EEEE, MMMM d, yyyy \'at\' h:mm:ss a')
}
