import type { WSEvent } from '@/hooks/useWebSocket'
import { buildSettingsRoutePath } from './settingsSections'

type DesktopTarget = { path: string; search?: { run: string } } | { dock: { chatId: string } | { runId: string } }

export function desktopNotificationTarget(event: WSEvent, slug: string): DesktopTarget {
  const data = event.data ?? {}
  const base = `/w/${encodeURIComponent(slug)}`
  const id = typeof data.entity_id === 'string' ? encodeURIComponent(data.entity_id) : ''
  if (data.entity_type === 'agent_run') {
    if (typeof data.dock_chat_id === 'string' && data.dock_chat_id) return { dock: { chatId: data.dock_chat_id } }
    if (typeof data.parent_task_id === 'string' && data.parent_task_id) {
      return { path: `${base}/pm/tasks/${encodeURIComponent(data.parent_task_id)}`, search: { run: typeof data.run_id === 'string' && data.run_id ? data.run_id : String(data.entity_id) } }
    }
    if (id) return { dock: { runId: typeof data.run_id === 'string' && data.run_id ? data.run_id : String(data.entity_id) } }
  }
  if (id) {
    switch (data.entity_type) {
      case 'task': return { path: `${base}/pm/tasks/${id}`, ...(typeof data.run_id === 'string' && data.run_id ? { search: { run: data.run_id } } : {}) }
      case 'epic': return { path: `${base}/pm/epics/${id}` }
      case 'objective': return { path: `${base}/pm/objectives/${id}` }
      case 'sprint': return { path: `${base}/pm/sprints` }
      case 'support_conversation': return { path: `${base}/support/${id}` }
      case 'doc': case 'document': return { path: `${base}/docs/documents/${id}` }
      case 'external_mcp_server': return { path: buildSettingsRoutePath(slug, 'external-mcp') }
    }
  }
  return { path: `${base}/notifications` }
}
