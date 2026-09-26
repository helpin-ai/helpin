import { api, API_BASE, fetchWithSessionAuth } from '../api';
import { getUsermavenAnonymousId } from '../analytics';
import type {
  AuthResponse,
  RecoveryCodesResponse,
  SigninResponse,
  TwoFASetupResponse,
  TwoFAStatusResponse,
  User,
} from '../types';

export type AuthConfig = {
  public_widget_url?: string;
  public_sdk_url?: string;
  email_verification_required: boolean;
  app_email_configured: boolean;
  google_login_enabled: boolean;
  demo_enabled?: boolean;
  demo_requires_email?: boolean;
  /** Whether the API serves the workspace Setup guide; absent on older APIs. */
  setup_guide_enabled?: boolean;
  /** Who may self-signup; absent on older APIs, which allow anyone. */
  signup_mode?: 'open' | 'invite_only' | 'domains';
  /** Allowed email domains when signup_mode is `domains`. */
  signup_allowed_domains?: string[];
  /** True until the first account exists; it may always sign up and becomes the server admin. */
  signup_first_user?: boolean;
};

/** Signup accepted, but the account must confirm its email before signing in. */
export type SignupVerificationResponse = { verification_required: true; email: string };

/** Whether this server lets people create their own account. */
export function selfSignupAllowed(config: AuthConfig | null | undefined): boolean {
  if (!config) return true;
  return Boolean(config.signup_first_user) || config.signup_mode !== 'invite_only';
}

export const authService = {
  config: () => api.get<AuthConfig>('/auth/config'),
  signup: (email: string, password: string, fullName: string) =>
    api.post<AuthResponse | SignupVerificationResponse>('/auth/signup', { email, password, full_name: fullName, anonymous_id: getUsermavenAnonymousId() }),
  demoSignin: (email?: string) => api.post<SigninResponse>('/auth/demo', email ? { email } : {}),
  signin: (email: string, password: string, rememberMe = true) =>
    api.post<SigninResponse>('/auth/signin', { email, password, remember_me: rememberMe }),
  verify2FASignin: (twoFaToken: string, code: string, useRecoveryCode = false) =>
    api.post<AuthResponse>('/auth/2fa/verify-signin', useRecoveryCode
      ? { two_fa_token: twoFaToken, recovery_code: code }
      : { two_fa_token: twoFaToken, totp_code: code }),
  stepUp2FA: (code: string, useRecoveryCode = false) =>
    api.post<AuthResponse>('/auth/2fa/step-up', useRecoveryCode
      ? { recovery_code: code }
      : { totp_code: code }),
  me: () => api.get<User>('/auth/me'),
  updateProfile: (data: { full_name?: string; avatar_url?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string; default_workspace_id?: string }) =>
    api.put<User>('/auth/me', data),
  refresh: () =>
    api.post<AuthResponse>('/auth/refresh', {}),
  signout: () => api.post<{ message: string }>('/auth/signout', {}),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.put<{ message: string }>('/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  forgotPassword: (email: string) =>
    api.post<{ message: string }>('/auth/forgot-password', { email }),
  resetPassword: (token: string, password: string) =>
    api.post<{ message: string }>('/auth/reset-password', { token, password }),
  get2FAStatus: () => api.get<TwoFAStatusResponse>('/auth/2fa/status'),
  setup2FA: (password: string) =>
    api.post<TwoFASetupResponse>('/auth/2fa/setup', { password }),
  verify2FASetup: (totpCode: string) =>
    api.post<AuthResponse>('/auth/2fa/verify', { totp_code: totpCode }),
  disable2FA: (password: string) =>
    api.del<{ message: string }>('/auth/2fa', { password }),
  regenerateRecoveryCodes: (password: string, totpCode: string) =>
    api.post<RecoveryCodesResponse>('/auth/2fa/regenerate-recovery-codes', { password, totp_code: totpCode }),
  verifyEmail: (token: string) =>
    api.post<User>('/auth/verify-email', { token }),
  resendVerification: () =>
    api.post<{ message: string }>('/auth/resend-verification', {}),
  uploadAvatar: async (file: File): Promise<{ data: User | null; error: string | null }> => {
    const formData = new FormData();
    formData.append('avatar', file);
    try {
      const res = await fetchWithSessionAuth(API_BASE, '/auth/me/avatar', {
        method: 'POST',
        body: formData,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data = await res.json();
      return { data, error: null };
    } catch (e) {
      return { data: null, error: e instanceof Error ? e.message : 'Upload failed' };
    }
  },
  deleteAvatar: () => api.del<User>('/auth/me/avatar'),
};
