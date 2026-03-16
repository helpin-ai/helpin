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

export type WSSendFn = (type: string, data: Record<string, unknown>) => void;

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
  // Typing indicators: conversationId → content string when typing, false when not
  typingIndicators: Record<string, string | false>;
  // Agent typing: conversationId → map of actorId → content (supports multiple agents)
  agentTyping: Record<string, Record<string, string>>;
  // Viewing presence: conversationId → set of agent userIds currently viewing
  viewingAgents: Record<string, string[]>;
  // Drafts: conversationId → unsent textarea content
  drafts: Record<string, string>;

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
  setTyping: (conversationId: string, isTyping: boolean, content?: string) => void;
  setAgentTyping: (conversationId: string, actorId: string | null, content?: string) => void;
  clearOneAgentTyping: (conversationId: string, actorId: string) => void;
  setViewingAgent: (conversationId: string, actorId: string, viewing: boolean) => void;
  setDraft: (conversationId: string, content: string) => void;
  clearDraft: (conversationId: string) => void;
  // WS send function — set by useRealtimeSync when connection is established
  wsSend: WSSendFn | null;
  setWsSend: (fn: WSSendFn | null) => void;
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
    typingIndicators: {},
    agentTyping: {},
    viewingAgents: {},
    drafts: {},
    wsSend: null,
    setWsSend: (fn) => set({ wsSend: fn }),

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
    setTyping: (conversationId, isTyping, content) =>
      set((state) => ({
        typingIndicators: { ...state.typingIndicators, [conversationId]: isTyping ? (content ?? '') : false },
      })),
    setAgentTyping: (conversationId, actorId, content) =>
      set((state) => {
        const current = state.agentTyping[conversationId] ?? {};
        if (!actorId) {
          // Clear all agent typing for this conversation
          if (Object.keys(current).length === 0) return state;
          return { agentTyping: { ...state.agentTyping, [conversationId]: {} } };
        }
        return {
          agentTyping: {
            ...state.agentTyping,
            [conversationId]: { ...current, [actorId]: content ?? '' },
          },
        };
      }),
    clearOneAgentTyping: (conversationId, actorId) =>
      set((state) => {
        const current = state.agentTyping[conversationId];
        if (!current || !(actorId in current)) return state;
        const { [actorId]: _, ...rest } = current;
        return { agentTyping: { ...state.agentTyping, [conversationId]: rest } };
      }),
    setViewingAgent: (conversationId, actorId, viewing) =>
      set((state) => {
        const current = state.viewingAgents[conversationId] ?? [];
        if (viewing && current.includes(actorId)) return state;
        if (!viewing && !current.includes(actorId)) return state;
        const next = viewing ? [...current, actorId] : current.filter((id) => id !== actorId);
        return { viewingAgents: { ...state.viewingAgents, [conversationId]: next } };
      }),
    setDraft: (conversationId, content) =>
      set((state) => {
        if (!content) {
          const { [conversationId]: _, ...rest } = state.drafts;
          return { drafts: rest };
        }
        return { drafts: { ...state.drafts, [conversationId]: content } };
      }),
    clearDraft: (conversationId) =>
      set((state) => {
        if (!(conversationId in state.drafts)) return state;
        const { [conversationId]: _, ...rest } = state.drafts;
        return { drafts: rest };
      }),
  };
});
