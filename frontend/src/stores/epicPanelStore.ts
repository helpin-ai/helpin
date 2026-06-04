import { create } from 'zustand';

interface EpicPanelState {
  epicId: string | null;
  requestKey: number;
  lastClosedEpicId: string | null;
  lastClosedAt: number;
  openEpic: (epicId: string) => void;
  close: () => void;
  rememberClosedEpic: (epicId: string) => void;
  shouldSuppressOpen: (epicId: string) => boolean;
}

const RECENTLY_CLOSED_EPIC_SUPPRESSION_MS = 2_000;

export const useEpicPanelStore = create<EpicPanelState>((set, get) => ({
  epicId: null,
  requestKey: 0,
  lastClosedEpicId: null,
  lastClosedAt: 0,
  openEpic: (epicId) =>
    set({
      epicId,
      requestKey: get().requestKey + 1,
    }),
  close: () => set({ epicId: null }),
  rememberClosedEpic: (epicId) =>
    set({
      lastClosedEpicId: epicId,
      lastClosedAt: Date.now(),
    }),
  shouldSuppressOpen: (epicId) => {
    const state = get();
    return (
      state.lastClosedEpicId === epicId &&
      Date.now() - state.lastClosedAt < RECENTLY_CLOSED_EPIC_SUPPRESSION_MS
    );
  },
}));
