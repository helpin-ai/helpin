// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'

vi.mock('@/lib/services/supportService', () => ({
  supportService: {
    markConversationRead: vi.fn(),
    markConversationUnread: vi.fn(),
    updateConversationStatus: vi.fn(),
  },
}))

import {
  useMarkConversationRead,
  useMarkConversationUnread,
  useUpdateConversationStatus,
} from '@/hooks/queries/useSupport'
import { supportService } from '@/lib/services/supportService'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const captured = {
  read: null as ReturnType<typeof useMarkConversationRead> | null,
  unread: null as ReturnType<typeof useMarkConversationUnread> | null,
  status: null as ReturnType<typeof useUpdateConversationStatus> | null,
}

function Harness({ onReady }: { onReady: (hooks: typeof captured) => void }) {
  const read = useMarkConversationRead('ws-1')
  const unread = useMarkConversationUnread('ws-1')
  const status = useUpdateConversationStatus('ws-1')
  useEffect(() => {
    onReady({ read, unread, status })
  }, [onReady, read, status, unread])
  return null
}

describe('support workspace unread invalidation', () => {
  afterEach(() => {
    captured.read = null
    captured.unread = null
    captured.status = null
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('refreshes workspace unread after marking a conversation read', async () => {
    vi.mocked(supportService.markConversationRead).mockResolvedValue({
      data: {
        workspace_id: 'ws-1',
        conversation_id: 'conv-1',
        user_id: 'user-1',
        unread_customer_message_count: 0,
        manually_unread: false,
        relevance_mask: 0,
        version: 2,
      },
      error: null,
      status: 200,
    } as never)
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness onReady={(hooks) => Object.assign(captured, hooks)} />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.read?.mutateAsync({ conversationId: 'conv-1' })
    })

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: queryKeys.support.workspaceUnread(),
    })
    act(() => root.unmount())
  })

  it('refreshes workspace unread after marking a conversation unread', async () => {
    vi.mocked(supportService.markConversationUnread).mockResolvedValue({
      data: {
        workspace_id: 'ws-1',
        conversation_id: 'conv-1',
        user_id: 'user-1',
        unread_customer_message_count: 0,
        manually_unread: true,
        relevance_mask: 0,
        version: 2,
      },
      error: null,
      status: 200,
    } as never)
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness onReady={(hooks) => Object.assign(captured, hooks)} />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.unread?.mutateAsync('conv-1')
    })

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: queryKeys.support.workspaceUnread(),
    })
    act(() => root.unmount())
  })

  it('refreshes workspace unread after a conversation status changes', async () => {
    vi.mocked(supportService.updateConversationStatus).mockResolvedValue({
      data: {
        id: 'conv-1',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Reopened',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        created_at: '2026-09-03T10:00:00Z',
        updated_at: '2026-09-03T10:01:00Z',
      },
      error: null,
      status: 200,
    } as never)
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness onReady={(hooks) => Object.assign(captured, hooks)} />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.status?.mutateAsync({ conversationId: 'conv-1', status: 'open' })
    })

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: queryKeys.support.workspaceUnread(),
    })
    act(() => root.unmount())
  })
})
