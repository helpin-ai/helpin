// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const routerMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  search: {} as Record<string, unknown>,
}))

const supportMocks = vi.hoisted(() => ({
  searchResponse: undefined as unknown,
}))

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router')
  return {
    ...actual,
    useNavigate: () => routerMocks.navigate,
    useSearch: () => routerMocks.search,
  }
})

vi.mock('@/hooks/useTitle', () => ({
  useTitle: vi.fn(),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { id: string; slug: string } }) => unknown) => selector({
    currentWorkspace: { id: 'ws-1', slug: 'test-workspace' },
  }),
}))

vi.mock('@/hooks/queries/useSupport', async () => {
  const actual = await vi.importActual<typeof import('@/hooks/queries/useSupport')>('@/hooks/queries/useSupport')
  return {
    ...actual,
    useSupportConversationSearch: vi.fn(() => ({ data: supportMocks.searchResponse, isFetching: false, error: null })),
    useSupportMailboxes: vi.fn(() => ({ data: [] })),
    useSupportTags: vi.fn(() => ({ data: [] })),
  }
})

import { SupportSearchPage, SupportSearchToolbar } from '../SupportSearch'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function renderToolbar() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <SupportSearchToolbar
        draft={{ q: 'billing', sort: 'relevance' }}
        mailboxes={[
          {
            id: 'box-1',
            workspace_id: 'ws-1',
            name: 'Escalations',
            handle: 'escalations',
            icon: '',
            triage_eligible: true,
            visibility_mode: 'members_only',
            assignment_mode: 'manual',
            position: 0,
            active: true,
            created_by_id: 'user-1',
            created_at: '2026-06-30T00:00:00Z',
            updated_at: '2026-06-30T00:00:00Z',
          },
        ]}
        tags={[{
          id: 'tag-1',
          workspace_id: 'ws-1',
          name: 'VIP',
          color: '#f59e0b',
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
        }]}
        onDraftChange={vi.fn()}
        onSubmit={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('SupportSearchToolbar', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    routerMocks.search = {}
    routerMocks.navigate.mockClear()
    supportMocks.searchResponse = undefined
    window.sessionStorage.clear()
  })

  it('keeps primary search above the epics-style filter row', () => {
    const rendered = renderToolbar()

    const primaryRow = rendered.container.querySelector('[data-slot="support-search-primary-row"]')
    const filterRow = rendered.container.querySelector('[data-slot="support-search-filter-row"]')
    const searchInput = rendered.container.querySelector('input[placeholder="Search conversations by email, #number, title, customer, or message"]')
    const closeSearch = rendered.container.querySelector<HTMLButtonElement>('[aria-label="Close search"]')

    expect(primaryRow).toBeTruthy()
    expect(filterRow).toBeTruthy()
    expect(searchInput).toBeTruthy()
    expect(closeSearch?.title).toBe('Close Search')
    expect(primaryRow?.contains(searchInput)).toBe(true)
    expect(filterRow?.compareDocumentPosition(primaryRow as Element) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()

    for (const label of ['Status', 'Priority', 'Assignee', 'Inbox', 'Tag', 'AI state']) {
      expect(filterRow?.textContent).toContain(label)
    }
    expect(filterRow?.textContent).toContain('More')
    expect(filterRow?.textContent).toContain('Sort by:')
    expect(rendered.container.textContent).not.toContain('Customer email')

    rendered.cleanup()
  })

  it('does not show the introductory search section before a search is started', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SupportSearchPage />)
    })

    expect(container.textContent).not.toContain('Search all support conversations')
    expect(container.textContent).not.toContain('Search every support conversation')

    act(() => root.unmount())
    container.remove()
  })

  it('preserves spaces while typing a conversation search', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SupportSearchPage />)
    })

    const searchInput = container.querySelector<HTMLInputElement>(
      'input[placeholder="Search conversations by email, #number, title, customer, or message"]',
    )

    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(searchInput, 'billing ')
      searchInput?.dispatchEvent(new Event('input', { bubbles: true }))
    })

    expect(searchInput?.value).toBe('billing ')

    act(() => root.unmount())
    container.remove()
  })

  it('closes search back to the saved support inbox context', () => {
    window.sessionStorage.setItem(
      'support-search-return:test-workspace',
      '/w/test-workspace/support/conv-42?view=team&team_inbox=mailbox-billing',
    )
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SupportSearchPage />)
    })

    act(() => {
      container.querySelector<HTMLButtonElement>('[aria-label="Close search"]')?.click()
    })

    expect(routerMocks.navigate).toHaveBeenCalledWith({
      to: '/w/$slug/support/$conversationId',
      params: { slug: 'test-workspace', conversationId: 'conv-42' },
      search: { view: 'team', team_inbox: 'mailbox-billing' },
    })

    act(() => root.unmount())
    container.remove()
  })

  it('renders search results as a structured row list with a dedicated match column', () => {
    routerMocks.search = { q: 'refund', per_page: 50, page: 1 }
    supportMocks.searchResponse = {
      data: [{
        conversation: {
          id: 'conv-1',
          workspace_id: 'ws-1',
          display_id: 842,
          subject: 'Refund request for annual plan',
          status: 'open',
          priority: 'high',
          source: 'email',
          customer_name: 'Ada Lovelace',
          customer_email: 'ada@example.com',
          mailbox_name: 'Billing',
          last_message: 'I need help with a refund',
          created_at: '2026-06-29T12:00:00Z',
          updated_at: '2026-06-30T05:30:00Z',
        },
        display_id: 842,
        matched_fields: ['message'],
        snippet: 'I need help with a refund',
        highlights: [{
          field: 'message',
          text: 'I need help with a refund',
          ranges: [{ start: 19, end: 25 }],
        }],
        score: 42,
      }],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
      meta: { sort: 'relevance', query: 'refund', total_capped: false, total_cap: 1000 },
    }
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SupportSearchPage />)
    })

    const results = container.querySelector('[data-slot="support-search-results"]')
    const row = container.querySelector('[data-slot="support-search-result-row"]')

    expect(results).toBeTruthy()
    for (const heading of ['Conversation', 'Match', 'Customer', 'State', 'Updated']) {
      expect(results?.textContent).toContain(heading)
    }
    expect(row?.textContent).toContain('#842')
    expect(row?.textContent).toContain('Refund request for annual plan')
    expect(row?.textContent).toContain('Ada Lovelace')
    expect(row?.textContent).toContain('ada@example.com')
    expect(row?.textContent).toContain('Message')
    expect(row?.querySelector('mark')?.textContent).toBe('refund')

    act(() => root.unmount())
    container.remove()
  })

  it('opens team inbox conversations with the matching inbox selected', () => {
    routerMocks.search = { q: 'refund', per_page: 50, page: 1 }
    supportMocks.searchResponse = {
      data: [{
        conversation: {
          id: 'conv-1',
          workspace_id: 'ws-1',
          mailbox_id: 'mailbox-billing',
          display_id: 842,
          subject: 'Refund request for annual plan',
          status: 'open',
          priority: 'high',
          source: 'email',
          customer_email: 'ada@example.com',
          mailbox_name: 'Billing',
          last_message: 'I need help with a refund',
          created_at: '2026-06-29T12:00:00Z',
          updated_at: '2026-06-30T05:30:00Z',
        },
        display_id: 842,
        matched_fields: ['message'],
        snippet: 'I need help with a refund',
        highlights: [],
        score: 42,
      }],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
      meta: { sort: 'relevance', query: 'refund', total_capped: false, total_cap: 1000 },
    }
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SupportSearchPage />)
    })

    act(() => {
      container.querySelector<HTMLButtonElement>('[data-slot="support-search-result-row"]')?.click()
    })

    expect(routerMocks.navigate).toHaveBeenCalledWith({
      to: '/w/$slug/support/$conversationId',
      params: { slug: 'test-workspace', conversationId: 'conv-1' },
      search: { view: 'team', team_inbox: 'mailbox-billing' },
    })

    act(() => root.unmount())
    container.remove()
  })
})
