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
