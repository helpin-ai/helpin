// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider, type InfiniteData } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'
import type { ConversationListResponse, SupportConversation } from '@/lib/pmTypes'

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('@/lib/services/supportService', () => ({
  supportService: {
    updateConversationPriority: vi.fn(),
    assignConversationUser: vi.fn(),
  },
}))

import { useAssignConversationUser } from '@/hooks/queries/useSupport'
import { useUpdateConversationPriority } from '@/hooks/queries/useSupportOperations'
import { supportService } from '@/lib/services/supportService'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const conversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: 'Export failing', status: 'open', priority: 'medium',
  source: 'email', unread_count: 0, assigned_user_id: 'user-a', created_at: '2026-10-27T10:00:00Z', updated_at: '2026-10-27T10:00:00Z',
} as SupportConversation
const other = { ...conversation, id: 'conv-2' }
const detailKey = queryKeys.support.conversation('ws-1', 'conv-1')
const infiniteKey = ['support', 'ws-1', 'conversations', 'infinite', { view: 'all' }]
const pageKey = ['support', 'ws-1', 'conversations', { page: 1 }]

type Hooks = { priority: ReturnType<typeof useUpdateConversationPriority>; assign: ReturnType<typeof useAssignConversationUser> }
let hooks: Hooks | null = null

function Harness() {
  const priority = useUpdateConversationPriority('ws-1')
  const assign = useAssignConversationUser('ws-1')
  useEffect(() => { hooks = { priority, assign } }, [priority, assign])
  return null
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

async function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } } })
  client.setQueryData(detailKey, conversation)
  client.setQueryData<InfiniteData<ConversationListResponse>>(infiniteKey, {
    pages: [{ data: [conversation, other], total: 2, page: 1, per_page: 50, total_pages: 1 }], pageParams: [1],
  })
  client.setQueryData<ConversationListResponse>(pageKey, { data: [other, conversation], total: 2, page: 1, per_page: 50, total_pages: 1 })
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(<QueryClientProvider client={client}><Harness /></QueryClientProvider>)
  })
  return client
}

function cached(client: QueryClient, field: 'priority' | 'assigned_user_id') {
  const infinite = client.getQueryData<InfiniteData<ConversationListResponse>>(infiniteKey)!.pages[0].data
  const page = client.getQueryData<ConversationListResponse>(pageKey)!.data
  return {
    detail: client.getQueryData<SupportConversation>(detailKey)?.[field],
    infinite: infinite.find((item) => item.id === 'conv-1')?.[field],
    page: page.find((item) => item.id === 'conv-1')?.[field],
    untouched: infinite.find((item) => item.id === 'conv-2')?.[field],
  }
}

describe('support picker changes', () => {
  afterEach(() => {
    hooks = null
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('shows a new priority everywhere before the server answers', async () => {
    const response = deferred<unknown>()
    vi.mocked(supportService.updateConversationPriority).mockReturnValue(response.promise as never)
    const client = await setup()

    await act(async () => { hooks!.priority.mutate({ conversationId: 'conv-1', priority: 'urgent' }) })
    // onMutate is async; wait for the optimistic update instead of racing it.
    await vi.waitFor(() => expect(cached(client, 'priority')).toEqual({ detail: 'urgent', infinite: 'urgent', page: 'urgent', untouched: 'medium' }), { timeout: 5000 })

    await act(async () => { response.resolve({ data: { ...conversation, priority: 'urgent' }, error: null, status: 200 }) })
    expect(cached(client, 'priority').detail).toBe('urgent')
  })

  it('puts the old priority back when saving fails', async () => {
    vi.mocked(supportService.updateConversationPriority).mockResolvedValue({ data: null, error: 'Conversation not found', status: 404 } as never)
    const client = await setup()

    await act(async () => { await hooks!.priority.mutateAsync({ conversationId: 'conv-1', priority: 'high' }).catch(() => undefined) })
    expect(cached(client, 'priority')).toEqual({ detail: 'medium', infinite: 'medium', page: 'medium', untouched: 'medium' })
  })

  it('shows a new assignee everywhere before the server answers', async () => {
    const response = deferred<unknown>()
    vi.mocked(supportService.assignConversationUser).mockReturnValue(response.promise as never)
    const client = await setup()

    await act(async () => { hooks!.assign.mutate({ conversationId: 'conv-1', userId: null }) })
    await vi.waitFor(() => expect(cached(client, 'assigned_user_id')).toEqual({ detail: undefined, infinite: undefined, page: undefined, untouched: 'user-a' }), { timeout: 5000 })

    await act(async () => { response.resolve({ data: { ...conversation, assigned_user_id: undefined }, error: null, status: 200 }) })
  })

  it('puts the old assignee back when saving fails', async () => {
    vi.mocked(supportService.assignConversationUser).mockResolvedValue({ data: null, error: 'Not allowed', status: 403 } as never)
    const client = await setup()

    await act(async () => { await hooks!.assign.mutateAsync({ conversationId: 'conv-1', userId: 'user-b' }).catch(() => undefined) })
    expect(cached(client, 'assigned_user_id')).toEqual({ detail: 'user-a', infinite: 'user-a', page: 'user-a', untouched: 'user-a' })
  })
})
