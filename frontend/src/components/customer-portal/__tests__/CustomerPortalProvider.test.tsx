// @vitest-environment jsdom
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  pathname: '/portal/acme/callback',
  configuration: vi.fn(),
  session: vi.fn(),
  exchangeMagicLink: vi.fn(),
  requests: vi.fn(),
  navigate: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mocks.navigate,
  useRouterState: ({ select }: { select: (state: { location: { pathname: string } }) => string }) =>
    select({ location: { pathname: mocks.pathname } }),
  Outlet: () => <div data-testid="portal-content">Portal content</div>,
  Link: () => null,
}))
vi.mock('@/lib/services/customerPortalService', () => ({
  CustomerPortalApiError: class extends Error {
    constructor(public status: number, message: string) { super(message) }
  },
  customerPortalService: mocks,
}))

import { CustomerPortalProvider } from '../CustomerPortal'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const config = { enabled: true, requests_only: true, intake_enabled: true, branding: { name: 'Acme' } }
const customer = { customer: { id: '1', email: 'customer@example.com' } }

async function settle() {
  await act(async () => { await Promise.resolve(); await Promise.resolve() })
}

describe('CustomerPortalProvider', () => {
  let container: HTMLDivElement
  let root: Root

  afterEach(async () => {
    await act(async () => root?.unmount())
    container?.remove()
    vi.resetAllMocks()
  })

  it('leaves session restoration to the callback exchange', async () => {
    mocks.pathname = '/portal/acme/callback'
    mocks.configuration.mockResolvedValue(config)
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    expect(mocks.session).not.toHaveBeenCalled()
    expect(container.querySelector('[data-testid="portal-content"]')).not.toBeNull()
  })

  it('remounts with loading state rather than showing the prior slug content', async () => {
    mocks.pathname = '/portal/acme'
    mocks.configuration.mockResolvedValueOnce(config).mockReturnValueOnce(new Promise<never>(() => undefined))
    mocks.session.mockResolvedValue(customer)
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider key="acme" slug="acme" />))
    await settle()
    expect(container.querySelector('[data-testid="portal-content"]')).not.toBeNull()

    mocks.pathname = '/portal/other'
    await act(async () => root.render(<CustomerPortalProvider key="other" slug="other" />))
    expect(container.querySelector('[data-testid="portal-content"]')).toBeNull()
    expect(mocks.configuration).toHaveBeenCalledWith('other')
  })
})
