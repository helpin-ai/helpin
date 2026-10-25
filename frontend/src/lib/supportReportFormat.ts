import type { SupportReportParams } from './supportReportTypes'

export type SupportReportRangePreset = '7d' | '30d' | '90d' | 'month'

export const SUPPORT_REPORT_RANGE_OPTIONS: { value: SupportReportRangePreset; label: string }[] = [
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
  { value: 'month', label: 'This month' },
]

function startOfLocalDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

/**
 * The preset's period, from a local midnight to now. Ending at now (not the
 * end of today) keeps the comparison with the previous, equally long period
 * fair while today is still in progress.
 */
export function supportReportRange(preset: SupportReportRangePreset, now = new Date()): { from: Date; to: Date } {
  const to = new Date(now)
  to.setSeconds(0, 0)
  if (preset === 'month') {
    return { from: new Date(now.getFullYear(), now.getMonth(), 1), to }
  }
  const days = preset === '7d' ? 7 : preset === '30d' ? 30 : 90
  const from = startOfLocalDay(now)
  from.setDate(from.getDate() - (days - 1))
  return { from, to }
}

export function supportReportParams(preset: SupportReportRangePreset, mailboxIds: string[], now = new Date()): SupportReportParams {
  const { from, to } = supportReportRange(preset, now)
  let tz = 'UTC'
  try {
    tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    // Fall back to UTC when the runtime has no timezone data.
  }
  return { from: from.toISOString(), to: to.toISOString(), tz, mailboxIds }
}

/** Formats seconds as a compact duration: 45s, 12m, 2h 5m, 3d 4h. */
export function formatReportDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds)) return '—'
  const total = Math.max(0, Math.round(seconds))
  if (total < 60) return `${total}s`
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  if (hours < 24) return remainingMinutes ? `${hours}h ${remainingMinutes}m` : `${hours}h`
  const days = Math.floor(hours / 24)
  const remainingHours = hours % 24
  return remainingHours ? `${days}d ${remainingHours}h` : `${days}d`
}

export function formatReportPercent(rate: number | null | undefined): string {
  if (rate === null || rate === undefined || !Number.isFinite(rate)) return '—'
  return `${Math.round(rate * 100)}%`
}

export function formatReportRating(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '—'
  return value.toFixed(1)
}

export type ReportTrend = { label: string; tone: 'positive' | 'negative' | 'neutral' } | null

/**
 * Describes the change from the previous period. `better` says which
 * direction is an improvement; omit it for volumes, which are neither.
 */
export function describeReportTrend(
  current: number | null | undefined,
  previous: number | null | undefined,
  options: { better?: 'up' | 'down'; kind?: 'count' | 'rate' | 'duration' | 'rating' } = {},
): ReportTrend {
  if (current === null || current === undefined || previous === null || previous === undefined) return null
  const delta = current - previous
  if (delta === 0) return { label: 'No change', tone: 'neutral' }
  let label: string
  switch (options.kind) {
    case 'rate':
      label = `${delta > 0 ? '+' : '−'}${Math.abs(Math.round(delta * 100))} pts`
      break
    case 'duration':
      label = `${delta > 0 ? '+' : '−'}${formatReportDuration(Math.abs(delta))}`
      break
    case 'rating':
      label = `${delta > 0 ? '+' : '−'}${Math.abs(delta).toFixed(1)}`
      break
    default:
      if (previous === 0) {
        label = `+${delta}`
      } else {
        const pct = Math.round((delta / previous) * 100)
        label = `${pct > 0 ? '+' : '−'}${Math.abs(pct)}%`
      }
  }
  if (label === '+0%' || label === '−0%' || label === '+0 pts' || label === '−0 pts') return { label: 'No change', tone: 'neutral' }
  const improved = options.better === undefined ? null : (options.better === 'up') === delta > 0
  return { label, tone: improved === null ? 'neutral' : improved ? 'positive' : 'negative' }
}

export const SUPPORT_REPORT_CHANNEL_LABELS: Record<string, string> = {
  widget: 'Chat',
  email: 'Email',
  portal: 'Portal',
  api: 'API',
  internal: 'Started by team',
}

export function reportChannelLabel(channel: string): string {
  return SUPPORT_REPORT_CHANNEL_LABELS[channel] ?? channel.charAt(0).toUpperCase() + channel.slice(1)
}
