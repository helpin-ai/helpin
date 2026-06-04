import { create } from 'zustand';

interface ShortcutComposerStore {
  openRequest: number;
  requestSeq: number;
  seedShortCode?: string;
  seedContent?: string;
  openCreate: (opts?: { seedShortCode?: string; seedContent?: string }) => void;
  clear: () => void;
}

export const useShortcutComposerStore = create<ShortcutComposerStore>((set) => ({
  openRequest: 0,
  requestSeq: 0,
  openCreate: (opts) =>
    set((state) => {
      const nextRequest = state.requestSeq + 1;
      return {
        openRequest: nextRequest,
        requestSeq: nextRequest,
        seedShortCode: opts?.seedShortCode,
        seedContent: opts?.seedContent,
      };
    }),
  clear: () => set({ openRequest: 0, seedShortCode: undefined, seedContent: undefined }),
}));
