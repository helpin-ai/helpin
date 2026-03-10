import { useQuery, useMutation, useQueryClient, useInfiniteQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { notificationsService } from '@/lib/services/notificationsService'
import { unwrap } from '@/lib/queryUtils'
import type { NotificationFilter, UpdateNotificationPreferencesRequest, UpdateUserNotificationSettingsRequest } from '@/lib/notificationTypes'

// Account-level notification settings (no workspace scope)
export function useUserNotificationSettings() {
  return useQuery({
    queryKey: queryKeys.userNotificationSettings.all(),
    queryFn: async () => unwrap(await notificationsService.getUserSettings()),
    staleTime: 5 * 60_000,
  })
}

export function useUpdateUserNotificationSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: UpdateUserNotificationSettingsRequest) =>
      notificationsService.updateUserSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.userNotificationSettings.all() })
    },
  })
}

// Workspace-scoped notification hooks
export function useNotifications(wsId: string, filter: NotificationFilter = 'all') {
  return useInfiniteQuery({
    queryKey: queryKeys.notifications.list(wsId, filter),
    queryFn: async ({ pageParam }) =>
      unwrap(await notificationsService.list(wsId, { filter, limit: 20, cursor: pageParam as string | undefined })),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    enabled: !!wsId,
    staleTime: 30_000,
  })
}

export function useUnreadCount(wsId: string) {
  return useQuery({
    queryKey: queryKeys.notifications.unreadCount(wsId),
    queryFn: async () => unwrap(await notificationsService.unreadCount(wsId)),
    enabled: !!wsId,
    staleTime: 30_000,
    refetchInterval: 30_000,
  })
}

export function useMarkAsRead(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notifId: string) => notificationsService.update(wsId, notifId, { status: 'read' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useMarkAsUnread(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notifId: string) => notificationsService.update(wsId, notifId, { status: 'unread' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useArchiveNotification(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notifId: string) => notificationsService.update(wsId, notifId, { status: 'archived' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useSnoozeNotification(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ notifId, until }: { notifId: string; until: string }) =>
      notificationsService.update(wsId, notifId, { snoozed_until: until }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useMarkAllAsRead(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => notificationsService.markAllRead(wsId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useArchiveAllRead(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => notificationsService.archiveAllRead(wsId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useDeleteNotification(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notifId: string) => notificationsService.delete(wsId, notifId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) })
    },
  })
}

export function useNotificationPreferences(wsId: string) {
  return useQuery({
    queryKey: queryKeys.notifications.preferences(wsId),
    queryFn: async () => unwrap(await notificationsService.getPreferences(wsId)),
    enabled: !!wsId,
    staleTime: 5 * 60_000,
  })
}

export function useUpdateNotificationPreferences(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: UpdateNotificationPreferencesRequest) =>
      notificationsService.updatePreferences(wsId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.preferences(wsId) })
    },
  })
}

export function useFollowEntity(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ entityType, entityId }: { entityType: string; entityId: string }) =>
      notificationsService.follow(wsId, entityType, entityId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.following(wsId) })
    },
  })
}

export function useUnfollowEntity(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ entityType, entityId }: { entityType: string; entityId: string }) =>
      notificationsService.unfollow(wsId, entityType, entityId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.following(wsId) })
    },
  })
}

export function useIsFollowing(wsId: string, entityType: string, entityId: string) {
  return useQuery({
    queryKey: queryKeys.notifications.isFollowing(wsId, entityType, entityId),
    queryFn: async () => unwrap(await notificationsService.isFollowing(wsId, entityType, entityId)),
    enabled: !!wsId && !!entityType && !!entityId,
    staleTime: 60_000,
  })
}
