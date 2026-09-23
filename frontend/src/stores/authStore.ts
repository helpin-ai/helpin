import { create } from 'zustand';
import type { User } from '@/lib/types';
import { authService, type AuthConfig } from '@/lib/services/authService';
import { passkeyService } from '@/lib/services/passkeyService';
import { stopTokenRefreshTimer } from '@/lib/api';
import { queryClient } from '@/lib/queryClient';
import { resetAnalytics } from '@/lib/analytics';
import { resetHelpinIdentity } from '@/lib/helpin';
import { clearSession, hydrateSessionStorage, writeSession } from '@helpin-ai/support-core';

interface AuthState {
  configuration: AuthConfig | null;
  user: User | null;
  loading: boolean;
  serverUnreachable: boolean;
  initialize: () => Promise<void>;
  signIn: (email: string, password: string, rememberMe?: boolean) => Promise<{ error: string | null; requires2FA?: boolean; twoFAToken?: string }>;
  signInWithPasskey: (
    emailHint?: string,
    rememberMe?: boolean,
    options?: { useAutofill?: boolean },
  ) => Promise<{ error: string | null; requires2FA?: boolean; twoFAToken?: string; cancelled?: boolean }>;
  verify2FASignIn: (twoFaToken: string, code: string, useRecoveryCode: boolean, rememberMe?: boolean) => Promise<{ error: string | null }>;
  signUp: (email: string, password: string, fullName: string) => Promise<{ error: string | null }>;
  signInDemo: (email?: string) => Promise<{ error: string | null }>;
  signOut: () => Promise<void>;
  updateUser: (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => Promise<void>;
}

let _initializing = false;
let _configurationRequest: Promise<AuthConfig | null> | null = null;

/**
 * Returns the public auth configuration, fetching it once if it has not loaded.
 * Route guards await this so they never decide on a missing configuration.
 * A failed request resolves to null (conservative defaults) and may be retried.
 */
export function ensureAuthConfiguration(): Promise<AuthConfig | null> {
  const loaded = useAuthStore.getState().configuration;
  if (loaded) return Promise.resolve(loaded);
  if (!_configurationRequest) {
    _configurationRequest = authService.config()
      .then(({ data }) => {
        if (data) useAuthStore.setState({ configuration: data });
        return data ?? null;
      })
      .catch(() => null)
      .finally(() => { _configurationRequest = null; });
  }
  return _configurationRequest;
}

export async function clearClientSession() {
  resetAnalytics();
  await resetHelpinIdentity();
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

export async function persistAuthSession(user: User, accessToken: string, refreshToken: string, rememberMe: boolean) {
  await writeSession({ accessToken, refreshToken, rememberMe });
  useAuthStore.setState({ user, serverUnreachable: false });
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  configuration: null,
  loading: true,
  serverUnreachable: false,

  initialize: async () => {
    if (_initializing) return;
    _initializing = true;
    try {
      // Public capabilities are also needed on the login page. A failed config
      // request must not turn off verification or invalidate an existing session.
      await ensureAuthConfiguration();
      await hydrateSessionStorage();
      const { data, error, isNetworkError } = await authService.me();
      if (data && !error) {
        set({ user: data, loading: false, serverUnreachable: false });
      } else if (isNetworkError) {
        // Server unreachable / CORS error — keep tokens, don't log out
        set({ loading: false, serverUnreachable: true });
      } else {
        // Genuine auth failure (401, invalid token, etc.) — clear session
        await clearSession();
        set({ user: null, loading: false, serverUnreachable: false });
      }
    } catch {
      set({ user: null, loading: false, serverUnreachable: false });
    } finally {
      _initializing = false;
    }
  },

  signIn: async (email: string, password: string, rememberMe = true) => {
    const { data, error } = await authService.signin(email, password, rememberMe);
    if (error || !data) return { error: error || 'Sign in failed' };

    if (data.requires_2fa && data.two_fa_token) {
      return { error: null, requires2FA: true, twoFAToken: data.two_fa_token };
    }
    if (!data.user) {
      return { error: 'Sign in failed' };
    }

    await persistAuthSession(data.user, data.access_token ?? '', data.refresh_token ?? '', rememberMe);
    return { error: null };
  },

  signInWithPasskey: async (emailHint?: string, rememberMe = true, options?: { useAutofill?: boolean }) => {
    const { data, error, cancelled } = await passkeyService.beginAuthentication(emailHint, rememberMe, options);
    if (cancelled) return { error: null, cancelled: true };
    if (error || !data) return { error: error || 'Passkey sign in failed' };

    if (data.requires_2fa && data.two_fa_token) {
      return { error: null, requires2FA: true, twoFAToken: data.two_fa_token };
    }
    if (!data.user) {
      return { error: 'Passkey sign in failed' };
    }

    await persistAuthSession(data.user, data.access_token ?? '', data.refresh_token ?? '', rememberMe);
    return { error: null };
  },

  verify2FASignIn: async (twoFaToken: string, code: string, useRecoveryCode: boolean, rememberMe = true) => {
    const { data, error } = await authService.verify2FASignin(twoFaToken, code, useRecoveryCode);
    if (error || !data) return { error: error || 'Verification failed' };

    await persistAuthSession(data.user, data.access_token, data.refresh_token, rememberMe);
    return { error: null };
  },

  signUp: async (email: string, password: string, fullName: string) => {
    const { data, error } = await authService.signup(email, password, fullName);
    if (error || !data) return { error: error || 'Sign up failed' };
    await persistAuthSession(data.user, data.access_token, data.refresh_token, false);
    return { error: null };
  },

  signInDemo: async (email?: string) => {
    const { data, error } = await authService.demoSignin(email);
    if (error || !data) return { error: error || 'Demo sign in failed' };
    if (!data.user) {
      return { error: 'Demo sign in failed' };
    }

    await persistAuthSession(data.user, data.access_token ?? '', data.refresh_token ?? '', false);
    return { error: null };
  },

  signOut: async () => {
    await authService.signout();
    await clearClientSession();
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
