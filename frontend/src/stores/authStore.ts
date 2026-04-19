import { create } from 'zustand';
import type { User } from '@/lib/types';
import { authService } from '@/lib/services/authService';
import { stopTokenRefreshTimer } from '@/lib/api';
import { queryClient } from '@/lib/queryClient';

interface AuthState {
  user: User | null;
  loading: boolean;
  serverUnreachable: boolean;
  initialize: () => Promise<void>;
  signIn: (email: string, password: string, rememberMe?: boolean) => Promise<{ error: string | null; requires2FA?: boolean; twoFAToken?: string }>;
  verify2FASignIn: (twoFaToken: string, code: string, useRecoveryCode: boolean, rememberMe?: boolean) => Promise<{ error: string | null }>;
  signUp: (email: string, password: string, fullName: string) => Promise<{ error: string | null }>;
  signOut: () => void;
  updateUser: (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => Promise<void>;
}

let _initializing = false;

export function clearClientSession() {
  stopTokenRefreshTimer();
  queryClient.clear();

  try {
    localStorage.clear();
  } catch {
    // Ignore storage access failures during logout.
  }

  try {
    sessionStorage.clear();
  } catch {
    // Ignore storage access failures during logout.
  }
}

function persistAuthSession(user: User, accessToken: string, refreshToken: string, rememberMe: boolean) {
  localStorage.setItem('access_token', accessToken);
  localStorage.setItem('refresh_token', refreshToken);
  localStorage.setItem('remember_me', rememberMe ? '1' : '0');
  useAuthStore.setState({ user, serverUnreachable: false });
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  serverUnreachable: false,

  initialize: async () => {
    if (_initializing) return;
    _initializing = true;
    try {
      const token = localStorage.getItem('access_token');
      if (token) {
        const { data, error, isNetworkError } = await authService.me();
        if (data && !error) {
          set({ user: data, loading: false, serverUnreachable: false });
        } else if (isNetworkError) {
          // Server unreachable / CORS error — keep tokens, don't log out
          set({ loading: false, serverUnreachable: true });
        } else {
          // Genuine auth failure (401, invalid token, etc.) — clear session
          localStorage.removeItem('access_token');
          localStorage.removeItem('refresh_token');
          set({ loading: false, serverUnreachable: false });
        }
      } else {
        set({ loading: false });
      }
    } finally {
      _initializing = false;
    }
  },

  signIn: async (email: string, password: string, rememberMe = false) => {
    const { data, error } = await authService.signin(email, password, rememberMe);
    if (error || !data) return { error: error || 'Sign in failed' };

    if (data.requires_2fa && data.two_fa_token) {
      return { error: null, requires2FA: true, twoFAToken: data.two_fa_token };
    }
    if (!data.user || !data.access_token || !data.refresh_token) {
      return { error: 'Sign in failed' };
    }

    persistAuthSession(data.user, data.access_token, data.refresh_token, rememberMe);
    return { error: null };
  },

  verify2FASignIn: async (twoFaToken: string, code: string, useRecoveryCode: boolean, rememberMe = false) => {
    const { data, error } = await authService.verify2FASignin(twoFaToken, code, useRecoveryCode);
    if (error || !data) return { error: error || 'Verification failed' };

    persistAuthSession(data.user, data.access_token, data.refresh_token, rememberMe);
    return { error: null };
  },

  signUp: async (email: string, password: string, fullName: string) => {
    const { data, error } = await authService.signup(email, password, fullName);
    if (error || !data) return { error: error || 'Sign up failed' };
    persistAuthSession(data.user, data.access_token, data.refresh_token, false);
    return { error: null };
  },

  signOut: () => {
    clearClientSession();
    set({ user: null });
    window.location.replace('/login');
  },

  updateUser: async (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => {
    const { data: updated, error } = await authService.updateProfile(data);
    if (error || !updated) {
      throw new Error(error || 'Failed to update profile');
    }
    set({ user: updated });
  },
}));
