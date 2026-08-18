import { create } from 'zustand';
import type { ConversationListFilters } from '@/lib/supportInboxFilters';
import { defaultConversationListFiltersForNav, parseSupportInboxViewFilters } from '@/lib/supportInboxFilters';

export type NavFilter = 'inbox' | 'mine' | 'waiting' | 'resolved' | 'spam' | 'ai_active' | 'resolved_by_ai';
export type ReplyMode = 'reply' | 'note';
export type DetailSidebarMode = 'details' | 'agents';
export type ActivePanel = 'nav' | 'list' | 'thread' | 'detail';
export type ConversationHandoff = {
  fromConversationId: string;
  toConversationId: string | null;
};

const STORAGE_KEY = 'support_inbox_ui';
const DRAFTS_STORAGE_KEY = 'support_inbox_drafts';
const STORE_VERSION = 3;

interface PersistedState {
  navCollapsed: boolean;
  detailSidebarCollapsed: boolean;
  selectedMailboxId: string;
  version: number;
}

export function supportInboxBuiltinViewKey(navFilter: NavFilter, selectedMailboxId = 'all'): string {
  return navFilter === 'inbox' && selectedMailboxId !== 'all'
    ? `team:${selectedMailboxId}`
    : `nav:${navFilter}`;
}

function loadPersisted(): PersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<PersistedState>;
      const version = typeof parsed.version === 'number' ? parsed.version : 1;
      let selectedMailboxId = parsed.selectedMailboxId ?? 'all';
      if (version < STORE_VERSION && selectedMailboxId === 'shared') {
        selectedMailboxId = 'all';
      }
      return {
        navCollapsed: parsed.navCollapsed ?? false,
        detailSidebarCollapsed: parsed.detailSidebarCollapsed ?? false,
        selectedMailboxId,
        version: STORE_VERSION,
      };
    }
  } catch {
    // Ignore unavailable or corrupt local storage.
  }
  return {
    navCollapsed: false,
    detailSidebarCollapsed: false,
    selectedMailboxId: 'all',
    version: STORE_VERSION,
  };
}

function savePersisted(state: PersistedState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Ignore local storage write failures.
  }
}

function recordStringMapEqual(a: Record<string, string>, b: Record<string, string>): boolean {
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  return aKeys.length === bKeys.length && aKeys.every((key) => a[key] === b[key]);
}

function builtinViewFilterMapsEqual(
  a: Record<string, Record<string, string>>,
  b: Record<string, Record<string, string>>,
): boolean {
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  return aKeys.length === bKeys.length && aKeys.every((key) => b[key] && recordStringMapEqual(a[key], b[key]));
}

function loadDrafts(): Record<string, string> {
  try {
    const raw = localStorage.getItem(DRAFTS_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') return parsed as Record<string, string>;
    }
  } catch {
    // Ignore unavailable or corrupt draft storage.
  }
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
    } catch {
      // Ignore local storage write failures.
    }
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
  } catch {
    // Ignore unavailable or corrupt draft storage.
  }
}

function parseNavFilter(value: unknown): NavFilter {
  return value === 'mine' ||
    value === 'waiting' ||
    value === 'resolved' ||
    value === 'spam' ||
    value === 'ai_active' ||
    value === 'resolved_by_ai'
    ? value
    : 'inbox';
}

interface SupportInboxState {
  // Navigation
  navFilter: NavFilter;
  navCollapsed: boolean;
  selectedMailboxId: string;
  // Filters
  statusFilter: string;
  searchQuery: string;
  conversationListFilters: ConversationListFilters;
  activeCustomViewId: string | null;
  customViewDirty: boolean;
  builtinViewFilters: Record<string, Record<string, string>>;
  // Selection
  selectedConversationId: string | null;
  conversationHandoff: ConversationHandoff | null;
  // Reply
  replyMode: ReplyMode;
  // Detail sidebar
  detailSidebarCollapsed: boolean;
  detailSidebarMode: DetailSidebarMode;
  // Create dialog
  createDialogOpen: boolean;
  teamInboxDialogOpen: boolean;
  editMailboxId: string | null;
  // Mobile
  activePanel: ActivePanel;
  // Drafts: conversationId → unsent textarea content
  drafts: Record<string, string>;

