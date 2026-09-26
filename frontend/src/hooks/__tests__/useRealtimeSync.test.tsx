// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useAuthStore } from '@/stores/authStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { queryKeys } from '@/lib/queryKeys'
import type { ConversationListResponse, SupportMessage } from '@/lib/pmTypes'
import { flattenSupportMessagePages, seedSupportMessagePages, type SupportMessagePages } from '@/lib/supportMessagePages'

const captured = {
  onEvent: null as ((event: unknown) => void) | null,
  onPresenceSnapshot: null as ((snapshot: unknown) => void) | null,
  connected: true,
  send: vi.fn(),
}

vi.mock('../useWebSocket', () => ({
  useWebSocket: vi.fn(({ onEvent, onPresenceSnapshot }) => {
    captured.onEvent = onEvent
    captured.onPresenceSnapshot = onPresenceSnapshot
    return { send: captured.send, isConnected: captured.connected }
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
import { toast } from 'sonner'

vi.mock('sonner', () => ({ toast: vi.fn() }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness({ workspaceId }: { workspaceId: string }) {
  useRealtimeSync(workspaceId)
  return null
}

describe('useRealtimeSync task ordering events', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    captured.connected = true
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
    useSupportInboxStore.setState({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      selectedConversationId: null,
      activePanel: 'list',
      statusFilter: 'all',
      searchQuery: '',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
        mailboxIds: [],
        tagIds: [],
        aiStates: ['handoff'],
        sort: 'newest',
      },
      activeCustomViewId: null,
      customViewDirty: false,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    captured.onEvent = null
  })

  it('opens Ask Agent from attention toasts and ignores other recipients and resolution updates', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
    const onOpen = vi.fn()
    window.addEventListener('helpin:ask-agents', onOpen)
    const event = { entity: 'notification', action: 'created', workspace_id: 'ws-1', data: { event_type: 'task.agent_attention_required', recipient_id: 'user-1', dock_chat_id: 'chat-1', run_id: 'run-1' } }
    act(() => captured.onEvent?.(event))
    expect(toast).toHaveBeenCalledTimes(1)
    const options = vi.mocked(toast).mock.calls[0][1]
    expect(options?.action).toMatchObject({ label: 'Open chat' })
    const action = options?.action as { onClick: () => void }
    action.onClick()
    expect(onOpen.mock.calls[0][0].detail).toEqual({ chatId: 'chat-1' })
    act(() => captured.onEvent?.({ ...event, action: 'updated' }))
    expect(toast).toHaveBeenCalledTimes(2) // A new interaction on the same run updates the inbox row.
    act(() => captured.onEvent?.({ ...event, data: { ...event.data, recipient_id: 'other' } }))
    act(() => captured.onEvent?.({ entity: 'notification', action: 'updated', workspace_id: 'ws-1' }))
    expect(toast).toHaveBeenCalledTimes(2)
    window.removeEventListener('helpin:ask-agents', onOpen)
    act(() => root.unmount())
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
    expect(created).not.toHaveBeenCalled()
    expect((updated.mock.calls[0]?.[0] as CustomEvent).detail).toEqual(expect.objectContaining({
      entity_id: 'run-2',
      parent_type: 'task',
      parent_id: 'task-1',
      agent_id: 'agent-2',
      status: 'running',
      pause_reason: '',
      update_kind: 'progress',
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

  it('patches a running transition once without invalidating broad queries on repeated progress', async () => {
    usePMBoardStore.setState({
      columns: [{
        state: { id: 'state-todo', state_type: 'backlog' },
        tasks: [{
          id: 'task-1',
          latest_run_id: 'run-1',
          latest_run_agent_id: 'agent-1',
          latest_run_status: 'queued',
          latest_run_pause_reason: null,
          latest_run_at: '2026-08-04T08:00:00Z',
        }],
        task_count: 1,
        point_total: 0,
        has_more: false,
      }] as never,
      memberColumns: [],
    })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData(queryKeys.automation.runs('ws-1', 1, 100), {
      data: [{ id: 'run-1', agent_id: 'agent-1', status: 'queued', pause_reason: 'none' }],
      total: 1,
    })
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

    const emitProgress = (sentAt: string) => captured.onEvent?.({
      action: 'updated',
      entity: 'agent_run',
      entity_id: 'run-1',
      workspace_id: 'ws-1',
      parent_type: 'task',
      parent_id: 'task-1',
      sent_at: sentAt,
      data: { agent_id: 'agent-1', status: 'running', pause_reason: 'none' },
    })

    await act(async () => {
      emitProgress('2026-08-04T09:00:00Z')
      await Promise.resolve()
    })
    const firstTask = usePMBoardStore.getState().columns[0]?.tasks[0]
    expect(firstTask?.latest_run_at).toBe('2026-08-04T09:00:00Z')
    expect(client.getQueryData(queryKeys.automation.runs('ws-1', 1, 100))).toEqual(expect.objectContaining({
      data: [expect.objectContaining({ status: 'running' })],
    }))

    await act(async () => {
      emitProgress('2026-08-04T09:01:00Z')
      vi.advanceTimersByTime(500)
      await Promise.resolve()
    })
    const secondTask = usePMBoardStore.getState().columns[0]?.tasks[0]
    expect(secondTask).toBe(firstTask)
    expect(secondTask?.latest_run_at).toBe('2026-08-04T09:00:00Z')
    expect(invalidateQueries.mock.calls.map(([options]) => options.queryKey)).toEqual([queryKeys.automation.runAttentionCount('ws-1')])

    act(() => root.unmount())
    container.remove()
  })

  it.each([['agent_run', 'coding_session'], ['coding_session', 'agent_run']])('keeps repeated typed states quiet with aliases %s then %s', async (first, second) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidations = vi.spyOn(client, 'invalidateQueries')
    const listener = vi.fn()
    window.addEventListener('agent_run-updated', listener)
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
    const data = { status: 'paused', pause_reason: 'awaiting_user_message', change_kind: 'state' }
    const emit = async (change = {}) => act(async () => {
      for (const entity of [first, second]) captured.onEvent?.({ entity, action: 'updated', entity_id: 'run-1', workspace_id: 'ws-1', data: { ...data, ...change } })
      vi.advanceTimersByTime(500)
    })
    await emit()
    expect(listener).toHaveBeenCalledTimes(1)
    expect(listener.mock.calls[0]?.[0].detail.update_kind).toBe('lifecycle')
    invalidations.mockClear()
    await emit()
    expect(listener).toHaveBeenCalledTimes(1)
    await emit({ change_kind: 'message' })
    expect(invalidations).not.toHaveBeenCalled()
    expect(listener.mock.calls.at(-1)?.[0].detail.update_kind).toBe('content')
    await emit({ pause_reason: 'human_approval' })
    expect(invalidations).toHaveBeenCalledWith(expect.objectContaining({ queryKey: queryKeys.automation.runAttentionCount('ws-1') }))
    invalidations.mockClear()
    await emit({ status: 'running', pause_reason: 'none' })
    expect(invalidations).toHaveBeenCalledWith(expect.objectContaining({ queryKey: queryKeys.automation.runAttentionCount('ws-1') }))
    act(() => root.unmount())
    window.removeEventListener('agent_run-updated', listener)
  })

  it('does not retain cancelled debounce handles when switching workspace', async () => {
    const client = new QueryClient()
    const invalidations = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    const root = createRoot(container)
    const render = (id: string) => act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId={id} /></QueryClientProvider>))
    const emit = (id: string) => captured.onEvent?.({ entity: 'agent_run', action: 'updated', entity_id: 'run-1', workspace_id: id, data: { change_kind: 'state', status: 'paused', pause_reason: 'human_approval' } })
    render('ws-1')
    act(() => emit('ws-1'))
    render('ws-2')
    await act(async () => { emit('ws-2'); vi.advanceTimersByTime(500) })
    expect(invalidations).toHaveBeenCalledWith(expect.objectContaining({ queryKey: queryKeys.automation.runAttentionCount('ws-2') }))
    expect(invalidations).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: queryKeys.automation.runAttentionCount('ws-1') }))
    act(() => root.unmount())
  })

  it.each([false, true])('recovers queries on websocket reconnect with hidden=%s', (hidden) => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue(hidden ? 'hidden' : 'visible')
    const client = new QueryClient()
    const invalidations = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    const root = createRoot(container)
    const render = () => act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
    render()
    invalidations.mockClear()
    captured.connected = false
    render()
    captured.connected = true
    render()
    for (const key of [queryKeys.automation.runAttentionCount('ws-1'), queryKeys.support.workspaceUnread(), queryKeys.support.teammatePresence('ws-1'), queryKeys.workspaces.memberPresence('ws-1')]) {
      expect(invalidations).toHaveBeenCalledWith(expect.objectContaining({ queryKey: key, refetchType: hidden ? 'none' : 'active' }))
    }
    act(() => root.unmount())
    visibility.mockRestore()
  })

  it('coalesces lifecycle invalidations and refreshes the standalone attention count', async () => {
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

    const pausedEvent = {
      action: 'updated',
      entity: 'agent_run',
      entity_id: 'run-1',
      workspace_id: 'ws-1',
      parent_type: 'task',
      parent_id: 'task-1',
      data: { agent_id: 'agent-1', status: 'paused', pause_reason: 'human_input' },
    }
    await act(async () => {
      captured.onEvent?.(pausedEvent)
      captured.onEvent?.(pausedEvent)
      vi.advanceTimersByTime(450)
      await Promise.resolve()
    })

    expect(invalidateQueries.mock.calls.filter(([options]) =>
      JSON.stringify(options.queryKey) === JSON.stringify(queryKeys.automation.runsRoot('ws-1')),
    )).toHaveLength(1)
    expect(invalidateQueries.mock.calls.filter(([options]) =>
      JSON.stringify(options.queryKey) === JSON.stringify(queryKeys.automation.runAttentionCount('ws-1')),
    )).toHaveLength(1)

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

  it('patches a targeted personal read and refreshes personal counters without refetching lists', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    client.setQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'), {
      data: [{
        id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: 'Unread', status: 'open',
        priority: 'medium', source: 'widget', unread_count: 2, personal_state_version: 3,
        created_at: '2026-06-04T08:00:00Z', updated_at: '2026-06-04T08:00:00Z',
      }],
      total: 1, page: 1, per_page: 50, total_pages: 1,
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>)
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'updated', entity: 'support_personal_read', entity_id: 'conv-1',
        workspace_id: 'ws-1', actor_id: 'user-1', target_user_id: 'user-1',
        data: { unread_count: 0, personal_state_version: 4 },
      })
      await Promise.resolve()
    })

    const updated = client.getQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'))
    expect(updated?.data[0]).toEqual(expect.objectContaining({ unread_count: 0, personal_state_version: 4 }))
    expect(invalidateQueries).not.toHaveBeenCalledWith({ queryKey: queryKeys.support.conversations('ws-1') })
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.support.unreadStats('ws-1') })
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.support.inboxScopes('ws-1') })
    expect(invalidateQueries).not.toHaveBeenCalledWith({ queryKey: queryKeys.support.inboxViewCounts('ws-1') })
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.support.workspaceUnread() })

    act(() => root.unmount())
    container.remove()
  })

  it('moves support conversation rows for message activity without marking agent replies unread', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'), {
      data: [
        {
          id: 'conv-old',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Older',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 0,
          created_at: '2026-06-04T08:00:00Z',
          updated_at: '2026-06-04T08:00:00Z',
        },
        {
          id: 'conv-replied',
          workspace_id: 'ws-1',
          display_id: 2,
          subject: 'Replied',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 2,
          awaiting_reply: true,
          last_message: 'Customer question',
          created_at: '2026-06-04T08:00:00Z',
          updated_at: '2026-06-04T08:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      per_page: 50,
      total_pages: 1,
    })
    const existingMessage: SupportMessage = {
      id: 'msg-0',
      workspace_id: 'ws-1',
      conversation_id: 'conv-replied',
      sender_type: 'customer',
      content: 'Customer question',
      is_internal: false,
      created_at: '2026-06-04T08:00:00Z',
      updated_at: '2026-06-04T08:00:00Z',
    }
    client.setQueryData(
      queryKeys.support.messages('ws-1', 'conv-replied'),
      seedSupportMessagePages([existingMessage]),
    )

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
        action: 'created',
        entity: 'support_conversation_message',
        entity_id: 'msg-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
        parent_id: 'conv-replied',
        sent_at: '2026-06-04T09:05:00Z',
        data: {
          content: 'We will check this.',
          created_at: '2026-06-04T09:00:00Z',
          sender_type: 'user',
          message_type: 'reply',
        },
      })
      await Promise.resolve()
    })

    const updated = client.getQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'))
    expect(updated?.data.map((conversation) => conversation.id)).toEqual(['conv-replied', 'conv-old'])
    expect(updated?.data[0]).toEqual(expect.objectContaining({
      unread_count: 2,
      awaiting_reply: false,
      last_message: 'We will check this.',
      updated_at: '2026-06-04T09:00:00Z',
      list_last_activity_at: '2026-06-04T09:00:00Z',
      list_last_message_at: '2026-06-04T09:00:00Z',
    }))
    const messagePages = client.getQueryData<SupportMessagePages>(queryKeys.support.messages('ws-1', 'conv-replied'))
    expect(flattenSupportMessagePages(messagePages).map((message) => message.id)).toEqual(['msg-0', 'msg-1'])

    act(() => root.unmount())
    container.remove()
  })

  it.each([
    { sender_type: 'user', sender_name: 'Teammate', sender_avatar: '/teammate.png', expectedID: 'user-2' },
    { sender_type: 'user', sender_name: 'Legacy name', sender_avatar: '/legacy.png', sender_display_name: 'Teammate', sender_avatar_url: '/teammate.png', sender_user_id: 'actual-author', expectedID: 'actual-author' },
    { sender_type: 'customer', sender_name: 'Teammate', sender_avatar: '/teammate.png', expectedID: undefined },
  ])('normalizes realtime sender identity before caching ($sender_type/$expectedID)', async ({ expectedID, ...data }) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const key = queryKeys.support.messages('ws-1', 'thread')
    client.setQueryData(key, seedSupportMessagePages([]))
    const container = document.createElement('div')
    const root = createRoot(container)
    try {
      act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
      await act(async () => {
        captured.onEvent?.({ action: 'created', entity: 'support_conversation_message', entity_id: 'reply', workspace_id: 'ws-1', actor_id: 'user-2', parent_id: 'thread', data: { ...data, content: 'Hello', message_type: 'reply' } })
      })
      const [message] = flattenSupportMessagePages(client.getQueryData<SupportMessagePages>(key))
      expect(message.sender_display_name).toBe('Teammate')
      expect(message.sender_avatar_url).toBe('/teammate.png')
      expect(message.sender_user_id).toBe(expectedID)
    } finally {
      act(() => root.unmount())
      client.clear()
    }
  })

  it('invalidates filtered Waiting lists so new teammate replies can enter an empty view', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const waitingKey = ['support', 'ws-1', 'conversations', 'infinite', { filter: 'waiting' }]
    client.setQueryData(waitingKey, { pages: [{ data: [], total: 0 }], pageParams: [1] })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
    await act(async () => {
      captured.onEvent?.({ action: 'created', entity: 'support_conversation_message', entity_id: 'reply', workspace_id: 'ws-1', actor_id: 'user-1', parent_id: 'thread', data: { content: 'Team reply', sender_type: 'user', message_type: 'reply', created_at: '2026-09-11T09:00:00Z' } })
      await Promise.resolve()
    })
    expect(client.getQueryState(waitingKey)?.isInvalidated).toBe(true)
    act(() => root.unmount())
    container.remove()
  })

  it('refreshes workspace unread after a customer message', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>)
    })

    await act(async () => {
      captured.onEvent?.({
        action: 'created',
        entity: 'support_conversation_message',
        entity_id: 'msg-customer',
        workspace_id: 'ws-1',
        actor_id: 'widget:visitor',
        parent_id: 'conv-1',
        data: {
          content: 'I still need help.',
          sender_type: 'customer',
          message_type: 'reply',
          is_internal: false,
        },
      })
      vi.advanceTimersByTime(250)
      await Promise.resolve()
    })

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: queryKeys.support.workspaceUnread() })

    act(() => root.unmount())
    container.remove()
  })

  it('patches selected reopened conversations into the active inbox without marking unread', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'), {
      data: [
        {
          id: 'conv-reopen',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Reopen me',
          status: 'resolved',
          flow_state: 'resolved_by_human',
          priority: 'medium',
          source: 'widget',
          mailbox_id: 'mailbox-billing',
          unread_count: 0,
          created_at: '2026-06-04T08:00:00Z',
          updated_at: '2026-06-04T08:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
    })
    useSupportInboxStore.setState({
      navFilter: 'resolved',
      selectedMailboxId: 'mailbox-billing',
      selectedConversationId: 'conv-reopen',
      activePanel: 'thread',
    })

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
        entity_id: 'conv-reopen',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
        data: {
          old_status: 'resolved',
          status: 'open',
          flow_state: 'assigned_to_human',
          updated_at: '2026-06-04T09:00:00Z',
          mailbox_id: 'mailbox-billing',
        },
      })
      await Promise.resolve()
    })

    const updated = client.getQueryData<ConversationListResponse>(queryKeys.support.conversations('ws-1'))
    expect(updated?.data[0]).toEqual(expect.objectContaining({
      status: 'open',
      flow_state: 'assigned_to_human',
      unread_count: 0,
      updated_at: '2026-06-04T09:00:00Z',
    }))
    expect(useSupportInboxStore.getState()).toEqual(expect.objectContaining({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      selectedConversationId: 'conv-reopen',
      activePanel: 'thread',
    }))

    act(() => root.unmount())
    container.remove()
  })

  it('refreshes cached message identities when a customer is anonymized', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const messageKey = queryKeys.support.messages('ws-1', 'conv-deleted')
    const visitorKey = queryKeys.support.visitorContext('ws-1', 'conv-deleted')
    client.setQueryData(messageKey, { messages: [] })
    client.setQueryData(visitorKey, { name: 'Old name' })
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>))
    await act(async () => {
      captured.onEvent?.({ action: 'updated', entity: 'support_conversation', entity_id: 'conv-deleted', workspace_id: 'ws-1', data: { reason: 'customer_anonymized' } })
      await Promise.resolve()
    })
    expect(client.getQueryState(messageKey)?.isInvalidated).toBe(true)
    expect(client.getQueryState(visitorKey)?.isInvalidated).toBe(true)
    act(() => root.unmount())
    client.clear()
  })

  it('refreshes live company visitor context and conversation associations', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const visitorKey = queryKeys.support.visitorContext('ws-1', 'conv-company')
    const associationKey = queryKeys.support.conversationAssociations('ws-1', 'conv-company')
    client.setQueryData(visitorKey, { company_context_status: 'ok' })
    client.setQueryData(associationKey, { crm_records: [] })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(<QueryClientProvider client={client}><Harness workspaceId="ws-1" /></QueryClientProvider>)
    })

    await act(async () => {
      captured.onEvent?.({ action: 'updated', entity: 'support_conversation', entity_id: 'conv-company', workspace_id: 'ws-1' })
      await Promise.resolve()
    })
    expect(client.getQueryState(visitorKey)?.isInvalidated).toBe(true)
    expect(client.getQueryState(associationKey)?.isInvalidated).toBe(true)

    client.setQueryData(visitorKey, { company_context_status: 'ok' })
    await act(async () => {
      captured.onEvent?.({ action: 'updated', entity: 'crm_company', entity_id: 'company-1', workspace_id: 'ws-1' })
      await Promise.resolve()
    })
    expect(client.getQueryState(visitorKey)?.isInvalidated).toBe(true)

    act(() => root.unmount())
    container.remove()
  })
})
