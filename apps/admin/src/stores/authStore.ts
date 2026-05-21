import { create } from 'zustand'
import { stopTokenRefreshTimer } from '@/lib/api'
import { authService } from '@/lib/services/authService'
import { passkeyService } from '@/lib/services/passkeyService'
import type { User } from '@/lib/types'

interface AdminTokenClaims {
  pa?: boolean
  is_platform_admin?: boolean
  mfa?: boolean
  mfa_satisfied?: boolean
}

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
  initialize: () => Promise<void>
  signInWithPasskey: (
    emailHint?: string,
    rememberMe?: boolean,
    options?: { useAutofill?: boolean },
  ) => Promise<{ error: string | null; cancelled?: boolean }>
  signInWithPassword: (
    email: string,
    password: string,
    rememberMe?: boolean,
  ) => Promise<{ error: string | null; requires2FA?: boolean; twoFAToken?: string }>
  verify2FASignIn: (
    twoFaToken: string,
    code: string,
    useRecoveryCode: boolean,
    rememberMe?: boolean,
  ) => Promise<{ error: string | null }>
  signOut: () => void
}

let initializing = false

function decodeClaims(accessToken: string): AdminTokenClaims | null {
  const [, payload] = accessToken.split('.')
  if (!payload) {
    return null
  }

  try {
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const decoded = atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '='))
    return JSON.parse(decoded) as AdminTokenClaims
  } catch {
    return null
  }
}

function claimsAllowAdmin(accessToken: string): boolean {
  const claims = decodeClaims(accessToken)
  return Boolean((claims?.pa || claims?.is_platform_admin) && (claims?.mfa || claims?.mfa_satisfied))
}

function clearAuthSession() {
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('remember_me')
  stopTokenRefreshTimer()
}

function withTokenState(user: User, accessToken: string): User {
  return { ...user, mfa_satisfied_in_token: claimsAllowAdmin(accessToken) }
}

function persistAuthSession(user: User, accessToken: string, refreshToken: string, rememberMe: boolean, set: (state: Partial<AuthState>) => void): string | null {
  if (!user.is_platform_admin || !claimsAllowAdmin(accessToken)) {
    clearAuthSession()
    set({ user: null, serverUnreachable: false, loading: false })
    return 'This account is not authorized for admin tools.'
  }

  void refreshToken
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
  localStorage.setItem('remember_me', rememberMe ? '1' : '0')
  set({ user: withTokenState(user, accessToken), serverUnreachable: false, loading: false })
  return null
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
      const { data, error, isNetworkError } = await authService.me()
      if (data && !error && data.is_platform_admin && data.mfa_satisfied_in_token) {
        set({ user: { ...data, mfa_satisfied_in_token: true }, loading: false, serverUnreachable: false })
        return
      }

      if (isNetworkError) {
        set({ loading: false, serverUnreachable: true })
        return
      }

      clearAuthSession()
      set({ user: null, loading: false, serverUnreachable: false })
    } finally {
      initializing = false
    }
  },

  signInWithPasskey: async (emailHint?: string, rememberMe = true, options?: { useAutofill?: boolean }) => {
    const { data, error, code, cancelled } = await passkeyService.beginAuthentication(emailHint, rememberMe, options)
    if (cancelled) {
      return { error: null, cancelled: true }
    }
    if (error || !data) {
      if (code === 'no_passkey') {
        return { error: 'Register a passkey in the main app before using admin tools.' }
      }
      return { error: error || 'Passkey sign in failed' }
    }
    if (!data.user || !data.access_token) {
      return { error: 'Passkey sign in failed' }
    }

    const adminError = persistAuthSession(data.user, data.access_token, data.refresh_token ?? '', rememberMe, set)
    if (adminError) {
      return { error: adminError }
    }
    return { error: null }
  },

  signInWithPassword: async (email: string, password: string, rememberMe = true) => {
    const { data, error } = await authService.signin(email, password, rememberMe)
    if (error || !data) {
      return { error: error || 'Sign in failed' }
    }
    if (data.requires_2fa && data.two_fa_token) {
      return { error: null, requires2FA: true, twoFAToken: data.two_fa_token }
    }
    if (!data.user || !data.access_token) {
      return { error: 'Sign in failed' }
    }

    const adminError = persistAuthSession(data.user, data.access_token, data.refresh_token ?? '', rememberMe, set)
    if (adminError) {
      return { error: adminError }
    }
    return { error: null }
  },

  verify2FASignIn: async (twoFaToken: string, code: string, useRecoveryCode: boolean, rememberMe = true) => {
    const { data, error } = await authService.verify2FASignin(twoFaToken, code, useRecoveryCode)
    if (error || !data || !data.user || !data.access_token) {
      return { error: error || 'Verification failed' }
    }

    const adminError = persistAuthSession(data.user, data.access_token, data.refresh_token ?? '', rememberMe, set)
    if (adminError) {
      return { error: adminError }
    }
    return { error: null }
  },

  signOut: () => {
    void authService.signout()
    clearAuthSession()
    set({ user: null, serverUnreachable: false })
    window.location.href = '/admin/login'
  },
}))
