// Relative times read like chat ("5 min ago"); the full time is on hover.

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

export function formatPortalAbsoluteTime(value: string) {
  return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

export function formatPortalRelativeTime(value: string | null | undefined, now = Date.now()) {
  if (!value) return ''
  const time = new Date(value).getTime()
  const elapsed = Math.max(0, now - time)
  if (elapsed < MINUTE) return 'just now'
  if (elapsed < HOUR) return `${Math.floor(elapsed / MINUTE)} min ago`
  if (elapsed < DAY) {
    const hours = Math.floor(elapsed / HOUR)
    return `${hours} ${hours === 1 ? 'hour' : 'hours'} ago`
  }
  if (elapsed < 2 * DAY) return 'yesterday'
  if (elapsed < 7 * DAY) return `${Math.floor(elapsed / DAY)} days ago`
  const date = new Date(time)
  const sameYear = date.getFullYear() === new Date(now).getFullYear()
  return date.toLocaleDateString(undefined, sameYear ? { month: 'short', day: 'numeric' } : { dateStyle: 'medium' })
}

/**
 * When a reply is due, in the reader's time zone: "today at 14:00", "Tuesday
 * at 9:00". Null once the time has passed, for a page left open: customers
 * never see a late notice.
 */
export function formatPortalReplyBy(value: string | null | undefined, now = new Date()): string | null {
  if (!value) return null
  const due = new Date(value)
  if (due.getTime() <= now.getTime()) return null
  const time = due.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const days = Math.round((startOfDay(due) - startOfDay(now)) / DAY)
  if (days <= 0) return `today at ${time}`
  if (days === 1) return `tomorrow at ${time}`
  if (days < 7) return `${due.toLocaleDateString(undefined, { weekday: 'long' })} at ${time}`
  return `${due.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} at ${time}`
}
