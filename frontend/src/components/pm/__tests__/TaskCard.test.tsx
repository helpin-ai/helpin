// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { TaskCard } from '../TaskCard'
import { BoardDataContext, BoardCallbacksContext, type BoardCallbacksContextValue } from '../KanbanBoard.contexts'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { Agent, Task } from '@/lib/pmTypes'
import type { AssignableMember } from '@/lib/types'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
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

vi.mock('@/stores/boardDisplayStore', () => ({
  useBoardDisplayStore: (selector: (state: { properties: Record<string, boolean> }) => unknown) => selector({
    properties: {
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
    },
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

function renderTaskCard(task: Task) {
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

  act(() => {
    root.render(
      <TooltipProvider>
        <BoardDataContext.Provider
          value={{
            workspaceId: 'workspace-1',
            ownerNameMap: new Map([['member-1', 'Owner Person']]),
            agentById: new Map([['agent-1', agent]]),
            assignableMembers: [owner],
            automatedStateIds: new Set(),
            findTeamName: () => undefined,
          }}
        >
          <BoardCallbacksContext.Provider value={callbacks}>
            <TaskCard task={task} />
          </BoardCallbacksContext.Provider>
        </BoardDataContext.Provider>
      </TooltipProvider>,
    )
  })

  return { container, root }
}

describe('TaskCard', () => {
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
})
