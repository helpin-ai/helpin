import { create } from 'zustand';

interface StoryPanelState {
  storyId: string | null;
  requestKey: number;
  lastClosedStoryId: string | null;
  lastClosedAt: number;
  openStory: (storyId: string) => void;
  close: () => void;
  rememberClosedStory: (storyId: string) => void;
  shouldSuppressOpen: (storyId: string) => boolean;
}

const RECENTLY_CLOSED_STORY_SUPPRESSION_MS = 250;

export const useStoryPanelStore = create<StoryPanelState>((set, get) => ({
  storyId: null,
  requestKey: 0,
  lastClosedStoryId: null,
  lastClosedAt: 0,
  openStory: (storyId) =>
    set({
      storyId,
      requestKey: get().requestKey + 1,
    }),
  close: () => {
    // Clean up ?story= URL param
    const url = new URL(window.location.href);
    if (url.searchParams.has('story')) {
      url.searchParams.delete('story');
      window.history.replaceState({}, '', url.toString());
    }
    set({ storyId: null });
  },
  rememberClosedStory: (storyId) =>
    set({
      lastClosedStoryId: storyId,
      lastClosedAt: Date.now(),
    }),
  shouldSuppressOpen: (storyId) => {
    const state = get();
    return (
      state.lastClosedStoryId === storyId &&
      Date.now() - state.lastClosedAt < RECENTLY_CLOSED_STORY_SUPPRESSION_MS
    );
  },
}));
