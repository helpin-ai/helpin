import { create } from 'zustand';
import type { DockChat } from '@/lib/dockTypes';

const COLLAPSED_KEY = 'helpin:ask-agents-dock-collapsed';
const ACTIVE_CHAT_KEY = 'helpin:dock-active-chat';

export type DockView = 'chat' | 'chats';

interface DockState {
  collapsed: boolean;
  view: DockView;
  activeChatId: string | null;
  chats: DockChat[];
  setCollapsed: (collapsed: boolean) => void;
  setView: (view: DockView) => void;
  setActiveChatId: (chatId: string | null) => void;
  setChats: (chats: DockChat[]) => void;
  upsertChat: (chat: DockChat) => void;
}

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) !== '0';
  } catch {
    return true;
  }
}

function readActiveChat(): string | null {
  try {
    return localStorage.getItem(ACTIVE_CHAT_KEY);
  } catch {
    return null;
  }
}

export const useDockStore = create<DockState>((set) => ({
  collapsed: readCollapsed(),
  view: 'chat',
  activeChatId: readActiveChat(),
  chats: [],
  setCollapsed: (collapsed) => {
    try {
      localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0');
    } catch {
      // best effort
    }
    set({ collapsed });
  },
  setView: (view) => set({ view }),
  setActiveChatId: (chatId) => {
    try {
      if (chatId) localStorage.setItem(ACTIVE_CHAT_KEY, chatId);
      else localStorage.removeItem(ACTIVE_CHAT_KEY);
    } catch {
      // best effort
    }
    set({ activeChatId: chatId });
  },
  setChats: (chats) => set({ chats }),
  upsertChat: (chat) =>
    set((state) => {
      const index = state.chats.findIndex((c) => c.id === chat.id);
      if (index < 0) return { chats: [chat, ...state.chats] };
      const next = state.chats.slice();
      next[index] = chat;
      return { chats: next };
    }),
}));
