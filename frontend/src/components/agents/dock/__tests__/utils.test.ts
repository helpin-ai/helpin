import { describe, expect, it } from 'vitest';

import type { AgentRun } from '@/lib/pmTypes';
import { displayAgentName } from '@/lib/agentTerminology';
import { planKindLabel, planUpdatedAt, runDisplayTitle } from '../utils';
import type { CommandBarRunPlan } from '../planSummary';

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
  it('uses the persisted plan timestamp when no child run was created', () => {
    const plan: CommandBarRunPlan = {
      id: 'plan-1',
      steps: [],
      runIdsByStep: {},
      createdAt: '2026-05-01T00:00:00Z',
      updatedAt: '2026-05-01T00:02:00Z',
    };

    expect(planUpdatedAt(plan, {})).toBe(Date.parse('2026-05-01T00:02:00Z'));
  });

  it('labels a single delegated command as a sub-agent', () => {
    expect(planKindLabel('one_shot_command', 1)).toBe('Sub-agent');
  });

  it('normalizes historical Command Agent names for display', () => {
    expect(displayAgentName('Command Agent')).toBe('Sub-agent');
    expect(displayAgentName('Command Agent (one-shot)')).toBe('Sub-agent');
  });

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
