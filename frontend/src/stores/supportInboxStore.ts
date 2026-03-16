import { create } from 'zustand';

export type NavFilter = 'my_inbox' | 'all' | 'unassigned';
export type ReplyMode = 'reply' | 'note';
export type ActivePanel = 'nav' | 'list' | 'thread' | 'detail';

const STORAGE_KEY = 'support_inbox_ui';

interface PersistedState {
  navCollapsed: boolean;
  detailSidebarCollapsed: boolean;
}

function loadPersisted(): PersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...{ navCollapsed: false, detailSidebarCollapsed: false }, ...JSON.parse(raw) };
  } catch {}
  return { navCollapsed: false, detailSidebarCollapsed: false };
}

function savePersisted(state: PersistedState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {}
}

interface SupportInboxState {
  // Navigation
  navFilter: NavFilter;
  navCollapsed: boolean;
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

  // Actions
  setNavFilter: (filter: NavFilter) => void;
  toggleNavCollapsed: () => void;
  setStatusFilter: (status: string) => void;
  setSearchQuery: (query: string) => void;
  selectConversation: (id: string | null) => void;
  setReplyMode: (mode: ReplyMode) => void;
  toggleDetailSidebar: () => void;
  setCreateDialogOpen: (open: boolean) => void;
  setActivePanel: (panel: ActivePanel) => void;
}

export const useSupportInboxStore = create<SupportInboxState>((set, get) => {
  const persisted = loadPersisted();

  return {
    navFilter: 'all',
    navCollapsed: persisted.navCollapsed,
    statusFilter: 'open',
    searchQuery: '',
    selectedConversationId: null,
    replyMode: 'reply',
    detailSidebarCollapsed: persisted.detailSidebarCollapsed,
    createDialogOpen: false,
    activePanel: 'list',

    setNavFilter: (filter) => set({ navFilter: filter }),
    toggleNavCollapsed: () => {
      const next = !get().navCollapsed;
      set({ navCollapsed: next });
      savePersisted({ navCollapsed: next, detailSidebarCollapsed: get().detailSidebarCollapsed });
    },
    setStatusFilter: (status) => set({ statusFilter: status }),
    setSearchQuery: (query) => set({ searchQuery: query }),
    selectConversation: (id) => set({ selectedConversationId: id, activePanel: id ? 'thread' : 'list' }),
    setReplyMode: (mode) => set({ replyMode: mode }),
    toggleDetailSidebar: () => {
      const next = !get().detailSidebarCollapsed;
      set({ detailSidebarCollapsed: next });
      savePersisted({ navCollapsed: get().navCollapsed, detailSidebarCollapsed: next });
    },
    setCreateDialogOpen: (open) => set({ createDialogOpen: open }),
    setActivePanel: (panel) => set({ activePanel: panel }),
  };
});
