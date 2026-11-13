// @vitest-environment jsdom

import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { SpaceDialog } from '@/components/docs/SpaceDialog'
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

vi.mock('@/components/docs/SpaceDialog', () => ({ SpaceDialog: vi.fn(() => null) }))
vi.mock('@/components/docs/CreateCollectionDialog', () => ({ CreateCollectionDialog: () => null }))
vi.mock('@/components/docs/DeleteCollectionDialog', () => ({ DeleteCollectionDialog: () => null }))
vi.mock('@/components/docs/DeleteSpaceDialog', () => ({ DeleteSpaceDialog: () => null }))

describe('DocsRailNav', () => {
  beforeEach(() => {
    location.pathname = '/w/acme/docs/documents/doc-1'
    location.search = {}
    spaces.splice(1)
    localStorage.clear()
    vi.mocked(SpaceDialog).mockClear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('starts keyboard navigation on the current space when opening and reopening the picker', () => {
    spaces.push({ ...spaces[0], id: 'space-2', name: 'Support docs' })
    location.pathname = '/w/acme/docs/spaces/space-2'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onNavigate = vi.fn()

    act(() => root.render(
      <TooltipProvider>
        <DocsRailNav wsId="ws-1" wsSlug="acme" canEditDocs isActive={() => false}
          openCreate={vi.fn()} onNavigate={onNavigate} />
      </TooltipProvider>,
    ))
    try {
      const trigger = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!
      act(() => trigger.click())
      let input = document.querySelector<HTMLInputElement>('[role="combobox"]')!
      expect(document.querySelector('[role="option"][aria-selected="true"]')?.id).toBe('docs-space-option-space-2')
      expect(input.getAttribute('aria-activedescendant')).toBe('docs-space-option-space-2')

      act(() => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true })))
      expect(input.getAttribute('aria-activedescendant')).toBe('docs-space-option-space-1')
      act(() => trigger.click())
      act(() => trigger.click())
      input = document.querySelector<HTMLInputElement>('[role="combobox"]')!
      expect(input.getAttribute('aria-activedescendant')).toBe('docs-space-option-space-2')
      act(() => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })))
      expect(onNavigate).toHaveBeenCalledWith({
        to: '/w/$slug/docs/spaces/$spaceId', params: { slug: 'acme', spaceId: 'space-2' },
      })
    } finally {
      act(() => root.unmount())
      container.remove()
    }
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
    const spaceAvatar = container.querySelector('[data-slot="docs-space-avatar"]')
    expect(spaceAvatar?.className).toContain('rounded-full')
    expect(spaceAvatar?.className).toContain('bg-')
    expect(spaceAvatar?.className).not.toContain('bg-foreground')
    expect(container.querySelector('[aria-current="page"]')?.textContent).toContain('Welcome guide')
    const documentRows = Array.from(container.querySelectorAll<HTMLButtonElement>('[data-slot="docs-document-row"]'))
    const treeDocumentRow = documentRows.find((row) => !row.dataset.pinned)
    expect(treeDocumentRow?.querySelector('svg')).toBeNull()
    // My documents, All docs in space, and the collection share one centred count slot.
    const countColumns = Array.from(container.querySelectorAll('[data-slot="docs-sidebar-count"]'))
    expect(countColumns).toHaveLength(3)
    for (const count of countColumns) {
      expect(count.className).toContain('w-6')
      expect(count.className).toContain('text-center')
    }
    const myDocuments = Array.from(container.querySelectorAll('[data-slot="docs-quick-links"] button'))
      .find((button) => button.textContent?.startsWith('My documents'))
    expect(myDocuments?.querySelector('[data-slot="docs-sidebar-count"]')?.textContent).toBe('1')
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

  it('puts the collection menu and the "In this space" + in the count slot', () => {
    location.pathname = '/w/acme/docs/spaces/space-1'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => root.render(
      <TooltipProvider>
        <DocsRailNav wsId="ws-1" wsSlug="acme" canEditDocs isActive={() => false}
          openCreate={vi.fn()} onNavigate={vi.fn()} />
      </TooltipProvider>,
    ))
    try {
      const menu = container.querySelector('[data-slot="docs-tree-row-menu"]')!
      const add = container.querySelector('[data-slot="docs-tree-header-add"]')!
      // Same 24px slot, 8px from the right edge, as the counts (w-6 inside px-2 rows).
      expect(menu.className).toContain('w-6')
      expect(menu.className).toContain('right-2')
      expect(menu.className).not.toContain('right-0')
      expect(add.className).toContain('w-6')
      expect(add.parentElement?.className).toContain('px-2')
      const count = menu.parentElement?.querySelector('[data-slot="docs-sidebar-count"]')
      expect(count?.className).toContain('group-hover/collection:opacity-0')
    } finally {
      act(() => root.unmount())
      container.remove()
    }
  })

  it('opens All spaces from the space switcher and marks it current there', () => {
    location.pathname = '/w/acme/docs'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onNavigate = vi.fn()

    act(() => root.render(
      <TooltipProvider>
        <DocsRailNav wsId="ws-1" wsSlug="acme" canEditDocs={false} isActive={() => false}
          openCreate={vi.fn()} onNavigate={onNavigate} />
      </TooltipProvider>,
    ))
    try {
      expect(container.querySelector('[data-slot="docs-space-list"]')).not.toBeNull()
      expect(container.querySelector('[aria-label^="Manage "]')).toBeNull()
      expect(container.querySelector('[aria-label="New team space"]')).toBeNull()
      expect(container.querySelector('[aria-label="New external space"]')).toBeNull()
      act(() => container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!.click())
      const allSpaces = Array.from(document.querySelectorAll<HTMLButtonElement>('button'))
        .find((button) => button.textContent?.trim() === 'All spaces')
      expect(allSpaces?.getAttribute('aria-current')).toBe('page')
      expect(document.querySelector('[role="option"][aria-selected="true"]')).toBeNull()
      expect(document.querySelector('[aria-label^="Manage "]')).toBeNull()
      act(() => allSpaces!.click())
      expect(onNavigate).toHaveBeenCalledWith({ to: '/w/$slug/docs', params: { slug: 'acme' } })
    } finally {
      act(() => root.unmount())
      container.remove()
    }
  })

  it('lists the spaces instead of the last space on All spaces', () => {
    spaces.push({ ...spaces[0], id: 'space-2', name: 'Help Center', type: 'external_capable' })
    location.pathname = '/w/acme/docs'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onNavigate = vi.fn()

    act(() => root.render(
      <TooltipProvider>
        <DocsRailNav wsId="ws-1" wsSlug="acme" canEditDocs isActive={() => false}
          openCreate={vi.fn()} onNavigate={onNavigate} />
      </TooltipProvider>,
    ))
    try {
      const trigger = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!
      expect(trigger.textContent).toContain('All spaces')
      expect(trigger.textContent).toContain('2 spaces · 1 doc')
      expect(container.textContent).not.toContain('All docs in space')
      expect(container.textContent).not.toContain('In this space')
      expect(container.textContent).toContain('Team spaces')
      expect(container.textContent).toContain('External spaces')

      const rows = Array.from(container.querySelectorAll('[data-slot="docs-space-row"]'))
      expect(rows.map((row) => row.querySelector('button > .truncate')?.textContent))
        .toEqual(['Product docs', 'Help Center'])
      expect(rows.map((row) => row.querySelector('[data-slot="docs-sidebar-count"]')?.textContent))
        .toEqual(['1', '0'])
      // Counts and the row "⋯" share the slot, like collection rows.
      expect(rows[0].querySelector('[data-slot="docs-sidebar-count"]')?.className).toContain('group-hover/space:opacity-0')
      expect(rows[0].querySelector('[aria-label="Manage Product docs"]')).not.toBeNull()
      expect(container.querySelector('[aria-label="New team space"]')).not.toBeNull()
      expect(container.querySelector('[aria-label="New external space"]')).not.toBeNull()

      act(() => container.querySelector<HTMLButtonElement>('[aria-label="New team space"]')!.click())
      expect(vi.mocked(SpaceDialog).mock.calls.at(-1)?.[0]).toMatchObject({ open: true, defaultType: 'internal' })
      act(() => vi.mocked(SpaceDialog).mock.calls.at(-1)![0].onOpenChange(false))
      act(() => container.querySelector<HTMLButtonElement>('[aria-label="New external space"]')!.click())
      expect(vi.mocked(SpaceDialog).mock.calls.at(-1)?.[0]).toMatchObject({ open: true, defaultType: 'external_capable' })
      act(() => vi.mocked(SpaceDialog).mock.calls.at(-1)![0].onOpenChange(false))

      act(() => rows[1].querySelector('button')!.click())
      expect(onNavigate).toHaveBeenCalledWith({
        to: '/w/$slug/docs/spaces/$spaceId',
        params: { slug: 'acme', spaceId: 'space-2' },
      })
    } finally {
      act(() => root.unmount())
      container.remove()
    }
  })

  it('restores the selected space tree after leaving All spaces', () => {
    spaces.push({ ...spaces[0], id: 'space-2', name: 'Help Center', type: 'external_capable' })
    location.pathname = '/w/acme/docs'
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const render = () => act(() => root.render(
      <TooltipProvider>
        <DocsRailNav wsId="ws-1" wsSlug="acme" canEditDocs isActive={() => false}
          openCreate={vi.fn()} onNavigate={vi.fn()} />
      </TooltipProvider>,
    ))
    render()
    try {
      const rows = container.querySelectorAll('[data-slot="docs-space-row"]')
      act(() => rows[1].querySelector('button')!.click())
      for (const pathname of ['/w/acme/docs/spaces/space-2', '/w/acme/docs/my', '/w/acme/docs/recent']) {
        location.pathname = pathname
        render()
        expect(container.querySelector('[aria-haspopup="listbox"]')?.textContent).toContain('Help Center')
        expect(container.querySelector('[data-slot="docs-space-list"]')).toBeNull()
        expect(container.querySelector('[data-slot="docs-tree-scroll"]')).not.toBeNull()
      }
      location.pathname = '/w/acme/docs'
      render()
      expect(container.querySelector('[aria-haspopup="listbox"]')?.textContent).toContain('All spaces')
      expect(container.querySelector('[data-slot="docs-space-list"]')).not.toBeNull()
      expect(container.querySelector('[data-slot="docs-tree-scroll"]')).toBeNull()
    } finally {
      act(() => root.unmount())
      container.remove()
    }
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
