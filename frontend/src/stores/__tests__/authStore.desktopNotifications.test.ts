// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { clearClientSession } from '../authStore'
import { isDesktopEnabled, setDesktopEnabled } from '@/lib/desktopNotifications'
import { configureSessionStorage, createBrowserSessionStorage, writeSession, getAccessToken } from '@helpin-ai/support-core'

vi.mock('@/lib/analytics', () => ({ resetAnalytics: vi.fn() }))
vi.mock('@/lib/helpin', () => ({ resetHelpinIdentity: vi.fn().mockResolvedValue(undefined) }))

afterEach(() => { localStorage.clear(); sessionStorage.clear() })

it('keeps per-user desktop choices through logout while clearing session and notification state', async () => {
  configureSessionStorage(createBrowserSessionStorage(localStorage))
  await writeSession({ accessToken: 'test-access', refreshToken: 'test-refresh', rememberMe: true })
  setDesktopEnabled('user-1', true)
  setDesktopEnabled('user-2', false)
  localStorage.setItem('helpin:desktop:user-1:focus', '{"tabId":"old-tab"}')
  localStorage.setItem('helpin:desktop:user-1:delivered', '[]')
  localStorage.setItem('workspace-data', 'private workspace state')
  sessionStorage.setItem('draft', 'private draft')

  await clearClientSession()

  expect(isDesktopEnabled('user-1')).toBe(true)
  expect(isDesktopEnabled('user-2')).toBe(false)
  expect(isDesktopEnabled('user-3')).toBe(false)
  expect(getAccessToken()).toBeNull()
  expect(Object.keys(localStorage).sort()).toEqual(['helpin:desktop:user-1:enabled', 'helpin:desktop:user-2:enabled'])
  expect(sessionStorage.length).toBe(0)
})
