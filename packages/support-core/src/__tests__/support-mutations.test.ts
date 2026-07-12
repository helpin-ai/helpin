import React from 'react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import { configureSupportApi, supportService } from '../support-service'
import { supportQueryKeys } from '../support-query-keys'
import {
  useSendMessage,
  useUpdateConversationStatus,
  useAssignConversationUser,
  useConversationAssignees,
} from '../use-support-core'
import type { SupportMessage } from '../support-types'

type FakeApi = {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  del: ReturnType<typeof vi.fn>
}

function makeFakeApi(): FakeApi {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  }
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

const WORKSPACE_ID = 'ws-1'
const CONVERSATION_ID = 'conv-1'

function baseMessage(overrides: Partial<SupportMessage> = {}): SupportMessage {
  return {
    id: 'msg-1',
    workspace_id: WORKSPACE_ID,
    conversation_id: CONVERSATION_ID,
    sender_type: 'user',
    content: 'hello',
    is_internal: false,
    created_at: '2026-07-08T00:00:00.000Z',
    updated_at: '2026-07-08T00:00:00.000Z',
    ...overrides,
  }
}

describe('supportService mutations (unit, no React)', () => {
  let fakeApi: FakeApi

  beforeEach(() => {
    fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
  })

  it('sendMessage posts to the conversation messages path with the verified body shape', async () => {
    fakeApi.post.mockResolvedValue({ data: baseMessage(), error: null })

    await supportService.sendMessage(WORKSPACE_ID, CONVERSATION_ID, { content: 'hi there', is_internal: false })

    expect(fakeApi.post).toHaveBeenCalledTimes(1)
    const [path, body] = fakeApi.post.mock.calls[0]
    expect(path).toBe(`/support/inbox/conversations/${CONVERSATION_ID}/messages?workspace_id=${WORKSPACE_ID}`)
    expect(body).toEqual({ content: 'hi there', is_internal: false })
  })

  it('updateConversationStatus PUTs { status } to the status path', async () => {
    fakeApi.put.mockResolvedValue({ data: null, error: null })

    await supportService.updateConversationStatus(WORKSPACE_ID, CONVERSATION_ID, 'resolved')

    expect(fakeApi.put).toHaveBeenCalledTimes(1)
    const [path, body] = fakeApi.put.mock.calls[0]
    expect(path).toBe(`/support/inbox/conversations/${CONVERSATION_ID}/status?workspace_id=${WORKSPACE_ID}`)
    expect(body).toEqual({ status: 'resolved' })
  })

  it('assignConversationUser posts the verified { user_id } body', async () => {
    fakeApi.post.mockResolvedValue({ data: null, error: null })

    await supportService.assignConversationUser(WORKSPACE_ID, CONVERSATION_ID, { user_id: 'user-42' })

    expect(fakeApi.post).toHaveBeenCalledTimes(1)
    const [path, body] = fakeApi.post.mock.calls[0]
    expect(path).toBe(`/support/inbox/conversations/${CONVERSATION_ID}/assign-user?workspace_id=${WORKSPACE_ID}`)
    expect(body).toEqual({ user_id: 'user-42' })
  })

  it('listConversationAssignees fetches the assignees path', async () => {
    fakeApi.get.mockResolvedValue({ data: [], error: null })

    await supportService.listConversationAssignees(WORKSPACE_ID, CONVERSATION_ID)

    expect(fakeApi.get).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/assignees?workspace_id=${WORKSPACE_ID}`,
    )
  })
})

describe('useSendMessage (optimistic update)', () => {
  let fakeApi: FakeApi
  let queryClient: QueryClient

  beforeEach(() => {
    fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    queryClient.setQueryData(supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID), [] as SupportMessage[])
  })

  it('optimistically appends a pending message then replaces it with the server response', async () => {
    let resolvePost!: (value: { data: SupportMessage; error: null }) => void
    fakeApi.post.mockReturnValue(
      new Promise((resolve) => {
        resolvePost = resolve
      }),
    )

    const { result } = renderHook(() => useSendMessage(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })

    act(() => {
      result.current.mutate({ content: 'optimistic hello', is_internal: false })
    })

    await waitFor(() => {
      const cached = queryClient.getQueryData<SupportMessage[]>(
        supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID),
      )
      expect(cached).toHaveLength(1)
    })

    const pendingCached = queryClient.getQueryData<SupportMessage[]>(
      supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID),
    )!
    expect(pendingCached[0].pending).toBe(true)
    expect(pendingCached[0].sender_type).toBe('user')
    expect(pendingCached[0].content).toBe('optimistic hello')
    expect(pendingCached[0].id.startsWith('pending-')).toBe(true)

    const serverMessage = baseMessage({ id: 'server-msg-1', content: 'optimistic hello' })
    act(() => {
      resolvePost({ data: serverMessage, error: null })
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const finalCached = queryClient.getQueryData<SupportMessage[]>(
      supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID),
    )!
    expect(finalCached).toHaveLength(1)
    expect(finalCached[0].id).toBe('server-msg-1')
    expect(finalCached[0].pending).toBeFalsy()
  })

  it('removes the optimistic entry and surfaces the error on failure', async () => {
    fakeApi.post.mockResolvedValue({ data: null, error: 'boom' })

    const { result } = renderHook(() => useSendMessage(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })

    act(() => {
      result.current.mutate({ content: 'will fail', is_internal: false })
    })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    const finalCached = queryClient.getQueryData<SupportMessage[]>(
      supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID),
    )!
    expect(finalCached).toHaveLength(0)
  })
})

describe('useUpdateConversationStatus', () => {
  it('calls supportService.updateConversationStatus with the given status', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.put.mockResolvedValue({ data: { id: CONVERSATION_ID, status: 'waiting_on_customer' }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })

    const { result } = renderHook(() => useUpdateConversationStatus(WORKSPACE_ID), {
      wrapper: wrapperFor(queryClient),
    })

    act(() => {
      result.current.mutate({ conversationId: CONVERSATION_ID, status: 'waiting_on_customer' })
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(fakeApi.put).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/status?workspace_id=${WORKSPACE_ID}`,
      { status: 'waiting_on_customer' },
    )
  })
})

describe('useAssignConversationUser', () => {
  it('calls supportService.assignConversationUser with the given user_id', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({ data: { assigned: true }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })

    const { result } = renderHook(() => useAssignConversationUser(WORKSPACE_ID), {
      wrapper: wrapperFor(queryClient),
    })

    act(() => {
      result.current.mutate({ conversationId: CONVERSATION_ID, userId: 'user-7' })
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/assign-user?workspace_id=${WORKSPACE_ID}`,
      { user_id: 'user-7' },
    )
  })
})

describe('useConversationAssignees', () => {
  it('fetches assignees only when both workspaceId and conversationId are present', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.get.mockResolvedValue({ data: [{ id: 'member-1', role: 'admin', email: 'a@b.com', display_name: 'A' }], error: null })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    const { result } = renderHook(() => useConversationAssignees(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(fakeApi.get).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/assignees?workspace_id=${WORKSPACE_ID}`,
    )
    expect(result.current.data).toHaveLength(1)
  })

  it('does not fetch when conversationId is null', () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    renderHook(() => useConversationAssignees(WORKSPACE_ID, null), {
      wrapper: wrapperFor(queryClient),
    })

    expect(fakeApi.get).not.toHaveBeenCalled()
  })
})
