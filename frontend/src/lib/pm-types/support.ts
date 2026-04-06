// ── Support ─────────────────────────────────────────────────────────

export type ConversationStatus = 'open' | 'in_progress' | 'waiting' | 'resolved' | 'closed' | 'spam';
export type ConversationPriority = 'low' | 'medium' | 'high' | 'urgent';
export type TicketSource = 'widget' | 'internal' | 'email' | 'api';
export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai';
export type SupportConversationTriageStatus = 'not_run' | 'suggested' | 'auto_moved' | 'dismissed' | 'overridden';
export type SupportConversationTriageSource = 'rule' | 'ai';
export type SupportConversationTriageFeedbackAction = 'accepted' | 'dismissed' | 'corrected';

export interface SupportConversationTriage {
  id: string;
  workspace_id: string;
  conversation_id: string;
  status: SupportConversationTriageStatus;
  intent?: string | null;
  confidence?: number | null;
  reason?: string | null;
  classifier_source: SupportConversationTriageSource;
  suggested_mailbox_id?: string | null;
  auto_moved: boolean;
  locked_at?: string | null;
  evaluated_at?: string | null;
  feedback_action?: SupportConversationTriageFeedbackAction | null;
  created_at: string;
  updated_at: string;
}

export interface SupportConversation {
  id: string;
  workspace_id: string;
  mailbox_id?: string | null;
  display_id: number;
  subject: string;
  status: ConversationStatus;
  flow_state?: 'ai_handling' | 'waiting_for_human' | 'queued_for_human' | 'after_hours_queue' | 'assigned_to_human' | 'resolved_by_ai' | 'resolved_by_human' | null;
  priority: ConversationPriority;
  customer_name?: string;
  customer_email?: string;
  email_unsubscribed?: boolean;
  anonymous_id?: string;
  opened_by_user_id?: string;
  assigned_agent_id?: string;
  linked_story_id?: string;
  linked_task_id?: string;
  source: TicketSource;
  crm_contact_id?: string;
  ai_state?: 'pending' | 'resolved' | 'escalated' | null;
  ai_resolved_at?: string;
  ai_escalated_at?: string;
  ai_resolution_type?: 'confirmed' | 'assumed' | null;
  ai_turn_count?: number;
  customer_requested_human_at?: string;
  last_message?: string;
  unread_count?: number;
  mailbox_name?: string | null;
  mailbox_handle?: string | null;
  mailbox_icon?: string | null;
  team_last_seen_at?: string;
  contact_last_seen_at?: string;
  triage?: SupportConversationTriage | null;
  created_at: string;
  updated_at: string;
}

export interface UnreadStats {
  total: number;
  my_inbox: number;
  unassigned: number;
  ai_pending: number;
}

export interface SupportInboxScope {
  id: string;
  name: string;
  handle: string;
  icon: string;
  is_shared: boolean;
  is_default: boolean;
  unread_count: number;
  active: boolean;
  linked_team_id?: string | null;
}

export interface SupportInboxScopeListResponse {
  shared_inbox: SupportInboxScope;
  mailboxes: SupportInboxScope[];
}

export interface SupportMailbox {
  id: string;
  workspace_id: string;
  name: string;
  handle: string;
  icon: string;
  description?: string | null;
  routing_prompt?: string | null;
  triage_eligible: boolean;
  linked_team_id?: string | null;
  linked_team_name?: string | null;
  visibility_mode: 'members_only';
  assignment_mode: 'manual' | 'round_robin';
  position: number;
  active: boolean;
  member_count?: number;
  unread_count?: number;
  created_by_id: string;
  created_at: string;
  updated_at: string;
}

export interface SupportMailboxMember {
  workspace_member_id: string;
  user_id?: string | null;
  email: string;
  display_name: string;
  avatar_url?: string | null;
  role: string;
}

export interface SupportEmailRoute {
  id: string;
  workspace_id: string;
  mailbox_id?: string | null;
  route_key: string;
  inbound_address: string;
  source_address?: string | null;
  provider_type: 'forwarding';
  active: boolean;
  last_inbound_at?: string | null;
  created_by_id: string;
  created_at: string;
  updated_at: string;
  mailbox_name?: string | null;
  mailbox_handle?: string | null;
  mailbox_icon?: string | null;
}

export interface CreateSupportEmailRouteRequest {
  mailbox_id?: string | null;
  source_address?: string | null;
}

export interface CreateSupportMailboxRequest {
  name: string;
  handle: string;
  icon: string;
  description?: string | null;
  routing_prompt?: string | null;
  triage_eligible?: boolean;
  linked_team_id?: string | null;
  workspace_member_ids: string[];
  assignment_mode: 'manual' | 'round_robin';
  import_linked_team?: boolean;
}

