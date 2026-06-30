// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const routerMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  search: {} as Record<string, unknown>,
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
    useSupportConversationSearch: vi.fn(() => ({ data: undefined, isFetching: false, error: null })),
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
  })

  it('keeps primary search above the epics-style filter row', () => {
    const rendered = renderToolbar()

    const primaryRow = rendered.container.querySelector('[data-slot="support-search-primary-row"]')
    const filterRow = rendered.container.querySelector('[data-slot="support-search-filter-row"]')
    const searchInput = rendered.container.querySelector('input[placeholder="Search conversations by email, #number, title, customer, or message"]')

    expect(primaryRow).toBeTruthy()
    expect(filterRow).toBeTruthy()
    expect(searchInput).toBeTruthy()
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
})
