import { create } from 'zustand'

const STORAGE_KEY = 'support_pending_macros'

/** A macro inserted into a reply whose actions run when the reply is sent. */
export interface PendingSupportMacro {
  macroId: string
  /** Identifies the inserted text, so deleting it drops the actions. */
  marker: string
}

interface PendingMacroState {
  pending: Record<string, PendingSupportMacro>
  setPending: (conversationId: string, macro: PendingSupportMacro | null) => void
}

function load(): Record<string, PendingSupportMacro> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const parsed = raw ? (JSON.parse(raw) as Record<string, PendingSupportMacro>) : {}
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function save(pending: Record<string, PendingSupportMacro>) {
  try {
    if (Object.keys(pending).length === 0) localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, JSON.stringify(pending))
  } catch {
    // Storage is optional; the macro still runs in this session.
  }
}

/** Pending macros per conversation, kept with the reply drafts. */
export const usePendingMacroStore = create<PendingMacroState>((set) => ({
  pending: load(),
  setPending: (conversationId, macro) =>
    set((state) => {
      if (!macro && !(conversationId in state.pending)) return state
      const next = { ...state.pending }
      if (macro) next[conversationId] = macro
      else delete next[conversationId]
      save(next)
      return { pending: next }
    }),
}))
