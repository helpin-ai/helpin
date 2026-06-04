// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { pmTaskService } from '@/lib/services/pmTaskService'
import { MyWorkPage } from '../MyWork'

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => ({ pathname: '/w/acme/pm/my-work', search: {} }),
  useNavigate: () => vi.fn(),
}))

vi.mock('@/hooks/useTitle', () => ({
  useTitle: vi.fn(),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ id: 'ws-1', slug: 'acme' }),
}))

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({
    data: {
      membership: { id: 'member-1' },
    },
  }),
}))

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => ({
    teams: [{ id: 'team-1', name: 'Engineering' }],
    hasTeams: true,
    isAdmin: false,
    findTeamName: (id?: string) => (id === 'team-1' ? 'Engineering' : undefined),
  }),
}))

vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/pm/task-detail/taskRouteNavigation', () => ({
  openTaskRoute: vi.fn(),
}))

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    list: vi.fn(),
  },
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const task = {
  id: 'task-1',
  workspace_id: 'ws-1',
  team_id: 'team-1',
  name: 'Testing one more',
  task_key: 'TESTD-25',
  priority: 'none',
  state_name: 'To Do',
  state_type: 'backlog',
  completed: false,
  blocked: false,
  deadline: null,
  updated_at: '2026-05-05T00:00:00Z',
  latest_run_status: null,
} as any

describe('MyWorkPage', () => {
  beforeEach(() => {
    vi.mocked(pmTaskService.list).mockResolvedValue({
      data: {
        data: [task],
      },
    } as any)
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('keeps long task keys on one line while preserving the title start', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<MyWorkPage />)
      await Promise.resolve()
      await Promise.resolve()
    })

    const taskKey = Array.from(container.querySelectorAll('span')).find(
      (node) => node.textContent === 'TESTD-25' && String(node.className).includes('font-mono'),
    )

    expect(taskKey).toBeTruthy()
    expect(taskKey?.className).toContain('whitespace-nowrap')
    expect(taskKey?.parentElement?.className).toContain('relative')
    expect(taskKey?.parentElement?.className).toContain('w-14')

    act(() => {
      root.unmount()
    })
  })
})
