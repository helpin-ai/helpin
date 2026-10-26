import { differenceInCalendarDays, format } from 'date-fns'

/** Short snooze time: the time today, the weekday within a week, else the date. */
export function formatSnoozeUntil(until: Date, now: Date = new Date()): string {
  const days = differenceInCalendarDays(until, now)
  if (days === 0) return format(until, 'HH:mm')
  if (days > 0 && days < 7) return format(until, 'EEE HH:mm')
  return format(until, 'MMM d, HH:mm')
}
