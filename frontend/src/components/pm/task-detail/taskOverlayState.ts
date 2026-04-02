import type { TaskDetail, TaskRecurringSummary, WorkflowState } from '@/lib/pmTypes';

export interface LoadedTaskState {
  taskId: string;
  taskDetail: TaskDetail;
  states: WorkflowState[];
  recurringSummary: TaskRecurringSummary | null;
}

export interface TaskOverlayPresentationState {
  open: boolean;
  loading: boolean;
  taskDetail: TaskDetail | null;
  states: WorkflowState[];
  recurringSummary: TaskRecurringSummary | null;
}

export function getTaskOverlayPresentationState(
  activeTaskId: string | null,
  loadedTask: LoadedTaskState | null,
): TaskOverlayPresentationState {
  if (!activeTaskId) {
    return {
      open: false,
      loading: false,
      taskDetail: null,
      states: [],
      recurringSummary: null,
    };
  }

  if (!loadedTask || loadedTask.taskId !== activeTaskId) {
    return {
      open: true,
      loading: true,
      taskDetail: null,
      states: [],
      recurringSummary: null,
    };
  }

  return {
    open: true,
    loading: false,
    taskDetail: loadedTask.taskDetail,
    states: loadedTask.states,
    recurringSummary: loadedTask.recurringSummary,
  };
}
