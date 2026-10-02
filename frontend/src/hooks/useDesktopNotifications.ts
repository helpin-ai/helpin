import { useCallback, useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { anyTabFocused, deliverDesktopNotification, getDesktopPermission, isDesktopEnabled, trackDesktopFocus } from '@/lib/desktopNotifications'
import { notificationsService } from '@/lib/services/notificationsService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrapRequired } from '@/lib/queryUtils'
import { desktopNotificationTarget } from '@/lib/desktopNotificationNavigation'
import type { WSEvent } from './useWebSocket'

export function useDesktopNotifications(workspaceId: string) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const userId = useAuthStore(state => state.user?.id)

  useEffect(() => {
    if (userId && getDesktopPermission() !== 'unsupported') return trackDesktopFocus(userId)
  }, [userId])

  return useCallback(async (event: WSEvent) => {
    if (!userId || event.entity !== 'notification' || event.data?.recipient_id !== userId
      || !isDesktopEnabled(userId) || getDesktopPermission() !== 'granted') return
    const workspace = useWorkspaceStore.getState().currentWorkspace
    if (workspace?.id !== workspaceId) return
    const isCurrent = () => useAuthStore.getState().user?.id === userId
    try {
      // Foreground activity must not become a delayed popup after a tab switch.
      // Delivery checks focus again in case the user returns during the request.
      if (anyTabFocused(userId)) return
      // Recheck saved pause, mute and category settings, including changes made in another tab.
      const preferences = await queryClient.fetchQuery({
        queryKey: queryKeys.notifications.preferences(workspaceId),
        queryFn: async () => unwrapRequired(await notificationsService.getPreferences(workspaceId), 'Notification preferences'),
        staleTime: 0,
      })
      await deliverDesktopNotification(event, {
        userId, workspaceId, preferences, isCurrent,
        onClick: () => {
          const slug = workspace.slug
          if (!slug) return
          const target = desktopNotificationTarget(event, slug)
          if ('dock' in target) {
            if (useWorkspaceStore.getState().currentWorkspace?.id === workspaceId) {
              window.dispatchEvent(new CustomEvent('helpin:ask-agents', { detail: target.dock }))
            } else if ('chatId' in target.dock) {
              // Shared-chat links are consumed when the destination workspace mounts.
              window.location.assign(`/w/${encodeURIComponent(slug)}?ask_chat=${encodeURIComponent(target.dock.chatId)}`)
            } else {
              void navigate({ to: `/w/${encodeURIComponent(slug)}/automation/activity`, search: { page: 1, run_id: target.dock.runId } })
            }
          } else void navigate({ to: target.path, search: target.search })
        },
      })
    } catch (error) {
      console.warn('Desktop notification preferences could not be checked', error)
    }
  }, [userId, workspaceId, queryClient, navigate])
}
