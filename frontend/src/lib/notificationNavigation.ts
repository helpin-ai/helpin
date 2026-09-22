import type { Notification } from './notificationTypes'
import { buildSettingsRoutePath } from './settingsSections'

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

export function getExternalMCPNotificationTarget(notification: Notification, workspaceSlug: string): string | null {
  if (!workspaceSlug || notification.entity_type !== 'external_mcp_server') {
    return null
  }

  return buildSettingsRoutePath(workspaceSlug, 'external-mcp')
}
