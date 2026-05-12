import { describe, expect, it } from 'vitest';

import { getTaskAgentRunPrimaryAction } from '../AgentRunPanel';
import type { AgentRun } from '@/lib/pmTypes';

function run(overrides: Partial<AgentRun>): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    target_type: 'task',
    target_id: 'task-1',
    invocation_mode: 'interactive',
    status: 'completed',
    pause_reason: 'none',
    approval_state: 'not_required',
    input: {},
    output: null,
    output_summary: null,
    error: null,
    tokens_used: 0,
    input_tokens: 0,
    output_tokens: 0,
    cached_input_tokens: 0,
    created_at: '2026-05-12T10:00:00Z',
    updated_at: '2026-05-12T10:00:00Z',
    started_at: null,
    completed_at: null,
    ...overrides,
  };
}

describe('getTaskAgentRunPrimaryAction', () => {
  it('opens the active task run instead of offering another start', () => {
    expect(
      getTaskAgentRunPrimaryAction({
        selectedAgentName: 'Forge',
        activeRun: run({ id: 'run-active', status: 'running' }),
        activeRunAgentName: 'Atlas',
        triggering: false,
      }),
    ).toMatchObject({
      kind: 'open',
      runId: 'run-active',
      label: 'Open Atlas run',
      status: 'Atlas is running.',
    });
  });

  it('uses pause-specific labels for active task runs', () => {
    expect(
      getTaskAgentRunPrimaryAction({
        selectedAgentName: 'Forge',
        activeRun: run({ status: 'paused', pause_reason: 'human_approval' }),
        activeRunAgentName: 'Forge',
        triggering: false,
      }).label,
    ).toBe('Review Forge request');

    expect(
      getTaskAgentRunPrimaryAction({
        selectedAgentName: 'Forge',
        activeRun: run({ status: 'paused', pause_reason: 'human_input' }),
        activeRunAgentName: 'Forge',
        triggering: false,
      }).label,
    ).toBe('Reply to Forge');

    expect(
      getTaskAgentRunPrimaryAction({
        selectedAgentName: 'Forge',
        activeRun: run({ status: 'paused', pause_reason: 'authentication' }),
        activeRunAgentName: 'Forge',
        triggering: false,
      }).label,
    ).toBe('Complete Forge sign-in');
  });

  it('starts the selected task agent when there is no active run', () => {
    expect(
      getTaskAgentRunPrimaryAction({
        selectedAgentName: 'Forge',
        activeRun: null,
        activeRunAgentName: null,
        triggering: false,
      }),
    ).toMatchObject({
      kind: 'start',
      runId: null,
      label: 'Run',
      status: 'Choose an agent to run on this task.',
    });
  });
});
