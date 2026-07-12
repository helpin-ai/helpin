import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import type { ViewSelection } from '@mobile/inbox/use-inbox-filters'

const DEFAULT_SELECTION: ViewSelection = { kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' }

interface SupportViewState {
  selection: ViewSelection
  setSelection: (selection: ViewSelection) => void
}

/**
 * Which inbox view/mailbox/custom-view is active. Persisted to sessionStorage so
 * it survives inbox↔thread navigation within a session, but resets on app
 * restart (a fresh launch starts on Inbox).
 */
export const useSupportViewStore = create<SupportViewState>()(
  persist(
    (set) => ({
      selection: DEFAULT_SELECTION,
      setSelection: (selection) => set({ selection }),
    }),
    {
      name: 'support-view-selection',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({ selection: state.selection }),
    },
  ),
)
