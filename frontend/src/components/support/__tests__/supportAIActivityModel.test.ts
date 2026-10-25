import { describe, expect, it } from 'vitest'
import type { SupportMessage } from '@/lib/pmTypes'
import { foldedSupportAICardIds } from '../supportAIActivityModel'

function note(id: string, createdAt: string, metadata: Record<string, unknown>): SupportMessage {
  return {
    id, workspace_id: 'ws', conversation_id: 'conv', sender_type: 'agent', message_type: 'note', is_internal: true,
    content: '', metadata: JSON.stringify(metadata), created_at: createdAt, updated_at: createdAt,
  }
}

describe('foldedSupportAICardIds', () => {
  it('folds AI cards from before the latest handoff', () => {
    const messages = [
      note('handoff-1', '2026-10-25T09:00:00Z', { ai_handoff_brief: true, handoff: { reason: 'x' } }),
      note('check-1', '2026-10-25T09:00:01Z', { code_check: { status: 'done' } }),
      note('teammate', '2026-10-25T09:30:00Z', {}),
      note('handoff-2', '2026-10-25T10:00:00Z', { ai_handoff_brief: true, handoff: { reason: 'y' } }),
      note('check-2', '2026-10-25T10:00:01Z', { code_check: { status: 'running' } }),
    ]
    expect([...foldedSupportAICardIds(messages)].sort()).toEqual(['check-1', 'handoff-1'])
  })

  it('folds nothing with a single handoff', () => {
    expect(foldedSupportAICardIds([note('handoff', '2026-10-25T09:00:00Z', { ai_handoff_brief: true })]).size).toBe(0)
  })
})
