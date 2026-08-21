import { create } from 'zustand'
import { failedSendsReducer, type FailedSend } from './failed-sends-reducer'

export type ComposerMode = 'reply' | 'note'

export interface Draft {
  text: string
  mode: ComposerMode
  /** Sends that failed after `useSendMessage` rolled back its optimistic bubble — kept per-conversation (and persisted) so a chip from conversation A can never leak into, or be retried against, conversation B. */
  failedSends: FailedSend[]
}

export const DEFAULT_DRAFT: Draft = { text: '', mode: 'reply', failedSends: [] }

const STORAGE_KEY = 'support_composer_drafts'

/** Persisted entries may predate `failedSends` (or be partially corrupt) — normalize so every hydrated draft has the full shape. */
function normalizeDraft(value: Partial<Draft>): Draft {
  return {
    text: typeof value.text === 'string' ? value.text : '',
    mode: value.mode === 'note' ? 'note' : 'reply',
    failedSends: Array.isArray(value.failedSends) ? value.failedSends : [],
  }
}

let warnedOnce = false

/** One warning per session — enough of a breadcrumb to debug "my drafts don't survive navigation" without spamming the console on every keystroke. */
function warnPersistenceFailure(operation: string, error: unknown): void {
  if (warnedOnce) return
  warnedOnce = true
  console.warn(`[draft-store] sessionStorage ${operation} failed — drafts will not survive route changes`, error)
}

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
    if (parsed && typeof parsed === 'object') {
      const normalized: Record<string, Draft> = {}
      for (const [key, value] of Object.entries(parsed as Record<string, Partial<Draft> | null>)) {
        if (value && typeof value === 'object') normalized[key] = normalizeDraft(value)
      }
      return normalized
    }
  } catch (error) {
    warnPersistenceFailure('hydrate', error)
  }
  return {}
}

function persistDrafts(drafts: Record<string, Draft>): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(drafts))
  } catch (error) {
    // Storage unavailable/full — the in-memory zustand state still works for
    // this session, it just won't survive a route change.
    warnPersistenceFailure('write', error)
  }
}

interface DraftState {
  drafts: Record<string, Draft>
  setText: (conversationId: string, text: string) => void
  setMode: (conversationId: string, mode: ComposerMode) => void
  clearDraft: (conversationId: string) => void
  addFailedSend: (conversationId: string, failedSend: FailedSend) => void
  removeFailedSend: (conversationId: string, id: string) => void
}

/**
 * Per-conversation composer draft (text + reply/note mode + failed-send retry
 * chips), persisted to `sessionStorage` so all of it survives route changes
 * (navigating away from a conversation and back) but not app restarts. The
 * text/mode draft is cleared only when a send is initiated (see
 * `composer.tsx`) — never on navigation or blur — and failed sends stay until
 * retried successfully or dismissed, so typed content is never silently lost
 * (short of the app closing while a send is still in flight).
 */
export const useDraftStore = create<DraftState>((set, get) => {
  const update = (conversationId: string, updater: (current: Draft) => Draft) => {
    const current = get().drafts[conversationId] ?? DEFAULT_DRAFT
    const drafts = { ...get().drafts, [conversationId]: updater(current) }
    persistDrafts(drafts)
    set({ drafts })
  }

  return {
    drafts: loadDrafts(),
    setText: (conversationId, text) => update(conversationId, (current) => ({ ...current, text })),
    setMode: (conversationId, mode) => update(conversationId, (current) => ({ ...current, mode })),
    // "Clear" resets text+mode but must NOT drop failed-send chips: clearing
    // fires when a new send starts, and any earlier failures still need their
    // retry affordance. With no chips left, the whole entry is removed so the
    // persisted map doesn't accumulate empty husks for every conversation
    // ever replied to.
    clearDraft: (conversationId) => {
      const current = get().drafts[conversationId]
      if (current && current.failedSends.length > 0) {
        update(conversationId, () => ({ ...DEFAULT_DRAFT, failedSends: current.failedSends }))
        return
      }
      const drafts = { ...get().drafts }
      delete drafts[conversationId]
      persistDrafts(drafts)
      set({ drafts })
    },
    addFailedSend: (conversationId, failedSend) =>
      update(conversationId, (current) => ({
        ...current,
        failedSends: failedSendsReducer(current.failedSends, { type: 'add', failedSend }),
      })),
    removeFailedSend: (conversationId, id) =>
      update(conversationId, (current) => ({
        ...current,
        failedSends: failedSendsReducer(current.failedSends, { type: 'remove', id }),
      })),
  }
})
