// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { pmTaskService } from '@/lib/services/pmTaskService'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import type { Task } from '@/lib/pmTypes'
import { MyWorkPage } from '../MyWork'

const stableMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  access: {
    data: { membership: { id: 'member-1' } },
    isLoading: false,
  },
  accessibleTeams: {
    teams: [{ id: 'team-1', name: 'Engineering' }],
    hasTeams: true,
    isAdmin: false,
    loading: false,
    findTeamName: (id?: string) => (id === 'team-1' ? 'Engineering' : undefined),
  },
}))

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => ({ pathname: '/w/acme/pm/my-work', search: {} }),
  useNavigate: () => stableMocks.navigate,
}))

vi.mock('@/hooks/useTitle', () => ({
  useTitle: vi.fn(),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ id: 'ws-1', slug: 'acme' }),
}))

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => stableMocks.access,
}))

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => stableMocks.accessibleTeams,
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
} as unknown as Task

type TaskListResponse = Awaited<ReturnType<typeof pmTaskService.list>>

function taskListResponse(tasks: Task[]): TaskListResponse {
  return { data: { data: tasks }, error: null } as unknown as TaskListResponse
}

function taskListError(error: string): TaskListResponse {
  return { data: null, error } as unknown as TaskListResponse
}

async function renderPage() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(<MyWorkPage />)
    await Promise.resolve()
    await Promise.resolve()
  })

  return { container, root }
}

describe('MyWorkPage', () => {
  beforeEach(() => {
    stableMocks.access.isLoading = false
    stableMocks.accessibleTeams.teams = [{ id: 'team-1', name: 'Engineering' }]
    stableMocks.accessibleTeams.hasTeams = true
    stableMocks.accessibleTeams.isAdmin = false
    stableMocks.accessibleTeams.loading = false
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse([task]))
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('keeps long task keys on one line while preserving the title start', async () => {
    const { container, root } = await renderPage()

    const taskKey = Array.from(container.querySelectorAll('span')).find(
      (node) => node.textContent === 'TESTD-25' && String(node.className).includes('font-mono'),
    )

    expect(taskKey).toBeTruthy()
    expect(taskKey?.className).toContain('whitespace-nowrap')
    expect(taskKey?.className).toContain('font-mono')
    expect(container.textContent).toContain('Testing one more')

    act(() => {
      root.unmount()
    })
  })

  it('uses the Quiet shell, tabs, metric blocks, and stacked task facts', async () => {
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse([{
      ...task,
      priority: 'high',
      state_name: 'In progress',
      state_type: 'started',
      blocked: true,
      latest_run_status: 'running',
    }]))

    const { container, root } = await renderPage()

    expect(container.querySelector('h1')?.textContent).toBe('My Work')
    expect(container.querySelector('[role="tablist"]')).toBeTruthy()
    expect(container.querySelector('.max-w-7xl')).toBeTruthy()
    expect(container.querySelector('.max-w-4xl')).toBeFalsy()
    expect(container.textContent).toContain('Assigned to me')
    expect(container.textContent).toContain('Requested by me')
    expect(container.textContent).toContain('Tasks currently underway')
    expect(container.textContent).toContain('High priority')
    expect(container.textContent).toContain('Blocked')
    expect(container.textContent).toContain('Agent running')

    act(() => {
      root.unmount()
    })
  })

  it('requests the correct task ownership filter for each tab', async () => {
    const { container, root } = await renderPage()

    expect(pmTaskService.list).toHaveBeenCalledWith('ws-1', expect.objectContaining({ owner_member_ids: 'member-1' }))

    const requestedTab = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Requested by me')
    await act(async () => {
      requestedTab?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(pmTaskService.list).toHaveBeenLastCalledWith('ws-1', expect.objectContaining({ requester_member_id: 'member-1' }))

    act(() => {
      root.unmount()
    })
  })

  it('ignores a late response from the previously selected tab', async () => {
    let resolveAssigned: ((response: TaskListResponse) => void) | undefined
    const assignedRequest = new Promise<TaskListResponse>((resolve) => {
      resolveAssigned = resolve
    })
    const requestedTask = { ...task, id: 'task-requested', name: 'Requested task' }
    const lateAssignedTask = { ...task, id: 'task-assigned-late', name: 'Late assigned task' }
    vi.mocked(pmTaskService.list)
      .mockImplementationOnce(() => assignedRequest)
      .mockResolvedValueOnce(taskListResponse([requestedTask]))

    const { container, root } = await renderPage()
    const requestedTab = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Requested by me')
    await act(async () => {
      requestedTab?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(container.textContent).toContain('Requested task')

    await act(async () => {
      resolveAssigned?.(taskListResponse([lateAssignedTask]))
      await Promise.resolve()
    })

    expect(container.textContent).toContain('Requested task')
    expect(container.textContent).not.toContain('Late assigned task')

    act(() => {
      root.unmount()
    })
  })

  it('opens a task through the existing route helper', async () => {
    const { container, root } = await renderPage()
    const taskButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Testing one more'))

    act(() => {
      taskButton?.click()
    })

    expect(openTaskRoute).toHaveBeenCalledWith(stableMocks.navigate, expect.anything(), 'acme', 'task-1')

    act(() => {
      root.unmount()
    })
  })

  it('shows a quiet error state and retries the request', async () => {
    vi.mocked(pmTaskService.list)
      .mockResolvedValueOnce(taskListError('Service unavailable'))
      .mockResolvedValueOnce(taskListResponse([task]))

    const { container, root } = await renderPage()
    expect(container.textContent).toContain('Unable to load your work')
    expect(container.textContent).toContain('Service unavailable')

    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Try again')
    await act(async () => {
      retry?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(pmTaskService.list).toHaveBeenCalledTimes(2)
    expect(container.textContent).toContain('Testing one more')

    act(() => {
      root.unmount()
    })
  })

  it('renders the team access blocker without an illustrated card', async () => {
    stableMocks.accessibleTeams.teams = []
    stableMocks.accessibleTeams.hasTeams = false
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse([]))
    const { container, root } = await renderPage()

    expect(container.textContent).toContain('No team assigned')
    expect(container.textContent).toContain('Team access required')
    expect(container.querySelector('.rounded-lg')).toBeFalsy()

    act(() => {
      root.unmount()
    })
  })

  it('renders the original illustrated empty workflow below the quiet page header', async () => {
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse([]))
    const { container, root } = await renderPage()

    expect(container.textContent).toContain('No tasks assigned to you yet')
    expect(container.textContent).toContain('Create tasks')
    expect(container.textContent).toContain('Assign to team')
    expect(container.textContent).toContain('Track progress')
    expect(container.querySelector('.rounded-lg')).toBeTruthy()
    expect(container.querySelector('h1')?.textContent).toBe('My Work')

    act(() => {
      root.unmount()
    })
  })
})
