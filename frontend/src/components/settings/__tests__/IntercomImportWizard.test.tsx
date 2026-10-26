// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const service = vi.hoisted(() => ({
  previewDocs: vi.fn(),
  previewConversations: vi.fn(),
  startDocs: vi.fn(),
  startConversations: vi.fn(),
  listConversationImports: vi.fn(),
  cancelConversationImport: vi.fn(),
}))
vi.mock('@/lib/services/intercomImportService', () => ({ intercomImportService: service }))
vi.mock('@/lib/services/docsImportService', () => ({ docsImportService: { getStatus: vi.fn() } }))
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: { permissions: ['docs.import', 'support.admin'] } }) }))
vi.mock('@/hooks/queries', () => ({ useDocsSpaces: () => ({ data: [] }) }))
vi.mock('@/hooks/queries/useSupport', () => ({ useInboxScopes: () => ({ data: { shared_inbox: { name: 'Main inbox' }, mailboxes: [] } }) }))

import { IntercomImportWizard } from '../IntercomImportWizard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

async function flush() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

function setInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

describe('IntercomImportWizard', () => {
  let container: HTMLDivElement
  let unmount: () => void
  beforeEach(() => {
    service.previewDocs.mockResolvedValue({ data: { workspace_name: 'Acme', collections: 1, articles_published: 3, articles_draft: 1 }, error: null })
    service.previewConversations.mockResolvedValue({ data: { workspace_name: 'Acme', conversations: 42 }, error: null })
    service.startConversations.mockResolvedValue({ data: { id: 'job-1', status: 'pending', imported: 0, skipped: 0, failed: 0, total: 0, failures: [] }, error: null })
    service.listConversationImports.mockResolvedValue({ data: [], error: null })
    container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={new QueryClient()}><IntercomImportWizard workspaceId="ws" /></QueryClientProvider>))
    unmount = () => act(() => root.unmount())
  })
  afterEach(() => {
    unmount()
    container.remove()
    vi.clearAllMocks()
  })

  it('previews both parts and starts only the selected one', async () => {
    act(() => setInput(container.querySelector('#intercom-token')!, 'token-1'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush()
    expect(service.previewDocs).toHaveBeenCalledWith('ws', 'token-1', 'us')
    expect(container.textContent).toContain('1 collection, 3 published and 1 draft article.')
    expect(container.textContent).toContain('42 conversations')

    // Untick Help Center, then start.
    const helpCenter = container.querySelectorAll('button[role="checkbox"]')[0] as HTMLButtonElement
    act(() => helpCenter.click())
    const start = [...container.querySelectorAll('button')].find((button) => button.textContent === 'Start import')!
    await act(async () => start.click())
    await flush()
    expect(service.startDocs).not.toHaveBeenCalled()
    expect(service.startConversations).toHaveBeenCalledWith('ws', expect.objectContaining({ access_token: 'token-1', region: 'us' }))
    expect(container.textContent).toContain('Importing from Intercom')
  })

  it('shows the API error for a rejected token', async () => {
    service.previewDocs.mockResolvedValue({ data: null, error: 'Intercom rejected the access token; check it and its region' })
    service.previewConversations.mockResolvedValue({ data: null, error: 'Intercom rejected the access token; check it and its region' })
    act(() => setInput(container.querySelector('#intercom-token')!, 'bad'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush()
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('rejected the access token')
    expect(container.textContent).not.toContain('Start import')
  })

  it('offers the part the token can read when the other is refused', async () => {
    service.previewDocs.mockResolvedValue({ data: null, error: 'the access token is missing a permission this import needs' })
    act(() => setInput(container.querySelector('#intercom-token')!, 'token-2'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush()
    expect(container.querySelector('[role="alert"]')).toBeNull()
    expect(container.textContent).toContain('42 conversations')
    expect(container.querySelector('[data-testid="intercom-docs-unavailable"]')?.textContent).toContain('missing a permission')
    expect([...container.querySelectorAll('button')].some((button) => button.textContent === 'Start import')).toBe(true)
  })

  it('keeps the count for the latest date when answers arrive out of order', async () => {
    act(() => setInput(container.querySelector('#intercom-token')!, 'token-3'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush()
    let resolveSlow: (value: unknown) => void = () => {}
    service.previewConversations
      .mockReturnValueOnce(new Promise((resolve) => { resolveSlow = resolve }))
      .mockResolvedValueOnce({ data: { workspace_name: 'Acme', conversations: 7 }, error: null })
    const date = container.querySelector('#intercom-since') as HTMLInputElement
    act(() => setInput(date, '2025-01-01'))
    act(() => setInput(date, '2026-01-01'))
    await flush()
    await act(async () => resolveSlow({ data: { workspace_name: 'Acme', conversations: 900 }, error: null }))
    await flush()
    expect(container.textContent).toContain('7 conversations')
    expect(container.textContent).not.toContain('900 conversations')
  })
})
