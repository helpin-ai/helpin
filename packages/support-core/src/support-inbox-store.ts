import { create } from 'zustand'

export type NavFilter = 'my_inbox' | 'all' | 'unassigned' | 'mentions' | 'ai_active' | 'resolved_by_ai'
export type ReplyMode = 'reply' | 'note'
export type ActivePanel = 'nav' | 'list' | 'thread' | 'detail'

const STORAGE_KEY = 'support_inbox_ui'
const DRAFTS_STORAGE_KEY = 'support_inbox_drafts'
const STORE_VERSION = 2

interface PersistedState {
  navCollapsed: boolean
  detailSidebarCollapsed: boolean
  selectedMailboxId: string
  version: number
}

function loadPersisted(): PersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<PersistedState>
      const version = typeof parsed.version === 'number' ? parsed.version : 1
      let selectedMailboxId = parsed.selectedMailboxId ?? 'all'
      if (version < STORE_VERSION && selectedMailboxId === 'shared') {
        selectedMailboxId = 'all'
      }
      return {
        navCollapsed: parsed.navCollapsed ?? false,
        detailSidebarCollapsed: parsed.detailSidebarCollapsed ?? false,
        selectedMailboxId,
        version: STORE_VERSION,
      }
    }
  } catch {}
  return { navCollapsed: false, detailSidebarCollapsed: false, selectedMailboxId: 'all', version: STORE_VERSION }
}

function savePersisted(state: PersistedState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {}
}

function loadDrafts(): Record<string, string> {
  try {
    const raw = localStorage.getItem(DRAFTS_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === 'object') return parsed as Record<string, string>
    }
  } catch {}
  return {}
}

let draftSaveTimer: ReturnType<typeof setTimeout> | null = null

function saveDraftsDebounced(drafts: Record<string, string>) {
  if (draftSaveTimer) clearTimeout(draftSaveTimer)
  draftSaveTimer = setTimeout(() => {
    try {
      const filtered = Object.fromEntries(Object.entries(drafts).filter(([, value]) => value.length > 0))
      if (Object.keys(filtered).length === 0) {
        localStorage.removeItem(DRAFTS_STORAGE_KEY)
      } else {
        localStorage.setItem(DRAFTS_STORAGE_KEY, JSON.stringify(filtered))
      }
    } catch {}
  }, 500)
}

function removeDraftFromStorage(conversationId: string) {
  try {
    const raw = localStorage.getItem(DRAFTS_STORAGE_KEY)
    if (!raw) return
    const drafts = JSON.parse(raw) as Record<string, string>
    if (conversationId in drafts) {
      delete drafts[conversationId]
      if (Object.keys(drafts).length === 0) {
        localStorage.removeItem(DRAFTS_STORAGE_KEY)
      } else {
        localStorage.setItem(DRAFTS_STORAGE_KEY, JSON.stringify(drafts))
      }
    }
  } catch {}
}

interface SupportInboxState {
  navFilter: NavFilter
  navCollapsed: boolean
  selectedMailboxId: string
  statusFilter: string
  searchQuery: string
  selectedConversationId: string | null
  replyMode: ReplyMode
  detailSidebarCollapsed: boolean
  createDialogOpen: boolean
  teamInboxDialogOpen: boolean
  editMailboxId: string | null
  activePanel: ActivePanel
  drafts: Record<string, string>
  setNavFilter: (filter: NavFilter) => void
  setSelectedMailboxId: (mailboxId: string) => void
  toggleNavCollapsed: () => void
  setStatusFilter: (status: string) => void
  setSearchQuery: (query: string) => void
  selectConversation: (id: string | null) => void
  setReplyMode: (mode: ReplyMode) => void
  toggleDetailSidebar: () => void
  setCreateDialogOpen: (open: boolean) => void
  setTeamInboxDialogOpen: (open: boolean) => void
  setEditMailboxId: (id: string | null) => void
  setActivePanel: (panel: ActivePanel) => void
  setDraft: (conversationId: string, content: string) => void
  clearDraft: (conversationId: string) => void
}

export const useSupportInboxStore = create<SupportInboxState>((set, get) => {
  const persisted = loadPersisted()
  const persistedDrafts = loadDrafts()

  return {
    navFilter: 'all',
    navCollapsed: persisted.navCollapsed,
    selectedMailboxId: persisted.selectedMailboxId,
    statusFilter: 'all',
    searchQuery: '',
    selectedConversationId: null,
    replyMode: 'reply',
    detailSidebarCollapsed: persisted.detailSidebarCollapsed,
    createDialogOpen: false,
    teamInboxDialogOpen: false,
    editMailboxId: null,
    activePanel: 'list',
    drafts: persistedDrafts,
    setNavFilter: (filter) => {
      set({
        navFilter: filter,
        statusFilter: 'all',
        selectedMailboxId: 'all',
        selectedConversationId: null,
        activePanel: 'list',
      })
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: 'all',
        version: STORE_VERSION,
      })
    },
    setSelectedMailboxId: (mailboxId) => {
      set({
        selectedMailboxId: mailboxId,
        selectedConversationId: null,
        activePanel: 'list',
      })
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: mailboxId,
        version: STORE_VERSION,
      })
    },
    toggleNavCollapsed: () => {
      const next = !get().navCollapsed
      set({ navCollapsed: next })
      savePersisted({
        navCollapsed: next,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: get().selectedMailboxId,
        version: STORE_VERSION,
      })
    },
    setStatusFilter: (status) => set({ statusFilter: status }),
    setSearchQuery: (query) => set({ searchQuery: query }),
    selectConversation: (id) => set({ selectedConversationId: id, activePanel: id ? 'thread' : 'list' }),
    setReplyMode: (mode) => set({ replyMode: mode }),
    toggleDetailSidebar: () => {
      const next = !get().detailSidebarCollapsed
      set({ detailSidebarCollapsed: next })
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: next,
        selectedMailboxId: get().selectedMailboxId,
        version: STORE_VERSION,
      })
    },
    setCreateDialogOpen: (open) => set({ createDialogOpen: open }),
    setTeamInboxDialogOpen: (open) => set({ teamInboxDialogOpen: open, ...(!open && { editMailboxId: null }) }),
    setEditMailboxId: (id) => set({ editMailboxId: id }),
    setActivePanel: (panel) => set({ activePanel: panel }),
    setDraft: (conversationId, content) =>
      set((state) => {
        if (!content) {
          const { [conversationId]: ignored, ...rest } = state.drafts
          void ignored
          saveDraftsDebounced(rest)
          return { drafts: rest }
        }
        const next = { ...state.drafts, [conversationId]: content }
        saveDraftsDebounced(next)
        return { drafts: next }
      }),
    clearDraft: (conversationId) =>
      set((state) => {
        if (!(conversationId in state.drafts)) return state
        const { [conversationId]: ignored, ...rest } = state.drafts
        void ignored
        removeDraftFromStorage(conversationId)
        return { drafts: rest }
      }),
  }
})
