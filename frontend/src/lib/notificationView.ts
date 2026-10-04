export type NotificationViewType = 'chat' | 'run' | 'support_conversation'

// Mounted, active surfaces only. Persisted selections can point at closed panels.
const views = new Map<symbol, { workspaceId: string; type: NotificationViewType; id: string }>()

export function registerNotificationView(workspaceId: string, type: NotificationViewType, id: string) {
  const token = Symbol()
  views.set(token, { workspaceId, type, id })
  return () => { views.delete(token) }
}

export function isViewingNotificationTarget(workspaceId: string, type: NotificationViewType, id: string) {
  if (!id || document.visibilityState !== 'visible' || !document.hasFocus()) return false
  return Array.from(views.values()).some(view => view.workspaceId === workspaceId && view.type === type && view.id === id)
}
