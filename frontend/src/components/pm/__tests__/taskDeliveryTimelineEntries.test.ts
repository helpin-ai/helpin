import { describe, expect, it } from 'vitest';

import type { AgentRun, TaskGitLink } from '@/lib/pmTypes';
import { buildTaskDeliveryTimelineEntries } from '../taskDeliveryTimelineEntries';

function run(id: string, updatedAt: string, completedAt?: string): AgentRun {
  return {
    id,
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    target_type: 'task',
    target_id: 'task-1',
    invocation_mode: 'interactive',
    status: completedAt ? 'completed' : 'running',
    pause_reason: 'none',
    approval_state: 'not_required',
    input: {},
    output_summary: {},
    tokens_used: 0,
    input_tokens: 0,
    output_tokens: 0,
    cached_input_tokens: 0,
    created_at: updatedAt,
    updated_at: updatedAt,
    completed_at: completedAt,
  };
}

function link(id: string, updatedAt: string): TaskGitLink {
  return {
    id,
    workspace_id: 'ws-1',
    task_id: 'task-1',
    integration_id: 'integration-1',
    provider: 'github',
    repo: 'helpin/helpin',
    branch: 'task-1',
    created_at: updatedAt,
    updated_at: updatedAt,
  };
}

describe('buildTaskDeliveryTimelineEntries', () => {
  it('merges agent runs and Git activity into one newest-first chronology', () => {
    const entries = buildTaskDeliveryTimelineEntries(
      [
        run('older-run', '2026-08-11T10:00:00Z', '2026-08-11T10:05:00Z'),
        run('current-run', '2026-08-11T10:20:00Z'),
      ],
      [link('branch-link', '2026-08-11T10:10:00Z')],
    );

    expect(entries.map((entry) => entry.id)).toEqual([
      'run:current-run',
      'git:branch-link',
      'run:older-run',
    ]);
  });
});
