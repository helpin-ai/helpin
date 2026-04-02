import type { TaskDetail, TaskRecurringSummary, WorkflowState } from '@/lib/pmTypes';

export interface LoadedTaskState {
  storyId: string;
  storyDetail: TaskDetail;
  states: WorkflowState[];
  recurringSummary: TaskRecurringSummary | null;
}

export interface TaskOverlayPresentationState {
  open: boolean;
  loading: boolean;
  storyDetail: TaskDetail | null;
  states: WorkflowState[];
  recurringSummary: TaskRecurringSummary | null;
}

export function getTaskOverlayPresentationState(
  activeStoryId: string | null,
  loadedStory: LoadedTaskState | null,
): TaskOverlayPresentationState {
  if (!activeStoryId) {
    return {
      open: false,
      loading: false,
      storyDetail: null,
      states: [],
      recurringSummary: null,
    };
  }

  if (!loadedStory || loadedStory.storyId !== activeStoryId) {
    return {
      open: true,
      loading: true,
      storyDetail: null,
      states: [],
      recurringSummary: null,
    };
  }

  return {
    open: true,
    loading: false,
    storyDetail: loadedStory.storyDetail,
    states: loadedStory.states,
    recurringSummary: loadedStory.recurringSummary,
  };
}