  // Actions
  setNavFilter: (filter: NavFilter) => void;
  setSelectedMailboxId: (mailboxId: string) => void;
  setMailboxFilter: (mailboxId: string) => void;
  syncRouteState: (state: {
    navFilter: NavFilter;
    selectedMailboxId: string;
    statusFilter: string;
    searchQuery: string;
    activeCustomViewId?: string | null;
    conversationListFilters?: ConversationListFilters;
  }) => void;
  toggleNavCollapsed: () => void;
  setStatusFilter: (status: string) => void;
  setSearchQuery: (query: string) => void;
  setConversationListFilter: <K extends keyof ConversationListFilters>(key: K, value: ConversationListFilters[K]) => void;
  setConversationMailboxFilters: (mailboxIds: string[]) => void;
  resetConversationListFilters: () => void;
  applyCustomView: (view: { id: string; filters: Record<string, string> }) => void;
  markCustomViewClean: () => void;
  setBuiltinViewFilters: (filters: Record<string, Record<string, string>>) => void;
  setBuiltinViewFilter: (key: string, filters: Record<string, string>) => void;
  selectConversation: (id: string | null) => void;
  startConversationHandoff: (fromConversationId: string, toConversationId: string | null) => void;
  finishConversationHandoff: (toConversationId: string | null) => void;
  cancelConversationHandoff: () => void;
  setReplyMode: (mode: ReplyMode) => void;
  toggleDetailSidebar: () => void;
  setDetailSidebarMode: (mode: DetailSidebarMode) => void;
  setCreateDialogOpen: (open: boolean) => void;
  setTeamInboxDialogOpen: (open: boolean) => void;
  setEditMailboxId: (id: string | null) => void;
  setActivePanel: (panel: ActivePanel) => void;
  showReopenedConversationInInbox: (conversationId: string, conversationMailboxId?: string | null) => void;
  setDraft: (conversationId: string, content: string) => void;
  clearDraft: (conversationId: string) => void;
}

