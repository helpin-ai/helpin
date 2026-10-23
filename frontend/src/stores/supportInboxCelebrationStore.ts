import { create } from 'zustand';

import type { InboxClearedMessage } from '@/components/support/inboxCleared/inboxClearedMessages';

/** A celebration shown after the viewer clears an inbox. */
export interface InboxCelebration {
  id: string;
  scopeKey: string;
  message: InboxClearedMessage;
  /** Conversations closed in this clear; scales the confetti. */
  clearedCount: number;
}

interface SupportInboxCelebrationState {
  /** Conversation id → when the viewer's own resolve or spam removed it. */
  ownRemovals: Record<string, number>;
  celebration: InboxCelebration | null;
  noteOwnRemovals: (conversationIds: string[]) => void;
  forgetOwnRemovals: (conversationIds: string[]) => void;
  showCelebration: (celebration: InboxCelebration) => void;
  dismissCelebration: () => void;
}

const OWN_REMOVAL_TTL_MS = 60_000;

export const useSupportInboxCelebrationStore = create<SupportInboxCelebrationState>((set) => ({
  ownRemovals: {},
  celebration: null,
  noteOwnRemovals: (conversationIds) => set((state) => {
    const now = Date.now();
    const ownRemovals: Record<string, number> = {};
    for (const [id, at] of Object.entries(state.ownRemovals)) {
      if (now - at <= OWN_REMOVAL_TTL_MS) ownRemovals[id] = at;
    }
    for (const id of conversationIds) ownRemovals[id] = now;
    return { ownRemovals };
  }),
  forgetOwnRemovals: (conversationIds) => set((state) => {
    const ownRemovals = { ...state.ownRemovals };
    for (const id of conversationIds) delete ownRemovals[id];
    return { ownRemovals };
  }),
  showCelebration: (celebration) => set({ celebration }),
  dismissCelebration: () => set((state) => (state.celebration ? { celebration: null } : state)),
}));
