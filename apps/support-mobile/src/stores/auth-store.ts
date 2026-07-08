import { create } from 'zustand'
import {
  clearSession,
  getAccessToken,
  getRefreshToken,
  hydrateSessionStorage,
} from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
import { unregisterPush } from '@mobile/push/push-registration'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import type { User } from '@mobile/lib/types'

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
}

export const useAuthStore = create<AuthState>(() => ({
  user: null,
  loading: true,
  serverUnreachable: false,
}))

export async function bootstrapAuth(): Promise<void> {
  await hydrateSessionStorage()
  if (!getAccessToken() && !getRefreshToken()) {
    useAuthStore.setState({ user: null, loading: false })
    return
  }
  const { data, error, isNetworkError } = await authService.me()
  if (data && !error) {
    useAuthStore.setState({ user: data, loading: false, serverUnreachable: false })
  } else if (isNetworkError) {
    // Server unreachable — keep tokens, do NOT clear the session
    useAuthStore.setState({ loading: false, serverUnreachable: true })
  } else {
    // Genuine auth failure (401, invalid token) — clear session
    await clearSession()
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  }
}

export async function signOut(): Promise<void> {
  // Unregister the device's push token BEFORE the session is cleared —
  // DELETE /user/push-devices needs the still-valid auth header. Failures
  // are swallowed internally by unregisterPush(): sign-out must never fail
  // or hang because a push-device delete didn't go through.
  await unregisterPush()
  // clearSession() can throw (e.g. a storage adapter failure) — the finally
  // block below ensures the app-visible auth/workspace state clears
  // regardless, so the user always lands back on the login screen instead of
  // getting stuck mid-sign-out.
  try {
    await clearSession()
  } finally {
    useAuthStore.setState({ user: null })
    // Tear down realtime: RootRealtimeMount (router.tsx) keys its websocket on
    // useWorkspaceStore's currentWorkspace.id. Leaving it populated after the
    // tokens are cleared would keep a token-less socket looping reconnect
    // attempts forever on the login screen.
    useWorkspaceStore.getState().setCurrentWorkspace(null)
  }
}
