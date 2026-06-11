export type ConversationStatus = 'open' | 'waiting_on_customer' | 'resolved' | 'spam'
export type ConversationPriority = 'low' | 'medium' | 'high' | 'urgent'
export type TicketSource = 'widget' | 'internal' | 'email' | 'api'
export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai'
export type SupportConversationTriageStatus =
  | 'not_run'
  | 'suggested'
  | 'auto_moved'
  | 'dismissed'
  | 'overridden'
export type SupportConversationTriageSource = 'rule' | 'ai'
export type SupportConversationTriageFeedbackAction = 'accepted' | 'dismissed' | 'corrected'

export interface SupportConversationTriage {
  id: string
  workspace_id: string
  conversation_id: string
  status: SupportConversationTriageStatus
  intent?: string | null
  confidence?: number | null
  reason?: string | null
  classifier_source: SupportConversationTriageSource
  suggested_mailbox_id?: string | null
  auto_moved: boolean
  locked_at?: string | null
  evaluated_at?: string | null
  feedback_action?: SupportConversationTriageFeedbackAction | null
  created_at: string
  updated_at: string
}

export interface SupportConversation {
  id: string
  workspace_id: string
  mailbox_id?: string | null
  display_id: number
  subject: string
  status: ConversationStatus
  flow_state?:
    | 'ai_handling'
    | 'waiting_for_human'
    | 'queued_for_human'
    | 'after_hours_queue'
    | 'assigned_to_human'
    | 'resolved_by_ai'
    | 'resolved_by_human'
    | null
  priority: ConversationPriority
  customer_name?: string
  customer_email?: string
  email_unsubscribed?: boolean
  anonymous_id?: string
  opened_by_user_id?: string
  assigned_user_id?: string
  assigned_agent_id?: string
  linked_story_id?: string
  linked_task_id?: string
  source: TicketSource
  crm_contact_id?: string
  ai_state?: 'pending' | 'resolved' | 'escalated' | null
  ai_resolved_at?: string
  ai_escalated_at?: string
  ai_resolution_type?: 'confirmed' | 'assumed' | null
  ai_turn_count?: number
  customer_requested_human_at?: string
  last_message?: string
  unread_count?: number
  mailbox_name?: string | null
  mailbox_handle?: string | null
  mailbox_icon?: string | null
  country_code?: string | null
  country_name?: string | null
  team_last_seen_at?: string
  contact_last_seen_at?: string
  triage?: SupportConversationTriage | null
  created_at: string
  updated_at: string
}

export interface UnreadStats {
  total: number
  my_inbox: number
  unassigned: number
  ai_active: number
}

export interface SupportInboxScope {
  id: string
  name: string
  handle: string
  icon: string
  is_shared: boolean
  is_default: boolean
  unread_count: number
  active: boolean
  linked_team_id?: string | null
}

export interface SupportInboxScopeListResponse {
  shared_inbox: SupportInboxScope
  mailboxes: SupportInboxScope[]
}

export interface SupportMailbox {
  id: string
  workspace_id: string
  name: string
  handle: string
  icon: string
  description?: string | null
  routing_prompt?: string | null
  triage_eligible: boolean
  linked_team_id?: string | null
  linked_team_name?: string | null
  visibility_mode: 'members_only'
  assignment_mode: 'manual' | 'round_robin'
  position: number
  active: boolean
  member_count?: number
  unread_count?: number
  created_by_id: string
  created_at: string
  updated_at: string
}

export interface SupportMailboxMember {
  workspace_member_id: string
  user_id?: string | null
  email: string
  display_name: string
  avatar_url?: string | null
  avatar_style?: string | null
  avatar_seed?: string | null
  avatar_background_mode?: string | null
  avatar_background_color?: string | null
  role: string
}

export interface SupportTeammatePresenceStatus {
  user_id: string
  status: 'online' | 'away' | 'offline'
  source: 'auto' | 'manual'
  manual_status?: 'online' | 'away' | 'offline'
  last_seen_at?: string
}

export interface ConversationListMeta {
  unread: UnreadStats
}

export interface ConversationListResponse {
  data: SupportConversation[]
  total: number
  page: number
  per_page: number
  total_pages: number
  meta: ConversationListMeta
}

export interface SupportAttachmentPayload {
  id: string
  file_key: string
  file_name: string
  file_type: string
  file_size: number
  url: string
}

export interface SupportMessage {
  id: string
  workspace_id: string
  conversation_id: string
  sender_type: MessageSenderType
  sender_user_id?: string
  sender_agent_id?: string
  sender_display_name?: string
  sender_avatar_url?: string
  content: string
  message_type?: string
  is_internal: boolean
  metadata?: string
  via_channel?: 'email' | 'widget' | null
  email_notified_at?: string
  email_read_at?: string
  attachments?: SupportAttachmentPayload[]
  created_at: string
  updated_at: string
}
