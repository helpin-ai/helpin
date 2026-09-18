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
  canEdit: true,
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

vi.mock('@/components/pm/my-work/AISuggestions', () => ({ AISuggestions: () => <div>Meeting follow-up suggestions</div> }))

vi.mock('@/hooks/queries/useWorkspaces', () => ({ useWorkspaceMembers: () => ({data: [{id:'sara',full_name:'Sara Khan',email:'sara@example.com'}]}) }))

vi.mock('@/hooks/useTitle', () => ({
  useTitle: vi.fn(),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: () => ({ id: 'ws-1', slug: 'acme' }),
}))

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => stableMocks.access,
  usePermissions: () => ({ has: (permission: string) => permission !== 'pm.edit' || stableMocks.canEdit }),
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
    localStorage.clear()
    stableMocks.canEdit = true
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

  it('preserves task facts in the compact list and filter toolbar', async () => {
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
    expect(container.textContent).toContain('In progress')
    expect(container.textContent).toContain('High')
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

  it('opens AI suggestions without requesting another task list and restores assigned work', async () => {
    const { container, root } = await renderPage()
    const calls = vi.mocked(pmTaskService.list).mock.calls.length
    const tab = (name: string) => Array.from(container.querySelectorAll('[role="tab"]')).find((node) => node.textContent === name)
    await act(async () => {
      tab('AI suggestions')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
    })
    expect(container.textContent).toContain('Meeting follow-up suggestions')
    expect(container.textContent).not.toContain('Testing one more')
    expect(pmTaskService.list).toHaveBeenCalledTimes(calls)
    await act(async () => {
      tab('Assigned to me')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
    })
    expect(container.textContent).toContain('Testing one more')
    act(() => root.unmount())
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
  it('searches across loaded pages and opens collapsed completed matches', async () => {
    vi.mocked(pmTaskService.list)
      .mockResolvedValueOnce({data:{data:[task],total_pages:2},error:null} as TaskListResponse)
      .mockResolvedValueOnce({data:{data:[{...task,id:'done',task_key:'DONE-1',name:'Historical fix',completed:true}],total_pages:2},error:null} as TaskListResponse)
    const {container,root}=await renderPage()
    expect(pmTaskService.list).toHaveBeenCalledTimes(2)
    expect(container.textContent).not.toContain('Historical fix')
    await act(async()=>{
      const input=container.querySelector<HTMLInputElement>('input[aria-label="Search tasks"]')!
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value')!.set!.call(input,'DONE-1')
      input.dispatchEvent(new Event('input',{bubbles:true}))
    })
    expect(container.textContent).toContain('Historical fix')
    expect(container.textContent).not.toContain('Testing one more')
    act(()=>root.unmount())
  })

  it('remembers separate quick filters for assigned and requested tabs', async () => {
    const {container,root}=await renderPage()
    const tab=(name:string)=>Array.from(container.querySelectorAll('[role="tab"]')).find(node=>node.textContent===name)
    act(()=>Array.from(container.querySelectorAll('button')).find(node=>node.textContent?.startsWith('Blocked ('))?.click())
    expect(container.textContent).toContain('No matching tasks')
    await act(async()=>{tab('Requested by me')?.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}))})
    expect(container.textContent).toContain('Testing one more')
    await act(async()=>{tab('Assigned to me')?.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}))})
    expect(container.textContent).toContain('No matching tasks')
    act(()=>root.unmount())
  })

  it('preserves all agent outcomes and pause reasons, independently of task completion', async () => {
    const statuses=[['queued',null,'Agent queued'],['running',null,'Agent running'],['completed',null,'Agent completed'],['failed',null,'Agent failed'],['cancelled',null,'Agent cancelled'],['paused','human_approval','Agent needs approval'],['paused','authentication','Agent needs auth'],['paused','awaiting_user_message','Agent awaiting reply'],['paused',null,'Agent needs input']]
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse(statuses.map(([status,reason],i)=>({...task,id:`run-${i}`,latest_run_status:status,latest_run_pause_reason:reason}) as Task)))
    const {container,root}=await renderPage()
    for(const [, ,label] of statuses)expect(container.textContent).toContain(label)
    expect(container.textContent).not.toContain('Recently completed')
    act(()=>root.unmount())
  })

  it('shows requested-task owners and searches by owner name', async () => {
    vi.mocked(pmTaskService.list).mockResolvedValue(taskListResponse([{...task,owner_member_ids:['sara']}]))
    const {container,root}=await renderPage()
    await act(async()=>{Array.from(container.querySelectorAll('[role="tab"]')).find(node=>node.textContent==='Requested by me')?.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,button:0}))})
    expect(container.textContent).toContain('Sara Khan')
    await act(async()=>{
      const input=container.querySelector<HTMLInputElement>('input[aria-label="Search tasks"]')!
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value')!.set!.call(input,'Sara')
      input.dispatchEvent(new Event('input',{bubbles:true}))
    })
    expect(container.textContent).toContain('Testing one more')
    act(()=>root.unmount())
  })

  it('keeps task facts readable without exposing edit controls to read-only members', async () => {
    stableMocks.canEdit=false
    const {container,root}=await renderPage()
    expect(container.textContent).toContain('To Do')
    expect(container.querySelector('button[aria-label^="Change stage"]')).toBeNull()
    expect(container.querySelector('button[aria-label^="Change priority"]')).toBeNull()
    expect(container.querySelector('button[aria-label^="Due date"]')).toBeNull()
    act(()=>root.unmount())
  })

})
