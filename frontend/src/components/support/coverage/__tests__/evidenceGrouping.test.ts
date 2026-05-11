import { describe, expect, it } from 'vitest'
import type { SupportGapEvidence } from '@/lib/supportCoverageTypes'
import {
  conversationCardCountText,
  conversationCardHeaderLabel,
  groupEvidenceByConversation,
  isMessageEvidence,
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

describe('isMessageEvidence', () => {
  it('returns true only when message_id is set', () => {
    expect(isMessageEvidence(ev({ message_id: 'm1' }))).toBe(true)
    expect(isMessageEvidence(ev({ message_id: null }))).toBe(false)
    expect(isMessageEvidence(ev({ conversation_id: 'c1', message_id: null }))).toBe(false)
  })
})

describe('conversationCardHeaderLabel', () => {
  it('uses the default label when all items are messages', () => {
    const items = [ev({ id: 'a', message_id: 'm1' }), ev({ id: 'b', message_id: 'm2' })]
    expect(conversationCardHeaderLabel(items, 'AI Handoff')).toBe('AI Handoff')
  })

  it('uses the default label when all items are events', () => {
    const items = [ev({ id: 'a' }), ev({ id: 'b' })]
    expect(conversationCardHeaderLabel(items, 'Agent Feedback')).toBe('Agent Feedback')
  })

  it('falls back to a generic label when items are mixed', () => {
    const items = [ev({ id: 'a', message_id: 'm1' }), ev({ id: 'b' })]
    expect(conversationCardHeaderLabel(items, 'AI Handoff')).toBe('Conversation evidence')
  })
})

describe('conversationCardCountText', () => {
  it('counts messages when messages dominate', () => {
    const items = [
      ev({ id: 'a', message_id: 'm1' }),
      ev({ id: 'b', message_id: 'm2' }),
      ev({ id: 'c' }),
    ]
    expect(conversationCardCountText(items)).toBe('2 messages')
  })

  it('counts events only when there are no messages and more than one event', () => {
    const items = [ev({ id: 'a' }), ev({ id: 'b' })]
    expect(conversationCardCountText(items)).toBe('2 events')
  })

  it('returns empty string for a single item', () => {
    expect(conversationCardCountText([ev({ id: 'a', message_id: 'm1' })])).toBe('')
    expect(conversationCardCountText([ev({ id: 'a' })])).toBe('')
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
