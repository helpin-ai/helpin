import React from 'react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import { configureSupportApi, supportService } from '../support-service'
import { supportQueryKeys } from '../support-query-keys'
import { flattenSupportMessagePages, seedSupportMessagePages, type SupportMessagePages } from '../support-pages'
import {
  useSendMessage,
  useUploadSupportAttachment,
  useUpdateConversationStatus,
  useAssignConversationUser,
  useConversationAssignees,
  useCreateTaskFromConversation,
  useApproveAgentRun,
  useUpdateMySupportTeammatePresence,
} from '../use-support-core'
import type { SupportMessage } from '../support-types'

type FakeApi = {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  patch: ReturnType<typeof vi.fn>
  del: ReturnType<typeof vi.fn>
}

function makeFakeApi(): FakeApi {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
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

  it('createTaskFromConversation posts the selected team to the verified endpoint', async () => {
    fakeApi.post.mockResolvedValue({ data: { task_id: 'task-1', task_key: 'ENG-1' }, error: null })

    await supportService.createTaskFromConversation(WORKSPACE_ID, CONVERSATION_ID, { team_id: 'team-1' })

    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/create-task?workspace_id=${WORKSPACE_ID}`,
      { team_id: 'team-1' },
    )
  })

  it('dismissConversationTriage posts to the verified endpoint', async () => {
    fakeApi.post.mockResolvedValue({ data: { status: 'dismissed' }, error: null })

    await supportService.dismissConversationTriage(WORKSPACE_ID, CONVERSATION_ID)

    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/triage/dismiss?workspace_id=${WORKSPACE_ID}`,
      {},
    )
  })

  it('lists and resolves support AI-run interactions on conversation-scoped endpoints', async () => {
    fakeApi.get.mockResolvedValue({ data: { run_id: 'run-1', interactions: [] }, error: null })
    fakeApi.post.mockResolvedValue({ data: { id: 'int-1', status: 'resolved' }, error: null })
    await supportService.listAIRunInteractions(WORKSPACE_ID, CONVERSATION_ID)
    await supportService.resolveAIRunInteraction(WORKSPACE_ID, CONVERSATION_ID, 'int/1', { response_payload: { decision: 'approve' } })
    expect(fakeApi.get).toHaveBeenCalledWith(`/support/inbox/conversations/${CONVERSATION_ID}/ai-run/interactions?workspace_id=${WORKSPACE_ID}`)
    expect(fakeApi.post).toHaveBeenCalledWith(`/support/inbox/conversations/${CONVERSATION_ID}/ai-run/interactions/int%2F1/resolve?workspace_id=${WORKSPACE_ID}`, { response_payload: { decision: 'approve' } })
  })

  it('lists conversation agent runs and messages, then approves a draft', async () => {
    fakeApi.get.mockResolvedValue({ data: [], error: null })
    fakeApi.post.mockResolvedValue({ data: { id: 'run-1' }, error: null })

    await supportService.listConversationAgentRuns(WORKSPACE_ID, 'conv/1')
    await supportService.listAgentRunMessages(WORKSPACE_ID, 'run/1')
    await supportService.approveAgentRun(WORKSPACE_ID, 'run/1')

    expect(fakeApi.get).toHaveBeenNthCalledWith(
      1,
      '/automation/runs?workspace_id=ws-1&target_type=support_conversation&target_id=conv%2F1',
    )
    expect(fakeApi.get).toHaveBeenNthCalledWith(
      2,
      '/automation/runs/run%2F1/messages?workspace_id=ws-1',
    )
    expect(fakeApi.post).toHaveBeenCalledWith(
      '/automation/runs/run%2F1/approve?workspace_id=ws-1',
      { send_message: true },
    )
  })

  it('uses the conversation-scoped Ask Agents chat contract', async () => {
    fakeApi.post.mockResolvedValue({ data: { id: 'chat-1' }, error: null })
    fakeApi.get.mockResolvedValue({ data: { messages: [] }, error: null })
    const pageContext = { entity_type: 'support_conversation', entity_id: CONVERSATION_ID, display_title: 'Refund request' } as const

    await supportService.ensureConversationDockChat(WORKSPACE_ID, CONVERSATION_ID)
    await supportService.listSupportDockChatMessages(WORKSPACE_ID, 'chat/1')
    await supportService.sendSupportDockChatMessage(WORKSPACE_ID, 'chat/1', {
      client_message_id: '00000000-0000-4000-8000-000000000001', content: 'Draft a reply', page_context: pageContext,
    })
    await supportService.listSupportDockRunInteractions(WORKSPACE_ID, 'chat/1')
    await supportService.resolveSupportDockRunInteraction(WORKSPACE_ID, 'chat/1', 'int/1', { response_payload: { decision: 'approve' } })

    expect(fakeApi.post).toHaveBeenNthCalledWith(1, '/dock/chats?workspace_id=ws-1', {
      title: '', support_conversation_id: CONVERSATION_ID, module_id: 'support', visibility: 'module',
    })
    expect(fakeApi.get).toHaveBeenNthCalledWith(1, '/dock/chats/chat%2F1/messages?workspace_id=ws-1&limit=50')
    expect(fakeApi.post).toHaveBeenNthCalledWith(2, '/dock/chats/chat%2F1/messages?workspace_id=ws-1', {
      client_message_id: '00000000-0000-4000-8000-000000000001', content: 'Draft a reply', page_context: pageContext,
    })
    expect(fakeApi.get).toHaveBeenNthCalledWith(2, '/dock/chats/chat%2F1/run/interactions?workspace_id=ws-1')
    expect(fakeApi.post).toHaveBeenNthCalledWith(3, '/dock/chats/chat%2F1/interactions/int%2F1/resolve?workspace_id=ws-1', {
      response_payload: { decision: 'approve' },
    })
  })

  it('updates the current teammate support presence override', async () => {
    fakeApi.put.mockResolvedValue({ data: { user_id: 'user-1', status: 'away', source: 'manual', manual_status: 'away' }, error: null })
    await supportService.updateMyTeammatePresence(WORKSPACE_ID, 'away')
    expect(fakeApi.put).toHaveBeenCalledWith('/support/inbox/me/presence?workspace_id=ws-1', { manual_status: 'away' })
  })
})

