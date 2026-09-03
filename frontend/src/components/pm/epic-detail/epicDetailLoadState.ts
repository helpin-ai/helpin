import { getEpicTaskCount, type EpicWithStats, type Task } from '@/lib/pmTypes'

export function shouldShowEpicLoading(epic: EpicWithStats | null, isEpicPending: boolean): boolean {
  return !epic && isEpicPending
}

export function getEpicTaskDisplayCount(
  epic: EpicWithStats,
  tasks: Task[],
  tasksUnavailable: boolean,
): number {
  return tasksUnavailable ? getEpicTaskCount(epic.stats) : tasks.length
}
