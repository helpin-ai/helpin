import { api } from '../api'
import type {
  NotificationListResponse,
  UnreadCountResponse,
  NotificationPreferences,
  UpdateNotificationPreferencesRequest,
  UserNotificationSettings,
  UpdateUserNotificationSettingsRequest,
  EntityFollower,
  NotificationFilter,
} from '../notificationTypes'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export const notificationsService = {
  // Account-level notification settings (no workspace scope)
  getUserSettings: () =>
    api.get<UserNotificationSettings>(`/user/notification-settings?timezone=${encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC')}`),

  updateUserSettings: (data: UpdateUserNotificationSettingsRequest) =>
    api.put<UserNotificationSettings>('/user/notification-settings', data),

  // Workspace-scoped endpoints below
  list: (wsId: string, params?: { status?: string; filter?: NotificationFilter; limit?: number; cursor?: string }) => {
    const searchParams = new URLSearchParams({ workspace_id: wsId })
    if (params?.status) searchParams.set('status', params.status)
    if (params?.filter && params.filter !== 'all') searchParams.set('filter', params.filter)
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.cursor) searchParams.set('cursor', params.cursor)
    return api.get<NotificationListResponse>(`/notifications?${searchParams.toString()}`)
  },

  unreadCount: (wsId: string) =>
    api.get<UnreadCountResponse>(`/notifications/unread-count${qs(wsId)}&timezone=${encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC')}`),

  update: (wsId: string, notifId: string, data: { status?: string; snoozed_until?: string }) =>
    api.patch<{ status: string }>(`/notifications/${notifId}${qs(wsId)}`, data),

  markAllRead: (wsId: string) =>
    api.post<{ status: string }>(`/notifications/mark-all-read${qs(wsId)}`, {}),

  archiveAllRead: (wsId: string) =>
    api.post<{ status: string }>(`/notifications/archive-all-read${qs(wsId)}`, {}),

  delete: (wsId: string, notifId: string) =>
    api.del<{ status: string }>(`/notifications/${notifId}${qs(wsId)}`),

  getPreferences: (wsId: string) =>
    api.get<NotificationPreferences>(`/notifications/preferences${qs(wsId)}`),

  updatePreferences: (wsId: string, data: UpdateNotificationPreferencesRequest) =>
    api.put<{ status: string }>(`/notifications/preferences${qs(wsId)}`, data),

  listFollowing: (wsId: string) =>
    api.get<EntityFollower[]>(`/notifications/following${qs(wsId)}`),

  // Entity follower endpoints
  listFollowers: (wsId: string, entityType: string, entityId: string) =>
    api.get<EntityFollower[]>(`/pm/${entityType}/${entityId}/followers${qs(wsId)}`),

  isFollowing: (wsId: string, entityType: string, entityId: string) =>
    api.get<{ following: boolean }>(`/pm/${entityType}/${entityId}/followers/check${qs(wsId)}`),

  follow: (wsId: string, entityType: string, entityId: string) =>
    api.post<{ status: string }>(`/pm/${entityType}/${entityId}/followers${qs(wsId)}`, {}),

  unfollow: (wsId: string, entityType: string, entityId: string) =>
    api.del<{ status: string }>(`/pm/${entityType}/${entityId}/followers${qs(wsId)}`),
}
