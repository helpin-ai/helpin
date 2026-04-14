import { create } from 'zustand'

export type WSSendFn = (type: string, data: Record<string, unknown>) => void

export interface AgentTypingState {
  content: string
  name?: string
  avatarUrl?: string
}

interface SupportPresenceState {
  typingIndicators: Record<string, string | false>
  agentTyping: Record<string, Record<string, AgentTypingState>>
  viewingAgents: Record<string, string[]>
  onlineVisitors: Record<string, true>
  wsSend: WSSendFn | null
  wsConnected: boolean
  setTyping: (conversationId: string, isTyping: boolean, content?: string) => void
  replaceAgentTyping: (conversationId: string, next: Record<string, AgentTypingState>) => void
  setAgentTyping: (
    conversationId: string,
    actorId: string | null,
    content?: string,
    metadata?: { name?: string; avatarUrl?: string },
  ) => void
  clearOneAgentTyping: (conversationId: string, actorId: string) => void
  setViewingAgent: (conversationId: string, actorId: string, viewing: boolean) => void
  replaceViewingAgents: (conversationId: string, actorIds: string[]) => void
  setVisitorOnline: (anonymousId: string) => void
  setVisitorOffline: (anonymousId: string) => void
  setOnlineVisitors: (visitors: string[]) => void
  setWsSend: (fn: WSSendFn | null) => void
  setWsConnected: (connected: boolean) => void
}

export const useSupportPresenceStore = create<SupportPresenceState>((set) => ({
  typingIndicators: {},
  agentTyping: {},
  viewingAgents: {},
  onlineVisitors: {},
  wsSend: null,
  wsConnected: false,
  setWsSend: (fn) => set({ wsSend: fn }),
  setWsConnected: (connected) => set({ wsConnected: connected }),
  setVisitorOnline: (anonymousId) =>
    set((state) => ({
      onlineVisitors: { ...state.onlineVisitors, [anonymousId]: true as const },
    })),
  setVisitorOffline: (anonymousId) =>
    set((state) => {
      if (!(anonymousId in state.onlineVisitors)) return state
      const { [anonymousId]: ignored, ...rest } = state.onlineVisitors
      void ignored
      return { onlineVisitors: rest }
    }),
  setOnlineVisitors: (visitors) =>
    set({
      onlineVisitors: Object.fromEntries(visitors.map((id) => [id, true as const])),
    }),
  setTyping: (conversationId, isTyping, content) =>
    set((state) => ({
      typingIndicators: { ...state.typingIndicators, [conversationId]: isTyping ? (content ?? '') : false },
    })),
  replaceAgentTyping: (conversationId, next) =>
    set((state) => ({
      agentTyping: { ...state.agentTyping, [conversationId]: next },
    })),
  setAgentTyping: (conversationId, actorId, content, metadata) =>
    set((state) => {
      const current = state.agentTyping[conversationId] ?? {}
      if (!actorId) {
        if (Object.keys(current).length === 0) return state
        return { agentTyping: { ...state.agentTyping, [conversationId]: {} } }
      }
      const existing = current[actorId]
      return {
        agentTyping: {
          ...state.agentTyping,
          [conversationId]: {
            ...current,
            [actorId]: {
              content: content ?? '',
              name: metadata?.name ?? existing?.name,
              avatarUrl: metadata?.avatarUrl ?? existing?.avatarUrl,
            },
          },
        },
      }
    }),
  clearOneAgentTyping: (conversationId, actorId) =>
    set((state) => {
      const current = state.agentTyping[conversationId]
      if (!current || !(actorId in current)) return state
      const { [actorId]: ignored, ...rest } = current
      void ignored
      return { agentTyping: { ...state.agentTyping, [conversationId]: rest } }
    }),
  setViewingAgent: (conversationId, actorId, viewing) =>
    set((state) => {
      const current = state.viewingAgents[conversationId] ?? []
      if (viewing && current.includes(actorId)) return state
      if (!viewing && !current.includes(actorId)) return state
      const next = viewing ? [...current, actorId] : current.filter((id) => id !== actorId)
      return { viewingAgents: { ...state.viewingAgents, [conversationId]: next } }
    }),
  replaceViewingAgents: (conversationId, actorIds) =>
    set((state) => ({
      viewingAgents: { ...state.viewingAgents, [conversationId]: actorIds },
    })),
}))
