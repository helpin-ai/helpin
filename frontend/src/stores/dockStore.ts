import { create } from 'zustand';
import type { DockChat } from '@/lib/dockTypes';

const COLLAPSED_KEY = 'helpin:ask-agents-dock-collapsed';
const SELECTION_KEY_PREFIX = 'helpin:agent-dock-selection:';

export type DockView = 'chat' | 'chats';
export type DockTab = 'agents' | 'chats';

interface PersistedSelection {
  tab?: DockTab;
  chatId?: string | null;
  runId?: string | null;
}

interface DockState {
  collapsed: boolean;
  view: DockView;
  tab: DockTab;
  workspaceId: string | null;
  activeChatId: string | null;
  activeRunId: string | null;
  chats: DockChat[];
  drafts: Record<string, string>;
  lastAttentionIds: string[];
  activateWorkspace: (workspaceId: string) => void;
  setCollapsed: (collapsed: boolean) => void;
  setView: (view: DockView) => void;
  setTab: (tab: DockTab) => void;
  setActiveChatId: (chatId: string | null) => void;
  setActiveRunId: (runId: string | null) => void;
  setChats: (chats: DockChat[]) => void;
  upsertChat: (chat: DockChat) => void;
  setDraft: (key: string, value: string) => void;
  clearDraft: (key: string) => void;
  setLastAttentionIds: (ids: string[]) => void;
}

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) !== '0';
  } catch {
    return true;
  }
}

function readSelection(workspaceId: string): PersistedSelection {
  try {
    const raw = localStorage.getItem(`${SELECTION_KEY_PREFIX}${workspaceId}`);
    return raw ? JSON.parse(raw) as PersistedSelection : {};
  } catch {
    return {};
  }
}

function writeSelection(state: Pick<DockState, 'workspaceId' | 'tab' | 'activeChatId' | 'activeRunId'>) {
  if (!state.workspaceId) return;
  try {
    localStorage.setItem(`${SELECTION_KEY_PREFIX}${state.workspaceId}`, JSON.stringify({
      tab: state.tab,
      chatId: state.activeChatId,
      runId: state.activeRunId,
    } satisfies PersistedSelection));
  } catch {
    // Persistence is best effort; the in-memory dock remains fully usable.
  }
}

export const useDockStore = create<DockState>((set, get) => ({
  collapsed: readCollapsed(),
  view: 'chat',
  tab: 'agents',
  workspaceId: null,
  activeChatId: null,
  activeRunId: null,
  chats: [],
  drafts: {},
  lastAttentionIds: [],
  activateWorkspace: (workspaceId) => {
    if (get().workspaceId === workspaceId) return;
    const selection = readSelection(workspaceId);
    set({
      workspaceId,
      tab: selection.tab ?? 'agents',
      view: selection.tab === 'chats' ? 'chat' : 'chats',
      activeChatId: selection.chatId ?? null,
      activeRunId: selection.runId ?? null,
      chats: [],
      drafts: {},
      lastAttentionIds: [],
    });
  },
  setCollapsed: (collapsed) => {
    try {
      localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0');
    } catch {
      // best effort
    }
    set({ collapsed });
  },
  setView: (view) => set((state) => {
    const tab: DockTab = view === 'chats' ? 'chats' : state.tab;
    const next = { ...state, view, tab };
    writeSelection(next);
    return { view, tab };
  }),
  setTab: (tab) => set((state) => {
    const next = { ...state, tab, view: tab === 'chats' ? 'chat' as const : 'chats' as const };
    writeSelection(next);
    return { tab, view: next.view };
  }),
  setActiveChatId: (activeChatId) => set((state) => {
    const next = { ...state, activeChatId };
    writeSelection(next);
    return { activeChatId };
  }),
  setActiveRunId: (activeRunId) => set((state) => {
    const next = { ...state, activeRunId };
    writeSelection(next);
    return { activeRunId };
  }),
  setChats: (chats) => set({ chats }),
  upsertChat: (chat) => set((state) => {
    const index = state.chats.findIndex((candidate) => candidate.id === chat.id);
    if (index < 0) return { chats: [chat, ...state.chats] };
    const next = state.chats.slice();
    next[index] = chat;
    return { chats: next };
  }),
  setDraft: (key, value) => set((state) => ({ drafts: { ...state.drafts, [key]: value } })),
  clearDraft: (key) => set((state) => {
    const drafts = { ...state.drafts };
    delete drafts[key];
    return { drafts };
  }),
  setLastAttentionIds: (lastAttentionIds) => set((state) => {
    if (
      state.lastAttentionIds.length === lastAttentionIds.length
      && state.lastAttentionIds.every((id, index) => id === lastAttentionIds[index])
    ) return state;
    return { lastAttentionIds };
  }),
}));
