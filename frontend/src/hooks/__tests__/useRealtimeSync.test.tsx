// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useAuthStore } from '@/stores/authStore'

const captured = {
  onEvent: null as ((event: unknown) => void) | null,
}

vi.mock('../useWebSocket', () => ({
  useWebSocket: vi.fn(({ onEvent }) => {
    captured.onEvent = onEvent
    return { send: vi.fn(), isConnected: true }
  }),
  useWSStore: { setState: vi.fn() },
}))

vi.mock('@/lib/services/pmStoryService', () => ({
  pmStoryService: {
    get: vi.fn(),
  },
}))

import { pmStoryService } from '@/lib/services/pmStoryService'
import { useRealtimeSync } from '../useRealtimeSync'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness({ workspaceId }: { workspaceId: string }) {
  useRealtimeSync(workspaceId)
  return null
}

describe('useRealtimeSync story ordering events', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    useAuthStore.setState({ user: { id: 'user-1' } as never })
    usePMBoardStore.setState({
      refreshBoard: vi.fn() as never,
      patchStory: vi.fn() as never,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    captured.onEvent = null
  })

  it('refreshes the board for moved story events instead of hydrating a single story', async () => {
    vi.mocked(pmStoryService.get).mockResolvedValue({
      data: {
        story: {
          id: 'story-1',
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
        entity: 'story',
        entity_id: 'story-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      })
      vi.advanceTimersByTime(250)
      await Promise.resolve()
    })

    expect(pmStoryService.get).not.toHaveBeenCalled()
    expect(patchStory).not.toHaveBeenCalled()
    expect(refreshBoard).toHaveBeenCalledTimes(1)

    act(() => root.unmount())
    container.remove()
  })

  it('refreshes the board for reordered story events instead of hydrating a single story', async () => {
    vi.mocked(pmStoryService.get).mockResolvedValue({
      data: {
        story: {
          id: 'story-1',
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
        entity: 'story',
        entity_id: 'story-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      } as never)
      vi.advanceTimersByTime(250)
      await Promise.resolve()
    })

    expect(pmStoryService.get).not.toHaveBeenCalled()
    expect(patchStory).not.toHaveBeenCalled()
    expect(refreshBoard).toHaveBeenCalledTimes(1)

    act(() => root.unmount())
    container.remove()
  })

  it('still hydrates and patches plain story updates', async () => {
    vi.mocked(pmStoryService.get).mockResolvedValue({
      data: {
        story: {
          id: 'story-1',
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
        entity: 'story',
        entity_id: 'story-1',
        workspace_id: 'ws-1',
        actor_id: 'user-2',
      })
      await Promise.resolve()
    })

    expect(pmStoryService.get).toHaveBeenCalledWith('ws-1', 'story-1')
    expect(patchStory).toHaveBeenCalledWith(
      'updated',
      'story-1',
      expect.objectContaining({
        id: 'story-1',
        workflow_state_id: 'state-todo',
      }),
    )
    expect(refreshBoard).not.toHaveBeenCalled()

    act(() => root.unmount())
    container.remove()
  })
})
