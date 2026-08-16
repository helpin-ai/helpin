// ── Support ─────────────────────────────────────────────────────────

export type ConversationStatus = 'open' | 'waiting_on_customer' | 'resolved' | 'spam';
export type ConversationPriority = 'low' | 'medium' | 'high' | 'urgent';
export type TicketSource = 'widget' | 'internal' | 'email' | 'api';
export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai';
export type SupportConversationTriageStatus = 'not_run' | 'suggested' | 'auto_moved' | 'dismissed' | 'overridden';
export type SupportConversationTriageSource = 'rule' | 'ai';
export type SupportConversationTriageFeedbackAction = 'accepted' | 'dismissed' | 'corrected';
export type SupportSystemTag = 'ai_handoff' | 'ai_resolved';

export interface SupportTag {
  id: string;
  workspace_id: string;
  name: string;
  color?: string | null;
  created_at: string;
  updated_at: string;
}

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
  primary_recipient_state?: 'confirmed' | 'unconfirmed';
  suggested_primary_recipient_email?: string | null;
  suggested_primary_recipient_name?: string | null;
  email_cc?: string[];
  email_thread_participants?: string[];
  email_unsubscribed?: boolean;
  anonymous_id?: string;
  opened_by_user_id?: string;
  assigned_user_id?: string;
  assigned_agent_id?: string;
  linked_story_id?: string;
  linked_task_id?: string;
  source: TicketSource;
  crm_contact_id?: string;
  crm_company_id?: string | null;
  ai_state?: 'pending' | 'resolved' | 'escalated' | null;
  ai_resolved_at?: string;
  ai_escalated_at?: string;
  ai_resolution_type?: 'confirmed' | 'assumed' | null;
  ai_turn_count?: number;
  customer_requested_human_at?: string;
  human_takeover?: boolean | null;
  last_message?: string;
  last_message_sender_type?: MessageSenderType | null;
  last_message_sender_display_name?: string | null;
  unread_count?: number;
  awaiting_reply?: boolean;
  mailbox_name?: string | null;
  mailbox_handle?: string | null;
  mailbox_icon?: string | null;
  country_code?: string | null;
  country_name?: string | null;
  team_last_seen_at?: string;
  contact_last_seen_at?: string;
  triage?: SupportConversationTriage | null;
  tags?: SupportTag[];
  system_tags?: SupportSystemTag[];
  created_at: string;
  updated_at: string;
}

export interface SendSupportConversationTranscriptResponse {
  success: boolean;
  message: string;
  email: string;
}

export interface UnreadStats {
  inbox: number;
  mine: number;
  waiting: number;
  ai_active: number;
  inbox_total?: number;
  mine_total?: number;
  waiting_total?: number;
  ai_active_total?: number;
  total?: number;
  my_inbox?: number;
  unassigned?: number;
}

export interface SupportInboxScope {
  id: string;
  name: string;
  handle: string;
  icon: string;
  is_shared: boolean;
  is_default: boolean;
  total_count?: number;
  unread_count: number;
  active: boolean;
  linked_team_id?: string | null;
}

export type SupportInboxViewFilters = Record<string, string>;

