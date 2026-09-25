// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TaskCard } from '../TaskCard'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { toast } from 'sonner'
import { shouldIgnoreTaskCardDrag } from '../TaskCard.sortable'
import { BoardDataContext, BoardCallbacksContext, type BoardCallbacksContextValue } from '../KanbanBoard.contexts'
import { getDragStartTaskRect } from '../KanbanBoard.dnd'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { Agent, Epic, Task } from '@/lib/pmTypes'
import type { AssignableMember } from '@/lib/types'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

vi.mock('@/lib/services/pmTaskService', () => ({ pmTaskService: { update: vi.fn() } }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: { 'data-sortable-activator': 'true' },
    listeners: {},
    setNodeRef: vi.fn(),
    setActivatorNodeRef: vi.fn(),
    transform: null,
    transition: undefined,
    isDragging: false,
  }),
}))

vi.mock('@/hooks/queries', () => ({
  useTeamEstimateSettingsForTeam: () => null,
  useTeamFieldVisibilityForTeam: () => ({
    task_type: true,
    priority: true,
    severity: true,
    epic: true,
    sprint: true,
    labels: true,
    estimate: true,
    due_date: true,
    blocked: true,
  }),
}))

const displayStoreMock = vi.hoisted(() => {
  const defaultProperties = {
    task_id: true,
    task_type: true,
    priority: true,
    severity: true,
    agent: true,
    epic: true,
    sprint: true,
    labels: true,
    estimate: true,
    due_date: true,
    blocked: true,
    assignee: true,
  }

  return {
    defaultProperties,
    properties: { ...defaultProperties },
  }
})

vi.mock('@/stores/boardDisplayStore', () => ({
  useBoardDisplayStore: (selector: (state: { properties: Record<string, boolean> }) => unknown) => selector({
    properties: displayStoreMock.properties,
  }),
}))

vi.mock('@/components/agents/AgentAvatar', () => ({
  AgentAvatar: ({ agent, className }: { agent?: Pick<Agent, 'name'> | null; className?: string }) => (
    <span className={className} data-testid="agent-avatar">{agent?.name ?? 'Agent'}</span>
  ),
  resolveAgentPersonaKey: () => 'generic',
}))

const agent: Agent = {
  id: 'agent-1',
  workspace_id: 'workspace-1',
  is_system: false,
  name: 'Build Agent',
  role: 'Engineer',
  status: 'active',
  runtime_kind: 'native_sdk',
  skills: [],
  trigger_mode: 'manual',
  tools: [],
  tokens_used_this_month: 0,
  allowed_tools: [],
  allowed_commands: [],
  allowed_targets: [],
  approval_mode: 'preset_default',
  max_concurrent_runs: 1,
  default_invocation_mode: 'interactive',
  created_at: '2026-05-05T00:00:00Z',
  updated_at: '2026-05-05T00:00:00Z',
}

const owner: AssignableMember = {
  id: 'member-1',
  user_id: 'user-1',
  role: 'member',
  email: 'owner@example.com',
  display_name: 'Owner Person',
  status: 'active',
}

function buildTask(patch: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    workspace_id: 'workspace-1',
    display_id: 42,
    task_key: 'HLP-42',
    name: 'Ship fixed agent card row',
    task_type: 'feature',
    workflow_id: 'workflow-1',
    workflow_state_id: 'state-1',
    team_id: 'team-1',
    owner_member_id: 'member-1',
    owner_member_ids: ['member-1'],
    estimate: 3,
    priority: 'high',
    severity: 'none',
    deadline: '2026-05-08',
    position: 1,
    started: false,
    completed: false,
    blocked: false,
    archived: false,
    created_at: '2026-05-05T00:00:00Z',
    updated_at: '2026-05-05T00:00:00Z',
    ...patch,
  }
}

