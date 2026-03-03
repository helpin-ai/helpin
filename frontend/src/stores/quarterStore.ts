import { create } from 'zustand';
import type { Quarter } from '@/lib/types';
import { quartersService } from '@/lib/services/quartersService';

interface QuarterState {
  quarters: Quarter[];
  currentQuarter: Quarter | null;
  loading: boolean;
  loadQuarters: (workspaceId: string) => Promise<void>;
  setCurrentQuarter: (q: Quarter) => void;
}

export const useQuarterStore = create<QuarterState>((set) => ({
  quarters: [],
  currentQuarter: null,
  loading: false,

  loadQuarters: async (workspaceId: string) => {
    set({ loading: true });
    const { data } = await quartersService.list(workspaceId);
    if (data) {
      const active = data.find((q) => q.status === 'active');
      set({
        quarters: data,
        currentQuarter: active ?? data[0] ?? null,
        loading: false,
      });
    } else {
      set({ loading: false });
    }
  },

  setCurrentQuarter: (q: Quarter) => {
    set({ currentQuarter: q });
  },
}));
