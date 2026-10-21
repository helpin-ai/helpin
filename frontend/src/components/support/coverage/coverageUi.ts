import type { SupportCoverageGapDetail, SupportCoverageGapListItem, SupportGapSuggestion } from '@/lib/supportCoverageTypes'

export const EVIDENCE_TYPE_LABELS: Record<string, string> = {
  ai_answer_feedback: 'Visitor Feedback',
  ai_handoff_triggered: 'AI Handoff',
  article_feedback_submitted: 'Article Feedback',
  widget_search_performed: 'Widget Search',
  docs_issue_feedback: 'Agent Feedback',
  human_reply_after_ai: 'Human Reply',
  daily_conversation_analysis: 'Daily Analysis',
}

export function evidenceTypeLabel(type: string): string {
  return EVIDENCE_TYPE_LABELS[type] ?? type
}

type ExtractedNode = { type?: string; text?: string; content?: ExtractedNode[] }

function extractTextFromTipTap(content: unknown): string {
  if (!content || typeof content !== 'object') return ''
  const node = content as ExtractedNode
  if (node.text) return node.text
  if (Array.isArray(node.content)) {
    return node.content.map((child) => extractTextFromTipTap(child)).join('')
  }
  return ''
}

export function coverageSuggestionPreview(suggestion: SupportGapSuggestion): string {
  if (!suggestion.content) return suggestion.evidence_summary || ''
  try {
    const node = suggestion.content as ExtractedNode
    if (!node.content) return ''
    const blocks: string[] = []
    for (const block of node.content) {
      const text = extractTextFromTipTap(block)
      if (!text) continue
      blocks.push(block.type === 'heading' ? text.toUpperCase() : text)
    }
    return blocks.join('\n\n')
  } catch {
    return ''
  }
}

export function coverageConfidenceLabel(confidence: number): { text: string; className: string } {
  if (confidence >= 0.7) return { text: 'High confidence', className: 'text-green-600 dark:text-green-400' }
  if (confidence >= 0.4) return { text: 'Medium confidence', className: 'text-amber-600' }
  return { text: 'Low confidence', className: 'text-muted-foreground' }
}

export function coverageTopicLabel(issueKey: string): string {
  if (!issueKey) return 'Unknown'
  return issueKey.replace(/[_-]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export function formatCoverageImpact(gap: Partial<SupportCoverageGapListItem | SupportCoverageGapDetail>): string {
  if (gap.impact_explanation) return gap.impact_explanation
  const conversations = gap.conversations_30d
  if (conversations !== undefined) {
    const parts = [`${conversations} ${conversations === 1 ? 'conversation' : 'conversations'}`]
    const customers = gap.distinct_customers_30d ?? 0
    if (customers > 0) parts.push(`${customers} known ${customers === 1 ? 'customer' : 'customers'}`)
    return parts.join(' · ') + ' in the last 30 days'
  }
  const records = gap.evidence_all ?? gap.evidence_count ?? 0
  return `${records} ${records === 1 ? 'evidence record' : 'evidence records'}`
}

// Mirrors the backend ImpactTier(evidence30d) cutoffs so the detail drawer can show a
// tier without a dedicated payload field (the detail response omits impact_tier).
export function coverageImpactTier(evidence30d?: number): 'low' | 'medium' | 'high' {
  const recent = evidence30d ?? 0
  if (recent >= 10) return 'high'
  if (recent >= 3) return 'medium'
  return 'low'
}

// Plain-language diagnosis driven by the failure classification. Returns null when the gap
// is genuinely unclassified (needs_review with no KB signal) so the UI can omit the line
// rather than assert something it can't back up.
export function coverageDiagnosis(
  gap: Pick<
    SupportCoverageGapDetail,
    'failure_mode' | 'gap_kind' | 'v1_gap_type' | 'nearest_content_title'
  >,
): string | null {
  const article = gap.nearest_content_title?.trim()
  const quoted = article ? `“${article}”` : ''

  if (gap.gap_kind === 'data') {
    return 'The AI lacked the customer or account data needed to answer this.'
  }
  if (gap.gap_kind === 'policy') {
    return 'The AI needs an agreed policy or escalation rule to answer this safely.'
  }
  if (gap.gap_kind === 'action') {
    return 'The AI couldn’t perform the action this request needs.'
  }

  switch (gap.failure_mode) {
    case 'missing_content':
      return 'No article in your knowledge base covers this yet.'
    case 'no_retrieval':
      return article
        ? `${quoted} likely answers this, but the AI didn’t surface it.`
        : 'Relevant content may exist, but the AI didn’t retrieve it.'
    case 'weak_retrieval':
      return article
        ? `${quoted} is close but doesn’t fully answer this.`
        : 'The closest content isn’t strong enough to resolve this.'
  }

  switch (gap.v1_gap_type) {
    case 'missing_article':
      return 'No article in your knowledge base covers this yet.'
    case 'weak_article':
      return article
        ? `${quoted} is close but doesn’t fully answer this.`
        : 'The closest article doesn’t fully cover this.'
    case 'outdated_or_conflicting_article':
      return article
        ? `${quoted} may be outdated or conflicting.`
        : 'Existing guidance may be outdated or conflicting.'
  }

  return null
}

export function coverageKbSignal(gap: Partial<SupportCoverageGapListItem | SupportCoverageGapDetail>): string {
  const score = gap.nearest_content_score ?? 0
  if (gap.failure_mode === 'missing_content') return 'No nearby content'
  if (score <= 0) return 'No nearby content'
  if (gap.failure_mode === 'no_retrieval' || gap.failure_mode === 'weak_retrieval') {
    return 'Retrieval issue likely'
  }
  return 'Existing article nearby'
}

export function coveragePrimaryAddLabel(
  gap: SupportCoverageGapDetail,
  suggestion?: SupportGapSuggestion | null,
): string {
  if (suggestion?.suggestion_type === 'update_article') {
    const articleTitle = suggestion.target_document_title || gap.related_articles.find(article => article.document_id === suggestion.target_document_id)?.article_title || (!suggestion.target_document_id ? gap.related_articles[0]?.article_title : undefined)
    return articleTitle ? `Add to "${articleTitle}"` : 'Add to article'
  }
  return 'Create new article'
}
