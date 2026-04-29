import type { SupportCoverageGapDetail, SupportGapSuggestion } from '@/lib/supportCoverageTypes'

export const EVIDENCE_TYPE_LABELS: Record<string, string> = {
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
  if (confidence >= 0.7) return { text: 'High confidence', className: 'text-green-600' }
  if (confidence >= 0.4) return { text: 'Medium confidence', className: 'text-amber-600' }
  return { text: 'Low confidence', className: 'text-muted-foreground' }
}

export function coverageTopicLabel(issueKey: string): string {
  if (!issueKey) return 'Unknown'
  return issueKey.replace(/[_-]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export function coveragePrimaryAddLabel(
  gap: SupportCoverageGapDetail,
  suggestion?: SupportGapSuggestion | null,
): string {
  if (suggestion?.suggestion_type === 'update_article') {
    const articleTitle = gap.related_articles[0]?.article_title
    return articleTitle ? `Add to "${articleTitle}"` : 'Add to article'
  }
  return 'Create new article'
}
