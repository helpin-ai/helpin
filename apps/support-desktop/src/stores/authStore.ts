import { create } from 'zustand'
import { clearSession, getAccessToken, hydrateSessionStorage, writeSession } from '@helpin-ai/support-core'
import { stopTokenRefreshTimer } from '@/lib/api'
import { authService } from '@/lib/services/authService'
import type { User } from '@/lib/types'

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
  initialize: () => Promise<void>
  signIn: (email: string, password: string, rememberMe?: boolean) => Promise<{ error: string | null }>
  signOut: () => void
}

let initializing = false

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  serverUnreachable: false,

  initialize: async () => {
    if (initializing) {
      return
    }

    initializing = true

    try {
      await hydrateSessionStorage()

      if (!getAccessToken()) {
        set({ user: null, loading: false, serverUnreachable: false })
        return
      }

      const { data, error, isNetworkError } = await authService.me()
      if (data && !error) {
        set({ user: data, loading: false, serverUnreachable: false })
        return
      }

      if (isNetworkError) {
        set({ user: null, loading: false, serverUnreachable: true })
        return
      }

      await clearSession()
      set({ user: null, loading: false, serverUnreachable: false })
    } catch {
      set({ user: null, loading: false, serverUnreachable: false })
    } finally {
      initializing = false
    }
  },

  signIn: async (email: string, password: string, rememberMe = false) => {
    const { data, error } = await authService.signin(email, password, rememberMe)
    if (error || !data) {
      return { error: error || 'Sign in failed' }
    }

    try {
      await writeSession({
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        rememberMe,
      })
    } catch {
      return { error: 'Failed to persist your session on this device' }
    }
    set({ user: data.user, loading: false, serverUnreachable: false })
    return { error: null }
  },

  signOut: () => {
    void (async () => {
      try {
        await clearSession()
      } finally {
        stopTokenRefreshTimer()
        set({ user: null, serverUnreachable: false })
        window.location.href = '/login'
      }
    })()
  },
}))
