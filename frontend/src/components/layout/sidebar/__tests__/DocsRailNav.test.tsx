// @vitest-environment jsdom

import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { DocsRailNav } from '../DocsRailNav'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const location = {
  pathname: '/w/acme/docs/documents/doc-1',
  search: {},
}

const spaces = [
  {
    id: 'space-1',
    workspace_id: 'ws-1',
    team_ids: [],
    name: 'Product docs',
    slug: 'product-docs',
    visibility: 'workspace_wide',
    type: 'internal',
    is_system: false,
    position: 0,
    created_by: 'user-1',
    created_at: '',
    updated_at: '',
  },
]

const collections = [
  {
    id: 'collection-1',
    space_id: 'space-1',
    workspace_id: 'ws-1',
    parent_collection_id: null,
    depth: 0,
    name: 'Getting started',
    slug: 'getting-started',
    position: 0,
    sort_key: 'a',
    created_by: 'user-1',
    created_at: '',
    updated_at: '',
  },
]

const documents = [
  {
    id: 'doc-1',
    workspace_id: 'ws-1',
    space_id: 'space-1',
    collection_id: 'collection-1',
    title: 'Welcome guide',
    status: 'draft',
    visibility: 'workspace_wide',
    tags: [],
    position: 0,
    sort_key: 'a',
    is_pinned: true,
    is_publicly_shared: false,
    is_locked: false,
    created_by: 'user-1',
    created_at: '',
    updated_at: '',
  },
]

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => location,
}))

vi.mock('@/hooks/queries', () => ({
  useDocsSpaces: () => ({ data: spaces, isLoading: false }),
  useDocsDocument: (_wsId: string, documentId: string) => ({ data: documentId ? documents[0] : undefined }),
  useDocsCollections: () => ({ data: collections, isLoading: false }),
  useDocsDocuments: () => ({ data: documents, isLoading: false }),
  useDeleteDocsCollection: () => ({ mutateAsync: vi.fn() }),
  useDeleteDocsSpace: () => ({ mutateAsync: vi.fn() }),
}))

vi.mock('@/components/docs/SpaceDialog', () => ({ SpaceDialog: () => null }))
vi.mock('@/components/docs/CreateCollectionDialog', () => ({ CreateCollectionDialog: () => null }))
vi.mock('@/components/docs/DeleteCollectionDialog', () => ({ DeleteCollectionDialog: () => null }))
vi.mock('@/components/docs/DeleteSpaceDialog', () => ({ DeleteSpaceDialog: () => null }))

describe('DocsRailNav', () => {
  beforeEach(() => {
    location.pathname = '/w/acme/docs/documents/doc-1'
    location.search = {}
    localStorage.clear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders one active space with quick links and the active document tree', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <TooltipProvider>
          <DocsRailNav
            wsId="ws-1"
            wsSlug="acme"
            canEditDocs
            isActive={() => false}
            openCreate={vi.fn()}
            onNavigate={vi.fn()}
          />
        </TooltipProvider>,
      )
    })

    expect(container.textContent).toContain('Product docs')
    expect(container.textContent).toContain('Recent docs')
    expect(container.textContent).toContain('My documents')
    expect(container.textContent).toContain('All docs in space')
    expect(container.textContent).toContain('Getting started')
    expect(container.textContent).toContain('Welcome guide')
    expect(container.querySelector('[aria-current="page"]')?.textContent).toContain('Welcome guide')
    const documentRows = Array.from(container.querySelectorAll<HTMLButtonElement>('[data-slot="docs-document-row"]'))
    const treeDocumentRow = documentRows.find((row) => !row.dataset.pinned)
    expect(treeDocumentRow?.querySelector('svg')).toBeNull()
    const countColumns = container.querySelectorAll('.w-6.text-right.tabular-nums')
    expect(countColumns).toHaveLength(2)
    const disclosure = container.querySelector<HTMLButtonElement>('[aria-label="Collapse Getting started"]')
    const collectionRow = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Getting started'),
    )
    expect(disclosure).toBeTruthy()
    expect(collectionRow).toBeTruthy()
    expect(disclosure).not.toBe(collectionRow)
    expect(disclosure?.className).toContain('-left-2')
    expect(disclosure?.className).toContain('before:w-4')
    expect(collectionRow?.className).toContain('px-2')
    expect(container.querySelector('[data-slot="docs-quick-links"]')?.className).toContain('px-2')
    expect(container.querySelector('[data-slot="docs-tree-scroll"]')?.className).toContain('px-2')
    expect(container.querySelector('[data-slot="docs-tree-scroll"]')?.className).toContain('no-scrollbar')
    expect(container.querySelector('[data-slot="docs-quick-links"] button')?.className).toContain('text-sm')
    expect(collectionRow?.querySelector('span')?.className).toContain('text-sm')
    expect(treeDocumentRow?.className).toContain('text-sm')
    expect(container.querySelector('[data-slot="docs-tree-scroll"] > div')?.className).toContain('text-[11px]')

    act(() => root.unmount())
    container.remove()
  })

  it('expands a collection from its separate disclosure control', () => {
    location.pathname = '/w/acme/docs/spaces/space-1'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <TooltipProvider>
          <DocsRailNav
            wsId="ws-1"
            wsSlug="acme"
            canEditDocs
            isActive={() => false}
            openCreate={vi.fn()}
            onNavigate={vi.fn()}
          />
        </TooltipProvider>,
      )
    })

    const disclosure = container.querySelector<HTMLButtonElement>('[aria-label="Expand Getting started"]')
    expect(disclosure?.getAttribute('aria-expanded')).toBe('false')

    act(() => disclosure?.click())

    const expandedDisclosure = container.querySelector<HTMLButtonElement>('[aria-label="Collapse Getting started"]')
    expect(expandedDisclosure?.getAttribute('aria-expanded')).toBe('true')

    act(() => root.unmount())
    container.remove()
  })

  it('leaves global search to the shared sidebar footer', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <TooltipProvider>
          <DocsRailNav
            wsId="ws-1"
            wsSlug="acme"
            canEditDocs={false}
            isActive={() => false}
            openCreate={vi.fn()}
            onNavigate={vi.fn()}
          />
        </TooltipProvider>,
      )
    })

    expect(container.textContent).not.toContain('Search all spaces')

    act(() => root.unmount())
    container.remove()
  })
})
