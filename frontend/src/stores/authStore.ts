import { create } from 'zustand';
import type { User } from '@/lib/types';
import { authService } from '@/lib/services/authService';

interface AuthState {
  user: User | null;
  loading: boolean;
  serverUnreachable: boolean;
  initialize: () => Promise<void>;
  signIn: (email: string, password: string) => Promise<{ error: string | null }>;
  signUp: (email: string, password: string, fullName: string) => Promise<{ error: string | null }>;
  signOut: () => void;
  updateUser: (data: { full_name?: string }) => Promise<void>;
}

let _initializing = false;

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  serverUnreachable: false,

  initialize: async () => {
    if (_initializing) return;
    _initializing = true;
    const token = localStorage.getItem('access_token');
    if (token) {
      const { data, error, isNetworkError } = await authService.me();
      if (data && !error) {
        set({ user: data, loading: false, serverUnreachable: false });
      } else if (isNetworkError) {
        // Server unreachable / CORS error — keep tokens, don't log out
        // Reset _initializing so retry is possible
        _initializing = false;
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
  },

  signIn: async (email: string, password: string) => {
    const { data, error } = await authService.signin(email, password);
    if (error || !data) return { error: error || 'Sign in failed' };
    localStorage.setItem('access_token', data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    set({ user: data.user });
    return { error: null };
  },

  signUp: async (email: string, password: string, fullName: string) => {
    const { data, error } = await authService.signup(email, password, fullName);
    if (error || !data) return { error: error || 'Sign up failed' };
    localStorage.setItem('access_token', data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    set({ user: data.user });
    return { error: null };
  },

  signOut: () => {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    set({ user: null });
    window.location.href = '/login';
  },

  updateUser: async (data: { full_name?: string }) => {
    const { data: updated } = await authService.updateProfile(data);
    if (updated) set({ user: updated });
  },
}));
