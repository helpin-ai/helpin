import { create } from 'zustand';
import { registerAppRefreshGuard } from '@/lib/appLoadRecovery';
import type { DockChat, DockChatDetail } from '@/lib/dockTypes';
import type { AgentRunMessage } from '@/lib/pmTypes';

const TRANSCRIPT_VIEW_KEY = 'helpin:agent-dock-transcript-view';
export type DockTranscriptView = 'timeline' | 'detailed';

function readTranscriptView(): DockTranscriptView {
  try { return localStorage.getItem(TRANSCRIPT_VIEW_KEY) === 'detailed' ? 'detailed' : 'timeline'; }
  catch { return 'timeline'; }
}

const COLLAPSED_KEY = 'helpin:ask-agents-dock-collapsed';
const SELECTION_KEY_PREFIX = 'helpin:agent-dock-selection:';
const DRAFTS_KEY_PREFIX = 'helpin:agent-dock-drafts:';

function readDrafts(workspaceId: string): Record<string, string> {
  try {
    const value = JSON.parse(sessionStorage.getItem(`${DRAFTS_KEY_PREFIX}${workspaceId}`) ?? '{}');
    return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
  } catch { return {}; }
}

function writeDrafts(workspaceId: string | null, drafts: Record<string, string>): boolean {
  if (!workspaceId) return true;
  try {
    const saved = Object.fromEntries(Object.entries(drafts).filter(([, text]) => text.length > 0));
    if (Object.keys(saved).length) sessionStorage.setItem(`${DRAFTS_KEY_PREFIX}${workspaceId}`, JSON.stringify(saved));
    else sessionStorage.removeItem(`${DRAFTS_KEY_PREFIX}${workspaceId}`);
    return true;
  } catch { return !Object.values(drafts).some(value => value.length > 0); }
}

export type DockView = 'chat' | 'chats';
export type DockTab = 'agents' | 'chats';

interface PersistedSelection {
  tab?: DockTab;
  chatId?: string | null;
  runId?: string | null;
}

export interface DockTranscriptCacheEntry {
  detail: DockChatDetail;
  messages: AgentRunMessage[];
  nextBefore: number | null;
}

interface DockState {
  transcriptView: DockTranscriptView;
  setTranscriptView: (view: DockTranscriptView) => void;
  collapsed: boolean;
  view: DockView;
  tab: DockTab;
  workspaceId: string | null;
  activeChatId: string | null;
  activeRunId: string | null;
  chats: DockChat[];
  transcripts: Record<string, DockTranscriptCacheEntry>;
  drafts: Record<string, string>;
  lastAttentionIds: string[];
  activateWorkspace: (workspaceId: string) => void;
  setCollapsed: (collapsed: boolean) => void;
  setView: (view: DockView) => void;
  setTab: (tab: DockTab) => void;
  setActiveChatId: (chatId: string | null) => void;
  setActiveRunId: (runId: string | null) => void;
  setChats: (chats: DockChat[]) => void;
  cacheTranscript: (chatId: string, transcript: DockTranscriptCacheEntry) => void;
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
  transcriptView: readTranscriptView(),
  setTranscriptView: (transcriptView) => {
    try { localStorage.setItem(TRANSCRIPT_VIEW_KEY, transcriptView); } catch { /* Best-effort persistence. */ }
    set({ transcriptView });
  },
  collapsed: readCollapsed(),
  view: 'chat',
  tab: 'agents',
  workspaceId: null,
  activeChatId: null,
  activeRunId: null,
  chats: [],
  transcripts: {},
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
      transcripts: {},
      drafts: readDrafts(workspaceId),
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
  cacheTranscript: (chatId, transcript) => set({ transcripts: { [chatId]: transcript } }),
  upsertChat: (chat) => set((state) => {
    const index = state.chats.findIndex((candidate) => candidate.id === chat.id);
    if (index < 0) return { chats: [chat, ...state.chats] };
    const next = state.chats.slice();
    next[index] = chat;
    return { chats: next };
  }),
  setDraft: (key, value) => set((state) => {
    const drafts = { ...state.drafts, [key]: value };
    writeDrafts(state.workspaceId, drafts);
    return { drafts };
  }),
  clearDraft: (key) => set((state) => {
    const drafts = { ...state.drafts };
    delete drafts[key];
    writeDrafts(state.workspaceId, drafts);
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

registerAppRefreshGuard(() => {
  const { workspaceId, drafts } = useDockStore.getState();
  return writeDrafts(workspaceId, drafts) ? null : 'Your Ask agent draft could not be saved. Copy it before refreshing.';
});
