import { describe, expect, it } from 'vitest';

import type { AgentRun } from '@/lib/pmTypes';
import { runDisplayTitle } from '../utils';

function run(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    target_type: 'workspace',
    target_id: 'ws-1',
    runtime_kind: 'native_sdk',
    invocation_mode: 'autonomous',
    approval_state: 'not_required',
    pause_reason: 'none',
    status: 'completed',
    input: {},
    output_summary: {},
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

describe('dock run utilities', () => {
  it('uses automation flow names for workspace-targeted runs', () => {
    expect(
      runDisplayTitle(run({
        input: {
          trigger: { source: 'automation_rule', trigger_type: 'cron' },
          event: { reason: 'automation rule "Competitors Changelog Tracking Report"' },
        },
      })),
    ).toBe('Competitors Changelog Tracking Report');
  });
});
