// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const navigate = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigate,
}))

vi.mock('@/hooks/useTitle', () => ({
  useTitle: () => {},
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { id: string; slug: string } | null }) => unknown) =>
    selector({ currentWorkspace: { id: 'ws-1', slug: 'acme' } }),
}))

vi.mock('radix-ui', () => ({
  Collapsible: {
    Root: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    Trigger: ({ children }: { children: ReactNode }) => <>{children}</>,
    Content: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  },
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props}>{children}</button>
  ),
}))

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/docs/CreateSpaceDialog', () => ({
  CreateSpaceDialog: () => null,
}))

vi.mock('@/components/docs/DocsArrangeTree', () => ({
  DocsArrangeTree: () => <div>Arrange tree</div>,
}))

vi.mock('@/hooks/queries/useSettings', () => ({
  useWorkspaceSettings: () => ({
    data: undefined,
    isLoading: false,
  }),
}))

vi.mock('@/hooks/queries', () => ({
  useDocsSpaces: () => ({
    data: [
      {
        id: 'space-1',
        workspace_id: 'ws-1',
        team_id: null,
        name: 'Support',
        slug: 'support',
        icon: null,
        visibility: 'workspace_wide',
        type: 'internal',
        default_review_days: null,
        is_system: false,
        position: 0,
        created_by: 'user-1',
        created_at: '2026-03-25T09:00:00Z',
        updated_at: '2026-03-25T09:00:00Z',
      },
    ],
    isLoading: false,
  }),
  useAllDocsCollections: () => ({
    data: [
      {
        id: 'coll-1',
        workspace_id: 'ws-1',
        space_id: 'space-1',
        name: 'Getting started',
        slug: 'getting-started',
        description: null,
        icon: null,
        position: 0,
        created_by: 'user-1',
        created_at: '2026-03-25T09:00:00Z',
        updated_at: '2026-03-25T09:00:00Z',
      },
    ],
  }),
  useDocsDocuments: () => ({
    data: [
      {
        id: 'doc-1',
        workspace_id: 'ws-1',
        space_id: 'space-1',
        collection_id: 'coll-1',
        title: 'Install the widget',
        status: 'draft',
        visibility: 'workspace_wide',
        owner_id: null,
        team_id: null,
        template_key: null,
        excerpt: null,
        icon: null,
        tags: [],
        position: 0,
        is_pinned: false,
        is_publicly_shared: false,
        share_token: null,
        is_locked: false,
        locked_by: null,
        last_reviewed_at: null,
        next_review_at: null,
        published_at: null,
        created_by: 'user-1',
        created_at: '2026-03-25T09:00:00Z',
        updated_at: new Date().toISOString(),
      },
    ],
  }),
  useCreateDocsSpace: () => ({
    mutateAsync: vi.fn(),
  }),
  useUpdateDocsDocument: () => ({
    mutateAsync: vi.fn(),
  }),
  useWorkspaceAccess: () => ({
    data: { permissions: ['docs.edit'] },
  }),
  usePermissions: () => ({
    canEditDocs: true,
  }),
}))

import { DocsHome } from '../DocsHome'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('DocsHome', () => {
  afterEach(() => {
    navigate.mockReset()
  })

  it('labels document timestamps as updated times in the docs tree', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<DocsHome />)
    })

    expect(container.textContent).toContain('Install the widget')
    expect(container.textContent).toContain('Updated:')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