export interface SupportInboxView {
  id: string;
  workspace_id: string;
  name: string;
  filters: SupportInboxViewFilters;
  is_shared: boolean;
  view_type: 'custom' | 'default' | 'team';
  view_key?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SupportInboxViewCount {
  view_id: string;
  total_count: number;
  unread_count: number;
}

export interface CreateSupportInboxViewRequest {
  name: string;
  filters: SupportInboxViewFilters;
  is_shared: boolean;
}

export interface UpdateSupportInboxViewRequest {
  name?: string;
  filters?: SupportInboxViewFilters;
  is_shared?: boolean;
}

export interface UpdateSupportInboxBuiltinViewRequest {
  view_key: string;
  filters: SupportInboxViewFilters;
}

export interface SupportInboxScopeListResponse {
  shared_inbox: SupportInboxScope;
  mailboxes: SupportInboxScope[];
}

export interface SupportWorkspaceUnreadCount {
  workspace_id: string;
  unread_count: number;
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
  reply_time_preset?: string | null;
  reply_time_custom_minutes?: number | null;
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
  avatar_style?: string | null;
  avatar_seed?: string | null;
  avatar_background_mode?: string | null;
  avatar_background_color?: string | null;
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
  confirmation_received_at?: string | null;
  verification_sent_at?: string | null;
  forwarding_verified_at?: string | null;
  forwarding_last_error?: string | null;
  created_by_id: string;
  created_at: string;
  updated_at: string;
  mailbox_name?: string | null;
  mailbox_handle?: string | null;
  mailbox_icon?: string | null;
  mailbox_ids?: string[];
}

export interface CreateSupportEmailRouteRequest {
  mailbox_id?: string | null;
  source_address?: string | null;
}

export interface SupportEmailSender {
  id: string;
  workspace_id: string;
  mailbox_id?: string | null;
  email: string;
  local_part: string;
  domain: string;
  display_name: string;
  postmark_domain_id?: number | null;
  return_path_domain: string;
  return_path_domain_cname_value: string;
  return_path_domain_verified: boolean;
  dkim_host: string;
  dkim_text_value: string;
  dkim_pending_host: string;
  dkim_pending_text_value: string;
  dkim_verified: boolean;
  dkim_update_status: string;
  dmarc_host: string;
  dmarc_policy: string;
  dmarc_record_present: boolean;
  dmarc_last_checked_at?: string | null;
  domain_status: 'pending_dns' | 'verified' | string;
  forwarding_status: 'not_started' | 'pending' | 'verified' | string;
  forwarding_address: string;
  forwarding_verified_at?: string | null;
  forwarding_last_checked_at?: string | null;
  forwarding_last_error?: string | null;
  email_route_id?: string | null;
  verification_status: 'pending_dns' | 'verified' | string;
  default_scope: 'none' | 'workspace' | 'mailbox' | string;
  active: boolean;
  last_checked_at?: string | null;
  last_error?: string | null;
  created_by_id: string;
  created_at: string;
  updated_at: string;
  mailbox_name?: string | null;
  mailbox_handle?: string | null;
  mailbox_icon?: string | null;
  mailbox_ids?: string[];
}

export interface CreateSupportEmailSenderRequest {
  email: string;
  display_name?: string;
  mailbox_id?: string | null;
}

export interface SetSupportEmailSenderDefaultRequest {
  default_scope: 'none' | 'workspace' | 'mailbox';
  mailbox_id?: string | null;
  mailbox_ids?: string[];
}

export interface UpdateSupportEmailSenderRequest {
  display_name?: string;
  default_scope?: 'none' | 'workspace' | 'mailbox';
  mailbox_id?: string | null;
  mailbox_ids?: string[];
}

export interface SupportEmailSenderDomain {
  id: string;
  workspace_id: string;
  domain: string;
  from_local_part: string;
  postmark_domain_id?: number | null;
  return_path_domain: string;
  return_path_domain_cname_value: string;
  return_path_domain_verified: boolean;
  dkim_host: string;
  dkim_text_value: string;
  dkim_pending_host: string;
  dkim_pending_text_value: string;
  dkim_verified: boolean;
  dkim_update_status: string;
  status: 'pending_dns' | 'verified' | string;
  active: boolean;
  last_checked_at?: string | null;
  last_error?: string | null;
  created_by_id: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSupportEmailSenderDomainRequest {
  domain: string;
  from_local_part: string;
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
  active?: boolean;
  import_linked_team?: boolean;
  reply_time_preset?: string;
  reply_time_custom_minutes?: number | null;
  clear_reply_time_preset?: boolean;
  clear_reply_time_custom_minutes?: boolean;
}

export interface SupportTriageRuleConditions {
  condition_logic?: 'all' | 'any';
  phrase_contains: string[];
  email_domain_equals: string[];
  sender_email_contains?: string[];
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

export interface SupportCannedResponse {
  id: string;
  workspace_id: string;
  short_code: string;
  content: string;
  tag: string;
  created_by_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateCannedResponseRequest {
  short_code: string;
  content: string;
  tag?: string;
  title?: string;
}

export interface UpdateCannedResponseRequest {
  short_code: string;
  content: string;
  tag?: string;
  title?: string;
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

export interface SupportSearchHighlightRange {
  start: number;
  end: number;
}

export interface SupportSearchHighlight {
  field: string;
  text: string;
  ranges: SupportSearchHighlightRange[];
}

export interface SupportConversationSearchResult {
  conversation: SupportConversation;
  display_id: number;
  matched_fields: string[];
  snippet: string;
  highlights: SupportSearchHighlight[];
  score: number;
}

export interface SupportConversationSearchMeta {
  sort: 'relevance' | 'newest' | 'oldest' | string;
  query: string;
  total_capped: boolean;
  total_cap: number;
}

export interface SupportConversationSearchResponse {
  data: SupportConversationSearchResult[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
  meta: SupportConversationSearchMeta;
}

export interface SupportConversationSearchParams {
  q?: string;
  sort?: 'relevance' | 'newest' | 'oldest';
  assigned_to?: string;
  mailbox_ids?: string;
  tag_ids?: string;
  customer_email?: string;
  created_from?: string;
  created_to?: string;
  statuses?: string;
  priorities?: string;
  title?: string;
  ai?: string;
  page?: number;
  per_page?: number;
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

/**
 * Canonical set of system_event_type values the backend emits on
 * message_type='system' rows. Renderers branch on this instead of
 * keyword-matching content. Source of truth:
 * server/internal/model/support_system_event.go.
 */
export const SUPPORT_SYSTEM_EVENT_TYPES = [
  'teammate_joined',
  'assigned',
  'unassigned',
  'took',
  'agent_assigned',
  'mailbox_moved',
  'triage_routed',
  'triage_dismissed',
  'ai_escalated',
  'customer_requested_human',
  'resolved',
  'reopened',
  'closed',
  'email_recipients_updated',
  'tag_added',
  'tag_removed',
  'task_created',
] as const;

export type SupportSystemEventType = (typeof SUPPORT_SYSTEM_EVENT_TYPES)[number];

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
  system_event_type?: SupportSystemEventType;
  is_internal: boolean;
  metadata?: string;
  via_channel?: 'email' | 'widget' | null;
  email_notified_at?: string;
  email_read_at?: string;
  cancellable_until?: string;
  attachments?: SupportAttachmentPayload[];
  /** Sanitized HTML body — present only for inbound email messages (via_channel === 'email'). */
  html_body?: string;
  /** Markdown-friendly plaintext body — present only for inbound email messages. */
  stripped_text?: string;
  /** Backend-projected visible reply text, excluding confidently detected history. */
  email_visible_text?: string;
  /** Backend-projected quoted history, retained for explicit expansion. */
  email_quoted_text?: string;
  /** Explicit quote-presence signal. Undefined means a legacy unprojected message. */
  email_has_quoted_content?: boolean;
  email_projection_confidence?: 'high' | 'medium' | 'none';
  email_projection_version?: number;
  /** Delivery status of the outbound email for this message. Only set when the message was sent via email. */
  email_delivery_status?: 'sent' | 'delivered' | 'opened' | 'bounced' | 'spam_complaint' | string;
  /** Human-readable bounce or complaint description. Empty unless delivery failed. */
  email_delivery_error?: string;
  /** Primary outbound email recipient, populated for messages sent via email. */
  email_to?: string;
  /** From header for inbound email messages, when present. */
  email_from?: string;
  /** Reply-To header for inbound email messages, when present. */
  email_reply_to?: string;
  /** Outbound email CC recipients, populated for messages sent via email. */
  email_cc?: string[];
  /** Outbound email BCC recipients, populated for messages sent via email. */
  email_bcc?: string[];
  created_at: string;
  updated_at: string;
}

export interface SupportMessageActionResponse {
  id: string;
  markdown?: string;
  email_already_sent: boolean;
}

export interface SupportMessageInfo {
  id: string;
  sent_at: string;
  sender: {
    id?: string;
    name: string;
    type: string;
    avatar_url?: string;
  };
  from: string;
  to_email?: string;
  cc_emails?: string[];
  bcc_emails?: string[];
  origin: string;
  type: string;
  email_delivery_status?: string;
  email_delivery_status_label?: string;
  delivered?: {
    channel: string;
    delivered_at: string;
  } | null;
  not_delivered_reason?: string | null;
  read: boolean;
  read_at?: string | null;
  edited: boolean;
  translated: boolean;
  automated: boolean;
}

export interface SupportMessageEmailDetail {
  id: string;
  message_id: string;
  direction: 'inbound' | 'outbound' | string;
  subject: string;
  from_email: string;
  to_email: string;
  reply_to?: string;
  cc_emails?: string[];
  bcc_emails?: string[];
  rfc_message_id?: string;
  in_reply_to?: string;
  references_header?: string;
  stripped_text?: string;
  html_body?: string;
  status: string;
  delivered_at?: string;
  opened_at?: string;
  bounced_at?: string;
  error_message?: string;
  created_at: string;
  forwarded_attribution?: SupportForwardedAttribution;
}

export interface SupportForwardedAttribution {
  original_sender_email: string;
  original_sender_name?: string;
  forwarded_by_email: string;
  forwarded_by_name?: string;
  confidence: number;
  confidence_level: string;
  source: string;
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
  issue_key: string;
  issue_summary: string;
  progress_signal: string;
  standalone_query: string;
  search_queries: string[];
  clarifying_question: string;
  greeting_reply: string;
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
    url?: string;
  }>;
  ai_confidence: number;
  ai_model: string;
  ai_tokens_used: number;
  ai_agent_id: string;
  ai_reply_kind?: string;
  ai_issue_key?: string;
  ai_issue_summary?: string;
  ai_progress_state?: string;
  link_previews?: SupportLinkPreview[];
}

export interface AgentKnowledgeSource {
  id: string;
  agent_id: string;
  scope_type?: 'space' | 'collection' | 'article';
  space_id: string;
  collection_id?: string | null;
  document_id?: string | null;
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
  collection_name?: string;
  document_title?: string;
  created_at: string;
  updated_at: string;
}

export type CuratedGuidanceIntent = 'pricing_general' | 'plan_recommendation' | 'billing_tax' | 'unknown';
export type CuratedGuidanceStatus = 'active' | 'disabled';

export interface CuratedGuidance {
  id: string;
  workspace_id: string;
  agent_id: string;
  title: string;
  question_patterns: string[];
  answer: string;
  intent: CuratedGuidanceIntent;
  topics: string[];
  language: string;
  status: CuratedGuidanceStatus;
  valid_from?: string | null;
  valid_until?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateCuratedGuidanceRequest {
  title: string;
  question_patterns: string[];
  answer: string;
  intent: CuratedGuidanceIntent;
  topics: string[];
  language: string;
  valid_from?: string | null;
  valid_until?: string | null;
}

export interface UpdateCuratedGuidanceRequest extends Partial<CreateCuratedGuidanceRequest> {
  status?: CuratedGuidanceStatus;
}

export interface AgentKnowledgeSourceRequest {
  scope_type: 'space' | 'collection' | 'article';
  space_id: string;
  collection_id?: string | null;
  document_id?: string | null;
}

export interface SupportContentSource {
  id: string;
  workspace_id: string;
  name: string;
  source_type?: 'website' | 'file';
  start_url: string;
  file_name?: string | null;
  file_size?: number;
  content_type?: string | null;
  storage_key?: string | null;
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

export interface CreateSupportContentSourceFileUploadRequest {
  name: string;
  file_name: string;
  file_size: number;
  content_type: string;
}

export interface CreateSupportContentSourceFileUploadResponse {
  source: SupportContentSource;
  upload_url: string;
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
  ai_assisted?: boolean;
  channels?: ('chat' | 'email')[];
  cc_emails?: string[];
  bcc_emails?: string[];
  attachment_ids?: string[];
}

export interface CreateConversationWithMessageRequest {
  mailbox_id?: string | null;
  subject: string;
  customer_name?: string;
  customer_email?: string;
  crm_contact_id?: string;
  channels?: ('chat' | 'email')[];
  content: string;
  attachment_ids?: string[];
  tag_ids?: string[];
  cc_emails?: string[];
  bcc_emails?: string[];
}

export interface CreateConversationWithMessageResponse {
  conversation: SupportConversation;
  message: SupportMessage;
}

/** @deprecated Use LinkTaskRequest instead */
export interface LinkStoryRequest {
  story_id: string;
}

export interface LinkTaskRequest {
  task_id: string;
}

export interface CreateTaskFromConversationRequest {
  team_id: string;
  name?: string;
  description?: string;
  task_type?: 'feature' | 'bug' | 'chore';
  workflow_id?: string;
  workflow_state_id?: string;
  epic_id?: string;
  sprint_id?: string;
  owner_member_id?: string;
  requester_member_id?: string;
  estimate?: number;
  priority?: 'none' | 'low' | 'medium' | 'high' | 'urgent';
  severity?: 'none' | 'minor' | 'major' | 'critical';
  deadline?: string;
  position?: number;
  blocked?: boolean;
  blocker?: string;
  template_id?: string;
  external_id?: string;
  owner_ids?: string[];
  follower_ids?: string[];
  label_ids?: string[];
  attachment_ids?: string[];
  checklist_items?: { text: string; position?: number }[];
  external_links?: { url: string; title?: string }[];
}

export interface CreateTaskFromConversationResponse {
  task_id: string;
  display_id: number;
  task_key: string;
  task_name: string;
  summary?: string;
  copied_contact_associations: number;
  copied_company_associations: number;
  copied_deal_associations: number;
}

export interface AssignConversationAgentRequest {
  agent_id: string;
}

export interface AssignConversationUserRequest {
  user_id: string | null;
}

export interface UpdateConversationCRMContactRequest {
  crm_contact_id: string | null;
}

export interface UpdateConversationCRMCompanyRequest {
  crm_company_id: string | null;
}

export interface UpdateConversationCustomerNameRequest {
  customer_name: string;
}

export interface UpdateConversationEmailRecipientsRequest {
  primary_recipient_email?: string;
  primary_recipient_name?: string;
  cc_emails?: string[];
  confirm_primary?: boolean;
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
  escalation_message_busy?: string;
  escalation_message_after_hours?: string;
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
  reply_time_preset?: string;
  reply_time_custom_minutes?: number | null;
  special_notice_text?: string | null;
  email_fallback_enabled: boolean;
  email_fallback_delay_secs: number;
  email_fallback_from_name: string;
  email_fallback_max_delivery_age_secs: number;
  forwarded_email_detection_enabled: boolean;
  forwarded_email_detection_mode: string;
  forwarded_email_min_confidence: number;
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

export interface SupportRoutingUsageStatus {
  triage_enabled: boolean;
  daily_budget: number;
  used_today: number;
  remaining_today?: number | null;
  reset_at: string;
  exhausted: boolean;
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
