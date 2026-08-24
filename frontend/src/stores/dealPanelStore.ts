import { create } from 'zustand';

interface DealPanelState {
  dealId: string | null;
  requestKey: number;
  lastClosedDealId: string | null;
  lastClosedAt: number;
  openDeal: (dealId: string) => void;
  close: () => void;
  rememberClosedDeal: (dealId: string) => void;
  shouldSuppressOpen: (dealId: string) => boolean;
}

const RECENTLY_CLOSED_DEAL_SUPPRESSION_MS = 2_000;

export const useDealPanelStore = create<DealPanelState>((set, get) => ({
  dealId: null,
  requestKey: 0,
  lastClosedDealId: null,
  lastClosedAt: 0,
  openDeal: (dealId) => set({ dealId, requestKey: get().requestKey + 1 }),
  close: () => set({ dealId: null }),
  rememberClosedDeal: (dealId) => set({ lastClosedDealId: dealId, lastClosedAt: Date.now() }),
  shouldSuppressOpen: (dealId) => {
    const state = get();
    return state.lastClosedDealId === dealId
      && Date.now() - state.lastClosedAt < RECENTLY_CLOSED_DEAL_SUPPRESSION_MS;
  },
}));
