import { describe, expect, it } from 'vitest'
import type { SupportAgentRun } from '@helpin-ai/support-core'
import { agentRunStatusLabel, agentRunSummary, formatAgentRunTimestamp, orderAgentRuns } from '../agent-runs'

function run(overrides: Partial<SupportAgentRun> = {}): SupportAgentRun {
  return {
    id: 'run-1', workspace_id: 'ws-1', agent_id: 'agent-1', target_type: 'support_conversation', target_id: 'conv-1',
    runtime_kind: 'native_sdk', invocation_mode: 'autonomous', approval_state: 'not_required', pause_reason: 'none',
    status: 'completed', input: {}, output_summary: {}, tokens_used: 0,
    created_at: '2026-08-20T10:00:00Z', updated_at: '2026-08-20T10:05:00Z',
    ...overrides,
  }
}

describe('agent run presentation', () => {
  it('orders newest first and formats machine labels', () => {
    expect(orderAgentRuns([run(), run({ id: 'run-2', created_at: '2026-08-20T11:00:00Z' })]).map((item) => item.id)).toEqual(['run-2', 'run-1'])
    expect(agentRunStatusLabel('changes_requested')).toBe('Changes requested')
  })

  it('prefers readable output summaries and falls back to errors', () => {
    expect(agentRunSummary(run({ output_summary: { summary: ' Draft ready ' }, error_message: 'ignored' }))).toBe('Draft ready')
    expect(agentRunSummary(run({ status: 'failed', error_message: 'Provider unavailable' }))).toBe('Provider unavailable')
  })

  it('safely rejects invalid timestamps', () => {
    expect(formatAgentRunTimestamp('not-a-date')).toBeNull()
  })
})
