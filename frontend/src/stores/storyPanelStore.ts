import { create } from 'zustand';
import type { StoryDetail } from '@/lib/pmTypes';

interface StoryPanelState {
  open: boolean;
  storyId: string | null;
  storyDetail: StoryDetail | null;
  loading: boolean;
  /** Monotonic counter so re-clicking the same story re-triggers the fetch */
  requestKey: number;

  /** Queue a story to be fetched — panel opens only after data is ready */
  openStory: (storyId: string) => void;
  /** Called by GlobalStoryPanel once data is loaded — opens the panel */
  reveal: (detail: StoryDetail) => void;
  close: () => void;
  setStoryDetail: (detail: StoryDetail | null) => void;
}

export const useStoryPanelStore = create<StoryPanelState>((set, get) => ({
  open: false,
  storyId: null,
  storyDetail: null,
  loading: false,
  requestKey: 0,

  openStory: (storyId) =>
    set({ storyId, storyDetail: null, loading: true, open: false, requestKey: get().requestKey + 1 }),

  reveal: (detail) =>
    set({ storyDetail: detail, loading: false, open: true }),

  close: () =>
    set({ open: false, storyId: null, storyDetail: null, loading: false }),

  setStoryDetail: (detail) =>
    set({ storyDetail: detail, loading: false }),
}));
