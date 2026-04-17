import type { ReactNode } from 'react';
import { create } from 'zustand';

interface PageHeaderState {
  titleOverride: string | null;
  actions: ReactNode;
  setTitleOverride: (value: string | null) => void;
  setActions: (node: ReactNode) => void;
  reset: () => void;
}

export const usePageHeaderStore = create<PageHeaderState>((set) => ({
  titleOverride: null,
  actions: null,
  setTitleOverride: (value) => set({ titleOverride: value }),
  setActions: (node) => set({ actions: node }),
  reset: () => set({ titleOverride: null, actions: null }),
}));
