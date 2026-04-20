import { create } from 'zustand'
import { stopTokenRefreshTimer } from '@/lib/api'
import { authService } from '@/lib/services/authService'
import { passkeyService } from '@/lib/services/passkeyService'
import type { User } from '@/lib/types'

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
  initialize: () => Promise<void>
  signIn: (email: string, password: string, rememberMe?: boolean) => Promise<{ error: string | null }>
  signInWithPasskey: (emailHint?: string, rememberMe?: boolean) => Promise<{ error: string | null }>
  signOut: () => void
}

let initializing = false

function persistAuthSession(user: User, accessToken: string, refreshToken: string, rememberMe: boolean, set: (state: Partial<AuthState>) => void) {
  localStorage.setItem('access_token', accessToken)
  localStorage.setItem('refresh_token', refreshToken)
  localStorage.setItem('remember_me', rememberMe ? '1' : '0')
  set({ user, serverUnreachable: false, loading: false })
}

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
      const token = localStorage.getItem('access_token')
      if (!token) {
        set({ loading: false, serverUnreachable: false })
        return
      }

      const { data, error, isNetworkError } = await authService.me()
      if (data && !error) {
        set({ user: data, loading: false, serverUnreachable: false })
        return
      }

      if (isNetworkError) {
        set({ loading: false, serverUnreachable: true })
        return
      }

      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
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
    if (!data.user || !data.access_token || !data.refresh_token) {
      return { error: data.requires_2fa ? 'Two-factor verification is only available in the main app.' : 'Sign in failed' }
    }

    persistAuthSession(data.user, data.access_token, data.refresh_token, rememberMe, set)
    return { error: null }
  },

  signInWithPasskey: async (emailHint?: string, rememberMe = false) => {
    const { data, error } = await passkeyService.beginAuthentication(emailHint, rememberMe)
    if (error || !data) {
      return { error: error || 'Passkey sign in failed' }
    }
    if (!data.user || !data.access_token || !data.refresh_token) {
      return { error: data.requires_2fa ? 'Two-factor verification is only available in the main app.' : 'Passkey sign in failed' }
    }

    persistAuthSession(data.user, data.access_token, data.refresh_token, rememberMe, set)
    return { error: null }
  },

  signOut: () => {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('remember_me')
    stopTokenRefreshTimer()
    set({ user: null, serverUnreachable: false })
    window.location.href = '/admin/login'
  },
}))
