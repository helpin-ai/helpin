import React from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { flattenSupportMessagePages, seedSupportMessagePages, type SupportMessagePages } from '../support-pages'
import { supportQueryKeys } from '../support-query-keys'
import { configureSupportApi, supportService } from '../support-service'
import type { SupportMessage } from '../support-types'
import { useDeleteSupportMessage, useMessageInfo } from '../use-support-core'

const workspaceId = 'ws-1'
const conversationId = 'conv-1'

function makeApi() {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    del: vi.fn(),
  }
}

function wrapper(queryClient: QueryClient) {
  return function TestWrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

function message(id: string): SupportMessage {
  return {
    id,
    workspace_id: workspaceId,
    conversation_id: conversationId,
    sender_type: 'user',
    sender_user_id: 'user-1',
    content: id,
    is_internal: false,
    created_at: '2026-08-20T00:00:00.000Z',
    updated_at: '2026-08-20T00:00:00.000Z',
  }
}

describe('support message action contracts', () => {
  const api = makeApi()

  beforeEach(() => {
    vi.clearAllMocks()
    configureSupportApi(api)
  })

  test('delete and edit-undo use the existing DELETE contract', async () => {
    api.del.mockResolvedValue({ data: { id: 'msg-1', email_already_sent: false }, error: null })

    await supportService.deleteConversationMessage(workspaceId, conversationId, 'msg-1')
    await supportService.deleteConversationMessage(workspaceId, conversationId, 'msg-1', true)

    expect(api.del).toHaveBeenNthCalledWith(
      1,
      `/support/inbox/conversations/${conversationId}/messages/msg-1?workspace_id=${workspaceId}`,
    )
    expect(api.del).toHaveBeenNthCalledWith(
      2,
      `/support/inbox/conversations/${conversationId}/messages/msg-1?workspace_id=${workspaceId}&undo=1`,
    )
  })

  test('message info uses the read-only message endpoint', async () => {
    api.get.mockResolvedValue({
      data: {
        id: 'msg-1',
        sent_at: '2026-08-20T00:00:00.000Z',
        sender: { name: 'Ada', type: 'user' },
        from: 'Ada',
        origin: 'inbox',
        type: 'reply',
        read: false,
        edited: false,
        translated: false,
        automated: false,
      },
      error: null,
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const { result } = renderHook(
      () => useMessageInfo(workspaceId, conversationId, 'msg-1'),
      { wrapper: wrapper(queryClient) },
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.get).toHaveBeenCalledWith(
      `/support/inbox/conversations/${conversationId}/messages/msg-1?workspace_id=${workspaceId}`,
    )
  })

  test('delete removes the message optimistically and restores it on failure', async () => {
    let rejectDelete!: (reason: Error) => void
    api.del.mockReturnValue(new Promise((_resolve, reject) => {
      rejectDelete = reject
    }))
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
    })
    const key = supportQueryKeys.messages(workspaceId, conversationId)
    queryClient.setQueryData(key, seedSupportMessagePages([message('msg-1'), message('msg-2')]))
    const { result } = renderHook(
      () => useDeleteSupportMessage(workspaceId, conversationId),
      { wrapper: wrapper(queryClient) },
    )

    act(() => {
      result.current.mutate({ messageId: 'msg-1' })
    })

    await waitFor(() => {
      const cached = queryClient.getQueryData<SupportMessagePages>(key)
      expect(flattenSupportMessagePages(cached).map((item) => item.id)).toEqual(['msg-2'])
    })

    act(() => rejectDelete(new Error('failed')))
    await waitFor(() => expect(result.current.isError).toBe(true))

    const restored = queryClient.getQueryData<SupportMessagePages>(key)
    expect(flattenSupportMessagePages(restored).map((item) => item.id)).toEqual(['msg-1', 'msg-2'])
  })
})
