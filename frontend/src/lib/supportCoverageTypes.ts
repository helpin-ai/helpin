// --- Gap types ---

export type SupportCoverageV1GapType =
  | 'missing_article'
  | 'weak_article'
  | 'outdated_or_conflicting_article'
  | 'needs_review'

export type SupportCoverageGapStatus =
  | 'open'
  | 'done'
  | 'rejected'

// --- DTOs ---

export interface SupportCoverageGapListItem {
  id: string
  workspace_id: string
  topic_id: string | null
  dedupe_key: string
  gap_category: string
  v1_gap_type: SupportCoverageV1GapType
  title: string
  issue_key: string
  status: SupportCoverageGapStatus
  confidence: number
  evidence_count: number
  evidence_30d: number
  evidence_all?: number
  distinct_customers_30d?: number
  distinct_customers_all?: number
  impact_score?: number
  impact_explanation?: string
  impact_tier: 'low' | 'medium' | 'high'
  gap_kind: string
  closed_at: string | null
  closed_evidence_count: number | null
  result_document_id: string | null
  rejection_reason: string | null
  failure_mode: string
  source_signal: string
  first_seen_at: string
  last_seen_at: string
  topic_title: string
  canonical_title: string
  suggestion_count: number
  related_article_id: string | null
  nearest_content_score?: number
  nearest_content_title?: string
  embedding_provider?: string
  embedding_model?: string
  embedding_version?: string
  embedding_updated_at?: string | null
}

export interface SupportGapEvidence {
  id: string
  gap_id: string
  evidence_type: string
  conversation_id: string | null
  message_id: string | null
  document_id: string | null
  source_signal: string
  excerpt: string
  sender_role: string
  created_at: string
}

export interface SupportGapSuggestion {
  id: string
  gap_id: string
  suggestion_type: string
  status: string
  is_active: boolean
  superseded_at: string | null
  title: string
  content: unknown
  evidence_summary: string
  target_space_id: string | null
  target_document_id: string | null
  target_document_title?: string | null
  result_document_id: string | null
  applied_at: string | null
  created_at: string
}

export interface SupportCoverageAnalysisExplanation {
  customer_need: string
  ai_failure: string
  human_resolution: string
  decision_reason: string
}

export interface SupportCoverageRecommendation {
  id: string
  workspace_id: string
  gap_id: string
  analysis_id: string | null
  recommendation_type: string
  target_type: string
  target_id: string | null
  target_title: string
  target_url: string
  priority: 'primary' | 'secondary' | string
  status: 'open' | 'accepted' | 'dismissed' | 'applied' | string
  rationale: string
  suggested_change: string
  implementation_notes: string
  suggestion_id: string | null
  created_at: string
  updated_at: string
}

export interface SupportCoverageGapDetail {
  id: string
  workspace_id: string
  v1_gap_type: SupportCoverageV1GapType
  title: string
  issue_key: string
  status: SupportCoverageGapStatus
  confidence: number
  evidence_count: number
  evidence_30d?: number
  evidence_all?: number
  distinct_customers_30d?: number
  distinct_customers_all?: number
  impact_score?: number
  impact_explanation?: string
  gap_kind: string
  closed_at: string | null
  closed_evidence_count: number | null
  result_document_id: string | null
  rejection_reason: string | null
  failure_mode: string
  nearest_content_score?: number
  nearest_content_title?: string
  embedding_provider?: string
  embedding_model?: string
  embedding_version?: string
  embedding_updated_at?: string | null
  first_seen_at: string
  last_seen_at: string
  status_changed_by: string | null
  status_changed_at: string | null
  status_changed_by_name: string
  issue_resolved: boolean | null
  topic_title: string
  analysis_explanation?: SupportCoverageAnalysisExplanation | null
  recommendations: SupportCoverageRecommendation[]
  evidence: SupportGapEvidence[]
  suggestions: SupportGapSuggestion[]
  related_articles: { id: string; gap_id: string; document_id: string; article_title: string }[]
}

