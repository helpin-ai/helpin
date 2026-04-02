// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useAuthStore } from '@/stores/authStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'

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
      patchStory: vi.fn() as never,
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
    const patchStory = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchStory })

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
    expect(patchStory).not.toHaveBeenCalled()
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
    const patchStory = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchStory })

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
    expect(patchStory).not.toHaveBeenCalled()
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
    const patchStory = vi.fn(() => true)
    usePMBoardStore.setState({ refreshBoard, patchStory })

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
    expect(patchStory).toHaveBeenCalledWith(
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
})
