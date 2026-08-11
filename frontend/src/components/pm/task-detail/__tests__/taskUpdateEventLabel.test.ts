import { describe, expect, it } from 'vitest';
import type { AgentRun, TaskUpdateEntry } from '@/lib/pmTypes';
import { taskUpdateAgentPresentation, taskUpdateEventLabel } from '../taskUpdateEventLabel';

function run(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'workspace-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    target_type: 'task',
    target_id: 'task-1',
    invocation_mode: 'interactive',
    status: 'completed',
    pause_reason: 'none',
    approval_state: 'not_required',
    input: { trigger: { source: 'manual' } },
    output_summary: {},
    tokens_used: 0,
    input_tokens: 0,
    output_tokens: 0,
    cached_input_tokens: 0,
    created_at: '2026-08-11T11:50:00Z',
    updated_at: '2026-08-11T12:00:00Z',
    ...overrides,
  };
}

function agentRunEntry(overrides: Partial<TaskUpdateEntry> = {}): TaskUpdateEntry {
  return {
    id: 'run:run-1',
    kind: 'agent_run',
    occurred_at: '2026-08-11T12:00:00Z',
    agent_name: 'Scribe',
    agent_run: run(),
    ...overrides,
  };
}

function runActivityEntry(newValue = 'note_added', overrides: Partial<TaskUpdateEntry> = {}): TaskUpdateEntry {
  return {
    id: 'activity:activity-1',
    kind: 'change',
    occurred_at: '2026-08-11T12:00:00Z',
    actor: {
      id: 'user-1',
      email: 'alex@example.com',
      full_name: 'Alex',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    activity: {
      id: 'activity-1',
      workspace_id: 'workspace-1',
      entity_type: 'task',
      entity_id: 'task-1',
      action: 'updated',
      field_name: 'agent_run',
      new_value: newValue,
      metadata: {
        agent_name: 'Forge',
        agent_preset_key: 'code_builder',
        run_id: 'run-1',
        note_snippet: 'Please keep the implementation focused.',
      },
      created_at: '2026-08-11T12:00:00Z',
    },
    ...overrides,
  };
}

describe('task update agent presentation', () => {
  it('keeps manual lifecycle rows compact and identifies the agent', () => {
    const presentation = taskUpdateAgentPresentation(agentRunEntry());

    expect(presentation).toMatchObject({
      agentName: 'Scribe',
      runId: 'run-1',
      title: 'Agent run completed · Scribe',
      automated: false,
    });
  });

  it('puts the agent first for automation-rule runs', () => {
    const automatedRun = run({ input: { trigger: { source: 'automation_rule' } } });
    const entry = agentRunEntry({ agent_run: automatedRun });

    expect(taskUpdateEventLabel(entry)).toBe('Scribe completed an automated run');
  });

  it('uses actor-first copy for human audit actions and keeps the note as detail', () => {
    const entry = runActivityEntry();
    const presentation = taskUpdateAgentPresentation(entry, run());

    expect(presentation).toMatchObject({
      agentName: 'Forge',
      agentPresetKey: 'code_builder',
      runId: 'run-1',
      title: 'Alex added a note to Forge’s run',
      detail: 'Please keep the implementation focused.',
      automated: false,
    });
  });

  it('puts the agent first for automation audit activity even when an actor is present', () => {
    const automatedRun = run({ input: { trigger: { source: 'automation_rule' } } });

    expect(taskUpdateEventLabel(runActivityEntry('approved'), automatedRun)).toBe(
      'Forge’s run was approved via automation',
    );
  });

  it('normalizes historical approval and cancellation action aliases', () => {
    expect(taskUpdateEventLabel(runActivityEntry('approve'), run())).toBe('Alex approved Forge’s run');
    expect(taskUpdateEventLabel(runActivityEntry('canceled'), run())).toBe('Alex cancelled Forge’s run');
  });

  it('uses neutral agent-first wording when the human actor is unavailable', () => {
    expect(taskUpdateEventLabel(runActivityEntry('note_added', { actor: undefined }), run())).toBe(
      'Forge’s run received a note',
    );
  });
});
