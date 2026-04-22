// Notification system TypeScript interfaces

export interface Notification {
  id: string
  workspace_id: string
  recipient_id: string
  actor_id?: string
  entity_type: string
  entity_id: string
  event_type: string
  title: string
  body?: string
  metadata: Record<string, unknown>
  latest_event_category: string
  actor_snapshot: ActorSnapshot
  entity_snapshot: EntitySnapshot
  parent_entity_snapshot?: ParentEntitySnapshot
  event_count: number
  last_event_at: string
  status: 'unread' | 'read' | 'archived' | 'snoozed'
  snoozed_until?: string
  read_at?: string
  archived_at?: string
  priority: 'urgent' | 'high' | 'normal' | 'low'
  created_at: string
  updated_at: string
}

export interface ActorSnapshot {
  id?: string
  name?: string
  avatar_url?: string
  type?: string
}

export interface EntitySnapshot {
  title?: string
  identifier?: string
  url?: string
  state?: string
  priority?: string
}

export interface ParentEntitySnapshot {
  type?: string
  id?: string
  title?: string
  identifier?: string
}

export interface NotificationListResponse {
  data: Notification[]
  next_cursor?: string
  unread_count: number
}

export interface UnreadCountResponse {
  count: number
}

// Account-level notification delivery settings (one row per user, no workspace scope).
export interface UserNotificationSettings {
  id?: string
  user_id: string
  email_enabled: boolean
  email_digest_frequency: string
  email_digest_time: string
  email_digest_day: number
  do_not_disturb: boolean
  dnd_until?: string | null
  badge_mode: 'all' | 'mentions_only' | 'none'
  timezone: string
}

export interface UpdateUserNotificationSettingsRequest {
  email_enabled?: boolean
  email_digest_frequency?: string
  email_digest_time?: string
  email_digest_day?: number
  do_not_disturb?: boolean
  dnd_until?: string | null
  badge_mode?: string
  timezone?: string
}

// Workspace-level notification preferences (per-category toggles + mute).
export interface NotificationPreferences {
  id?: string
  user_id: string
  workspace_id: string
  mute_workspace: boolean
  channel_preferences: Record<string, { in_app?: boolean; email?: boolean }>
  // Backward compat: account-level fields overlaid by backend
  do_not_disturb: boolean
  dnd_until?: string | null
  email_enabled: boolean
  email_digest_frequency: string
  email_digest_time: string
  email_digest_day: number
  timezone: string
  badge_mode: 'all' | 'mentions_only' | 'none'
}

export interface UpdateNotificationPreferencesRequest {
  mute_workspace?: boolean
  channel_preferences?: Record<string, { in_app?: boolean; email?: boolean }>
}

export interface EntityFollower {
  id: string
  user_id: string
  entity_type: string
  entity_id: string
  workspace_id: string
  reason: string
  created_at: string
}

export type NotificationFilter = 'all' | 'mentions' | 'assigned'

export interface NotificationCategory {
  key: string
  label: string
  description: string
  supportsEmail?: boolean
}

export const WORKSPACE_NOTIFICATION_CATEGORIES: NotificationCategory[] = [
  { key: 'assignments', label: 'Assignments', description: 'Assignments and ownership changes' },
  { key: 'agent_attention', label: 'Agent attention', description: 'Agents waiting for your input or approval on task runs' },
  { key: 'status_changes', label: 'Status changes', description: 'Changes to status, priority, and blocking' },
  { key: 'comments', label: 'Comments and replies', description: 'New comments on items you follow' },
  { key: 'mentions', label: 'Mentions', description: 'Mentions in comments, descriptions, or checklists' },
  { key: 'subscriptions', label: 'Subscriptions', description: 'Updates to items you follow' },
  { key: 'sprints', label: 'Sprints', description: 'Sprint creation and updates' },
]

export const SUPPORT_NOTIFICATION_CATEGORIES: NotificationCategory[] = [
  {
    key: 'support_replies',
    label: 'Customer replies',
    description: 'Unread customer replies on conversations you currently own',
    supportsEmail: true,
  },
  {
    key: 'support_mentions',
    label: 'Support mentions',
    description: 'Mentions in internal support notes and teammate collaboration',
    supportsEmail: true,
  },
]
