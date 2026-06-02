import { api } from '@/lib/api'
import type { SigninResponse, User } from '@/lib/types'

export const authService = {
  signin: (email: string, password: string, rememberMe = true) =>
    api.post<SigninResponse>('/auth/signin', { email, password, remember_me: rememberMe }),
  verify2FASignin: (twoFaToken: string, code: string, useRecoveryCode = false) =>
    api.post<SigninResponse>(
      '/auth/2fa/verify-signin',
      useRecoveryCode
        ? { two_fa_token: twoFaToken, recovery_code: code }
        : { two_fa_token: twoFaToken, totp_code: code },
    ),
  me: () => api.get<User>('/auth/me'),
  signout: () => api.post<{ message: string }>('/auth/signout', {}),
}