export interface UpdateSupportMailboxRequest {
  name?: string;
  handle?: string;
  icon?: string;
  description?: string | null;
  routing_prompt?: string | null;
  triage_eligible?: boolean;
  linked_team_id?: string | null;
  workspace_member_ids?: string[];
  assignment_mode?: 'manual' | 'round_robin';
  import_linked_team?: boolean;
}

export interface SupportTriageRuleConditions {
  phrase_contains: string[];
  email_domain_equals: string[];
}

export interface SupportTriageRule {
  id: string;
  workspace_id: string;
  priority: number;
  active: boolean;
  name: string;
  channels: TicketSource[];
  conditions: SupportTriageRuleConditions;
  target_mailbox_id: string;
  target_mailbox_name?: string | null;
  target_mailbox_handle?: string | null;
  created_by_id: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSupportTriageRuleRequest {
  priority: number;
  active?: boolean;
  name: string;
  channels: TicketSource[];
  conditions: SupportTriageRuleConditions;
  target_mailbox_id: string;
}

export interface UpdateSupportTriageRuleRequest {
  priority?: number;
  active?: boolean;
  name?: string;
  channels?: TicketSource[];
  conditions?: SupportTriageRuleConditions;
  target_mailbox_id?: string;
}

export interface SupportTeammatePresenceStatus {
  user_id: string;
  status: 'online' | 'away' | 'offline';
  source: 'auto' | 'manual';
  manual_status?: 'online' | 'away' | 'offline';
  last_seen_at?: string;
}

export interface ConversationListMeta {
  unread: UnreadStats;
}

export interface ConversationListResponse {
  data: SupportConversation[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
  meta: ConversationListMeta;
}

export interface SupportAttachmentPayload {
  id: string;
  file_key: string;
  file_name: string;
  file_type: string;
  file_size: number;
  url: string;
}

export interface SupportLinkPreview {
  url: string;
  title: string;
  description?: string;
  site_name?: string;
  image_url?: string;
  host: string;
}

export interface SupportMessage {
  id: string;
  workspace_id: string;
  conversation_id: string;
  sender_type: MessageSenderType;
  sender_user_id?: string;
  sender_agent_id?: string;
  sender_display_name?: string;
  sender_avatar_url?: string;
  content: string;
  message_type?: string;
  is_internal: boolean;
  metadata?: string;
  via_channel?: 'email' | 'widget' | null;
  email_notified_at?: string;
  email_read_at?: string;
  attachments?: SupportAttachmentPayload[];
  created_at: string;
  updated_at: string;
}

export interface SupportAIPreviewHistoryTurn {
  sender_type: MessageSenderType;
  message_type?: string;
  content: string;
}

export interface SupportAIPreviewRequest {
  message: string;
  conversation_id?: string;
  history?: SupportAIPreviewHistoryTurn[];
  include_answer?: boolean;
  max_results?: number;
}

export type SupportAIRewriteOperation =
  | 'expand'
  | 'rephrase'
  | 'fix_grammar'
  | 'more_friendly'
  | 'more_formal';

export interface SupportAIRewriteDraftRequest {
  content: string;
  operation: SupportAIRewriteOperation;
}

export interface SupportAIRewriteDraftResponse {
  content: string;
  operation: SupportAIRewriteOperation;
  provider: string;
  model: string;
}

export interface SupportAIPreviewQueryPlan {
  decision: string;
  standalone_query: string;
  search_queries: string[];
  clarifying_question: string;
  reason: string;
  tokens_used: number;
  fallback_used: boolean;
  error?: string;
}

export interface SupportAIPreviewSearchResult {
  reference_id: string;
  source_type: string;
  title: string;
  url?: string;
  chunk_index: number;
  combined_score: number;
  vector_score: number;
  lexical_score: number;
  snippet: string;
}

export interface SupportAIPreviewRetrieval {
  query_count: number;
  result_count: number;
  results: SupportAIPreviewSearchResult[];
  error?: string;
}

export interface SupportAIPreviewAnswer {
  content: string;
  can_answer: boolean;
  source_doc_ids: string[];
  llm_confidence: number;
  grounded_confidence: number;
  tokens_used: number;
  provider: string;
  model: string;
}

export interface SupportAIPreviewResponse {
  conversation_source: string;
  confidence_threshold: number;
  total_tokens_used: number;
  final_decision: string;
  final_reason: string;
  query_plan: SupportAIPreviewQueryPlan;
  retrieval: SupportAIPreviewRetrieval;
  answer?: SupportAIPreviewAnswer;
}

export interface AIMessageMetadata {
  ai_auto_reply: boolean;
  ai_sources: Array<{
    docId: string;
    title: string;
    snippet: string;
    confidence: number;
  }>;
  ai_confidence: number;
  ai_model: string;
  ai_tokens_used: number;
  ai_agent_id: string;
  link_previews?: SupportLinkPreview[];
}

export interface AgentKnowledgeSource {
  id: string;
  agent_id: string;
  space_id: string;
  workspace_id: string;
  sync_status: 'queued' | 'running' | 'ready' | 'failed' | 'stale' | 'disabled';
  sync_progress: number;
  indexed_documents: number;
  indexed_chunks: number;
  last_sync_error?: string | null;
  last_sync_started_at?: string | null;
  last_sync_completed_at?: string | null;
  space_name?: string;
  space_type?: string;
  created_at: string;
  updated_at: string;
}

export interface SupportContentSource {
  id: string;
  workspace_id: string;
  name: string;
  start_url: string;
  crawl_limit: number;
  crawl_depth: number;
  crawl_source: 'all' | 'sitemaps' | 'links';
  formats: string[];
  render: boolean;
  include_external_links: boolean;
  include_subdomains: boolean;
  include_patterns: string[];
  exclude_patterns: string[];
  crawl_purposes: string[];
  max_age_seconds: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
  sync_status: 'queued' | 'running' | 'ready' | 'failed' | 'stale' | 'disabled';
  sync_progress: number;
  indexed_pages: number;
  indexed_chunks: number;
  last_sync_error?: string | null;
  last_crawl_job_id?: string | null;
  last_sync_started_at?: string | null;
  last_sync_completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface SupportContentPage {
  id: string;
  workspace_id: string;
  content_source_id: string;
  url: string;
  title: string;
  http_status: number;
  content_format: string;
  content_hash: string;
  content_text?: string;
  content_length: number;
  last_crawled_at: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSupportContentSourceRequest {
  name: string;
  start_url: string;
  crawl_limit: number;
  crawl_depth: number;
  crawl_source: 'all' | 'sitemaps' | 'links';
  formats: string[];
  render: boolean;
  include_external_links: boolean;
  include_subdomains: boolean;
  include_patterns: string[];
  exclude_patterns: string[];
  crawl_purposes: string[];
  max_age_seconds: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
}

export interface UpdateSupportContentSourceRequest {
  name?: string;
  start_url?: string;
  crawl_limit?: number;
  crawl_depth?: number;
  crawl_source?: 'all' | 'sitemaps' | 'links';
  formats?: string[];
  render?: boolean;
  include_external_links?: boolean;
  include_subdomains?: boolean;
  include_patterns?: string[];
  exclude_patterns?: string[];
  crawl_purposes?: string[];
  max_age_seconds?: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
}

export interface CreateConversationRequest {
  mailbox_id?: string | null;
  subject: string;
  priority?: ConversationPriority;
  customer_name?: string;
  customer_email?: string;
  source?: TicketSource;
}

export interface CreateMessageRequest {
  content: string;
  is_internal?: boolean;
}

/** @deprecated Use LinkTaskRequest instead */
export interface LinkStoryRequest {
  story_id: string;
}

export interface LinkTaskRequest {
  task_id: string;
}

export interface AssignConversationAgentRequest {
  agent_id: string;
}

// ── Support Installation Settings ───────────────────────────────────

export interface BusinessHoursDay {
  start: string;
  end: string;
  enabled: boolean;
}

export interface SupportInboxSettings {
  require_email_before_chat: boolean;
  require_phone_after_email: boolean;
  welcome_message: string;
  ai_enabled: boolean;
  ai_agent_id: string | null;
  ai_confidence_threshold: number;
  ai_response_mode: string;
  ai_max_followups: number;
  ai_auto_resolve_timeout: number;
  show_talk_to_human: boolean;
  escalation_message: string;
  handoff_behavior: string;
  handoff_team_id: string | null;
  default_mailbox_id: string | null;
  ai_handoff_mailbox_id: string | null;
  triage_enabled: boolean;
  triage_auto_move_enabled: boolean;
  triage_confidence_threshold: number;
  triage_widget_enabled: boolean;
  triage_email_enabled: boolean;
  triage_internal_enabled: boolean;
  triage_fallback_behavior: 'shared' | 'default';
  triage_rerun_on_meaning_change: boolean;
  triage_daily_budget: number;
  triage_skip_spam_conversations: boolean;
  triage_deduplicate_first_message: boolean;
  business_hours_enabled: boolean;
  business_hours_timezone: string;
  business_hours_schedule: Record<string, BusinessHoursDay>;
  outside_hours_message: string;
  email_fallback_enabled: boolean;
  email_fallback_delay_secs: number;
  email_fallback_from_name: string;
  brand_color: string;
  show_branding: boolean;
  color_scheme: string;
  button_color: string;
  button_icon_color: string;
  logo_url: string;
  launcher_position: string;
  launcher_icon: string;
  widget_name: string;
  widget_avatar_url: string;
  widget_help_space_ids: string[];
  csat_enabled: boolean;
  file_uploads_enabled: boolean;
  force_visitor_identity: boolean;
}

export interface SupportInstallationResponse {
  id: string;
  workspace_id: string;
  widget_key: string;
  settings: SupportInboxSettings;
  active: boolean;
  created_at: string;
  updated_at: string;
}
