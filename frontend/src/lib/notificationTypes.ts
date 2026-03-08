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

export interface NotificationPreferences {
  id?: string
  user_id: string
  workspace_id: string
  do_not_disturb: boolean
  dnd_until?: string
  email_enabled: boolean
  email_digest_frequency: string
  email_digest_time: string
  email_digest_day: number
  timezone: string
  channel_preferences: Record<string, { in_app?: boolean; email?: boolean }>
  badge_mode: 'all' | 'mentions_only' | 'none'
}

export interface UpdateNotificationPreferencesRequest {
  do_not_disturb?: boolean
  dnd_until?: string
  email_enabled?: boolean
  email_digest_frequency?: string
  email_digest_time?: string
  email_digest_day?: number
  timezone?: string
  channel_preferences?: Record<string, { in_app?: boolean; email?: boolean }>
  badge_mode?: string
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
