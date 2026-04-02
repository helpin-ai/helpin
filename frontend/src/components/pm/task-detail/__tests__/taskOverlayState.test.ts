import { describe, expect, it } from 'vitest';

import { getTaskOverlayPresentationState } from '../taskOverlayState';
import type { TaskDetail, TaskRecurringSummary, WorkflowState } from '@/lib/pmTypes';

function makeTaskDetail(id: string): TaskDetail {
  return {
    task: {
      id,
    },
  } as TaskDetail;
}

function makeWorkflowState(id: string): WorkflowState {
  return {
    id,
  } as WorkflowState;
}

function makeRecurringSummary(templateId: string): TaskRecurringSummary {
  return {
    template_id: templateId,
  } as TaskRecurringSummary;
}

describe('getTaskOverlayPresentationState', () => {
  it('opens immediately and shows loading while the active task is still fetching', () => {
    expect(
      getTaskOverlayPresentationState('task-123', null),
    ).toEqual({
      open: true,
      loading: true,
      taskDetail: null,
      states: [],
      recurringSummary: null,
    });
  });

  it('keeps the overlay open and loading when cached data belongs to a different task', () => {
    expect(
      getTaskOverlayPresentationState('task-123', {
        taskId: 'task-999',
        taskDetail: makeTaskDetail('task-999'),
        states: [makeWorkflowState('done')],
        recurringSummary: makeRecurringSummary('template-999'),
      }),
    ).toEqual({
      open: true,
      loading: true,
      taskDetail: null,
      states: [],
      recurringSummary: null,
    });
  });

  it('shows the loaded task once the active task payload matches', () => {
    const loaded = {
      taskId: 'task-123',
      taskDetail: makeTaskDetail('task-123'),
      states: [makeWorkflowState('doing')],
      recurringSummary: makeRecurringSummary('template-123'),
    };

    expect(
      getTaskOverlayPresentationState('task-123', loaded),
    ).toEqual({
      open: true,
      loading: false,
      taskDetail: loaded.taskDetail,
      states: loaded.states,
      recurringSummary: loaded.recurringSummary,
    });
  });

  it('closes cleanly when there is no active task route', () => {
    expect(
      getTaskOverlayPresentationState(null, {
        taskId: 'task-123',
        taskDetail: makeTaskDetail('task-123'),
        states: [makeWorkflowState('doing')],
        recurringSummary: makeRecurringSummary('template-123'),
      }),
    ).toEqual({
      open: false,
      loading: false,
      taskDetail: null,
      states: [],
      recurringSummary: null,
    });
  });
});
