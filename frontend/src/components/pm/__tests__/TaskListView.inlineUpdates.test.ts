import { describe, expect, it } from 'vitest';

import { applyTaskListInlinePatch, mergeTaskListInlineUpdate } from '@/components/pm/TaskListView';
import type { Task } from '@/lib/pmTypes';

const task = (id: string, fields: Partial<Task> = {}): Task => ({
  id,
  workspace_id: 'ws-1',
  display_id: Number(id.replace(/\D/g, '')) || 1,
  task_key: `T-${id}`,
  name: `Task ${id}`,
  task_type: 'story',
  workflow_id: 'workflow-1',
  workflow_state_id: 'state-1',
  priority: 'medium',
  severity: 'none',
  position: 0,
  started: false,
  completed: false,
  blocked: false,
  archived: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...fields,
});

describe('TaskListView inline updates', () => {
  it('preserves an earlier row edit when a later row edit is merged', () => {
    const base = [task('task-1'), task('task-2')];

    const afterOwner = applyTaskListInlinePatch(base, 'task-1', { owner_member_ids: ['member-1'] });
    const afterDeadline = applyTaskListInlinePatch(afterOwner, 'task-2', { deadline: '2026-05-20' });

    expect(afterDeadline.find((entry) => entry.id === 'task-1')?.owner_member_ids).toEqual(['member-1']);
    expect(afterDeadline.find((entry) => entry.id === 'task-2')?.deadline).toBe('2026-05-20');
  });

  it('merges a confirmed response into the latest row state instead of an older snapshot', () => {
    const latest = [
      task('task-1', { owner_member_ids: ['member-1'] }),
      task('task-2', { deadline: '2026-05-20' }),
    ];

    const merged = mergeTaskListInlineUpdate(latest, task('task-1', {
      owner_member_ids: ['member-1'],
      updated_at: '2026-05-15T00:00:00Z',
    }));

    expect(merged.find((entry) => entry.id === 'task-1')?.owner_member_ids).toEqual(['member-1']);
    expect(merged.find((entry) => entry.id === 'task-2')?.deadline).toBe('2026-05-20');
  });
});
