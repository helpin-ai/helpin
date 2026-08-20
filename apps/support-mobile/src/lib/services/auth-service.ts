import { api } from '@mobile/lib/api'
import type { AuthResponse, SigninResponse, User } from '@mobile/lib/types'

export const authService = {
  signin: (email: string, password: string, rememberMe = false) =>
    api.post<SigninResponse>('/auth/signin', { email, password, remember_me: rememberMe }),
  verify2FASignin: (twoFAToken: string, code: string, recoveryCode: boolean) =>
    api.post<AuthResponse>('/auth/2fa/verify-signin', {
      two_fa_token: twoFAToken,
      ...(recoveryCode ? { recovery_code: code } : { totp_code: code }),
    }),
  exchangeGoogleMobileCode: (code: string) =>
    api.post<AuthResponse>('/auth/google/mobile-exchange', { code }),
  refreshBrowserSession: () => api.post<AuthResponse>('/auth/refresh'),
  me: () => api.get<User>('/auth/me'),
}
