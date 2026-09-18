export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai'

export interface Agent {
  id: string
  workspace_id: string
  name: string
  preset_key?: string
  allowed_targets?: string[]
}

export interface SupportAIPreviewHistoryTurn {
  sender_type: MessageSenderType
  message_type?: string
  content: string
}

export interface SupportAIPreviewRequest {
  message: string
  conversation_id?: string
  history?: SupportAIPreviewHistoryTurn[]
  include_answer?: boolean
  max_results?: number
}

export interface SupportAIPreviewQueryPlan {
  decision: string
  standalone_query: string
  search_queries: string[]
  clarifying_question: string
  reason: string
  tokens_used: number
  fallback_used: boolean
  error?: string
}

export interface SupportAIPreviewSearchResult {
  reference_id: string
  source_type: string
  title: string
  url?: string
  chunk_index: number
  combined_score: number
  vector_score: number
  lexical_score: number
  snippet: string
}

export interface SupportAIPreviewRetrieval {
  query_count: number
  result_count: number
  results: SupportAIPreviewSearchResult[]
  error?: string
}

export interface SupportAIPreviewAnswer {
  content: string
  can_answer: boolean
  source_doc_ids: string[]
  llm_confidence: number
  grounded_confidence: number
  tokens_used: number
  provider: string
  model: string
}

export interface SupportAIPreviewResponse {
  run_id?: string
  status?: string
  provider?: string
  model?: string
  profile_id?: string
  excluded_tools?: string[]
  conversation_source: string
  confidence_threshold: number
  total_tokens_used: number
  final_decision: string
  final_reason: string
  query_plan: SupportAIPreviewQueryPlan
  retrieval: SupportAIPreviewRetrieval
  answer?: SupportAIPreviewAnswer
}

export interface SupportInboxSettings {
  ai_agent_id: string | null
}

export interface SupportInstallationResponse {
  id: string
  workspace_id: string
  widget_key: string
  settings: SupportInboxSettings
  active: boolean
  created_at: string
  updated_at: string
}

// Webhook events
export interface WebhookEvent {
  id: string
  workspace_id?: string
  conversation_id?: string
  email_log_id?: string
  provider: string
  event_type: string
  postmark_message_id?: string
  message_stream?: string
  raw_payload: string
  received_at?: string
  created_at: string
}

export interface WebhookEventListResponse {
  data: WebhookEvent[]
  page: number
  per_page: number
  total: number
  total_pages: number
}

// Email diagnostics
export interface EmailDiagnosticsConfig {
  app_email_configured: boolean
  reply_email_configured: boolean
  route_email_configured: boolean
  redis_configured: boolean
  fallback_poller_enabled: boolean
  app_from_email?: string
  reply_from_email?: string
  verified_fallback_from_email?: string
  support_email_reply_domain: string
  support_email_route_domain: string
  reply_inbound_secret_set: boolean
  route_inbound_secret_set: boolean
  expected_fallback_from_shape: string
  expected_branded_from_shape: string
  expected_reply_to_shape: string
  outbound_from_behavior: string
}

export interface EmailLogCount {
  direction: string
  status: string
  count: number
}

export interface SupportEmailLog {
  id: string
  workspace_id: string
  conversation_id: string
  email_route_id?: string
  direction: string
  message_ids?: string[]
  from_email: string
  to_email: string
  recipient_address?: string
  subject: string
  rfc_message_id: string
  in_reply_to?: string
  references_header?: string
  postmark_message_id?: string
  stripped_text?: string
  html_body?: string
  status: string
  delivered_at?: string
  opened_at?: string
  bounced_at?: string
  error_message?: string
  created_at: string
}

export interface EmailDiagnosticsResponse {
  config: EmailDiagnosticsConfig
  queue: EmailQueueResponse
  recent_logs: SupportEmailLog[]
  log_counts: EmailLogCount[]
  recent_webhooks: WebhookEvent[]
}

export interface EmailFallbackConversationSettingsSummary {
  email_fallback_enabled: boolean
  email_fallback_delay_secs: number
  email_fallback_max_delivery_age_secs: number
}

export interface EmailFallbackConversationQueueSummary {
  queued: boolean
  fire_at?: string
  message_ids?: string[]
  redis_checked: boolean
  redis_error?: string
  delay_remaining_secs: number
}

export interface EmailFallbackMessageDiagnostics {
  id: string
  created_at: string
  sender_type: string
  message_type: string
  is_internal: boolean
  content_preview?: string
  cancellable_until?: string
  email_notified_at?: string
  email_read_at?: string
  email_log_id?: string
  email_log_status?: string
  postmark_message_id?: string
  eligible: boolean
  queued: boolean
  due: boolean
  reconcile_candidate: boolean
  reasons: string[]
}

export interface EmailFallbackConversationDiagnosticsResponse {
  conversation_id: string
  workspace_id: string
  subject: string
  status: string
  customer_email?: string
  email_unsubscribed: boolean
  contact_last_seen_at?: string
  visitor_online: boolean
  settings: EmailFallbackConversationSettingsSummary
  queue: EmailFallbackConversationQueueSummary
  messages: EmailFallbackMessageDiagnostics[]
}

// Email queue
export interface EmailQueueMessage {
  id: string
  content: string
  sender_display_name?: string
  created_at: string
}

export interface EmailQueueEntry {
  conversation_id: string
  workspace_id: string
  fire_at: string
  delay_remaining_secs: number
  customer_email: string
  customer_name?: string
  subject: string
  status: string
  message_count: number
  messages: EmailQueueMessage[]
}

export interface EmailQueueResponse {
  entries: EmailQueueEntry[]
  total: number
}
