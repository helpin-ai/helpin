import { create } from 'zustand';
import type { RewardQuarter } from '@/lib/types';

interface QuarterState {
  currentQuarter: RewardQuarter | null;
  setCurrentQuarter: (q: RewardQuarter) => void;
}

export const useRewardQuarterStore = create<QuarterState>((set) => ({
  currentQuarter: null,

  setCurrentQuarter: (q: RewardQuarter) => {
    set({ currentQuarter: q });
  },
}));