describe('useUpdateMySupportTeammatePresence', () => {
  it('invalidates teammate availability after changing the override', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.put.mockResolvedValue({ data: { user_id: 'user-1', status: 'online', source: 'auto' }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useUpdateMySupportTeammatePresence(WORKSPACE_ID), { wrapper: wrapperFor(queryClient) })
    await act(async () => { await result.current.mutateAsync(null) })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.teammatePresence(WORKSPACE_ID) })
  })
})

describe('useApproveAgentRun', () => {
  it('invalidates the run, conversation, and message caches after approval', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({ data: { id: 'run-1' }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useApproveAgentRun(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync('run-1')
    })

    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.agentRuns(WORKSPACE_ID, CONVERSATION_ID) })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.agentRunMessages(WORKSPACE_ID, 'run-1') })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID) })
  })
})

describe('useCreateTaskFromConversation', () => {
  it('returns the created task and invalidates conversation and message queries', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({
      data: {
        task_id: 'task-1', display_id: 1, task_key: 'ENG-1', task_name: 'Fix billing',
        copied_contact_associations: 1, copied_company_associations: 0, copied_deal_associations: 0,
      },
      error: null,
    })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useCreateTaskFromConversation(WORKSPACE_ID), {
      wrapper: wrapperFor(queryClient),
    })

    let created: { task_id: string } | undefined
    await act(async () => {
      created = await result.current.mutateAsync({ conversationId: CONVERSATION_ID, teamId: 'team-1' })
    })

    expect(created?.task_id).toBe('task-1')
    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.conversation(WORKSPACE_ID, CONVERSATION_ID) })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID) })
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
    queryClient.setQueryData(supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID), seedSupportMessagePages([]))
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
      const cached = queryClient.getQueryData<SupportMessagePages>(
        supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID),
      )
      expect(flattenSupportMessagePages(cached)).toHaveLength(1)
    })

    const pendingCached = flattenSupportMessagePages(
      queryClient.getQueryData<SupportMessagePages>(supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID)),
    )
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

    const finalCached = flattenSupportMessagePages(
      queryClient.getQueryData<SupportMessagePages>(supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID)),
    )
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

    const finalCached = flattenSupportMessagePages(
      queryClient.getQueryData<SupportMessagePages>(supportQueryKeys.messages(WORKSPACE_ID, CONVERSATION_ID)),
    )
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

