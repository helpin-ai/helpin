// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const testState = vi.hoisted(() => ({
  auth: {
    user: null as { email_verified: boolean } | null,
    loading: true,
    serverUnreachable: false,
  },
}))

vi.mock('@tanstack/react-router', () => ({
  Outlet: () => <div>Authenticated content</div>,
  useLocation: () => ({ pathname: '/workspaces', search: {} }),
}))

vi.mock('@/stores/authStore', () => {
  const useAuthStore = (selector: (state: { user: typeof testState.auth.user }) => unknown) =>
    selector({ user: testState.auth.user })
  useAuthStore.getState = () => ({ initialize: vi.fn() })
  return { useAuthStore }
})

vi.mock('@/components/auth/EmailVerificationBanner', () => ({
  EmailVerificationBanner: () => <div>Email verification</div>,
}))

import { AuthenticatedLayout } from '@/components/layout/AuthenticatedLayout'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('AuthenticatedLayout', () => {
  afterEach(() => {
    testState.auth = {
      user: null,
      loading: true,
      serverUnreachable: false,
    }
  })

  it('keeps a stable hook order when authentication resolves after a refresh', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(() => {
      act(() => {
        root.render(<AuthenticatedLayout auth={testState.auth} />)
      })

      testState.auth = {
        user: { email_verified: true },
        loading: false,
        serverUnreachable: false,
      }

      act(() => {
        root.render(<AuthenticatedLayout auth={testState.auth} />)
      })
    }).not.toThrow()

    const messages = consoleErrorSpy.mock.calls.flat().join(' ')
    expect(messages).not.toContain('Rendered more hooks')
    expect(messages).not.toContain('Rendered fewer hooks')

    act(() => {
      root.unmount()
    })
    container.remove()
    consoleErrorSpy.mockRestore()
  })
})
