// @vitest-environment jsdom
import React, { StrictMode, act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  pathname: '/portal/acme/callback',
  configuration: vi.fn(),
  session: vi.fn(),
  exchangeMagicLink: vi.fn(),
  requests: vi.fn(),
  navigate: vi.fn(),
  outlet: null as (() => React.ReactNode) | null,
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mocks.navigate,
  useRouterState: ({ select }: { select: (state: { location: { pathname: string } }) => string }) =>
    select({ location: { pathname: mocks.pathname } }),
  Outlet: () => mocks.outlet?.() ?? <div data-testid="portal-content">Portal content</div>,
  Link: () => null,
}))
vi.mock('@/lib/services/customerPortalService', () => ({
  CustomerPortalApiError: class extends Error {
    constructor(public status: number, message: string) { super(message) }
  },
  customerPortalService: mocks,
}))

import { CustomerPortalCallback, CustomerPortalProvider } from '../CustomerPortal'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const config = { enabled: true, intake_enabled: true, branding: { name: 'Acme' } }
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
    mocks.outlet = null
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

  it('exchanges a single-use sign-in link once under StrictMode', async () => {
    mocks.pathname = '/portal/acme/callback'
    mocks.configuration.mockResolvedValue(config)
    mocks.exchangeMagicLink.mockResolvedValue(customer)
    mocks.outlet = () => <CustomerPortalCallback token="link-token" />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<StrictMode><CustomerPortalProvider slug="acme" /></StrictMode>))
    await settle()
    await settle()
    expect(mocks.exchangeMagicLink).toHaveBeenCalledTimes(1)
    expect(mocks.navigate).toHaveBeenCalledWith({ to: '/portal/$slug', params: { slug: 'acme' }, replace: true })
    expect(container.textContent).not.toContain('This link is no longer valid')
  })

  it('offers a retry instead of rejecting the link after a temporary failure', async () => {
    mocks.pathname = '/portal/acme/callback'
    mocks.configuration.mockResolvedValue(config)
    const { CustomerPortalApiError } = await import('@/lib/services/customerPortalService')
    mocks.exchangeMagicLink
      .mockRejectedValueOnce(new CustomerPortalApiError(503, 'temporarily unavailable'))
      .mockResolvedValueOnce(customer)
    mocks.outlet = () => <CustomerPortalCallback token="link-token" />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(container.textContent).toContain('We couldn’t sign you in right now')
    expect(container.textContent).not.toContain('This link is no longer valid')
    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Try again')
    await act(async () => retry?.click())
    await settle()
    await settle()
    expect(mocks.exchangeMagicLink).toHaveBeenCalledTimes(2)
    expect(mocks.navigate).toHaveBeenCalledWith({ to: '/portal/$slug', params: { slug: 'acme' }, replace: true })
  })

  it('rejects an invalid link without offering a retry', async () => {
    mocks.pathname = '/portal/acme/callback'
    mocks.configuration.mockResolvedValue(config)
    const { CustomerPortalApiError } = await import('@/lib/services/customerPortalService')
    mocks.exchangeMagicLink.mockRejectedValue(new CustomerPortalApiError(401, 'invalid or expired link'))
    mocks.outlet = () => <CustomerPortalCallback token="link-token" />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(container.textContent).toContain('This link is no longer valid')
  })
})
