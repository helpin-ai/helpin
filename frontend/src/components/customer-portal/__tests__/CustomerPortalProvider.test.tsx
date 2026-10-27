// @vitest-environment jsdom
import React, { StrictMode, act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  pathname: '/portal/acme/callback',
  configuration: vi.fn(),
  session: vi.fn(),
  exchangeMagicLink: vi.fn(),
  requests: vi.fn(),
  suggestions: vi.fn(),
  helpArticles: vi.fn(),
  signOut: vi.fn(),
  navigate: vi.fn(),
  outlet: null as (() => React.ReactNode) | null,
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mocks.navigate,
  useRouterState: ({ select }: { select: (state: { location: { pathname: string } }) => string }) =>
    select({ location: { pathname: mocks.pathname } }),
  Outlet: () => mocks.outlet?.() ?? <div data-testid="portal-content">Portal content</div>,
  Link: ({ children }: { children?: React.ReactNode }) => <a href="/portal">{children}</a>,
}))
vi.mock('@/lib/services/customerPortalService', () => ({
  CustomerPortalApiError: class extends Error {
    constructor(public status: number, message: string) { super(message) }
  },
  customerPortalService: mocks,
  customerPortalSocketUrl: () => 'ws://portal.test/ws',
}))

import { CustomerPortalCallback, CustomerPortalHome, CustomerPortalNewRequestPage, CustomerPortalProvider } from '../CustomerPortal'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const config = { enabled: true, intake_enabled: true, branding: { name: 'Acme' } }
const customer = { customer: { id: '1', email: 'customer@example.com' } }

async function settle() {
  await act(async () => { await Promise.resolve(); await Promise.resolve() })
}

