import type { Notification } from './notificationTypes'
import { buildExternalMCPSettingsPath, buildSettingsRoutePath } from './settingsSections'

function stringMetadataField(notification: Notification, key: string): string | null {
  const value = notification.metadata?.[key]
  return typeof value === 'string' && value.trim() ? value.trim() : null
}

export function getNotificationDockTarget(notification: Notification): { chatId: string } | { runId: string } | null {
  if (notification.entity_type !== 'agent_run') return null
  const chatId = stringMetadataField(notification, 'dock_chat_id')
  if (chatId) return { chatId }
  if (getNotificationTaskRunTarget(notification)) return null
  return { runId: stringMetadataField(notification, 'run_id') ?? notification.entity_id }
}

export function getNotificationTaskRunTarget(notification: Notification): { taskId: string; runId?: string } | null {
  if (notification.entity_type === 'task') {
    return {
      taskId: notification.entity_id,
      runId: stringMetadataField(notification, 'run_id') ?? undefined,
    }
  }

  if (notification.entity_type !== 'agent_run') {
    return null
  }

  const taskId = stringMetadataField(notification, 'task_id')
    ?? (stringMetadataField(notification, 'target_type') === 'task' ? stringMetadataField(notification, 'target_id') : null)
  if (!taskId) {
    return null
  }

  return {
    taskId,
    runId: stringMetadataField(notification, 'run_id') ?? notification.entity_id,
  }
}

/**
 * getSettingsNotificationTarget opens the settings page a notification is
 * about: external MCP servers, or the help center's custom domain.
 */
export function getSettingsNotificationTarget(notification: Notification, workspaceSlug: string): string | null {
  if (!workspaceSlug) return null
  if (notification.entity_type === 'external_mcp_server') return buildExternalMCPSettingsPath(workspaceSlug)
  if (notification.entity_type === 'helpcenter_config') return buildSettingsRoutePath(workspaceSlug, 'helpcenter')
  return null
}

export function moduleNotificationTarget(entityType: unknown, entityId: string, slug: string) {
  if (!slug || !entityId) return null
  const base = `/w/${encodeURIComponent(slug)}`
  const id = encodeURIComponent(entityId)
  switch (entityType) {
    case 'doc': case 'document': return { path: `${base}/docs/documents/${id}` }
    case 'crm_contact': return { path: `${base}/crm/contacts/${id}` }
    case 'crm_company': return { path: `${base}/crm/companies/${id}` }
    case 'crm_deal': return { path: `${base}/crm/deals/${id}` }
    case 'crm_signal': return { path: `${base}/crm/insights`, search: { signal: entityId } }
    case 'automation_rule': return { path: `${base}/automation/flows`, search: { show_rule: entityId } }
    default: return null
  }
}
