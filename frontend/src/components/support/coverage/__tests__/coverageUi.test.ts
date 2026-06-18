import { describe, expect, it } from 'vitest'
import type { SupportCoverageGapDetail, SupportGapSuggestion } from '@/lib/supportCoverageTypes'
import {
  coverageConfidenceLabel,
  coverageDiagnosis,
  coverageImpactTier,
  coverageKbSignal,
  coveragePrimaryAddLabel,
  coverageSuggestionPreview,
  formatCoverageImpact,
} from '../coverageUi'

describe('coverage UI helpers', () => {
  it('labels confidence tiers', () => {
    expect(coverageConfidenceLabel(0.8).text).toBe('High confidence')
    expect(coverageConfidenceLabel(0.5).text).toBe('Medium confidence')
    expect(coverageConfidenceLabel(0.1).text).toBe('Low confidence')
  })

  it('extracts readable suggestion preview from TipTap JSON', () => {
    const suggestion = {
      content: {
        type: 'doc',
        content: [
          { type: 'heading', content: [{ type: 'text', text: 'Refund policy' }] },
          { type: 'paragraph', content: [{ type: 'text', text: 'Refunds take 5 days.' }] },
        ],
      },
      evidence_summary: 'Fallback',
    } as SupportGapSuggestion

    expect(coverageSuggestionPreview(suggestion)).toBe('REFUND POLICY\n\nRefunds take 5 days.')
  })

  it('uses linked article title for update-route primary add label', () => {
    const gap = {
      related_articles: [{ document_id: 'doc-1', article_title: 'Billing FAQ' }],
    } as SupportCoverageGapDetail
    const suggestion = { suggestion_type: 'update_article' } as SupportGapSuggestion

    expect(coveragePrimaryAddLabel(gap, suggestion)).toBe('Add to "Billing FAQ"')
  })

  it('formats impact and KB proximity signals', () => {
    expect(formatCoverageImpact({
      impact_score: 18.4,
      impact_explanation: '12 conversations, 5 customers this month, no nearby content',
    })).toContain('5 customers')
    expect(coverageKbSignal({ nearest_content_score: 0, failure_mode: '' })).toBe('No nearby content')
    expect(coverageKbSignal({ nearest_content_score: 0.7, failure_mode: 'no_retrieval' })).toBe('Retrieval issue likely')
  })

  it('singularizes a one-conversation impact line', () => {
    expect(formatCoverageImpact({ evidence_all: 1, distinct_customers_30d: 1, failure_mode: 'missing_content' }))
      .toBe('1 conversation, 1 customer this month, No nearby content')
  })

  it('derives impact tier from recent (30d) evidence, matching backend cutoffs', () => {
    expect(coverageImpactTier(12)).toBe('high')
    expect(coverageImpactTier(5)).toBe('medium')
    expect(coverageImpactTier(1)).toBe('low')
    expect(coverageImpactTier(undefined)).toBe('low')
  })

  it('produces a plain-language diagnosis driven by failure mode', () => {
    expect(coverageDiagnosis({ failure_mode: 'missing_content', gap_kind: 'content', v1_gap_type: 'missing_article' }))
      .toBe('No article in your knowledge base covers this yet.')
    expect(
      coverageDiagnosis({
        failure_mode: 'no_retrieval',
        gap_kind: 'content',
        v1_gap_type: 'weak_article',
        nearest_content_title: 'SSO setup',
      }),
    ).toBe('“SSO setup” likely answers this, but the AI didn’t surface it.')
    expect(coverageDiagnosis({ failure_mode: 'data_missing', gap_kind: 'data', v1_gap_type: 'needs_review' }))
      .toBe('The AI lacked the customer or account data needed to answer this.')
  })

  it('omits the diagnosis when the gap is genuinely unclassified', () => {
    expect(coverageDiagnosis({ failure_mode: '', gap_kind: 'content', v1_gap_type: 'needs_review' })).toBeNull()
  })
})