export interface SupportCoverageClusterRebuildRun {
  id: string
  workspace_id: string
  status: 'running' | 'completed' | 'failed' | string
  gaps_scanned: number
  clusters_found: number
  auto_merged: number
  suggestions_created: number
  skipped: number
  error_message: string | null
  metadata?: Record<string, unknown> | null
  started_at: string
  completed_at: string | null
}

export interface SupportCoverageClusterRebuildResult {
  run_id: string
  status: string
  gaps_scanned: number
  clusters_found: number
  auto_merged: number
  suggestions_created: number
  skipped: number
  embedding_status?: string
  embedding_error?: string
  embeddings_created?: number
  missing_embeddings?: number
  started_at: string
  completed_at?: string | null
}

export interface SupportCoverageGapMergeSuggestion {
  id: string
  workspace_id: string
  run_id: string | null
  source_gap_id: string
  target_gap_id: string
  status: 'pending' | 'applied' | 'dismissed' | string
  similarity_score: number
  reason: string
  combined_evidence_count: number
  source_gap?: {
    id: string
    title: string
    evidence_count: number
  } | null
  target_gap?: {
    id: string
    title: string
    evidence_count: number
  } | null
  created_at: string
}

export interface SupportCoverageSummary {
  new_gaps_this_week: number
  top_recurring_gaps: number
  gaps_fixed_this_week: number
  total_open_gaps: number
  total_evidence_count: number
  handoffs_after_fixes: number
  last_analyzed_at?: string | null
}

export interface SupportConversationCoverageState {
  docs_issue_feedback_submitted: boolean
  docs_issue_value: boolean | null
  gap_id: string | null
}

export interface SupportCoverageGapListResponse {
  items: SupportCoverageGapListItem[]
  total: number
}

export const V1_GAP_TYPE_LABELS: Record<SupportCoverageV1GapType, string> = {
  missing_article: 'Missing Article',
  weak_article: 'Weak Article',
  outdated_or_conflicting_article: 'Outdated / Conflicting',
  needs_review: 'Unclassified',
}

// Gap kind is the high-level grouping (content, data, action/policy).
// gap_category is the granular sub-type — we map it to one of 3 kinds for display.
export const GAP_KIND_LABELS: Record<string, string> = {
  content: 'Content',
  data: 'Data',
  action: 'Action',
  policy: 'Action',
}

export const GAP_KIND_DESCRIPTIONS: Record<string, string> = {
  content: 'Missing or incomplete docs and knowledge',
  data: 'AI lacked customer or account data',
  action: 'AI couldn\'t perform the required operation',
}

export const GAP_KIND_COLORS: Record<string, { bg: string; text: string; border: string; dot: string }> = {
  content: { bg: 'bg-blue-50 dark:bg-blue-950/30', text: 'text-blue-700 dark:text-blue-400', border: 'border-blue-200 dark:border-blue-800', dot: 'bg-blue-500' },
  data: { bg: 'bg-amber-50 dark:bg-amber-950/30', text: 'text-amber-700 dark:text-amber-400', border: 'border-amber-200 dark:border-amber-800', dot: 'bg-amber-500' },
  action: { bg: 'bg-purple-50 dark:bg-purple-950/30', text: 'text-purple-700 dark:text-purple-400', border: 'border-purple-200 dark:border-purple-800', dot: 'bg-purple-500' },
  policy: { bg: 'bg-purple-50 dark:bg-purple-950/30', text: 'text-purple-700 dark:text-purple-400', border: 'border-purple-200 dark:border-purple-800', dot: 'bg-purple-500' },
}

// Resolve gap_kind to display kind (policy → action).
export function resolveGapKindDisplay(gapKind: string): string {
  if (gapKind === 'policy') return 'action'
  return gapKind || 'content'
}

export const GAP_STATUS_LABELS: Record<SupportCoverageGapStatus, string> = {
  open: 'Open',
  done: 'Done',
  rejected: 'Rejected',
}
