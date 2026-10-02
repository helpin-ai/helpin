import { useEffect } from 'react'
import { registerNotificationView, type NotificationViewType } from '@/lib/notificationView'

export function useNotificationView(workspaceId: string, type: NotificationViewType, id: string | null | undefined, active = true) {
  useEffect(() => {
    if (workspaceId && id && active) return registerNotificationView(workspaceId, type, id)
  }, [workspaceId, type, id, active])
}
