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

/** A workspace-scoped, user-authored support tag. `color` is a hex string (e.g. `#2563eb`). */
export interface SupportTag {
  id: string
  workspace_id?: string
  name: string
  color?: string | null
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
  /** Sender type of the most recent message: 'contact' | 'user' | 'agent' | 'system'. */
  last_message_sender_type?: string | null
  last_message_sender_display_name?: string | null
  /** True when the latest inbound message is still awaiting a team reply. */
  awaiting_reply?: boolean
  /** Product-owned system tags on the conversation, e.g. 'ai_handoff', 'ai_resolved'. */
  system_tags?: string[]
  /** User-authored, workspace-scoped tags with optional colours. */
  tags?: SupportTag[]
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
  /** "reply" | "csat_survey" | "system" — server/internal/model/support_inbox.go:293. */
  message_type?: string
  /**
   * For system messages (`message_type === 'system'`), the specific event —
   * e.g. 'assigned', 'agent_assigned', 'resolved', 'ai_escalated',
   * 'triage_routed', 'tag_added'. Drives humanized narration + badges.
   */
  system_event_type?: string
  is_internal: boolean
  metadata?: string
  via_channel?: 'email' | 'widget' | null
  email_notified_at?: string
  email_read_at?: string
  /** Postmark delivery lifecycle for outbound email replies. */
  email_delivery_status?: 'sent' | 'delivered' | 'opened' | 'bounced' | 'spam_complaint' | string
  /** Inbound email envelope addresses (customer messages received by email). */
  email_from?: string
  email_reply_to?: string
  attachments?: SupportAttachmentPayload[]
  /**
   * Sanitized HTML body — only populated for inbound email messages
   * (via_channel === 'email') from the linked support_email_logs row.
   * Backend already marks quoted-reply wrappers with
   * `data-helpin-quote="true"` (server/internal/email/inboundhtml/convert.go)
   * so the frontend can collapse them; see thread/thread-helpers.ts#splitQuotedHtml.
   * Was missing from this type until Task 13 — verified against the web
   * client's canonical type at frontend/src/lib/pm-types/support.ts:554.
   */
  html_body?: string
  /** Markdown-friendly plaintext body — present only for inbound email messages. */
  stripped_text?: string
  created_at: string
  updated_at: string
  /** Client-only optimistic-send flag; never set by the backend. */
  pending?: boolean
}

export interface AssignableMember {
  id: string
  user_id?: string
  role: string
  email: string
  display_name: string
  avatar_url?: string
  avatar_style?: string
  avatar_seed?: string
  avatar_background_mode?: string
  avatar_background_color?: string
}

/**
 * Per-view count from GET /support/inbox/views/counts.
 * Verified against server/internal/model/support_inbox_view.go:47-51.
 * `view_id` matches a builtin or custom view's `id` (see SupportInboxView).
 */
export interface SupportInboxViewCount {
  view_id: string
  total_count: number
  unread_count: number
}

/**
 * A builtin or custom inbox view from GET /support/inbox/views (custom) or
 * /support/inbox/views/builtin (builtin). Verified against
 * server/internal/model/support_inbox_view.go:15-25.
 * Builtin views carry `view_key` = "nav:<navFilter>" (e.g. "nav:waiting") or
 * "team:<mailboxId>"; custom views have no `view_key`.
 */
export interface SupportInboxView {
  id: string
  workspace_id: string
  name: string
  filters: unknown
  is_shared: boolean
  view_type: string
  view_key?: string | null
  created_by: string
  created_at: string
  updated_at: string
}
