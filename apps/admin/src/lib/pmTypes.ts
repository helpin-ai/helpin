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
