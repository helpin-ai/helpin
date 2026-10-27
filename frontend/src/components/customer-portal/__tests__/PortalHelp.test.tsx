// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  helpArticles: vi.fn(),
  searchHelpArticles: vi.fn(),
  helpArticle: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to, params, className }: { children: ReactNode; to: string; params: Record<string, string>; className?: string }) => (
    <a href={Object.entries(params).reduce((path, [key, value]) => path.replace(`$${key}`, value), to)} className={className}>{children}</a>
  ),
}))
vi.mock('@/lib/services/customerPortalService', () => ({
  CustomerPortalApiError: class extends Error {
    constructor(public status: number, message: string) { super(message) }
  },
  customerPortalService: mocks,
}))

import { CustomerPortalApiError } from '@/lib/services/customerPortalService'
import { PortalHelpArticleView, PortalHelpList } from '../PortalHelp'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const articles = [
  { id: 'renewal', title: 'Renewal pricing', excerpt: 'How renewals are priced.', space_name: 'Customer guides', collection_name: 'Billing', published_at: '2026-10-27T00:00:00Z' },
  { id: 'sso', title: 'Set up SSO', space_name: 'Customer guides', published_at: '2026-10-27T00:00:00Z' },
]

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  vi.useFakeTimers()
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function flush() {
  await act(async () => { await Promise.resolve(); await Promise.resolve() })
}

describe('PortalHelpList', () => {
  it('groups articles by space and collection, then searches as the customer types', async () => {
    mocks.helpArticles.mockResolvedValue(articles)
    mocks.searchHelpArticles.mockResolvedValue([{ ...articles[1], snippet: 'Single sign-on with SAML.' }])
    await act(async () => root.render(<PortalHelpList slug="acme" onSessionError={() => false} />))
    await flush()
    const sections = Array.from(container.querySelectorAll('section')).map((section) => section.getAttribute('aria-label'))
    expect(sections).toEqual(['Customer guides · Billing', 'Customer guides'])
    expect(container.querySelector('a')?.getAttribute('href')).toBe('/portal/acme/help/renewal')

    const input = container.querySelector<HTMLInputElement>('input[aria-label="Search help articles"]')!
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      setValue.call(input, 'single sign on')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(mocks.searchHelpArticles).not.toHaveBeenCalled()
    await act(async () => { vi.advanceTimersByTime(300) })
    await flush()
    expect(mocks.searchHelpArticles).toHaveBeenCalledWith('acme', 'single sign on', expect.any(AbortSignal))
    const results = container.querySelector('section[aria-label="Search results"]')
    expect(results?.textContent).toContain('Set up SSO')
    expect(results?.textContent).toContain('Single sign-on with SAML.')
    expect(results?.textContent).not.toContain('Renewal pricing')
  })

  it('says so when the team shares no articles', async () => {
    mocks.helpArticles.mockResolvedValue([])
    await act(async () => root.render(<PortalHelpList slug="acme" onSessionError={() => false} />))
    await flush()
    expect(container.textContent).toContain('No help articles yet')
    expect(container.querySelector('input')).toBeNull()
  })

  it('leaves an expired session to the portal', async () => {
    const error = new CustomerPortalApiError(401, 'invalid portal session')
    mocks.helpArticles.mockRejectedValue(error)
    const onSessionError = vi.fn(() => true)
    await act(async () => root.render(<PortalHelpList slug="acme" onSessionError={onSessionError} />))
    await flush()
    expect(onSessionError).toHaveBeenCalledWith(error)
    expect(container.textContent).not.toContain('could not be loaded')
  })
})

describe('PortalHelpArticleView', () => {
  it('shows the sanitized article and reports its title', async () => {
    mocks.helpArticle.mockResolvedValue({ ...articles[0], content_html: '<h2>Prices</h2><p>Per contract.</p><img src="x" onerror="alert(1)"><script>alert(2)</script>' })
    const onTitle = vi.fn()
    await act(async () => root.render(<PortalHelpArticleView slug="acme" articleId="renewal" onSessionError={() => false} onTitle={onTitle} />))
    await flush()
    expect(onTitle).toHaveBeenCalledWith('Renewal pricing')
    expect(container.querySelector('article h2')?.textContent).toBe('Prices')
    expect(container.textContent).toContain('Customer guides')
    expect(container.textContent).toContain('Billing')
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')?.getAttribute('onerror')).toBeNull()
  })

  it('explains an article that is no longer available', async () => {
    mocks.helpArticle.mockRejectedValue(new CustomerPortalApiError(404, 'article not found'))
    await act(async () => root.render(<PortalHelpArticleView slug="acme" articleId="gone" onSessionError={() => false} />))
    await flush()
    expect(container.textContent).toContain('This article is not available.')
  })
})
