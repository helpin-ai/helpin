import type { StoryDetail, StoryRecurringSummary, WorkflowState } from '@/lib/pmTypes';

export interface LoadedStoryState {
  storyId: string;
  storyDetail: StoryDetail;
  states: WorkflowState[];
  recurringSummary: StoryRecurringSummary | null;
}

export interface StoryOverlayPresentationState {
  open: boolean;
  loading: boolean;
  storyDetail: StoryDetail | null;
  states: WorkflowState[];
  recurringSummary: StoryRecurringSummary | null;
}

export function getStoryOverlayPresentationState(
  activeStoryId: string | null,
  loadedStory: LoadedStoryState | null,
): StoryOverlayPresentationState {
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
