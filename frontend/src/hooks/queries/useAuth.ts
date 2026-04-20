import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { authService } from '@/lib/services/authService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { User } from '@/lib/types'

export function useCurrentUser(enabled = true) {
  return useQuery({
    queryKey: queryKeys.user.me,
    queryFn: async () => unwrap(await authService.me()),
    enabled,
    staleTime: 5 * 60_000,
  })
}

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: { full_name?: string; avatar_style?: string; avatar_seed?: string; avatar_background_mode?: string; avatar_background_color?: string }) => unwrap(await authService.updateProfile(data)),
    onSuccess: (user) => {
      qc.setQueryData<User>(queryKeys.user.me, user)
    },
  })
}

export function useSignIn() {
  return useMutation({
    mutationFn: async ({ email, password }: { email: string; password: string }) => {
      const res = await authService.signin(email, password)
      if (res.error || !res.data) throw new Error(res.error || 'Sign in failed')
      if (!res.data.access_token || !res.data.refresh_token) {
        throw new Error(res.data.requires_2fa ? 'Two-factor verification required' : 'Sign in failed')
      }
      localStorage.setItem('access_token', res.data.access_token)
      localStorage.setItem('refresh_token', res.data.refresh_token)
      return res.data.user
    },
  })
}

export function useSignUp() {
  return useMutation({
    mutationFn: async ({ email, password, fullName }: { email: string; password: string; fullName: string }) => {
      const res = await authService.signup(email, password, fullName)
      if (res.error || !res.data) throw new Error(res.error || 'Sign up failed')
      if (!res.data.access_token || !res.data.refresh_token) {
        throw new Error('Sign up failed')
      }
      localStorage.setItem('access_token', res.data.access_token)
      localStorage.setItem('refresh_token', res.data.refresh_token)
      return res.data.user
    },
  })
}
