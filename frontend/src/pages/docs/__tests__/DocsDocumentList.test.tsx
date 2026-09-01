// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'

const { useDocsDocuments } = vi.hoisted(() => ({
  useDocsDocuments: vi.fn(() => ({ data: [], isLoading: false })),
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock('@/hooks/useTitle', () => ({
  useTitle: () => {},
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { id: string; slug: string } }) => unknown) =>
    selector({ currentWorkspace: { id: 'ws-1', slug: 'acme' } }),
}))

vi.mock('@/hooks/queries', () => ({
  useDocsDocuments,
  useWorkspaceAccess: () => ({
    data: {
      membership: { id: 'member-1', user_id: 'user-1', role: 'member' },
      permissions: ['docs.read'],
    },
    isLoading: false,
  }),
  usePermissions: () => ({ canEditDocs: false }),
  useAssignableMembers: () => ({ data: [] }),
  useDocsSpaces: () => ({ data: [] }),
  useAllDocsCollections: () => ({ data: [] }),
  useArchiveDocsDocument: () => ({ mutate: vi.fn() }),
  useUnarchiveDocsDocument: () => ({ mutate: vi.fn() }),
  useDeleteDocsDocument: () => ({ mutateAsync: vi.fn() }),
  useDuplicateDocsDocument: () => ({ mutateAsync: vi.fn() }),
  usePublishDocsDocument: () => ({ mutate: vi.fn() }),
}))

vi.mock('@/components/pm/ConfirmDialog', () => ({
  ConfirmDialog: () => null,
}))

import { DocsDocumentList } from '../DocsDocumentList'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('DocsDocumentList', () => {
  afterEach(() => {
    useDocsDocuments.mockClear()
    document.body.innerHTML = ''
  })

  it('loads every document owned by the current workspace member', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <DocsDocumentList
          title="My Documents"
          description="All documents you own."
          filterMode="my"
        />,
      )
    })

    expect(useDocsDocuments).toHaveBeenCalledWith(
      'ws-1',
      { owner_id: 'member-1', include_archived: 'true' },
      { enabled: true },
    )
    expect(container.querySelector('header')?.className).toContain('border-b')

    act(() => root.unmount())
  })

  it('keeps Docs Status and Sort controls on the standard small-button typography', () => {
    const documentListSource = readFileSync(resolve(__dirname, '../DocsDocumentList.tsx'), 'utf8')
    const documentsTableSource = readFileSync(resolve(__dirname, '../spaceDetail/DocumentsTable.tsx'), 'utf8')
    const libraryListSource = readFileSync(resolve(__dirname, '../../../components/docs/DocsLibraryList.tsx'), 'utf8')

    for (const source of [documentListSource, documentsTableSource, libraryListSource]) {
      expect(source).not.toContain('className="text-xs text-muted-foreground"')
    }
    expect(documentListSource).toContain('<FilterHorizontalIcon className="h-4 w-4" />')
    expect(documentsTableSource).toContain('<FilterHorizontalIcon className="h-4 w-4" />')
    expect(libraryListSource).toContain('<ArrowUpDownIcon className="h-4 w-4" />')
  })
})
