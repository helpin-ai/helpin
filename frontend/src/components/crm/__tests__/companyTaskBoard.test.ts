import { describe, expect, it } from 'vitest';

import type { Task, WorkflowWithStates } from '@/lib/pmTypes';
import {
  COMPANY_TASK_STATE_GROUPS,
  companyTaskDropStates,
  groupCompanyTasksByStateType,
} from '../companyTaskBoard';

const workflows = [{
  workflow: { id: 'workflow-a', name: 'Sales work' },
  states: [
    { id: 'backlog-a', workflow_id: 'workflow-a', name: 'Backlog', state_type: 'backlog', position: 0 },
    { id: 'todo-a', workflow_id: 'workflow-a', name: 'To Do', state_type: 'unstarted', position: 1 },
    { id: 'review-a', workflow_id: 'workflow-a', name: 'Review', state_type: 'started', position: 3 },
    { id: 'doing-a', workflow_id: 'workflow-a', name: 'In Progress', state_type: 'started', position: 2 },
    { id: 'done-a', workflow_id: 'workflow-a', name: 'Done', state_type: 'done', position: 4 },
  ],
}] as WorkflowWithStates[];

function task(id: string, stateID: string, stateType?: Task['state_type']) {
  return {
    id,
    workflow_id: 'workflow-a',
    workflow_state_id: stateID,
    state_type: stateType,
  } as Task;
}

describe('company task board grouping', () => {
  it('uses the four canonical PM state groups', () => {
    expect(COMPANY_TASK_STATE_GROUPS.map((group) => group.label))
      .toEqual(['Backlog', 'Not Started', 'In Progress', 'Completed']);
  });

  it('groups tasks by enriched state type and falls back to their workflow state', () => {
    const grouped = groupCompanyTasksByStateType([
      task('task-backlog', 'backlog-a', 'backlog'),
      task('task-todo', 'todo-a'),
      task('task-doing', 'doing-a', 'started'),
      task('task-done', 'done-a', 'done'),
    ], workflows);

    expect(grouped.get('backlog')?.map((item) => item.id)).toEqual(['task-backlog']);
    expect(grouped.get('unstarted')?.map((item) => item.id)).toEqual(['task-todo']);
    expect(grouped.get('started')?.map((item) => item.id)).toEqual(['task-doing']);
    expect(grouped.get('done')?.map((item) => item.id)).toEqual(['task-done']);
  });

  it('offers only exact states from the dragged task workflow in board order', () => {
    const targets = companyTaskDropStates(task('task-1', 'todo-a'), workflows, 'started');
    expect(targets.map((state) => state.name)).toEqual(['In Progress', 'Review']);
  });
});
