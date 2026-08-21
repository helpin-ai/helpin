import React from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { flattenSupportMessagePages, type SupportMessagePages } from '../support-pages'
import { supportQueryKeys } from '../support-query-keys'
import { configureSupportApi, supportService } from '../support-service'
import type {
  CreateConversationWithMessageRequest,
  SupportConversation,
  SupportMessage,
} from '../support-types'
import { useCreateConversationWithMessage } from '../use-support-core'

const workspaceId = 'ws-1'
const payload: CreateConversationWithMessageRequest = {
  subject: 'Welcome',
  content: 'Hello there',
  customer_email: 'person@example.com',
  channels: ['email'],
  mailbox_id: null,
  tag_ids: ['tag-1'],
  cc_emails: ['copy@example.com'],
  bcc_emails: [],
}
const conversation: SupportConversation = {
  id: 'conv-new',
  workspace_id: workspaceId,
  display_id: 42,
  subject: payload.subject,
  status: 'open',
  priority: 'medium',
  source: 'internal',
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}
const message: SupportMessage = {
  id: 'msg-new',
  workspace_id: workspaceId,
  conversation_id: conversation.id,
  sender_type: 'user',
  content: payload.content,
  is_internal: false,
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}

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

describe('outbound conversation creation', () => {
  const api = makeApi()

  beforeEach(() => {
    vi.clearAllMocks()
    configureSupportApi(api)
  })

  test('posts the complete web contract to create-and-send', async () => {
    api.post.mockResolvedValue({ data: { conversation, message }, error: null })

    await supportService.createConversationWithMessage(workspaceId, payload)

    expect(api.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/create-and-send?workspace_id=${workspaceId}`,
      payload,
    )
  })

  test('seeds the created thread and refreshes inbox queries', async () => {
    api.post.mockResolvedValue({ data: { conversation, message }, error: null })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useCreateConversationWithMessage(workspaceId), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync(payload)
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(queryClient.getQueryData(
      supportQueryKeys.conversation(workspaceId, conversation.id),
    )).toEqual(conversation)
    expect(flattenSupportMessagePages(
      queryClient.getQueryData<SupportMessagePages>(
        supportQueryKeys.messages(workspaceId, conversation.id),
      ),
    )).toEqual([message])
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: supportQueryKeys.conversations(workspaceId),
    })
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: supportQueryKeys.inboxViewCounts(workspaceId),
    })
  })
})
