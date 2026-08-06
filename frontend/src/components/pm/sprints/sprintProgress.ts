import type { PMSprintStats, SprintCloseoutListItem } from '@/lib/pmTypes'

export interface SprintProgress {
  completedTasks: number
  committedTasks: number
  rolledOverTasks: number
  completedPoints: number
  committedPoints: number
  percentage: number
  historical: boolean
}

export function getSprintProgress(stats: PMSprintStats, closeout?: SprintCloseoutListItem | null): SprintProgress {
  const historical = Boolean(closeout)
  const completedTasks = closeout?.completed_count ?? stats.done_task_count
  const committedTasks = closeout?.committed_count ?? stats.task_count
  const rolledOverTasks = closeout?.rolled_over_count ?? 0
  const completedPoints = closeout?.completed_points ?? stats.done_points
  const committedPoints = closeout?.committed_points ?? stats.total_points

  return {
    completedTasks,
    committedTasks,
    rolledOverTasks,
    completedPoints,
    committedPoints,
    percentage: committedTasks > 0 ? Math.min(100, Math.round((completedTasks / committedTasks) * 100)) : 0,
    historical,
  }
}
