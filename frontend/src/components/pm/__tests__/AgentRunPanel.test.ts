import { describe, expect, it } from 'vitest';

import {
  getTaskAgentRunExecutionContextLockReason,
  getTaskAgentRunLaunchState,
  getTaskAgentRunPickerLabel,
  getTaskAgentRunPrimaryAction,
  getTaskAgentRunSelectedAgentId,
  getTaskAgentRunSuggestedAgent,
} from '../AgentRunPanel';
import type { Agent, AgentRun } from '@/lib/pmTypes';

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

function agent(overrides: Partial<Agent>): Agent {
  return {
    id: 'agent-1',
    workspace_id: 'ws-1',
    name: 'Scribe',
    description: '',
    role: '',
    instructions: '',
    avatar_url: null,
    status: 'idle',
    is_system: true,
    trigger_mode: 'manual',
    schedule: null,
    model: '',
    temperature: 0,
    tools: [],
    max_iterations: 0,
    timeout_seconds: 0,
    max_tokens_per_month: null,
    tokens_used_this_month: 0,
    created_by: null,
    created_at: '2026-05-12T10:00:00Z',
    updated_at: '2026-05-12T10:00:00Z',
    allowed_targets: ['task'],
    preset_key: 'task_planner',
    runtime_kind: 'native_sdk',
    approval_mode: 'preset_default',
    ...overrides,
  };
}

describe('getTaskAgentRunLaunchState', () => {
  it('keeps run disabled while a task run is active', () => {
    expect(
      getTaskAgentRunLaunchState({
        activeRun: run({ id: 'run-active', status: 'running' }),
        triggering: false,
      }),
    ).toMatchObject({
      label: 'Run',
      disabled: true,
      disabledReason: 'The current run is still in progress.',
    });
  });

  it('uses pause-specific disabled reasons', () => {
    expect(
      getTaskAgentRunLaunchState({
        activeRun: run({ status: 'paused', pause_reason: 'human_approval' }),
        triggering: false,
      }).disabledReason,
    ).toBe('The current run is waiting for approval.');

    expect(
      getTaskAgentRunLaunchState({
        activeRun: run({ status: 'paused', pause_reason: 'human_input' }),
        triggering: false,
      }).disabledReason,
    ).toBe('The current run is waiting for your response.');

    expect(
      getTaskAgentRunLaunchState({
        activeRun: run({ status: 'paused', pause_reason: 'authentication' }),
        triggering: false,
      }).disabledReason,
    ).toBe('The current run is waiting for sign-in.');
  });

  it('enables run when no current run blocks it', () => {
    expect(
      getTaskAgentRunLaunchState({
        activeRun: null,
        triggering: false,
      }),
    ).toMatchObject({
      label: 'Run',
      disabled: false,
      disabledReason: null,
    });
  });
});

describe('getTaskAgentRunPickerLabel', () => {
  it('uses one generic picker label regardless of pipeline or active run state', () => {
    expect(getTaskAgentRunPickerLabel({ activeRun: null, suggestedAgent: null })).toBe('Agent');
    expect(getTaskAgentRunPickerLabel({ activeRun: null, suggestedAgent: agent({ preset_key: 'task_planner' }) })).toBe('Agent');
    expect(getTaskAgentRunPickerLabel({ activeRun: null, suggestedAgent: agent({ preset_key: 'review_agent' }) })).toBe('Agent');
    expect(getTaskAgentRunPickerLabel({ activeRun: run({ status: 'running' }), suggestedAgent: agent({ preset_key: 'code_builder' }) })).toBe('Agent');
  });
});

describe('getTaskAgentRunSuggestedAgent', () => {
  const agents = [
    agent({ id: 'scribe', name: 'Scribe', preset_key: 'task_planner' }),
    agent({ id: 'forge', name: 'Forge', preset_key: 'code_builder' }),
    agent({ id: 'lens', name: 'Lens', preset_key: 'review_agent' }),
  ];

  it('follows the Scribe, Forge, Lens order from completed runs', () => {
    expect(getTaskAgentRunSuggestedAgent({ agents, runs: [], activeRun: null })?.id).toBe('scribe');
    expect(getTaskAgentRunSuggestedAgent({
      agents,
      runs: [run({ agent_id: 'scribe', status: 'completed' })],
      activeRun: null,
    })?.id).toBe('forge');
    expect(getTaskAgentRunSuggestedAgent({
      agents,
      runs: [
        run({ id: 'run-forge', agent_id: 'forge', status: 'completed' }),
        run({ id: 'run-scribe', agent_id: 'scribe', status: 'completed' }),
      ],
      activeRun: null,
    })?.id).toBe('lens');
  });

  it('does not advance to review from unpublished preview changes', () => {
    expect(getTaskAgentRunSuggestedAgent({
      agents,
      runs: [run({ agent_id: 'scribe', status: 'completed' }), run({ agent_id: 'forge', status: 'completed', input: { delivery_mode: 'preview' } })],
      activeRun: null,
    })?.id).toBe('forge');
  });

  it('shows the active run agent while blocked', () => {
    expect(getTaskAgentRunSuggestedAgent({
      agents,
      runs: [run({ agent_id: 'scribe', status: 'completed' })],
      activeRun: run({ agent_id: 'forge', status: 'running' }),
    })?.id).toBe('forge');
  });
});

describe('getTaskAgentRunSelectedAgentId', () => {
  const agents = [
    agent({ id: 'scribe', name: 'Scribe', preset_key: 'task_planner' }),
    agent({ id: 'forge', name: 'Forge', preset_key: 'code_builder' }),
  ];

  it('preserves a valid manual selection after the previous run completes', () => {
    expect(getTaskAgentRunSelectedAgentId({
      selectedAgentId: 'scribe',
      agents,
      suggestedAgent: agents[1],
      selectionLocked: false,
    })).toBe('scribe');
  });

  it('uses the suggested agent while an active run locks selection', () => {
    expect(getTaskAgentRunSelectedAgentId({
      selectedAgentId: 'scribe',
      agents,
      suggestedAgent: agents[1],
      selectionLocked: true,
    })).toBe('forge');
  });
});

describe('getTaskAgentRunExecutionContextLockReason', () => {
  it('locks repository and branch selection while a task run is active', () => {
    expect(
      getTaskAgentRunExecutionContextLockReason({
        activeRun: run({ status: 'running' }),
        activeRunAgentName: 'Forge',
      }),
    ).toBe('Forge is running. Repository and branch can be changed after this run finishes.');

    expect(
      getTaskAgentRunExecutionContextLockReason({
        activeRun: run({ status: 'paused', pause_reason: 'human_approval' }),
        activeRunAgentName: 'Lens',
      }),
    ).toBe('Lens is awaiting approval. Repository and branch can be changed after this run finishes.');
  });

  it('allows repository and branch selection when there is no active run', () => {
    expect(
      getTaskAgentRunExecutionContextLockReason({
        activeRun: null,
        activeRunAgentName: null,
      }),
    ).toBeNull();
  });
});
