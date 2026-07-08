import { create } from 'zustand'

export type ComposerMode = 'reply' | 'note'

export interface Draft {
  text: string
  mode: ComposerMode
}

export const DEFAULT_DRAFT: Draft = { text: '', mode: 'reply' }

const STORAGE_KEY = 'support_composer_drafts'

/**
 * Reads the persisted drafts map once at module load. Wrapped in try/catch
 * because `sessionStorage` can throw (private browsing quirks, disabled
 * storage) — a corrupt or inaccessible store should never crash the composer,
 * it should just start with no drafts.
 */
function loadDrafts(): Record<string, Draft> {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as unknown
    if (parsed && typeof parsed === 'object') return parsed as Record<string, Draft>
  } catch {
    // Ignore malformed/unavailable storage — fall through to an empty map.
  }
  return {}
}

function persistDrafts(drafts: Record<string, Draft>): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(drafts))
  } catch {
    // Storage unavailable/full — the in-memory zustand state still works for
    // this session, it just won't survive a route change.
  }
}

interface DraftState {
  drafts: Record<string, Draft>
  setText: (conversationId: string, text: string) => void
  setMode: (conversationId: string, mode: ComposerMode) => void
  clearDraft: (conversationId: string) => void
}

/**
 * Per-conversation composer draft (text + reply/note mode), persisted to
 * `sessionStorage` so drafts survive route changes (navigating away from a
 * conversation and back) but not app restarts. Cleared only on a confirmed
 * send success (see `composer.tsx`) — never on navigation or blur, so an
 * in-progress reply is never silently lost.
 */
export const useDraftStore = create<DraftState>((set, get) => ({
  drafts: loadDrafts(),
  setText: (conversationId, text) => {
    const current = get().drafts[conversationId] ?? DEFAULT_DRAFT
    const drafts = { ...get().drafts, [conversationId]: { ...current, text } }
    persistDrafts(drafts)
    set({ drafts })
  },
  setMode: (conversationId, mode) => {
    const current = get().drafts[conversationId] ?? DEFAULT_DRAFT
    const drafts = { ...get().drafts, [conversationId]: { ...current, mode } }
    persistDrafts(drafts)
    set({ drafts })
  },
  clearDraft: (conversationId) => {
    const drafts = { ...get().drafts }
    delete drafts[conversationId]
    persistDrafts(drafts)
    set({ drafts })
  },
}))
