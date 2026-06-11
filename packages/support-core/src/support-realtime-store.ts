import { create } from 'zustand'

export type SupportRealtimeStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'offline'
  | 'stale'
  | 'disconnected'
  | 'auth_expired'

interface SupportRealtimeState {
  status: SupportRealtimeStatus
  retryCount: number
  lastConnectedAt: number | null
  lastEventAt: number | null
  lastResyncAt: number | null
  lastDisconnectAt: number | null
  message: string | null
  setStatus: (status: SupportRealtimeStatus, message?: string | null, retryCount?: number) => void
  markConnected: () => void
  markEvent: () => void
  markResynced: () => void
  reset: () => void
}

const initialState = {
  status: 'idle' as const,
  retryCount: 0,
  lastConnectedAt: null,
  lastEventAt: null,
  lastResyncAt: null,
  lastDisconnectAt: null,
  message: null,
}

export const useSupportRealtimeStore = create<SupportRealtimeState>((set) => ({
  ...initialState,
  setStatus: (status, message = null, retryCount) =>
    set((state) => ({
      ...state,
      status,
      message,
      retryCount: retryCount ?? state.retryCount,
      lastDisconnectAt:
        status === 'reconnecting' || status === 'offline' || status === 'stale' || status === 'disconnected'
          ? Date.now()
          : state.lastDisconnectAt,
    })),
  markConnected: () =>
    set((state) => ({
      ...state,
      status: 'connected',
      message: null,
      retryCount: 0,
      lastConnectedAt: Date.now(),
    })),
  markEvent: () =>
    set((state) => ({
      ...state,
      lastEventAt: Date.now(),
    })),
  markResynced: () =>
    set((state) => ({
      ...state,
      lastResyncAt: Date.now(),
    })),
  reset: () => set(initialState),
}))
