// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useAuthStore } from '@/stores/authStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { queryKeys } from '@/lib/queryKeys'

const captured = {
  onEvent: null as ((event: unknown) => void) | null,
  onPresenceSnapshot: null as ((snapshot: unknown) => void) | null,
  send: vi.fn(),
}

vi.mock('../useWebSocket', () => ({
  useWebSocket: vi.fn(({ onEvent, onPresenceSnapshot }) => {
    captured.onEvent = onEvent
    captured.onPresenceSnapshot = onPresenceSnapshot
    return { send: captured.send, isConnected: true }
  }),
  useWSStore: { setState: vi.fn() },
}))

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    get: vi.fn(),
  },
}))

import { pmTaskService } from '@/lib/services/pmTaskService'
import { useRealtimeSync } from '../useRealtimeSync'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness({ workspaceId }: { workspaceId: string }) {
  useRealtimeSync(workspaceId)
  return null
}

describe('useRealtimeSync task ordering events', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    captured.send = vi.fn()
    captured.onPresenceSnapshot = null
    useAuthStore.setState({ user: { id: 'user-1' } as never })
    usePMBoardStore.setState({
      refreshBoard: vi.fn() as never,
      patchTask: vi.fn() as never,
    })
    useSupportPresenceStore.setState({
      typingIndicators: {},
      agentTyping: {},
      viewingAgents: {},
      onlineVisitors: {},
      wsSend: null,
      wsConnected: false,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    captured.onEvent = null
  })

  it('refreshes the board for moved task events instead of hydrating a single task', async () => {
    vi.mocked(pmTaskService.get).mockResolvedValue({
      data: {
        task: {
          id: 'task-1',
          workflow_state_id: 'state-done',
          updated_at: '2026-03-24T10:00:00Z',
        },
      },
      error: null,
      status: 200,
    } as never)
    const refreshBoard = vi.fn()
    const patchTask = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchTask })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'moved',
        entity: 'task',
        entity_id: 'task-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      })
      vi.advanceTimersByTime(250)
      await Promise.resolve()
    })

    expect(pmTaskService.get).not.toHaveBeenCalled()
    expect(patchTask).not.toHaveBeenCalled()
    expect(refreshBoard).toHaveBeenCalledTimes(1)

    act(() => root.unmount())
    container.remove()
  })

  it('refreshes the board for reordered task events instead of hydrating a single task', async () => {
    vi.mocked(pmTaskService.get).mockResolvedValue({
      data: {
        task: {
          id: 'task-1',
          workflow_state_id: 'state-todo',
          updated_at: '2026-03-24T10:00:00Z',
        },
      },
      error: null,
      status: 200,
    } as never)
    const refreshBoard = vi.fn()
    const patchTask = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchTask })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'reordered',
        entity: 'task',
        entity_id: 'task-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      } as never)
      vi.advanceTimersByTime(250)
      await Promise.resolve()
    })

    expect(pmTaskService.get).not.toHaveBeenCalled()
    expect(patchTask).not.toHaveBeenCalled()
    expect(refreshBoard).toHaveBeenCalledTimes(1)

    act(() => root.unmount())
    container.remove()
  })

  it('still hydrates and patches plain task updates', async () => {
    vi.mocked(pmTaskService.get).mockResolvedValue({
      data: {
        task: {
          id: 'task-1',
          workflow_state_id: 'state-todo',
          updated_at: '2026-03-24T10:00:00Z',
        },
        owner_member: {
          display_name: 'Ada',
          email: 'ada@example.com',
        },
      },
      error: null,
      status: 200,
    } as never)
    const refreshBoard = vi.fn()
    const patchTask = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchTask })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated',
        entity: 'task',
        entity_id: 'task-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      })
      await Promise.resolve()
    })

    expect(pmTaskService.get).toHaveBeenCalledWith('ws-1', 'task-1')
    expect(patchTask).toHaveBeenCalledWith(
      'updated',
      'task-1',
      expect.objectContaining({
        id: 'task-1',
        workflow_state_id: 'state-todo',
      }),
    )
    expect(refreshBoard).not.toHaveBeenCalled()

    act(() => root.unmount())
    container.remove()
  })

  it('invalidates git links and dispatches a task child event for task_git_link updates', async () => {
    const childUpdated = vi.fn()
    window.addEventListener('task-child-updated', childUpdated)

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated',
        entity: 'task_git_link',
        entity_id: 'link-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
        parent_type: 'task',
        parent_id: 'task-1',
      })
      await Promise.resolve()
    })

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.git.taskLinks('ws-1', 'task-1') })
    expect(childUpdated).toHaveBeenCalledTimes(1)
    expect((childUpdated.mock.calls[0]?.[0] as CustomEvent).detail).toEqual(expect.objectContaining({
      entity: 'task_git_link',
      parent_type: 'task',
      parent_id: 'task-1',
    }))

    window.removeEventListener('task-child-updated', childUpdated)
    act(() => root.unmount())
    container.remove()
  })

  it('patches board latest run fields and emits agent_run compatibility events', async () => {
    const refreshBoard = vi.fn()
    const patchTask = vi.fn(() => true)
    usePMBoardStore.setState({
      refreshBoard,
      patchTask,
      columns: [
        {
          state: { id: 'state-todo', state_type: 'backlog' },
          tasks: [
            {
              id: 'task-1',
              latest_run_id: 'old-run',
              latest_run_agent_id: 'agent-1',
              latest_run_status: 'completed',
              latest_run_at: '2026-04-23T08:00:00Z',
            },
          ],
          task_groups: [
            {
              key: 'group-1',
              label: 'Group 1',
              tasks: [
                {
                  id: 'task-1',
                  latest_run_id: 'old-run',
                  latest_run_agent_id: 'agent-1',
                  latest_run_status: 'completed',
                  latest_run_at: '2026-04-23T08:00:00Z',
                },
              ],
            },
          ],
          task_count: 1,
          point_total: 0,
          has_more: false,
        },
      ] as never,
      memberColumns: [
        {
          member: null,
          tasks: [
            {
              id: 'task-1',
              latest_run_id: 'old-run',
              latest_run_agent_id: 'agent-1',
              latest_run_status: 'completed',
              latest_run_at: '2026-04-23T08:00:00Z',
            },
          ],
          task_count: 1,
          point_total: 0,
          has_more: false,
        },
      ] as never,
    })

    const updated = vi.fn()
    const created = vi.fn()
    window.addEventListener('agent_run-updated', updated)
    window.addEventListener('agent_run-created', created)

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated',
        entity: 'agent_run',
        entity_id: 'run-2',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
        parent_type: 'task',
        parent_id: 'task-1',
        sent_at: '2026-04-23T09:00:00Z',
        data: {
          agent_id: 'agent-2',
          status: 'running',
          pause_reason: '',
        },
      })
      await Promise.resolve()
    })

    expect(updated).toHaveBeenCalledTimes(1)
    expect(created).toHaveBeenCalledTimes(1)
    expect((updated.mock.calls[0]?.[0] as CustomEvent).detail).toEqual(expect.objectContaining({
      entity_id: 'run-2',
      parent_type: 'task',
      parent_id: 'task-1',
      agent_id: 'agent-2',
      status: 'running',
      pause_reason: '',
    }))

    const state = usePMBoardStore.getState()
    expect(state.columns[0]?.tasks[0]).toEqual(expect.objectContaining({
      latest_run_id: 'run-2',
      latest_run_agent_id: 'agent-2',
      latest_run_status: 'running',
      latest_run_pause_reason: null,
      latest_run_at: '2026-04-23T09:00:00Z',
    }))
    expect(state.columns[0]?.task_groups?.[0]?.tasks[0]).toEqual(expect.objectContaining({
      latest_run_id: 'run-2',
      latest_run_status: 'running',
    }))
    expect(state.memberColumns[0]?.tasks[0]).toEqual(expect.objectContaining({
      latest_run_id: 'run-2',
      latest_run_status: 'running',
    }))

    window.removeEventListener('agent_run-updated', updated)
    window.removeEventListener('agent_run-created', created)
    act(() => root.unmount())
    container.remove()
  })

  it('dispatches coding_session websocket updates to local session listeners', async () => {
    const listener = vi.fn()
    window.addEventListener('coding_session-updated', listener)

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated',
        entity: 'coding_session',
        entity_id: 'run-2',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
        data: {
          parent_run_id: 'parent-run',
          status: 'paused',
          pause_reason: 'human_input',
        },
      })
      await Promise.resolve()
    })

    expect(listener).toHaveBeenCalledTimes(1)
    expect((listener.mock.calls[0]?.[0] as CustomEvent).detail).toEqual(expect.objectContaining({
      entity_id: 'run-2',
      data: expect.objectContaining({
        parent_run_id: 'parent-run',
        status: 'paused',
        pause_reason: 'human_input',
      }),
    }))

    window.removeEventListener('coding_session-updated', listener)
    act(() => root.unmount())
    container.remove()
  })

  it('replaces support presence state from authoritative snapshots', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    useSupportPresenceStore.setState({
      typingIndicators: {},
      agentTyping: {
        'conv-1': {
          'user-stale': { content: 'old draft', name: 'Stale Agent' },
        },
      },
      viewingAgents: {
        'conv-1': ['user-stale'],
      },
      onlineVisitors: {},
      wsSend: null,
      wsConnected: false,
    })

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onPresenceSnapshot?.({
        conversation_id: 'conv-1',
        viewers: [{ user_id: 'user-2', name: 'Bob Agent', avatar: 'https://example.com/bob.png' }],
        typers: {
          'user-3': {
            content: 'fresh draft',
            name: 'Cara Agent',
            avatar: 'https://example.com/cara.png',
          },
        },
      })
      await Promise.resolve()
    })

    const state = useSupportPresenceStore.getState()
    expect(state.viewingAgents['conv-1']).toEqual(['user-2'])
    expect(state.agentTyping['conv-1']).toEqual({
      'user-3': {
        content: 'fresh draft',
        name: 'Cara Agent',
        avatarUrl: 'https://example.com/cara.png',
      },
    })

    act(() => root.unmount())
    container.remove()
  })

  it('invalidates custom support view counts for conversation and message updates', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness workspaceId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated',
        entity: 'support_conversation',
        entity_id: 'conv-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      })
      captured.onEvent?.({
        action: 'created',
        entity: 'support_conversation_message',
        entity_id: 'msg-1',
        workspace_id: 'ws-1',
        actor_id: 'widget:visitor',
        parent_id: 'conv-1',
      })
      await Promise.resolve()
    })

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.support.inboxViewCounts('ws-1') })

    act(() => root.unmount())
    container.remove()
  })
})
