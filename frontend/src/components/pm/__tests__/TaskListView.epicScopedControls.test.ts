import { describe, expect, it } from 'vitest';
import type { Task } from '@/lib/pmTypes';
import {
  TASK_LIST_FILTER_ALL,
  TASK_LIST_FILTER_UNASSIGNED,
  applyTaskListInlinePatch,
  filterTaskListLocalTasks,
  mergeTaskListInlineUpdate,
  type LocalTaskFilterValues,
} from '../TaskListView';

const baseTask = {
  id: 'task-1',
  workspace_id: 'ws-1',
  display_id: 1,
  task_key: 'PM-1',
  name: 'Build onboarding flow',
  task_type: 'feature',
  workflow_id: 'workflow-1',
  workflow_state_id: 'state-todo',
  sprint_id: 'sprint-1',
  team_id: 'team-1',
  owner_member_ids: ['member-1'],
  requester_member_id: 'requester-1',
  priority: 'high',
  severity: 'minor',
  position: 1,
  started: false,
  completed: false,
  blocked: false,
  is_blocked_by_task: false,
  is_blocking_other_task: false,
  archived: false,
  labels: [
    {
      id: 'label-1',
      workspace_id: 'ws-1',
      name: 'Frontend',
      archived: false,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
  ],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
} satisfies Task;

function filters(overrides: Partial<LocalTaskFilterValues> = {}): LocalTaskFilterValues {
  return {
    owner: TASK_LIST_FILTER_ALL,
    requester: TASK_LIST_FILTER_ALL,
    state: TASK_LIST_FILTER_ALL,
    task_type: TASK_LIST_FILTER_ALL,
    priority: TASK_LIST_FILTER_ALL,
    severity: TASK_LIST_FILTER_ALL,
    label: TASK_LIST_FILTER_ALL,
    sprint: TASK_LIST_FILTER_ALL,
    blocked: TASK_LIST_FILTER_ALL,
    blocking: TASK_LIST_FILTER_ALL,
    ...overrides,
  };
}

describe('TaskListView epic-scoped task helpers', () => {
  it('filters external epic tasks by search text and local field filters', () => {
    const tasks: Task[] = [
      baseTask,
      {
        ...baseTask,
        id: 'task-2',
        display_id: 2,
        task_key: 'PM-2',
        name: 'Fix billing export',
        task_type: 'bug',
        workflow_state_id: 'state-done',
        owner_member_ids: [],
        requester_member_id: 'requester-2',
        priority: 'low',
        severity: 'critical',
        sprint_id: 'sprint-2',
        blocked: true,
        is_blocked_by_task: true,
        labels: [],
      },
    ];

    expect(filterTaskListLocalTasks(tasks, {
      filters: filters({ owner: TASK_LIST_FILTER_UNASSIGNED, blocked: 'true' }),
      query: 'billing',
      ownerNameMap: new Map([['member-1', 'Waqar']]),
      sprintNameMap: new Map([['sprint-2', 'June Sprint']]),
      stateNameMap: new Map([['state-done', 'Done']]),
    }).map((task) => task.id)).toEqual(['task-2']);
  });

  it('patches and merges inline updates without losing unrelated task data', () => {
    const tasks: Task[] = [baseTask, { ...baseTask, id: 'task-2', task_key: 'PM-2', name: 'Second task' }];

    const patched = applyTaskListInlinePatch(tasks, 'task-1', {
      workflow_state_id: 'state-done',
      completed: true,
    });
    expect(patched[0]).toMatchObject({ workflow_state_id: 'state-done', completed: true });
    expect(patched[1]).toBe(tasks[1]);

    const merged = mergeTaskListInlineUpdate(patched, {
      ...baseTask,
      name: 'Updated from server',
      updated_at: '2026-01-02T00:00:00Z',
    });
    expect(merged[0]).toMatchObject({
      id: 'task-1',
      name: 'Updated from server',
      updated_at: '2026-01-02T00:00:00Z',
    });
    expect(merged[1]).toBe(tasks[1]);
  });
});
