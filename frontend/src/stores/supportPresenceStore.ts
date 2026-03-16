import { create } from 'zustand';

export type WSSendFn = (type: string, data: Record<string, unknown>) => void;

interface SupportPresenceState {
  // Typing indicators: conversationId → content string when typing, false when not
  typingIndicators: Record<string, string | false>;
  // Agent typing: conversationId → map of actorId → content (supports multiple agents)
  agentTyping: Record<string, Record<string, string>>;
  // Viewing presence: conversationId → set of agent userIds currently viewing
  viewingAgents: Record<string, string[]>;
  // WS send function — set by useRealtimeSync when connection is established
  wsSend: WSSendFn | null;
  // WebSocket connection state — guards presence/typing sends
  wsConnected: boolean;

  // Actions
  setTyping: (conversationId: string, isTyping: boolean, content?: string) => void;
  setAgentTyping: (conversationId: string, actorId: string | null, content?: string) => void;
  clearOneAgentTyping: (conversationId: string, actorId: string) => void;
  setViewingAgent: (conversationId: string, actorId: string, viewing: boolean) => void;
  setWsSend: (fn: WSSendFn | null) => void;
  setWsConnected: (connected: boolean) => void;
}

export const useSupportPresenceStore = create<SupportPresenceState>((set) => ({
  typingIndicators: {},
  agentTyping: {},
  viewingAgents: {},
  wsSend: null,
  wsConnected: false,

  setWsSend: (fn) => set({ wsSend: fn }),
  setWsConnected: (connected) => set({ wsConnected: connected }),

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
}));
