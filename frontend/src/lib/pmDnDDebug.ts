import type { Task, TaskStateColumn } from './pmTypes'

const PM_DND_PREFIX = '[pm-dnd]'
const PM_DND_SUMMARY_LIMIT = 8

export function createPMDnDTraceID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `pm-dnd-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function summarizePMDnDTasks(tasks: Pick<Task, 'id' | 'position'>[]): string[] {
  const entries = tasks.slice(0, PM_DND_SUMMARY_LIMIT).map((task) => `${task.id}@${task.position}`)
  if (tasks.length > PM_DND_SUMMARY_LIMIT) {
    entries.push(`+${tasks.length - PM_DND_SUMMARY_LIMIT} more`)
  }
  return entries
}

export function summarizePMDnDColumn(column?: Pick<TaskStateColumn, 'state' | 'tasks' | 'task_count' | 'has_more'> | null) {
  if (!column) return null
  return {
    state_id: column.state.id,
    state_type: column.state.state_type,
    loaded_count: column.tasks.length,
    task_count: column.task_count,
    has_more: column.has_more,
    loaded_order: summarizePMDnDTasks(column.tasks),
  }
}

export function logPMDnD(event: string, fields: Record<string, unknown>) {
  if (!import.meta.env.DEV) return
  console.debug(PM_DND_PREFIX, event, fields)
}
