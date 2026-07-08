import { create } from 'zustand'
import {
  clearSession,
  getAccessToken,
  getRefreshToken,
  hydrateSessionStorage,
} from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import type { User } from '@mobile/lib/types'

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
}

export const useAuthStore = create<AuthState>(() => ({
  user: null,
  loading: true,
  serverUnreachable: false,
}))

export async function bootstrapAuth(): Promise<void> {
  await hydrateSessionStorage()
  if (!getAccessToken() && !getRefreshToken()) {
    useAuthStore.setState({ user: null, loading: false })
    return
  }
  const { data, error, isNetworkError } = await authService.me()
  if (data && !error) {
    useAuthStore.setState({ user: data, loading: false, serverUnreachable: false })
  } else if (isNetworkError) {
    // Server unreachable — keep tokens, do NOT clear the session
    useAuthStore.setState({ loading: false, serverUnreachable: true })
  } else {
    // Genuine auth failure (401, invalid token) — clear session
    await clearSession()
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  }
}

export async function signOut(): Promise<void> {
  await clearSession()
  useAuthStore.setState({ user: null })
  // Tear down realtime: RootRealtimeMount (router.tsx) keys its websocket on
  // useWorkspaceStore's currentWorkspace.id. Leaving it populated after the
  // tokens are cleared would keep a token-less socket looping reconnect
  // attempts forever on the login screen.
  useWorkspaceStore.getState().setCurrentWorkspace(null)
}
