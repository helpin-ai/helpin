import { api } from '@/lib/api'
import type { SigninResponse, User } from '@/lib/types'

export const authService = {
  signin: (email: string, password: string, rememberMe = false) =>
    api.post<SigninResponse>('/auth/signin', { email, password, remember_me: rememberMe }),
  me: () => api.get<User>('/auth/me'),
}