function renderTaskCard(task: Task, props: { isOverlay?: boolean } = {}, epics: Epic[] = []) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const callbacks = {
    current: {
      onTaskPatched: vi.fn(),
      onOpen: vi.fn(),
      onOpenAgentRun: vi.fn(),
      onCreate: vi.fn(),
      onCreateForMember: vi.fn(),
      onToggleCollapse: vi.fn(),
      onLoadMore: vi.fn(),
      onLoadMoreMember: vi.fn(),
    } satisfies BoardCallbacksContextValue,
  }

  let boardData = {
    workspaceId: 'workspace-1',
    epicById: new Map(epics.map((epic) => [epic.id, epic])),
    ownerNameMap: new Map([['member-1', 'Owner Person']]),
    agentById: new Map([['agent-1', agent]]),
    assignableMembers: [owner],
    automatedStateIds: new Set<string>(),
    findTeamName: () => undefined,
  }
  const render = (nextTask: Task, nextProps: { isOverlay?: boolean } = props, nextEpics?: Epic[]) => {
    if (nextEpics) boardData = { ...boardData, epicById: new Map(nextEpics.map((epic) => [epic.id, epic])) }
    root.render(
      <TooltipProvider>
        <BoardDataContext.Provider
          value={boardData}
        >
          <BoardCallbacksContext.Provider value={callbacks}>
            <TaskCard task={nextTask} {...nextProps} />
          </BoardCallbacksContext.Provider>
        </BoardDataContext.Provider>
      </TooltipProvider>,
    )
  }

  act(() => {
    render(task)
  })

  return { container, root, callbacks: callbacks.current, rerender: (nextTask: Task, nextProps?: { isOverlay?: boolean }, nextEpics?: Epic[]) => act(() => render(nextTask, nextProps, nextEpics)) }
}

