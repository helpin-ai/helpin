import type { SupportConversationSLA, SupportConversationSLAItem, SupportSLAMetric } from './pmTypes'

export const SUPPORT_SLA_METRIC_LABELS: Record<SupportSLAMetric, string> = {
  first_response: 'First response',
  next_response: 'Next response',
  resolution: 'Resolution',
}

export const SUPPORT_SLA_METRIC_SHORT: Record<SupportSLAMetric, string> = {
  first_response: 'FRT',
  next_response: 'NRT',
  resolution: 'TTR',
}

export type SupportSLATone = 'breached' | 'urgent' | 'warning' | 'ok'

export interface SupportSLAChip {
  metric: SupportSLAMetric
  tone: SupportSLATone
  /** e.g. "12m", "2h 5m", or "-5m" when overdue. */
  remaining: string
  dueAt: string
}

/** Formats a signed time left compactly; negative means overdue. */
export function formatSLARemaining(ms: number): string {
  const overdue = ms < 0
  const minutes = Math.floor(Math.abs(ms) / 60_000)
  let text: string
  if (minutes < 1) text = '<1m'
  else if (minutes < 60) text = `${minutes}m`
  else if (minutes < 24 * 60) {
    const hours = Math.floor(minutes / 60)
    const rest = minutes % 60
    text = rest ? `${hours}h ${rest}m` : `${hours}h`
  } else {
    const days = Math.floor(minutes / (24 * 60))
    const hours = Math.floor((minutes % (24 * 60)) / 60)
    text = hours ? `${days}d ${hours}h` : `${days}d`
  }
  return overdue ? `-${text}` : text
}

function isOpenTarget(item: SupportConversationSLAItem): boolean {
  if (!item.due_at) return false
  if (item.state === 'pending') return true
  // A missed target stays open until it is met (or the conversation closes).
  return item.state === 'breached' && !item.achieved_at
}

/**
 * The chip to show for a conversation: the open target due soonest, overdue
 * ones first. Returns null when nothing is running.
 */
export function supportSLAChip(sla: SupportConversationSLA | null | undefined, now = Date.now()): SupportSLAChip | null {
  if (!sla) return null
  const open = sla.metrics.filter(isOpenTarget).sort((a, b) => new Date(a.due_at!).getTime() - new Date(b.due_at!).getTime())
  const next = open[0]
  if (!next?.due_at) return null
  const remainingMs = new Date(next.due_at).getTime() - now
  let tone: SupportSLATone = 'ok'
  if (remainingMs <= 0) tone = 'breached'
  else if (remainingMs <= 15 * 60_000) tone = 'urgent'
  else if (remainingMs <= 60 * 60_000) tone = 'warning'
  return { metric: next.metric, tone, remaining: formatSLARemaining(remainingMs), dueAt: next.due_at }
}

export function describeSLAItem(item: SupportConversationSLAItem, now = Date.now()): string {
  const label = SUPPORT_SLA_METRIC_LABELS[item.metric]
  switch (item.state) {
    case 'pending':
      return item.due_at ? `${label}: due in ${formatSLARemaining(new Date(item.due_at).getTime() - now)}` : `${label}: running`
    case 'paused':
      return `${label}: paused while waiting on the customer`
    case 'achieved':
      return `${label}: met`
    case 'cancelled':
      return `${label}: not needed`
    case 'breached':
      if (item.achieved_at) return `${label}: missed`
      return item.due_at ? `${label}: overdue by ${formatSLARemaining(now - new Date(item.due_at).getTime())}` : `${label}: missed`
  }
}

/** Snooze presets relative to now, in the viewer's time zone. */
export function snoozePresets(now = new Date()): { key: string; label: string; until: Date }[] {
  const at = (days: number, hour: number) => {
    const date = new Date(now)
    date.setDate(date.getDate() + days)
    date.setHours(hour, 0, 0, 0)
    return date
  }
  const laterToday = new Date(now.getTime() + 3 * 60 * 60_000)
  const daysUntilMonday = ((8 - now.getDay()) % 7) || 7
  const presets = [
    { key: 'later', label: 'Later today', until: laterToday },
    { key: 'tomorrow', label: 'Tomorrow', until: at(1, 9) },
    { key: 'monday', label: 'Next Monday', until: at(daysUntilMonday, 9) },
    { key: 'week', label: 'In one week', until: at(7, 9) },
  ]
  // "Later today" only makes sense before the evening.
  return presets.filter((preset) => preset.key !== 'later' || laterToday.getDate() === now.getDate())
}
