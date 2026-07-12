import { create } from 'zustand'

interface ResolvedTransitionState {
  /**
   * A conversation resolved from the thread screen, handed to the inbox so it
   * can animate that row out of the list (and offer Undo) on return. Consumed
   * and cleared once by the inbox.
   */
  pendingResolvedId: string | null
  markResolved: (conversationId: string) => void
  clear: () => void
}

export const useResolvedTransitionStore = create<ResolvedTransitionState>((set) => ({
  pendingResolvedId: null,
  markResolved: (conversationId) => set({ pendingResolvedId: conversationId }),
  clear: () => set({ pendingResolvedId: null }),
}))
