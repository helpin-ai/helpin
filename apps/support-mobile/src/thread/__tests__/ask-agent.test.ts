import { describe, expect, it } from 'vitest'
import { starterSuggestionsForContext } from '@/components/agents/dock/starterSuggestions'
import type { SupportConversation } from '@helpin-ai/support-core'
import {
  SUPPORT_AGENT_STARTERS,
  buildSupportDockPageContext,
  newSupportDockClientMessageId,
  supportAgentRunLabel,
} from '../ask-agent'

const conversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: ' Refund request ', status: 'open', priority: 'medium', source: 'widget',
  customer_name: 'Taylor', created_at: '2026-08-20T10:00:00Z', updated_at: '2026-08-20T10:00:00Z',
} satisfies SupportConversation

describe('Ask Agent mobile parity', () => {
  it('keeps the web support starter prompts exactly aligned', () => {
    expect(SUPPORT_AGENT_STARTERS).toEqual(starterSuggestionsForContext('support_conversation'))
  })

  it('builds the same support conversation context contract', () => {
    expect(buildSupportDockPageContext(conversation)).toEqual({
      entity_type: 'support_conversation', entity_id: 'conv-1', display_title: 'Refund request',
    })
  })

  it('creates valid idempotency UUIDs and readable run labels', () => {
    expect(newSupportDockClientMessageId()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)
    expect(supportAgentRunLabel('running')).toBe('Agent is working…')
    expect(supportAgentRunLabel('paused')).toBe('Agent is waiting for input')
  })
})
