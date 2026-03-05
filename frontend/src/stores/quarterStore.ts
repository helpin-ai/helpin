import { create } from 'zustand';
import type { RewardQuarter } from '@/lib/types';
import { rewardQuartersService } from '@/lib/services/quartersService';

interface QuarterState {
  quarters: RewardQuarter[];
  currentQuarter: RewardQuarter | null;
  loading: boolean;
  loadQuarters: (workspaceId: string) => Promise<void>;
  setCurrentQuarter: (q: RewardQuarter) => void;
}

export const useRewardQuarterStore = create<QuarterState>((set) => ({
  quarters: [],
  currentQuarter: null,
  loading: false,

  loadQuarters: async (workspaceId: string) => {
    set({ loading: true });
    const { data } = await rewardQuartersService.list(workspaceId);
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

  setCurrentQuarter: (q: RewardQuarter) => {
    set({ currentQuarter: q });
  },
}));
