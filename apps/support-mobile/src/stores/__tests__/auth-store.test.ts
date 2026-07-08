import { configureSessionStorage, createMemorySessionStorage } from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
import type { User } from '@mobile/lib/types'
import { bootstrapAuth, signOut, useAuthStore } from '@mobile/stores/auth-store'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'

vi.mock('@mobile/lib/services/auth-service', () => ({
  authService: { me: vi.fn(), signin: vi.fn() },
}))

const mockMe = vi.mocked(authService.me)

const testUser: User = {
  id: 'user-1',
  email: 'a@example.com',
  full_name: 'A Example',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

beforeEach(() => {
  mockMe.mockReset()
  useAuthStore.setState({ user: null, loading: true, serverUnreachable: false })
})

test('no stored tokens: user stays null, loading finishes, me() is never called', async () => {
  configureSessionStorage(createMemorySessionStorage())

  await bootstrapAuth()

  expect(mockMe).not.toHaveBeenCalled()
  expect(useAuthStore.getState()).toMatchObject({
    user: null,
    loading: false,
    serverUnreachable: false,
  })
})

test('tokens present and me() succeeds: user is set', async () => {
  configureSessionStorage(createMemorySessionStorage({ accessToken: 'at', refreshToken: 'rt' }))
  mockMe.mockResolvedValue({ data: testUser, error: null })

  await bootstrapAuth()

  expect(mockMe).toHaveBeenCalledTimes(1)
  expect(useAuthStore.getState()).toMatchObject({
    user: testUser,
    loading: false,
    serverUnreachable: false,
  })
})

test('me() network error: serverUnreachable is true, user stays null, session is NOT cleared', async () => {
  const storage = createMemorySessionStorage({ accessToken: 'at', refreshToken: 'rt' })
  configureSessionStorage(storage)
  mockMe.mockResolvedValue({ data: null, error: 'Network error', isNetworkError: true })

  await bootstrapAuth()

  expect(useAuthStore.getState()).toMatchObject({
    user: null,
    loading: false,
    serverUnreachable: true,
  })
  // The whole point of the isNetworkError branch: don't log the user out just
  // because the server was briefly unreachable at startup.
  expect(storage.getAccessToken()).toBe('at')
  expect(storage.getRefreshToken()).toBe('rt')
})

test('signOut clears the session, user, AND the workspace store (realtime teardown)', async () => {
  const storage = createMemorySessionStorage({ accessToken: 'at', refreshToken: 'rt' })
  configureSessionStorage(storage)
  useAuthStore.setState({ user: testUser, loading: false, serverUnreachable: false })
  useWorkspaceStore.getState().setCurrentWorkspace({ id: 'ws-1', slug: 'acme', name: 'Acme' })

  await signOut()

  expect(useAuthStore.getState().user).toBeNull()
  expect(storage.getAccessToken()).toBeNull()
  // RootRealtimeMount derives its workspaceId from this store; if it stayed
  // populated, a token-less websocket would loop reconnects after logout.
  expect(useWorkspaceStore.getState().currentWorkspace).toBeNull()
})

test('signOut still clears the user and workspace stores even when clearSession() throws', async () => {
  const baseStorage = createMemorySessionStorage({ accessToken: 'at', refreshToken: 'rt' })
  configureSessionStorage({
    ...baseStorage,
    clearSession: () => {
      throw new Error('storage adapter failure')
    },
  })
  useAuthStore.setState({ user: testUser, loading: false, serverUnreachable: false })
  useWorkspaceStore.getState().setCurrentWorkspace({ id: 'ws-1', slug: 'acme', name: 'Acme' })

  await expect(signOut()).rejects.toThrow('storage adapter failure')

  // The finally block must still run: a thrown clearSession() must never
  // leave the app stuck mid-sign-out with stale user/workspace state.
  expect(useAuthStore.getState().user).toBeNull()
  expect(useWorkspaceStore.getState().currentWorkspace).toBeNull()
})
