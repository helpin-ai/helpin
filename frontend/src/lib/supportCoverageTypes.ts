// --- Gap types ---

export type SupportCoverageV1GapType =
  | 'missing_article'
  | 'weak_article'
  | 'outdated_or_conflicting_article'
  | 'needs_review'

export type SupportCoverageGapStatus =
  | 'open'
  | 'drafted'
  | 'fixed'
  | 'ignored'
  | 'merged'
  | 'human_only'

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
  failure_mode: string
  source_signal: string
  first_seen_at: string
  last_seen_at: string
  topic_title: string
  suggestion_count: number
  related_article_id: string | null
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
  created_at: string
}

export interface SupportGapSuggestion {
  id: string
  gap_id: string
  suggestion_type: string
  status: string
  title: string
  content: unknown
  evidence_summary: string
  target_space_id: string | null
  target_document_id: string | null
  result_document_id: string | null
  applied_at: string | null
  created_at: string
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
  failure_mode: string
  first_seen_at: string
  last_seen_at: string
  status_changed_by: string | null
  status_changed_at: string | null
  status_changed_by_name: string
  issue_resolved: boolean | null
  topic_title: string
  evidence: SupportGapEvidence[]
  suggestions: SupportGapSuggestion[]
  related_articles: { id: string; gap_id: string; document_id: string; article_title: string }[]
}

export interface SupportCoverageSummary {
  new_gaps_this_week: number
  top_recurring_gaps: number
  gaps_fixed_this_week: number
  total_open_gaps: number
  total_evidence_count: number
  handoffs_after_fixes: number
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
  needs_review: 'Needs Review',
}

export const GAP_STATUS_LABELS: Record<SupportCoverageGapStatus, string> = {
  open: 'Open',
  drafted: 'Drafted',
  fixed: 'Fixed',
  ignored: 'Ignored',
  merged: 'Merged',
  human_only: 'Requires Human',
}