describe('TaskCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
    HTMLElement.prototype.scrollIntoView = vi.fn()
    displayStoreMock.properties = { ...displayStoreMock.defaultProperties }
  })

  afterEach(() => vi.unstubAllGlobals())

  it.each([
    ['epic-1', 'epic-2', 'epic-2'],
    ['epic-1', 'epic-1', ''],
    ['epic-1', '__none__', ''],
    [undefined, 'epic-2', 'epic-2'],
  ])('saves board epic changes from %s via %s', async (currentId, selectedId, savedId) => {
    const task = buildTask({ epic_id: currentId, epic_name: currentId ? 'Same name' : undefined })
    const epics = [
      { id: 'epic-1', name: 'Same name', color: '#e2564a' },
      { id: 'epic-2', name: 'Same name', color: '#4e8fea' },
    ] as Epic[]
    const savedTask = { ...task, epic_id: savedId || undefined, epic_name: savedId ? 'Same name' : undefined }
    vi.mocked(pmTaskService.update).mockResolvedValue({ data: { task: savedTask }, error: null } as Awaited<ReturnType<typeof pmTaskService.update>>)
    const { container, root, callbacks, rerender } = renderTaskCard(task, {}, epics)
    try {
      const trigger = currentId
        ? container.querySelector('[title="Same name"]')!.closest('button')!
        : Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'No Epic')!
      expect(shouldIgnoreTaskCardDrag(trigger, container.querySelector('article'))).toBe(true)
      await act(async () => trigger.click())
      const options = document.querySelectorAll('[role="option"]')
      expect(Array.from(options, (option) => option.getAttribute('data-value'))).toEqual(['__none__', 'epic-1', 'epic-2'])
      expect(options[0].textContent).toBe('None')
      expect(document.querySelector<HTMLElement>('[role="option"][data-value="epic-1"] [aria-hidden="true"][style]')!.style.backgroundColor).toBe('rgb(226, 86, 74)')
      expect(document.querySelector<HTMLElement>('[role="option"][data-value="epic-2"] [aria-hidden="true"][style]')!.style.backgroundColor).toBe('rgb(78, 143, 234)')
      await act(async () => document.querySelector<HTMLElement>(`[role="option"][data-value="${selectedId}"]`)!.click())
      expect(pmTaskService.update).toHaveBeenCalledWith('workspace-1', 'task-1', { epic_id: savedId })
      expect(callbacks.onTaskPatched).toHaveBeenCalledWith(savedTask)
      expect(callbacks.onOpen).not.toHaveBeenCalled()
      rerender(savedTask)
      if (savedId) {
        expect(container.querySelector<HTMLElement>('[title="Same name"] [aria-hidden="true"]')!.style.backgroundColor).toBe('rgb(78, 143, 234)')
      } else {
        expect(container.textContent).toContain('No Epic')
      }
    } finally {
      act(() => root.unmount())
      container.remove()
    }
  })

  it('reports a failed epic save without changing the board task', async () => {
    const task = buildTask({ epic_id: 'epic-1', epic_name: 'Current epic' })
    const epics = [{ id: 'epic-1', name: 'Current epic', color: '#e2564a' }] as Epic[]
    vi.mocked(pmTaskService.update).mockResolvedValue({ data: null, error: 'Unable to save' } as Awaited<ReturnType<typeof pmTaskService.update>>)
    const { container, root, callbacks } = renderTaskCard(task, {}, epics)
    try {
      await act(async () => container.querySelector('[title="Current epic"]')!.closest('button')!.click())
      await act(async () => document.querySelector<HTMLElement>('[role="option"][data-value="epic-1"]')!.click())
      expect(toast.error).toHaveBeenCalledWith('Unable to save')
      expect(callbacks.onTaskPatched).not.toHaveBeenCalled()
      expect(container.querySelector('[title="Current epic"]')).not.toBeNull()
    } finally {
      act(() => root.unmount())
      container.remove()
    }
  })

  it.each([false, true])('updates an epic badge from board data, including overlays (%s)', (isOverlay) => {
    const task = buildTask({ epic_id: 'epic-1', epic_name: 'Stale name' })
    const epic = { id: 'epic-1', name: 'Current epic', color: '#e2564a' } as Epic
    const { container, root, rerender } = renderTaskCard(task, { isOverlay }, [epic])
    const badge = () => container.querySelector<HTMLElement>('[title="Current epic"]')!
    expect(badge().querySelector<HTMLElement>('[aria-hidden="true"]')!.style.backgroundColor).toBe('rgb(226, 86, 74)')
    expect(badge().closest('button') !== null).toBe(!isOverlay)
    rerender(task, { isOverlay }, [{ ...epic, color: '#4e8fea' }])
    expect(badge().querySelector<HTMLElement>('[aria-hidden="true"]')!.style.backgroundColor).toBe('rgb(78, 143, 234)')
    act(() => root.unmount())
    container.remove()
  })

  it('updates color when reassigned to an epic with the same name', () => {
    const task = buildTask({ epic_id: 'epic-1', epic_name: 'Same name' })
    const epics = [
      { id: 'epic-1', name: 'Same name', color: '#e2564a' },
      { id: 'epic-2', name: 'Same name', color: '#4e8fea' },
    ] as Epic[]
    const { container, root, rerender } = renderTaskCard(task, {}, epics)
    rerender({ ...task, epic_id: 'epic-2' })
    expect(container.querySelector<HTMLElement>('[title="Same name"] [aria-hidden="true"]')!.style.backgroundColor).toBe('rgb(78, 143, 234)')
    act(() => root.unmount())
    container.remove()
  })

  it('uses the whole card as the drag activator without rendering a separate handle', () => {
    const { container, root } = renderTaskCard(buildTask())

    const article = container.querySelector('article')
    const dragHandle = container.querySelector('[data-task-card-drag-handle="true"]')

    expect(article).not.toBeNull()
    expect(article?.getAttribute('data-sortable-activator')).toBe('true')
    expect(dragHandle).toBeNull()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('measures the live card DOM rect for the initial source placeholder when dnd-kit has not populated a rect yet', () => {
    const { container, root } = renderTaskCard(buildTask())
    const article = container.querySelector('[data-pm-task-card="true"]') as HTMLElement | null
    const title = article?.querySelector('h4')

    expect(article).not.toBeNull()
    article!.getBoundingClientRect = () => ({
      x: 10,
      y: 20,
      top: 20,
      left: 10,
      right: 290,
      bottom: 157,
      width: 280,
      height: 137,
      toJSON: () => ({}),
    })

    const rect = getDragStartTaskRect({
      activeId: 'task-1',
      activatorEvent: { target: title } as unknown as Event,
      dndRect: null,
    })

    expect(rect?.height).toBe(137)
    expect(rect?.width).toBe(280)

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders the task key before the title when ID display is enabled', () => {
    const { container, root } = renderTaskCard(buildTask())
    const title = container.querySelector('h4')

    expect(title?.textContent).toBe('HLP-42: Ship fixed agent card row')
    expect(title?.querySelector('span')?.className).toContain('font-mono')
    expect(title?.querySelector('span')?.className).toContain('text-muted-foreground')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('hides the task key before the title when ID display is disabled', () => {
    displayStoreMock.properties = { ...displayStoreMock.defaultProperties, task_id: false }
    const { container, root } = renderTaskCard(buildTask())
    const title = container.querySelector('h4')

    expect(title?.textContent).toBe('Ship fixed agent card row')
    expect(title?.textContent).not.toContain('HLP-42:')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders dragged overlay cards with a translucent lift that does not change visual size', () => {
    const { container, root } = renderTaskCard(buildTask(), { isOverlay: true })

    const article = container.querySelector('article')
    expect(article?.className).not.toContain('rotate-[')
    expect(article?.className).not.toContain('scale-[')
    expect(article?.className).toContain('opacity-80')
    expect(article?.className).toContain('shadow-2xl')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders paused approval runs in a dedicated agent row after the footer', () => {
    const { container, root } = renderTaskCard(buildTask({
      latest_run_id: 'run-1',
      latest_run_agent_id: 'agent-1',
      latest_run_status: 'paused',
      latest_run_pause_reason: 'human_approval',
      latest_run_at: '2026-05-05T12:00:00Z',
    }))

    const agentRow = container.querySelector('[data-task-card-agent-row="true"]')
    const footer = container.querySelector('[data-task-card-footer="true"]')
    const owners = container.querySelector('[data-task-card-footer-owners="true"]')
    const metadata = container.querySelector('[data-task-card-footer-metadata="true"]')
    const agentLabel = container.querySelector('[data-task-card-agent-label="true"]')
    const agentAvatar = container.querySelector('[data-testid="agent-avatar"]')

    expect(agentRow?.textContent).toContain('Awaiting approval')
    expect(agentRow?.textContent).toContain('Build Agent')
    expect(footer?.textContent).not.toContain('Awaiting approval')
    expect(metadata?.compareDocumentPosition(owners as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    expect(footer?.compareDocumentPosition(agentRow as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    expect(agentAvatar?.compareDocumentPosition(agentLabel as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('rerenders when enriched card labels change without an updated_at change', () => {
    const { container, root, rerender } = renderTaskCard(buildTask({ labels: [] }))

    expect(container.textContent).not.toContain('Frontend')

    rerender(buildTask({
      labels: [{
        id: 'label-1',
        workspace_id: 'workspace-1',
        name: 'Frontend',
        color: '#3b82f6',
        archived: false,
        created_at: '2026-05-05T00:00:00Z',
        updated_at: '2026-05-05T00:00:00Z',
      }],
    }))

    expect(container.textContent).toContain('Frontend')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('rerenders when the task key changes without an updated_at change', () => {
    const { container, root, rerender } = renderTaskCard(buildTask())

    expect(container.querySelector('h4')?.textContent).toBe('HLP-42: Ship fixed agent card row')

    rerender(buildTask({ task_key: 'HLP-43', display_id: 43 }))

    expect(container.querySelector('h4')?.textContent).toBe('HLP-43: Ship fixed agent card row')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
