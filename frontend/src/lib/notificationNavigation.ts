import type { Notification } from './notificationTypes'
import { buildAutomationToolConnectionsPath } from './automationUi'

function stringMetadataField(notification: Notification, key: string): string | null {
  const value = notification.metadata?.[key]
  return typeof value === 'string' && value.trim() ? value.trim() : null
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

  return buildAutomationToolConnectionsPath(workspaceSlug)
}
