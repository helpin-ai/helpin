import { create } from 'zustand';
import type {
  ConversationAssignmentFilter,
  ConversationListFilters,
  ConversationSortOrder,
  ConversationStateFilter,
} from '@/lib/supportInboxFilters';
import { defaultConversationListFiltersForNav } from '@/lib/supportInboxFilters';
import type { SupportSystemTag } from '@/lib/pmTypes';

export type NavFilter = 'inbox' | 'mine' | 'waiting' | 'resolved' | 'spam' | 'ai_active' | 'resolved_by_ai';
export type ReplyMode = 'reply' | 'note';
export type ActivePanel = 'nav' | 'list' | 'thread' | 'detail';

const STORAGE_KEY = 'support_inbox_ui';
const DRAFTS_STORAGE_KEY = 'support_inbox_drafts';
const STORE_VERSION = 2;

interface PersistedState {
  navCollapsed: boolean;
  detailSidebarCollapsed: boolean;
  selectedMailboxId: string;
  version: number;
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
  } catch {}
  return { navCollapsed: false, detailSidebarCollapsed: false, selectedMailboxId: 'all', version: STORE_VERSION };
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

function parseAssignmentFilters(value: unknown): ConversationAssignmentFilter[] {
  return parseStringList(value).filter((entry): entry is ConversationAssignmentFilter =>
    entry === 'me' ||
    entry === 'unassigned' ||
    entry === 'others'
  );
}

function parseSortOrder(value: unknown): ConversationSortOrder {
  return value === 'oldest' ? 'oldest' : 'newest';
}

function parseStringList(value: unknown): string[] {
  if (typeof value !== 'string') return [];
  return value.split(',').map((entry) => entry.trim()).filter(Boolean);
}

function parseSystemTags(value: unknown, legacyAI?: unknown): SupportSystemTag[] {
  const tags = parseStringList(value).filter((tag): tag is SupportSystemTag =>
    tag === 'ai_handoff' ||
    tag === 'ai_resolved'
  );
  if (legacyAI === 'needs_human' && !tags.includes('ai_handoff')) {
    tags.push('ai_handoff');
  }
  if (legacyAI === 'resolved_by_ai' && !tags.includes('ai_resolved')) {
    tags.push('ai_resolved');
  }
  return tags;
}

function parseStates(value: unknown, navFilter: NavFilter): ConversationStateFilter[] {
  if (typeof value !== 'string') {
    return defaultConversationListFiltersForNav(navFilter).states;
  }
  const states = value.split(',').filter((state): state is ConversationStateFilter =>
    state === 'open' ||
    state === 'waiting_on_customer' ||
    state === 'resolved' ||
    state === 'spam'
  );
  return states.length > 0 ? states : defaultConversationListFiltersForNav(navFilter).states;
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
  // Selection
  selectedConversationId: string | null;
  // Reply
  replyMode: ReplyMode;
  // Detail sidebar
  detailSidebarCollapsed: boolean;
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
  selectConversation: (id: string | null) => void;
  setReplyMode: (mode: ReplyMode) => void;
  toggleDetailSidebar: () => void;
  setCreateDialogOpen: (open: boolean) => void;
  setTeamInboxDialogOpen: (open: boolean) => void;
  setEditMailboxId: (id: string | null) => void;
  setActivePanel: (panel: ActivePanel) => void;
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
        activeCustomViewId: null,
        conversationListFilters: defaultConversationListFiltersForNav(filter),
        selectedMailboxId: 'all',
        selectedConversationId: null,
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
      set({
        navFilter: 'inbox',
        statusFilter: 'all',
        activeCustomViewId: null,
        conversationListFilters: defaultConversationListFiltersForNav('inbox'),
        selectedMailboxId: mailboxId,
        selectedConversationId: null,
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
        selectedMailboxId: mailboxId,
        selectedConversationId: null,
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
    setStatusFilter: (status) => set({ statusFilter: status, activeCustomViewId: null }),
    setSearchQuery: (query) => set({ searchQuery: query, activeCustomViewId: null }),
    setConversationListFilter: (key, value) =>
      set((state) => ({
        activeCustomViewId: null,
        conversationListFilters: {
          ...state.conversationListFilters,
          [key]: value,
        },
        selectedConversationId: null,
      })),
    setConversationMailboxFilters: (mailboxIds) =>
      set((state) => ({
        activeCustomViewId: null,
        selectedMailboxId: 'all',
        conversationListFilters: {
          ...state.conversationListFilters,
          mailboxIds,
        },
        selectedConversationId: null,
      })),
    resetConversationListFilters: () =>
      set({
        activeCustomViewId: null,
        conversationListFilters: defaultConversationListFiltersForNav(get().navFilter),
        selectedConversationId: null,
      }),
    applyCustomView: (view) => {
      const filters = view.filters ?? {};
      const navFilter = parseNavFilter(filters.nav_filter);
      set({
        activeCustomViewId: view.id,
        navFilter,
        selectedMailboxId: filters.mailbox_id || 'all',
        statusFilter: 'all',
        searchQuery: filters.search || '',
        conversationListFilters: {
          states: parseStates(filters.states, navFilter),
          assignment: parseAssignmentFilters(filters.assignment),
          mailboxIds: parseStringList(filters.mailbox_ids),
          tagIds: parseStringList(filters.tag_ids),
          systemTags: parseSystemTags(filters.system_tags, filters.ai),
          sort: parseSortOrder(filters.sort),
        },
        selectedConversationId: null,
        activePanel: 'list',
      });
      savePersisted({
        navCollapsed: get().navCollapsed,
        detailSidebarCollapsed: get().detailSidebarCollapsed,
        selectedMailboxId: filters.mailbox_id || 'all',
        version: STORE_VERSION,
      });
    },
    selectConversation: (id) => set({ selectedConversationId: id, activePanel: id ? 'thread' : 'list' }),
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
    setCreateDialogOpen: (open) => set({ createDialogOpen: open }),
    setTeamInboxDialogOpen: (open) => set({ teamInboxDialogOpen: open, ...(!open && { editMailboxId: null }) }),
    setEditMailboxId: (id) => set({ editMailboxId: id }),
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
