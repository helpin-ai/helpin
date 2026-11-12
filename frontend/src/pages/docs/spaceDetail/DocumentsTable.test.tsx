// @vitest-environment jsdom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DocsCollection, DocsDocument, DocStatus } from '@/lib/docsTypes'
import type { AssignableMember } from '@/lib/types'
import { TooltipProvider } from '@/components/ui/tooltip'
import { buildCollectionTree } from '@/components/docs/docsCollectionTree'
import { buildOrderGroups } from './docsOrder'
import {
  DocumentsTable,
  type DocumentsTableOrder,
  type DocumentsTablePathColumn,
  type DocumentsTableSortDir,
  type DocumentsTableSortField,
} from './DocumentsTable'

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function makeDoc(overrides: Partial<DocsDocument> & { id: string }): DocsDocument {
  return {
    workspace_id: 'ws',
    space_id: 'space',
    title: overrides.id,
    status: 'published',
    visibility: 'workspace',
    tags: [],
    position: 0,
    sort_key: '',
    is_pinned: false,
    is_publicly_shared: false,
    is_locked: false,
    created_by: 'user-1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as DocsDocument
}

function makeCollection(overrides: Partial<DocsCollection> & { id: string }): DocsCollection {
  return {
    workspace_id: 'ws',
    space_id: 'space',
    name: overrides.id,
    slug: overrides.id,
    depth: 0,
    position: 0,
    sort_key: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as DocsCollection
}

const members: AssignableMember[] = [
  { id: 'member-1', user_id: 'user-1', role: 'member', email: 'sara@example.com', display_name: 'Sara Malik', status: 'active' },
  { id: 'member-2', user_id: 'user-2', role: 'member', email: 'omar@example.com', display_name: 'Omar Ali', status: 'active' },
]

function Harness({
  documents,
  pathColumn = null,
  order = null,
  showOwner,
  showStatusFilter,
}: {
  documents: DocsDocument[]
  pathColumn?: DocumentsTablePathColumn | null
  order?: DocumentsTableOrder | null
  showOwner?: boolean
  showStatusFilter?: boolean
}) {
  const [statuses, setStatuses] = useState<DocStatus[]>(['draft', 'published'])
  const [sort, setSort] = useState<{ field: DocumentsTableSortField; dir: DocumentsTableSortDir }>({ field: 'updated_at', dir: 'desc' })
  return (
    <TooltipProvider>
      <DocumentsTable
        documents={documents}
        members={members}
        pathColumn={pathColumn}
        showOwner={showOwner}
        showStatusFilter={showStatusFilter}
        order={order}
        wsSlug="acme"
        wsId="ws"
        selectionScope="test"
        statuses={statuses}
        onStatusesChange={setStatuses}
        sortField={sort.field}
        sortDir={sort.dir}
        onSort={(field, dir) => setSort({ field, dir })}
        canEdit={false}
        duplicatingDocId={null}
        onArchive={vi.fn()}
        onUnarchive={vi.fn()}
        onDelete={vi.fn()}
        onDuplicate={vi.fn()}
        onMove={vi.fn()}
        onPublish={vi.fn()}
      />
    </TooltipProvider>
  )
}

let root: Root | null = null

function render(ui: React.ReactNode) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root!.render(ui))
  return container
}

function rowTitles(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-slot="docs-table-row"]'))
    .map((row) => row.querySelector('span.font-medium')?.textContent)
}

function click(element: Element | null) {
  if (!element) throw new Error('element not found')
  act(() => (element as HTMLElement).click())
}

describe('DocumentsTable', () => {
  afterEach(() => {
    act(() => root?.unmount())
    root = null
    document.body.innerHTML = ''
  })

  it('hides archived documents until the user asks for them', () => {
    const container = render(
      <Harness documents={[
        makeDoc({ id: 'Live', updated_at: '2026-01-03T00:00:00Z' }),
        makeDoc({ id: 'Old', status: 'archived', updated_at: '2026-01-02T00:00:00Z' }),
      ]} />,
    )

    expect(rowTitles(container)).toEqual(['Live'])
    expect(container.textContent).toContain('1 archived document hidden')

    click(Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Show') ?? null)

    expect(rowTitles(container)).toEqual(['Live', 'Old'])
    expect(container.textContent).not.toContain('archived document hidden')
  })

  it('sorts from the column headers and shows uncategorized documents in the list', () => {
    const container = render(
      <Harness
        documents={[
          makeDoc({ id: 'Beta', collection_id: 'c1', updated_at: '2026-01-03T00:00:00Z', owner_id: 'member-2' }),
          makeDoc({ id: 'Alpha', updated_at: '2026-01-02T00:00:00Z', owner_id: 'member-1' }),
        ]}
        pathColumn={{ label: 'Collection', emptyLabel: 'Uncategorized', pathFor: (d) => (d.collection_id ? 'Billing' : null) }}
      />,
    )

    expect(rowTitles(container)).toEqual(['Beta', 'Alpha'])
    expect(container.textContent).toContain('Uncategorized')
    expect(container.textContent).toContain('Sara Malik')

    click(container.querySelector('button[aria-label="Sort by Title"]'))
    expect(rowTitles(container)).toEqual(['Alpha', 'Beta'])

    click(container.querySelector('button[aria-label="Sort by Title"]'))
    expect(rowTitles(container)).toEqual(['Beta', 'Alpha'])

    // Documents without a collection sort last, in both directions.
    click(container.querySelector('button[aria-label="Sort by Collection"]'))
    expect(rowTitles(container)).toEqual(['Beta', 'Alpha'])
  })

  it('drops the Owner column and status filter when a list does not need them', () => {
    const container = render(
      <Harness documents={[makeDoc({ id: 'Mine', owner_id: 'member-2' })]} showOwner={false} showStatusFilter={false} />,
    )

    expect(container.querySelector('button[aria-label="Sort by Owner"]')).toBeNull()
    expect(container.textContent).not.toContain('Omar Ali')
    expect(container.querySelector('button[aria-label^="Status:"]')).toBeNull()
    expect(container.querySelector('button[aria-label="Sort by Updated"]')).not.toBeNull()
  })

  it('lists the saved order by group and leaves it when a column is sorted', () => {
    const collections = [
      makeCollection({ id: 'inbox', name: 'Inbox' }),
      makeCollection({ id: 'routing', name: 'Routing', parent_collection_id: 'inbox', depth: 1 }),
    ]
    const documents = [
      makeDoc({ id: 'Second', collection_id: 'inbox', position: 2, updated_at: '2026-01-05T00:00:00Z' }),
      makeDoc({ id: 'First', collection_id: 'inbox', position: 1 }),
      makeDoc({ id: 'Routed', collection_id: 'routing', position: 0, updated_at: '2026-01-09T00:00:00Z' }),
    ]
    const groups = buildOrderGroups(buildCollectionTree('space', collections, documents).topLevel[0])
    const onActiveChange = vi.fn()
    const container = render(
      <Harness
        documents={documents}
        order={{
          label: 'Help center order',
          description: 'The order customers see',
          groups,
          active: true,
          onActiveChange,
          onChange: vi.fn(async () => {}),
        }}
      />,
    )

    expect(rowTitles(container)).toEqual(['First', 'Second', 'Routed'])
    expect(container.querySelector('button[aria-pressed="true"]')?.textContent).toContain('Help center order')

    click(container.querySelector('button[aria-label="Sort by Updated"]'))
    expect(onActiveChange).toHaveBeenCalledWith(false)
  })
})