export const useSupportInboxStore = create<SupportInboxState>((set, get) => {
  const persisted = loadPersisted();
  const persistedDrafts = loadDrafts();

  return {
    navFilter: 'inbox',
    navCollapsed: persisted.navCollapsed,
    selectedMailboxId: persisted.selectedMailboxId,
    statusFilter: 'all',
    searchQuery: '',
    conversationListFilters: defaultConversationListFiltersForNav('inbox'),
    activeCustomViewId: null,
    customViewDirty: false,
    builtinViewFilters: {},
    selectedConversationId: null,
    conversationHandoff: null,
    replyMode: 'reply',
    detailSidebarCollapsed: persisted.detailSidebarCollapsed,
    detailSidebarMode: 'details',
    createDialogOpen: false,
    teamInboxDialogOpen: false,
    editMailboxId: null,
    activePanel: 'list',
    drafts: persistedDrafts,

    setNavFilter: (filter) => {
      const savedFilters = get().builtinViewFilters[supportInboxBuiltinViewKey(filter)];
      const savedState = savedFilters ? parseSupportInboxViewFilters(savedFilters, filter) : null;
      set({
        navFilter: filter,
        statusFilter: 'all',
        activeCustomViewId: null,
        customViewDirty: false,
        searchQuery: savedState?.searchQuery ?? '',
        conversationListFilters: savedState?.listFilters ?? defaultConversationListFiltersForNav(filter),
        selectedMailboxId: 'all',
        selectedConversationId: null,
        conversationHandoff: null,
        activePanel: 'list',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: 'all',
        version: STORE_VERSION,
      });
    },
    setSelectedMailboxId: (mailboxId) => {
      const savedFilters = get().builtinViewFilters[supportInboxBuiltinViewKey('inbox', mailboxId)];
      const savedState = savedFilters ? parseSupportInboxViewFilters(savedFilters, 'inbox') : null;
      set({
        navFilter: 'inbox',
        statusFilter: 'all',
        activeCustomViewId: null,
        customViewDirty: false,
        searchQuery: savedState?.searchQuery ?? '',
        conversationListFilters: savedState?.listFilters ?? defaultConversationListFiltersForNav('inbox'),
        selectedMailboxId: mailboxId,
        selectedConversationId: null,
        conversationHandoff: null,
        activePanel: 'list',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: mailboxId,
        version: STORE_VERSION,
      });
    },
    setMailboxFilter: (mailboxId) => {
      set({
        activeCustomViewId: null,
        customViewDirty: false,
        selectedMailboxId: mailboxId,
        selectedConversationId: null,
        conversationHandoff: null,
        activePanel: 'list',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: mailboxId,
        version: STORE_VERSION,
      });
    },
    syncRouteState: ({ navFilter, selectedMailboxId, statusFilter, searchQuery, activeCustomViewId, conversationListFilters }) => {
      set({
        navFilter,
        selectedMailboxId,
        statusFilter,
        searchQuery,
        activeCustomViewId: activeCustomViewId ?? null,
        customViewDirty: false,
        conversationListFilters: conversationListFilters ?? defaultConversationListFiltersForNav(navFilter),
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId,
        version: STORE_VERSION,
      });
    },
    toggleNavCollapsed: () => {
      const next = !get().navCollapsed;
      set({ navCollapsed: next });
      savePersisted({
        navCollapsed: next,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: get().selectedMailboxId,
        version: STORE_VERSION,
      });
    },
    setStatusFilter: (status) =>
      set((state) => ({
        statusFilter: status,
        customViewDirty: state.activeCustomViewId ? true : state.customViewDirty,
      })),
    setSearchQuery: (query) =>
      set((state) => ({
        searchQuery: query,
        customViewDirty: state.activeCustomViewId ? true : state.customViewDirty,
      })),
    setConversationListFilter: (key, value) =>
      set((state) => ({
        customViewDirty: state.activeCustomViewId ? true : state.customViewDirty,
        conversationListFilters: {
          ...state.conversationListFilters,
          [key]: value,
        },
        selectedConversationId: null,
        conversationHandoff: null,
      })),
    setConversationMailboxFilters: (mailboxIds) =>
      set((state) => ({
        customViewDirty: state.activeCustomViewId ? true : state.customViewDirty,
        selectedMailboxId: 'all',
        conversationListFilters: {
          ...state.conversationListFilters,
          mailboxIds,
        },
        selectedConversationId: null,
        conversationHandoff: null,
      })),
    resetConversationListFilters: () =>
      set({
        activeCustomViewId: null,
        customViewDirty: false,
        conversationListFilters: defaultConversationListFiltersForNav(get().navFilter),
        selectedConversationId: null,
        conversationHandoff: null,
      }),
    applyCustomView: (view) => {
      const filters = view.filters ?? {};
      const navFilter = parseNavFilter(filters.nav_filter);
      const parsed = parseSupportInboxViewFilters(filters, navFilter);
      set({
        activeCustomViewId: view.id,
        customViewDirty: false,
        navFilter,
        selectedMailboxId: parsed.selectedMailboxId,
        statusFilter: 'all',
        searchQuery: parsed.searchQuery,
        conversationListFilters: parsed.listFilters,
        selectedConversationId: null,
        conversationHandoff: null,
        activePanel: 'list',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: parsed.selectedMailboxId,
        version: STORE_VERSION,
      });
    },
    markCustomViewClean: () => set({ customViewDirty: false }),
    setBuiltinViewFilters: (filters) =>
      set((state) => (builtinViewFilterMapsEqual(state.builtinViewFilters, filters) ? state : { builtinViewFilters: filters })),
    setBuiltinViewFilter: (key, filters) =>
      set((state) => ({
        activeCustomViewId: null,
        customViewDirty: false,
        builtinViewFilters: {
          ...state.builtinViewFilters,
          [key]: filters,
        },
      })),
    selectConversation: (id) => set({ selectedConversationId: id, conversationHandoff: null, activePanel: id ? 'thread' : 'list' }),
    startConversationHandoff: (fromConversationId, toConversationId) =>
      set({ conversationHandoff: { fromConversationId, toConversationId } }),
    finishConversationHandoff: (toConversationId) =>
      set({ selectedConversationId: toConversationId, conversationHandoff: null, activePanel: toConversationId ? 'thread' : 'list' }),
    cancelConversationHandoff: () => set({ conversationHandoff: null }),
    setReplyMode: (mode) => set({ replyMode: mode }),
    toggleDetailSidebar: () => {
      const next = !get().detailSidebarCollapsed;
      set({ detailSidebarCollapsed: next });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: next,
        selectedMailboxId: get().selectedMailboxId,
        version: STORE_VERSION,
      });
    },
    setDetailSidebarMode: (mode) => set({ detailSidebarMode: mode }),
    setCreateDialogOpen: (open) => set({ createDialogOpen: open }),
    setTeamInboxDialogOpen: (open) => set({ teamInboxDialogOpen: open, ...(!open && { editMailboxId: null }) }),
    setEditMailboxId: (id) => set({ editMailboxId: id }),
    setActivePanel: (panel) => set({ activePanel: panel }),
    showReopenedConversationInInbox: (conversationId, conversationMailboxId) => {
      const currentMailboxId = get().selectedMailboxId;
      const nextMailboxId = conversationMailboxId && currentMailboxId === conversationMailboxId
        ? currentMailboxId
        : 'all';
      set({
        navFilter: 'inbox',
        selectedMailboxId: nextMailboxId,
        statusFilter: 'all',
        searchQuery: '',
        conversationListFilters: defaultConversationListFiltersForNav('inbox'),
        activeCustomViewId: null,
        customViewDirty: false,
        selectedConversationId: conversationId,
        conversationHandoff: null,
        activePanel: 'thread',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: nextMailboxId,
        version: STORE_VERSION,
      });
    },
    setDraft: (conversationId, content) =>
      set((state) => {
        if (!content) {
          const rest = { ...state.drafts };
          delete rest[conversationId];
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
        const rest = { ...state.drafts };
        delete rest[conversationId];
        removeDraftFromStorage(conversationId);
        return { drafts: rest };
      }),
  };
});
