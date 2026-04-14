import { create } from 'zustand';
import type { User } from '@/lib/types';
import { authService } from '@/lib/services/authService';
import { stopTokenRefreshTimer } from '@/lib/api';
import { queryClient } from '@/lib/queryClient';
import { clearSession, getAccessToken, hydrateSessionStorage, writeSession } from '@helpin-ai/support-core';

interface AuthState {
  user: User | null;
  loading: boolean;
  serverUnreachable: boolean;
  initialize: () => Promise<void>;
  signIn: (email: string, password: string, rememberMe?: boolean) => Promise<{ error: string | null }>;
  signUp: (email: string, password: string, fullName: string) => Promise<{ error: string | null }>;
  signOut: () => void;
  updateUser: (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => Promise<void>;
}

let _initializing = false;

export async function clearClientSession() {
  stopTokenRefreshTimer();
  queryClient.clear();

  await clearSession();

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

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  serverUnreachable: false,

  initialize: async () => {
    if (_initializing) return;
    _initializing = true;
    try {
      await hydrateSessionStorage();
      const token = getAccessToken();
      if (token) {
        const { data, error, isNetworkError } = await authService.me();
        if (data && !error) {
          set({ user: data, loading: false, serverUnreachable: false });
        } else if (isNetworkError) {
          // Server unreachable / CORS error — keep tokens, don't log out
          set({ loading: false, serverUnreachable: true });
        } else {
          // Genuine auth failure (401, invalid token, etc.) — clear session
          await clearSession();
          set({ loading: false, serverUnreachable: false });
        }
      } else {
        set({ loading: false });
      }
    } catch {
      set({ user: null, loading: false, serverUnreachable: false });
    } finally {
      _initializing = false;
    }
  },

  signIn: async (email: string, password: string, rememberMe = false) => {
    const { data, error } = await authService.signin(email, password, rememberMe);
    if (error || !data) return { error: error || 'Sign in failed' };
    try {
      await writeSession({
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        rememberMe,
      });
    } catch {
      return { error: 'Failed to persist your session on this device' };
    }
    set({ user: data.user, serverUnreachable: false });
    return { error: null };
  },

  signUp: async (email: string, password: string, fullName: string) => {
    const { data, error } = await authService.signup(email, password, fullName);
    if (error || !data) return { error: error || 'Sign up failed' };
    try {
      await writeSession({
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        rememberMe: false,
      });
    } catch {
      return { error: 'Failed to persist your session on this device' };
    }
    set({ user: data.user, serverUnreachable: false });
    return { error: null };
  },

  signOut: () => {
    void (async () => {
      try {
        await clearClientSession();
      } finally {
        set({ user: null });
        window.location.replace('/login');
      }
    })();
  },

  updateUser: async (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => {
    const { data: updated, error } = await authService.updateProfile(data);
    if (error || !updated) {
      throw new Error(error || 'Failed to update profile');
    }
    set({ user: updated });
  },
}));
