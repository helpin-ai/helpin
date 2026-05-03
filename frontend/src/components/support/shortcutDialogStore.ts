import { create } from 'zustand';

interface ShortcutComposerStore {
  openRequest: number;
  seedShortCode?: string;
  seedContent?: string;
  openCreate: (opts?: { seedShortCode?: string; seedContent?: string }) => void;
  clear: () => void;
}

export const useShortcutComposerStore = create<ShortcutComposerStore>((set) => ({
  openRequest: 0,
  openCreate: (opts) =>
    set((state) => ({
      openRequest: state.openRequest + 1,
      seedShortCode: opts?.seedShortCode,
      seedContent: opts?.seedContent,
    })),
  clear: () => set({ seedShortCode: undefined, seedContent: undefined }),
}));
