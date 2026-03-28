import { create } from 'zustand';

export type NavFilter = 'my_inbox' | 'all' | 'unassigned' | 'mentions' | 'ai_all' | 'ai_resolved' | 'ai_escalated' | 'ai_pending';
export type ReplyMode = 'reply' | 'note';
export type ActivePanel = 'nav' | 'list' | 'thread' | 'detail';

const STORAGE_KEY = 'support_inbox_ui';
const DRAFTS_STORAGE_KEY = 'support_inbox_drafts';

interface PersistedState {
  navCollapsed: boolean;
  detailSidebarCollapsed: boolean;
  selectedMailboxId: string;
}

function loadPersisted(): PersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...{ navCollapsed: false, detailSidebarCollapsed: false, selectedMailboxId: 'shared' }, ...JSON.parse(raw) };
  } catch {}
  return { navCollapsed: false, detailSidebarCollapsed: false, selectedMailboxId: 'shared' };
}

function savePersisted(state: PersistedState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {}
}

function loadDrafts(): Record<string, string> {
  try {
    const raw = localStorage.getItem(DRAFTS_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') return parsed as Record<string, string>;
    }
  } catch {}
  return {};
}

let _draftSaveTimer: ReturnType<typeof setTimeout> | null = null;

function saveDraftsDebounced(drafts: Record<string, string>) {
  if (_draftSaveTimer) clearTimeout(_draftSaveTimer);
  _draftSaveTimer = setTimeout(() => {
    try {
      const filtered = Object.fromEntries(
        Object.entries(drafts).filter(([, v]) => v.length > 0)
      );
      if (Object.keys(filtered).length === 0) {
        localStorage.removeItem(DRAFTS_STORAGE_KEY);
      } else {
        localStorage.setItem(DRAFTS_STORAGE_KEY, JSON.stringify(filtered));
      }
    } catch {}
  }, 500);
}

function removeDraftFromStorage(conversationId: string) {
  try {
    const raw = localStorage.getItem(DRAFTS_STORAGE_KEY);
    if (!raw) return;
    const drafts = JSON.parse(raw) as Record<string, string>;
    if (conversationId in drafts) {
      delete drafts[conversationId];
      if (Object.keys(drafts).length === 0) {
        localStorage.removeItem(DRAFTS_STORAGE_KEY);
      } else {
        localStorage.setItem(DRAFTS_STORAGE_KEY, JSON.stringify(drafts));
      }
    }
  } catch {}
}

interface SupportInboxState {
  // Navigation
  navFilter: NavFilter;
  navCollapsed: boolean;
  selectedMailboxId: string;
  // Filters
  statusFilter: string;
  searchQuery: string;
  // Selection
  selectedConversationId: string | null;
  // Reply
  replyMode: ReplyMode;
  // Detail sidebar
  detailSidebarCollapsed: boolean;
  // Create dialog
  createDialogOpen: boolean;
  // Mobile
  activePanel: ActivePanel;
  // Drafts: conversationId → unsent textarea content
  drafts: Record<string, string>;

  // Actions
  setNavFilter: (filter: NavFilter) => void;
  setSelectedMailboxId: (mailboxId: string) => void;
  toggleNavCollapsed: () => void;
  setStatusFilter: (status: string) => void;
  setSearchQuery: (query: string) => void;
  selectConversation: (id: string | null) => void;
  setReplyMode: (mode: ReplyMode) => void;
  toggleDetailSidebar: () => void;
  setCreateDialogOpen: (open: boolean) => void;
  setActivePanel: (panel: ActivePanel) => void;
  setDraft: (conversationId: string, content: string) => void;
  clearDraft: (conversationId: string) => void;
}

export const useSupportInboxStore = create<SupportInboxState>((set, get) => {
  const persisted = loadPersisted();
  const persistedDrafts = loadDrafts();

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
    activePanel: 'list',
    drafts: persistedDrafts,

    setNavFilter: (filter) => set({ navFilter: filter, statusFilter: 'all' }),
    setSelectedMailboxId: (mailboxId) => {
      set({ selectedMailboxId: mailboxId, selectedConversationId: null, activePanel: 'list' });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: mailboxId,
      });
    },
    toggleNavCollapsed: () => {
      const next = !get().navCollapsed;
      set({ navCollapsed: next });
      savePersisted({
        navCollapsed: next,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: get().selectedMailboxId,
      });
    },
    setStatusFilter: (status) => set({ statusFilter: status }),
    setSearchQuery: (query) => set({ searchQuery: query }),
    selectConversation: (id) => set({ selectedConversationId: id, activePanel: id ? 'thread' : 'list' }),
    setReplyMode: (mode) => set({ replyMode: mode }),
    toggleDetailSidebar: () => {
      const next = !get().detailSidebarCollapsed;
      set({ detailSidebarCollapsed: next });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: next,
        selectedMailboxId: get().selectedMailboxId,
      });
    },
    setCreateDialogOpen: (open) => set({ createDialogOpen: open }),
    setActivePanel: (panel) => set({ activePanel: panel }),
    setDraft: (conversationId, content) =>
      set((state) => {
        if (!content) {
          const { [conversationId]: _, ...rest } = state.drafts;
          saveDraftsDebounced(rest);
          return { drafts: rest };
        }
        const next = { ...state.drafts, [conversationId]: content };
        saveDraftsDebounced(next);
        return { drafts: next };
      }),
    clearDraft: (conversationId) =>
      set((state) => {
        if (!(conversationId in state.drafts)) return state;
        const { [conversationId]: _, ...rest } = state.drafts;
        removeDraftFromStorage(conversationId);
        return { drafts: rest };
      }),
  };
});
