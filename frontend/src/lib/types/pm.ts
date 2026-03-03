export type StoryStatus = 'backlog' | 'todo' | 'in_progress' | 'done'

export type StoryPriority = 'urgent' | 'high' | 'medium' | 'low' | 'none'

export interface Story {
  id: string
  identifier: string
  title: string
  description?: string
  status: StoryStatus
  priority: StoryPriority
  assignee?: {
    name: string
    avatar?: string
  }
  labels: string[]
  created_at: string
  sort_order: number
}

export const STATUS_COLUMNS: {
  id: StoryStatus
  label: string
  color: string
}[] = [
  { id: 'backlog', label: 'Backlog', color: '#a3a3a3' },
  { id: 'todo', label: 'Todo', color: '#f59e0b' },
  { id: 'in_progress', label: 'In Progress', color: '#3b82f6' },
  { id: 'done', label: 'Done', color: '#22c55e' },
]

export const PRIORITY_CONFIG: Record<
  StoryPriority,
  { label: string; color: string }
> = {
  urgent: { label: 'Urgent', color: '#ef4444' },
  high: { label: 'High', color: '#f97316' },
  medium: { label: 'Medium', color: '#f59e0b' },
  low: { label: 'Low', color: '#3b82f6' },
  none: { label: 'None', color: '#a3a3a3' },
}
