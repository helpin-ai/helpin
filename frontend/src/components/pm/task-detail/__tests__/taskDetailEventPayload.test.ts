import { describe, expect, it } from 'vitest';
import { buildPatchedTaskFromDetail } from '@/components/pm/task-detail/taskDetailEventPayload';
import type { TaskDetail } from '@/lib/pmTypes';

describe('buildPatchedTaskFromDetail', () => {
  it('merges board-enriched fields from task detail onto the task payload', () => {
    const detail = {
      task: {
        id: 'task-1',
        display_id: 12,
        workspace_id: 'ws-1',
        workflow_id: 'wf-1',
        workflow_state_id: 'state-1',
        name: 'Refine onboarding',
        description: '',
        task_type: 'feature',
        priority: 'medium',
        severity: 'none',
        position: 0,
        started: false,
        completed: false,
        blocked: false,
        archived: false,
        created_at: '2026-03-31T00:00:00Z',
        updated_at: '2026-03-31T00:00:00Z',
      },
      owners: [],
      followers: [],
      requester_member: undefined,
      labels: [{ id: 'label-1', workspace_id: 'ws-1', name: 'Bug', color: '#f00', created_at: '', updated_at: '' }],
      epic_name: 'Q2 Reliability',
      sprint_name: 'Sprint 18',
      state: {
        id: 'state-1',
        workflow_id: 'wf-1',
        name: 'In Progress',
        state_type: 'started',
        position: 1,
        created_at: '',
        updated_at: '',
        color: '#123456',
      },
    } satisfies TaskDetail;

    expect(buildPatchedTaskFromDetail(detail)).toMatchObject({
      id: 'task-1',
      epic_name: 'Q2 Reliability',
      sprint_name: 'Sprint 18',
      labels: [{ id: 'label-1', name: 'Bug' }],
      state_name: 'In Progress',
      state_type: 'started',
      state_color: '#123456',
    });
  });
});