describe('useUploadSupportAttachment', () => {
  function attachmentInit() {
    return {
      attachment: {
        id: 'att-1', workspace_id: WORKSPACE_ID, conversation_id: CONVERSATION_ID,
        file_name: 'guide.pdf', file_size: 5, content_type: 'application/pdf',
        storage_key: 'support/guide.pdf', public_url: 'https://files.test/guide.pdf',
        is_uploaded: false, uploaded_by_type: 'user' as const,
        created_at: '2026-08-20T00:00:00.000Z',
      },
      upload_url: 'https://storage.test/presigned',
      public_url: 'https://files.test/guide.pdf',
    }
  }

  it('initiates, PUTs to storage, confirms, and returns the attachment id', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({ data: attachmentInit(), error: null })
    fakeApi.patch.mockResolvedValue({ data: { message: 'confirmed' }, error: null })
    const storageFetch = vi.fn(async () => ({ ok: true }))
    vi.stubGlobal('fetch', storageFetch)
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useUploadSupportAttachment(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })
    const file = new File(['guide'], 'guide.pdf', { type: 'application/pdf' })

    let uploaded: { id: string; url: string } | undefined
    await act(async () => { uploaded = await result.current.mutateAsync(file) })

    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/attachments?workspace_id=${WORKSPACE_ID}`,
      { file_name: 'guide.pdf', file_size: file.size, content_type: 'application/pdf' },
    )
    expect(storageFetch).toHaveBeenCalledWith('https://storage.test/presigned', {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': 'application/pdf', 'x-amz-acl': 'public-read' },
    })
    expect(fakeApi.patch).toHaveBeenCalledWith(
      `/support/inbox/attachments/att-1/confirm?workspace_id=${WORKSPACE_ID}`,
    )
    expect(uploaded).toEqual({ id: 'att-1', url: 'https://files.test/guide.pdf' })
    vi.unstubAllGlobals()
  })

  it('does not confirm when the storage PUT fails', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({ data: attachmentInit(), error: null })
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false })))
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useUploadSupportAttachment(WORKSPACE_ID, CONVERSATION_ID), {
      wrapper: wrapperFor(queryClient),
    })

    await expect(result.current.mutateAsync(
      new File(['guide'], 'guide.pdf', { type: 'application/pdf' }),
    )).rejects.toThrow('Upload to storage failed')
    expect(fakeApi.patch).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })

  it('deletes a staged attachment using the workspace-scoped endpoint', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.del.mockResolvedValue({ data: { message: 'deleted' }, error: null })

    await supportService.deleteAttachment(WORKSPACE_ID, 'att-delete')

    expect(fakeApi.del).toHaveBeenCalledWith(
      `/support/inbox/attachments/att-delete?workspace_id=${WORKSPACE_ID}`,
    )
  })
})

describe('agent productivity service paths', () => {
  it('lists, adds, and removes conversation tags on the verified endpoints', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.get.mockResolvedValue({ data: [], error: null })
    fakeApi.post.mockResolvedValue({ data: { message: 'added' }, error: null })
    fakeApi.del.mockResolvedValue({ data: { message: 'removed' }, error: null })

    await supportService.listTags(WORKSPACE_ID)
    await supportService.addConversationTag(WORKSPACE_ID, CONVERSATION_ID, 'tag-1')
    await supportService.removeConversationTag(WORKSPACE_ID, CONVERSATION_ID, 'tag-1')

    expect(fakeApi.get).toHaveBeenCalledWith(`/support/inbox/tags?workspace_id=${WORKSPACE_ID}`)
    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/tags/tag-1?workspace_id=${WORKSPACE_ID}`,
      {},
    )
    expect(fakeApi.del).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/tags/tag-1?workspace_id=${WORKSPACE_ID}`,
    )
  })

  it('posts transcript recipient choices to the verified endpoint', async () => {
    const fakeApi = makeFakeApi()
    configureSupportApi(fakeApi)
    fakeApi.post.mockResolvedValue({
      data: { success: true, message: 'sent', email: 'ada@example.com' }, error: null,
    })

    await supportService.sendConversationTranscript(WORKSPACE_ID, CONVERSATION_ID, {
      email: 'ada@example.com', update_customer_email: true,
    })

    expect(fakeApi.post).toHaveBeenCalledWith(
      `/support/inbox/conversations/${CONVERSATION_ID}/transcript?workspace_id=${WORKSPACE_ID}`,
      { email: 'ada@example.com', update_customer_email: true },
    )
  })
})