describe('CustomerPortalProvider', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    mocks.helpArticles.mockResolvedValue([])
  })

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

  it('lets a company member switch between their own and company requests', async () => {
    window.sessionStorage.clear()
    mocks.pathname = '/portal/acme'
    mocks.configuration.mockResolvedValue(config)
    mocks.session.mockResolvedValue({
      ...customer,
      companies: [{ id: 'co-acme', name: 'Acme Corp', role: 'member' }, { id: 'co-other', name: 'Other', role: 'requester' }],
    })
    mocks.requests.mockImplementation(async (_slug: string, _filter: string, scope: { kind: string; companyId?: string }) =>
      scope.kind === 'company'
        ? [{ reference: 'bob', subject: 'Company outage', status: 'active', last_activity_at: null, own: false, requested_by: 'Bob Roe' }]
        : [{ reference: 'mine', subject: 'My invoice', status: 'active', last_activity_at: null }])
    mocks.outlet = () => <CustomerPortalHome />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(container.textContent).toContain('Your requests')
    expect(container.textContent).toContain('My invoice')
    const tabs = Array.from(container.querySelectorAll('[role="tab"]')).map((tab) => tab.textContent)
    expect(tabs).toEqual(['My requests', 'Acme Corp'])

    const companyTab = Array.from(container.querySelectorAll<HTMLElement>('[role="tab"]')).find((tab) => tab.textContent === 'Acme Corp')!
    await act(async () => companyTab.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 })))
    await settle()
    expect(mocks.requests).toHaveBeenLastCalledWith('acme', 'all', { kind: 'company', companyId: 'co-acme' })
    expect(container.textContent).toContain('Acme Corp requests')
    expect(container.textContent).toContain('Company outage')
    expect(container.textContent).toContain('Asked by Bob Roe')
    expect(container.textContent).not.toContain('My invoice')
  })

  it('suggests help center articles while a request is written', async () => {
    window.sessionStorage.clear()
    mocks.pathname = '/portal/acme/new'
    mocks.configuration.mockResolvedValue({ ...config, intake_enabled: true })
    mocks.session.mockResolvedValue(customer)
    mocks.suggestions.mockResolvedValue({ articles: [{ id: 'a', title: 'Exporting reports', url: 'https://help.example/export' }] })
    mocks.outlet = () => <CustomerPortalNewRequestPage />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    const subject = container.querySelector<HTMLInputElement>('#request-subject')!
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
    await act(async () => {
      setter?.call(subject, 'Export fails on Safari')
      subject.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 650)) })
    await settle()
    expect(mocks.suggestions).toHaveBeenCalledWith('acme', 'Export fails on Safari', expect.any(AbortSignal))
    expect(container.textContent).toContain('Related articles')
    const link = container.querySelector<HTMLAnchorElement>('a[href="https://help.example/export"]')
    expect(link?.target).toBe('_blank')
  })

  it('fills in a request handed off from the help assistant, then clears the address', async () => {
    window.sessionStorage.clear()
    window.history.replaceState(null, '', '/portal/acme/new?subject=Refund%20question&message=Can%20I%20get%20a%20refund%3F%0A%0AHelp%20assistant%3A%20The%20team%20can%20help.')
    mocks.pathname = '/portal/acme/new'
    mocks.configuration.mockResolvedValue({ ...config, intake_enabled: true })
    mocks.session.mockResolvedValue(customer)
    mocks.suggestions.mockResolvedValue({ articles: [] })
    mocks.outlet = () => <CustomerPortalNewRequestPage />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(container.querySelector<HTMLInputElement>('#request-subject')?.value).toBe('Refund question')
    expect(container.querySelector<HTMLTextAreaElement>('#request-message')?.value).toContain('Help assistant: The team can help.')
    expect(window.location.search).toBe('')
  })

  it('clears saved drafts when the customer signs out', async () => {
    const key = 'helpin.portal.acme.1.new-request.draft'
    window.sessionStorage.setItem(key, JSON.stringify({ subject: 'Private', message: 'Details' }))
    mocks.pathname = '/portal/acme'
    mocks.configuration.mockResolvedValue(config)
    mocks.session.mockResolvedValue(customer)
    mocks.requests.mockResolvedValue([])
    mocks.signOut.mockResolvedValue(undefined)
    mocks.outlet = () => <CustomerPortalHome />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    const button = Array.from(container.querySelectorAll('button')).find((item) => item.textContent === 'Sign out')!
    await act(async () => button.click())
    expect(mocks.signOut).toHaveBeenCalledWith('acme')
    expect(window.sessionStorage.getItem(key)).toBeNull()
  })

  it('shows no company tabs to a requester', async () => {
    window.sessionStorage.clear()
    mocks.pathname = '/portal/acme'
    mocks.configuration.mockResolvedValue(config)
    mocks.session.mockResolvedValue({ ...customer, companies: [{ id: 'co-acme', name: 'Acme Corp', role: 'requester' }] })
    mocks.requests.mockResolvedValue([])
    mocks.outlet = () => <CustomerPortalHome />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(container.querySelector('[role="tab"]')).toBeNull()
    expect(mocks.requests).toHaveBeenCalledWith('acme', 'all', { kind: 'mine' })
    expect(container.textContent).not.toContain('Help articles')
  })

  it('links to help articles when the team shares some with signed-in customers', async () => {
    window.sessionStorage.clear()
    mocks.pathname = '/portal/acme'
    mocks.configuration.mockResolvedValue(config)
    mocks.session.mockResolvedValue({ ...customer, companies: [] })
    mocks.requests.mockResolvedValue([])
    mocks.helpArticles.mockResolvedValueOnce([{ id: 'renewal', title: 'Renewal pricing', space_name: 'Customer guides', published_at: '2026-10-27T00:00:00Z' }])
    mocks.outlet = () => <CustomerPortalHome />
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => root.render(<CustomerPortalProvider slug="acme" />))
    await settle()
    await settle()
    expect(mocks.helpArticles).toHaveBeenCalledWith('acme')
    expect(Array.from(container.querySelectorAll('a')).map((link) => link.textContent)).toContain('Help articles')
  })
})
