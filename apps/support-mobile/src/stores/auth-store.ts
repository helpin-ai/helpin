import { create } from 'zustand'
import {
  clearSession,
  getAccessToken,
  getRefreshToken,
  hydrateSessionStorage,
} from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
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
}
