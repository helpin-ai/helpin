import type { Story, StoryStateColumn } from './pmTypes'

const PM_DND_PREFIX = '[pm-dnd]'
const PM_DND_SUMMARY_LIMIT = 8

export function createPMDnDTraceID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `pm-dnd-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function summarizePMDnDStories(stories: Pick<Story, 'id' | 'position'>[]): string[] {
  const entries = stories.slice(0, PM_DND_SUMMARY_LIMIT).map((story) => `${story.id}@${story.position}`)
  if (stories.length > PM_DND_SUMMARY_LIMIT) {
    entries.push(`+${stories.length - PM_DND_SUMMARY_LIMIT} more`)
  }
  return entries
}

export function summarizePMDnDColumn(column?: Pick<StoryStateColumn, 'state' | 'stories' | 'task_count' | 'has_more'> | null) {
  if (!column) return null
  return {
    state_id: column.state.id,
    state_type: column.state.state_type,
    loaded_count: column.stories.length,
    task_count: column.task_count,
    has_more: column.has_more,
    loaded_order: summarizePMDnDStories(column.stories),
  }
}

export function logPMDnD(event: string, fields: Record<string, unknown>) {
  if (!import.meta.env.DEV) return
  console.debug(PM_DND_PREFIX, event, fields)
}
