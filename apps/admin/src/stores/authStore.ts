import { create } from 'zustand'
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

    localStorage.setItem('access_token', data.access_token)
    localStorage.setItem('refresh_token', data.refresh_token)
    localStorage.setItem('remember_me', rememberMe ? '1' : '0')
    set({ user: data.user, serverUnreachable: false, loading: false })
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
