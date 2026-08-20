import type { AssignableMember, SupportTeammatePresenceStatus } from '@helpin-ai/support-core'

const PRESENCE_ORDER: Record<SupportTeammatePresenceStatus['status'], number> = {
  online: 0,
  away: 1,
  offline: 2,
}

export function teammatePresenceByUserId(statuses: SupportTeammatePresenceStatus[]) {
  return new Map(statuses.map((status) => [status.user_id, status]))
}

export function sortTeammatesByPresence(
  members: AssignableMember[],
  statuses: SupportTeammatePresenceStatus[],
): AssignableMember[] {
  const byUserId = teammatePresenceByUserId(statuses)
  return [...members].sort((left, right) => {
    const leftRank = left.user_id ? PRESENCE_ORDER[byUserId.get(left.user_id)?.status ?? 'offline'] : 3
    const rightRank = right.user_id ? PRESENCE_ORDER[byUserId.get(right.user_id)?.status ?? 'offline'] : 3
    return leftRank - rightRank || left.display_name.localeCompare(right.display_name)
  })
}

export function teammatePresenceLabel(status?: SupportTeammatePresenceStatus['status']): string {
  if (status === 'online') return 'Online'
  if (status === 'away') return 'Away'
  if (status === 'offline') return 'Offline'
  return 'Availability unknown'
}

export function teammatePresenceDotClass(status?: SupportTeammatePresenceStatus['status']): string {
  if (status === 'online') return 'bg-emerald-500'
  if (status === 'away') return 'bg-amber-500'
  return 'bg-slate-300 dark:bg-slate-600'
}
