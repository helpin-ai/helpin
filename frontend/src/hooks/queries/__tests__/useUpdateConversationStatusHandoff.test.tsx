// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'

vi.mock('@/lib/services/supportService', () => ({
  supportService: { updateConversationStatus: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }))

import { useUpdateConversationStatus } from '@/hooks/queries/useSupport'
import { supportService } from '@/lib/services/supportService'
import { useSupportInboxCelebrationStore } from '@/stores/supportInboxCelebrationStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let status: ReturnType<typeof useUpdateConversationStatus> | null = null

function Harness() {
  const mutation = useUpdateConversationStatus('ws-1')
  useEffect(() => {
    status = mutation
  }, [mutation])
  return null
}

const conversation = (id: string) => ({ id, workspace_id: 'ws-1', status: 'open', subject: id })

describe('resolving a conversation hands off on click', () => {
  let client: QueryClient
  let root: ReturnType<typeof createRoot>

  beforeEach(async () => {
    useSupportInboxStore.setState({ selectedConversationId: 'conv-1', conversationHandoff: null })
    useSupportInboxCelebrationStore.setState({ ownRemovals: {}, celebration: null })
    client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    client.setQueryData(queryKeys.support.conversations('ws-1'), {
      data: [conversation('conv-1'), conversation('conv-2')], total: 2, page: 1, per_page: 50, total_pages: 1,
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })
  })

  afterEach(() => {
    act(() => root.unmount())
    status = null
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('starts the hand-off and notes the own removal before the server answers', async () => {
    vi.mocked(supportService.updateConversationStatus).mockReturnValue(new Promise(() => {}))

    act(() => {
      status?.mutate({ conversationId: 'conv-1', status: 'resolved' })
    })
    await act(async () => {})

    expect(useSupportInboxStore.getState().conversationHandoff).toEqual({ fromConversationId: 'conv-1', toConversationId: 'conv-2' })
    expect(useSupportInboxCelebrationStore.getState().ownRemovals['conv-1']).toBeTypeOf('number')
  })

  it('undoes the hand-off when the update fails', async () => {
    vi.mocked(supportService.updateConversationStatus).mockResolvedValue({ data: null, error: 'Forbidden', status: 403 } as never)

    await act(async () => {
      await status?.mutateAsync({ conversationId: 'conv-1', status: 'resolved' }).catch(() => undefined)
    })

    expect(useSupportInboxStore.getState().conversationHandoff).toBeNull()
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    expect(useSupportInboxCelebrationStore.getState().ownRemovals['conv-1']).toBeUndefined()
  })
})
