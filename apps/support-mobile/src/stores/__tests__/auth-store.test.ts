import { configureSessionStorage, createMemorySessionStorage } from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
import type { User } from '@mobile/lib/types'
import { bootstrapAuth, useAuthStore } from '@mobile/stores/auth-store'

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
