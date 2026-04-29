import { describe, expect, it } from 'vitest'
import type { SupportGapEvidence } from '@/lib/supportCoverageTypes'
import {
  groupEvidenceByConversation,
  isOurSenderRole,
} from '../evidenceGrouping'

function ev(overrides: Partial<SupportGapEvidence>): SupportGapEvidence {
  return {
    id: 'e',
    gap_id: 'g',
    evidence_type: 'ai_handoff_triggered',
    conversation_id: null,
    message_id: null,
    document_id: null,
    source_signal: '',
    excerpt: '',
    sender_role: '',
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('groupEvidenceByConversation', () => {
  it('collapses repeated conversation_id into a single group', () => {
    const groups = groupEvidenceByConversation([
      ev({ id: 'a', conversation_id: 'c1' }),
      ev({ id: 'b', conversation_id: 'c1' }),
      ev({ id: 'c', conversation_id: 'c1' }),
    ])
    expect(groups).toHaveLength(1)
    expect(groups[0]).toMatchObject({ kind: 'conversation', conversationId: 'c1' })
    if (groups[0].kind === 'conversation') {
      expect(groups[0].items.map((e) => e.id)).toEqual(['a', 'b', 'c'])
    }
  })

  it('keeps non-conversation evidence as standalone groups', () => {
    const groups = groupEvidenceByConversation([
      ev({ id: 'a', document_id: 'd1' }),
      ev({ id: 'b', conversation_id: 'c1' }),
      ev({ id: 'c', document_id: 'd2' }),
    ])
    expect(groups.map((g) => g.kind)).toEqual(['standalone', 'conversation', 'standalone'])
    if (groups[0].kind === 'standalone') expect(groups[0].item.id).toBe('a')
    if (groups[2].kind === 'standalone') expect(groups[2].item.id).toBe('c')
  })

  it('preserves first-seen order between conversation groups', () => {
    const groups = groupEvidenceByConversation([
      ev({ id: 'a', conversation_id: 'c1' }),
      ev({ id: 'b', conversation_id: 'c2' }),
      ev({ id: 'c', conversation_id: 'c1' }),
      ev({ id: 'd', conversation_id: 'c2' }),
    ])
    expect(groups).toHaveLength(2)
    if (groups[0].kind === 'conversation') {
      expect(groups[0].conversationId).toBe('c1')
      expect(groups[0].items.map((e) => e.id)).toEqual(['a', 'c'])
    }
    if (groups[1].kind === 'conversation') {
      expect(groups[1].conversationId).toBe('c2')
      expect(groups[1].items.map((e) => e.id)).toEqual(['b', 'd'])
    }
  })
})

describe('isOurSenderRole', () => {
  it('treats user / agent / ai as ours (right-aligned)', () => {
    expect(isOurSenderRole('user')).toBe(true)
    expect(isOurSenderRole('agent')).toBe(true)
    expect(isOurSenderRole('ai')).toBe(true)
  })

  it('treats customer, empty, and unknown as the customer side', () => {
    expect(isOurSenderRole('customer')).toBe(false)
    expect(isOurSenderRole('')).toBe(false)
    expect(isOurSenderRole('something-else')).toBe(false)
  })
})
