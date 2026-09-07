// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useWorkspaceStore } from '@/stores/workspaceStore'
import { appendMessageToNewestPage, replaceMessageInPages, seedSupportMessagePages } from '@/lib/supportMessagePages'
import { MessageThread } from '../MessageThread'

const supportHooks = vi.hoisted(() => ({
  useConversation: vi.fn(),
  useConversationMessages: vi.fn(),
  markConversationRead: vi.fn(),
}))

function createTestQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => ({ pathname: '/w/acme/support/conv-1' }),
  useNavigate: () => vi.fn(),
}))

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversation: (...args: unknown[]) => supportHooks.useConversation(...args),
  useConversationMessages: (...args: unknown[]) => supportHooks.useConversationMessages(...args),
  useChatSettings: () => ({ data: null }),
  useInboxScopes: () => ({ data: null }),
  useSupportTeammatePresence: () => undefined,
  useUpdateConversationStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateTaskFromConversation: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useMoveConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useSendConversationTranscript: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDismissConversationTriage: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteSupportMessage: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useMarkConversationRead: () => ({ mutate: supportHooks.markConversationRead, isPending: false }),
  useMarkConversationUnread: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateConversationSubject: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteConversation: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({ data: null }),
  useUpdateSupportTaskPreferences: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

vi.mock('@/hooks/queries/useSettings', () => ({
  useWorkspaceSettings: () => ({ data: null }),
}))

vi.mock('@/hooks/queries/useWorkspaces', () => ({
  useWorkspaceMembers: () => ({ data: [] }),
}))

vi.mock('@/lib/services/agentService', () => ({
  agentService: {
    listRuns: vi.fn().mockResolvedValue({ data: { data: [] }, error: null }),
    approveRun: vi.fn().mockResolvedValue({ data: null, error: null }),
  },
}))

vi.mock('@/components/billing/UpgradeRequiredDialog', () => ({
  UpgradeRequiredDialog: () => null,
}))

vi.mock('../ReplyComposer', () => ({
  ReplyComposer: () => <div data-testid="reply-composer" />,
}))

vi.mock('../MessageBubble', () => ({
  MessageBubble: ({ message, isConsecutive }: { message: { content: string }; isConsecutive: boolean }) => <div data-testid="message-bubble" data-consecutive={String(isConsecutive)}>{message.content}</div>,
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('MessageThread', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    supportHooks.useConversation.mockReturnValue({ data: undefined, isFetched: false })
    supportHooks.useConversationMessages.mockReturnValue({
      data: seedSupportMessagePages([]),
      isLoading: true,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
      isFetchNextPageError: false,
    })
    supportHooks.markConversationRead.mockReset()
    useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', name: 'Acme', slug: 'acme' } })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('shows a full thread loading shell and hides the composer while switching conversations', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = createTestQueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread workspaceId="ws-1" conversationId="conv-1" />
        </QueryClientProvider>,
      )
    })
    act(() => {
      vi.runAllTimers()
    })

    expect(container.querySelector('[data-testid="support-thread-header-skeleton"]')).toBeTruthy()
    expect(container.querySelector('[data-testid="support-thread-message-skeleton"]')).toBeTruthy()
    expect(container.querySelector('[data-testid="reply-composer"]')).toBeNull()

    act(() => root.unmount())
  })

  it('marks the thread as transitioning briefly when the selected conversation changes', () => {
    supportHooks.useConversation.mockReturnValue({
      isFetched: true,
      data: {
        id: 'conv-1',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Question',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        unread_count: 0,
        created_at: '2026-06-03T09:00:00.000Z',
        updated_at: '2026-06-03T10:01:00.000Z',
      },
    })
    supportHooks.useConversationMessages.mockReturnValue({ isLoading: false, data: seedSupportMessagePages([]) })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = createTestQueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread workspaceId="ws-1" conversationId="conv-1" />
        </QueryClientProvider>,
      )
    })
    act(() => {
      vi.runAllTimers()
    })

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread workspaceId="ws-1" conversationId="conv-2" />
        </QueryClientProvider>,
      )
    })

    expect(container.querySelector('[data-support-message-thread]')?.getAttribute('data-transitioning')).toBe('true')

    act(() => {
      vi.advanceTimersByTime(180)
    })

    expect(container.querySelector('[data-support-message-thread]')?.getAttribute('data-transitioning')).toBeNull()

    act(() => root.unmount())
  })

  it('contains an open unread thread with intrinsically wide message content', () => {
    supportHooks.useConversation.mockReturnValue({
      isFetched: true,
      data: {
        id: 'conv-1',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Question',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        unread_count: 1,
        team_last_seen_at: '2026-06-03T10:00:00.000Z',
        created_at: '2026-06-03T09:00:00.000Z',
        updated_at: '2026-06-03T10:01:00.000Z',
      },
    })
    supportHooks.useConversationMessages.mockReturnValue({
      isLoading: false,
      data: seedSupportMessagePages([
        {
          id: 'msg-1',
          workspace_id: 'ws-1',
          conversation_id: 'conv-1',
          sender_type: 'customer',
          content: `See https://example.com/${'unbroken-path-segment-'.repeat(30)}`,
          message_type: 'reply',
          is_internal: false,
          created_at: '2026-06-03T10:01:00.000Z',
          updated_at: '2026-06-03T10:01:00.000Z',
        },
      ]),
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = createTestQueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread workspaceId="ws-1" conversationId="conv-1" />
        </QueryClientProvider>,
      )
    })
    act(() => {
      vi.runAllTimers()
    })

    expect(supportHooks.markConversationRead).toHaveBeenCalledWith({
      conversationId: 'conv-1',
      throughMessageId: 'msg-1',
    })
    const thread = container.querySelector('[data-support-message-thread]')
    expect(thread?.className).toContain('dark:bg-sidebar')
    const scrollArea = container.querySelector('[data-slot="scroll-area"]')
    expect(scrollArea?.className).toContain('min-w-0')
    expect(scrollArea?.className).toContain('dark:bg-sidebar')
    expect(scrollArea?.className).toContain('[&>[data-slot=scroll-area-viewport]>div]:!block')
    expect(scrollArea?.className).toContain('[&>[data-slot=scroll-area-viewport]>div]:!w-full')
    expect(scrollArea?.className).toContain('[&>[data-slot=scroll-area-viewport]>div]:!min-w-0')
    expect(scrollArea?.className).toContain('[&>[data-slot=scroll-area-viewport]>div]:!max-w-full')
    const messageList = container.querySelector('[data-support-message-list]')
    expect(messageList?.className).toContain('w-full')
    expect(messageList?.className).toContain('max-w-full')
    expect(messageList?.className).toContain('min-w-0')
    expect(messageList?.className).toContain('overflow-x-hidden')

    act(() => root.unmount())
  })

  it('does not show a manual Run action for an assigned support agent', () => {
    supportHooks.useConversation.mockReturnValue({
      isFetched: true,
      data: {
        id: 'conv-1',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Question',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        assigned_agent_id: 'agent-1',
        unread_count: 0,
        created_at: '2026-06-03T09:00:00.000Z',
        updated_at: '2026-06-03T10:01:00.000Z',
      },
    })
    supportHooks.useConversationMessages.mockReturnValue({ isLoading: false, data: seedSupportMessagePages([]) })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = createTestQueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread workspaceId="ws-1" conversationId="conv-1" />
        </QueryClientProvider>,
      )
      vi.runAllTimers()
    })

    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.trim() === 'Run')).toBe(false)
    act(() => root.unmount())
  })

  it('uses the compact Inbox-first header in the mobile sheet presentation', () => {
    supportHooks.useConversation.mockReturnValue({
      isFetched: true,
      data: {
        id: 'conv-1',
        workspace_id: 'ws-1',
        display_id: 42,
        subject: 'A long billing question',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        unread_count: 0,
        created_at: '2026-06-03T09:00:00.000Z',
        updated_at: '2026-06-03T10:01:00.000Z',
      },
    })
    supportHooks.useConversationMessages.mockReturnValue({ isLoading: false, data: seedSupportMessagePages([]) })
    const onBackToInbox = vi.fn()
    const onOpenDetails = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = createTestQueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MessageThread
            workspaceId="ws-1"
            conversationId="conv-1"
            presentation="mobile-sheet"
            onBackToInbox={onBackToInbox}
            onOpenDetails={onOpenDetails}
          />
        </QueryClientProvider>,
      )
      vi.runAllTimers()
    })

    const returnButton = container.querySelector<HTMLButtonElement>('[aria-label="Return to Inbox"]')
    const detailsButton = container.querySelector<HTMLButtonElement>('[aria-label="Open conversation details"]')
    expect(returnButton?.textContent?.trim()).toBe('')
    expect(detailsButton?.textContent).toContain('#42')
    expect(detailsButton?.textContent).toContain('A long billing question')
    expect(container.querySelector('[aria-label="Create task"]')).toBeTruthy()
    expect(container.querySelector('[aria-label="Resolve conversation"]')).toBeTruthy()
    expect(container.querySelector('[aria-label="Open conversation actions"]')).toBeTruthy()
    expect(container.textContent).not.toContain('Create Task')

    act(() => {
      returnButton?.click()
      detailsButton?.click()
    })
    expect(onBackToInbox).toHaveBeenCalledOnce()
    expect(onOpenDetails).toHaveBeenCalledOnce()

    act(() => root.unmount())
  })

  it.each(['event-first', 'ack-first', 'refetch-only', 'other-teammate'])('keeps the immediate reply stable with joined status arriving %s', (delivery) => {
    const customer = { id: 'customer', workspace_id: 'ws-1', conversation_id: 'conv-1', sender_type: (delivery === 'other-teammate' ? 'user' : 'customer') as 'user' | 'customer', sender_user_id: delivery === 'other-teammate' ? 'user-2' : undefined,
      content: 'Customer question', is_internal: false, message_type: 'reply', created_at: '2026-09-07T10:00:00Z', updated_at: '2026-09-07T10:00:00Z' }
    const optimistic = { ...customer, id: 'optimistic-send', client_message_id: 'optimistic-send', sender_type: 'user' as const,
      sender_user_id: 'user-1', content: 'Teammate reply', created_at: '2026-09-07T10:00:01Z' }
    const joined = { ...optimistic, id: 'joined', client_message_id: '', content: 'Waqar joined the conversation.',
      message_type: 'system', system_event_type: 'teammate_joined', metadata: JSON.stringify({ reply_client_message_id: optimistic.client_message_id }), created_at: '2026-09-07T10:00:02Z' }
    const saved = { ...optimistic, id: 'saved-reply', created_at: '2026-09-07T10:00:03Z' }
    supportHooks.useConversation.mockReturnValue({ isFetched: true, data: {
      id: 'conv-1', workspace_id: 'ws-1', subject: 'Question', status: 'open', priority: 'medium', source: 'widget',
      created_at: customer.created_at, updated_at: customer.created_at, unread_count: 0,
    } })
    const container = document.createElement('div'); document.body.appendChild(container)
    const root = createRoot(container); const client = createTestQueryClient()
    let pages = seedSupportMessagePages([customer])
    const render = () => {
      supportHooks.useConversationMessages.mockReturnValue({ data: pages, isLoading: false, hasNextPage: false })
      act(() => { root.render(<QueryClientProvider client={client}><MessageThread workspaceId="ws-1" conversationId="conv-1" /></QueryClientProvider>) })
      act(() => { vi.runAllTimers() })
    }
    const order = () => [...container.querySelectorAll('[data-support-message-id]')].map((row) => row.textContent)
    render()
    pages = appendMessageToNewestPage(pages, optimistic); render()
    const preview = order()
    const previewNode = container.querySelector('[data-support-message-id="optimistic-send"]')
    expect(previewNode?.firstElementChild?.getAttribute("data-consecutive")).toBe("false")
    if (delivery === 'ack-first') { pages = replaceMessageInPages(pages, optimistic.id, saved)!; render() }
    if (delivery !== 'refetch-only') { pages = appendMessageToNewestPage(pages, joined); render() }
    const realtimeJoin = order()
    expect(previewNode?.firstElementChild?.getAttribute("data-consecutive")).toBe("false")
    pages = appendMessageToNewestPage(pages, saved)
    pages = replaceMessageInPages(pages, optimistic.id, saved)!; render()
    const confirmed = order()
    const savedNode = container.querySelector('[data-support-message-id="saved-reply"]')
    pages = seedSupportMessagePages([customer, joined, { ...saved, client_message_id: undefined, metadata: JSON.stringify({ client_message_id: optimistic.client_message_id }) }]); render()
    const refreshed = order()
    expect(preview).toEqual(['Customer question', 'Teammate reply'])
    if (delivery !== 'refetch-only') expect(realtimeJoin).toEqual(['Customer question', 'Waqar joined the conversation.', 'Teammate reply'])
    expect(confirmed).toEqual(realtimeJoin)
    expect(refreshed).toEqual(['Customer question', 'Waqar joined the conversation.', 'Teammate reply'])
    expect(previewNode).toBe(savedNode)
    expect(container.querySelector('[data-support-message-id="saved-reply"]')).toBe(previewNode)
    expect(previewNode?.firstElementChild?.getAttribute("data-consecutive")).toBe("false")
    act(() => root.unmount()); client.clear()
  })

  it.each([{ historyCount: 1, reducedMotion: false, batchReply: false }, { historyCount: 5, reducedMotion: false, batchReply: false }, { historyCount: 1, reducedMotion: true, batchReply: false }, { historyCount: 5, reducedMotion: false, batchReply: true }])('keeps an immediate reply steady with $historyCount previous messages and reduced motion $reducedMotion, new reply $batchReply', ({ historyCount, reducedMotion, batchReply }) => {
    const history = Array.from({ length: historyCount }, (_, index) => ({
      id: `customer-${index}`, workspace_id: 'ws-1', conversation_id: 'conv-1', sender_type: 'customer' as const,
      content: 'Customer question', is_internal: false, message_type: 'reply', created_at: '2026-09-07T10:00:00Z', updated_at: '2026-09-07T10:00:00Z',
    }))
    const pending = { ...history[0], id: 'client-reply', client_message_id: 'client-reply', sender_type: 'user' as const, content: 'Immediate reply' }
    const joined = { ...history[0], id: 'joined', message_type: 'system', system_event_type: 'teammate_joined',
      metadata: JSON.stringify({ reply_client_message_id: pending.client_message_id }), content: 'Waqar joined the conversation.' }
    supportHooks.useConversation.mockReturnValue({ isFetched: true, data: {
      id: 'conv-1', workspace_id: 'ws-1', status: 'open', source: 'widget', created_at: history[0].created_at,
    } })
    const container = document.createElement('div'); document.body.appendChild(container)
    const root = createRoot(container); const client = createTestQueryClient()
    let pages = seedSupportMessagePages(history)
    const render = (settle = true) => {
      supportHooks.useConversationMessages.mockReturnValue({ data: pages, isLoading: false, hasNextPage: false })
      act(() => root.render(<QueryClientProvider client={client}><MessageThread workspaceId="ws-1" conversationId="conv-1" /></QueryClientProvider>))
      if (settle) act(() => { vi.runAllTimers() })
    }
    render()
    const viewport = container.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')!
    let scrollTop = 0
    const rows = () => [...container.querySelectorAll<HTMLElement>('[data-support-message-id]')]
    Object.defineProperties(viewport, {
      scrollHeight: { configurable: true, get: () => Math.max(300, rows().length * 100) },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, get: () => scrollTop, set: (value) => { scrollTop = Math.max(0, Math.min(value, viewport.scrollHeight - 300)) } },
    })
    const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      const rowIndex = rows().indexOf(this)
      const top = rowIndex >= 0 ? rowIndex * 100 - scrollTop : 0
      return { top, bottom: top + (this === viewport ? 300 : 100), left: 0, right: 300, height: this === viewport ? 300 : 100, width: 300, x: 0, y: top, toJSON: () => ({}) }
    })
    const previousMatchMedia = window.matchMedia
    window.matchMedia = vi.fn().mockReturnValue({ matches: reducedMotion })
    const previousAnimate = HTMLElement.prototype.animate
    const animate = vi.fn()
    HTMLElement.prototype.animate = animate
    try {
      pages = appendMessageToNewestPage(pages, pending); render()
      const reply = container.querySelector<HTMLElement>('[data-support-message-id="client-reply"]')!
      expect(reply.textContent).toBe('Immediate reply')
      const top = reply.getBoundingClientRect().top
      animate.mockClear()
      pages = appendMessageToNewestPage(pages, joined)
      if (batchReply) pages = appendMessageToNewestPage(pages, { ...history[0], id: 'new-customer-reply', content: 'New customer reply' })
      render(false)
      expect(rows().indexOf(reply)).toBe(historyCount + 1)
      if (historyCount === 1 && !reducedMotion) {
        expect(animate).toHaveBeenCalledWith([{ transform: 'translateY(-100px)' }, { transform: 'translateY(0)' }], expect.objectContaining({ duration: 160 }))
      } else {
        if (historyCount > 1) expect(reply.getBoundingClientRect().top).toBe(top - (batchReply ? 100 : 0))
        expect(animate).not.toHaveBeenCalled()
      }
    } finally {
      act(() => root.unmount()); client.clear(); rect.mockRestore()
      window.matchMedia = previousMatchMedia
      if (previousAnimate) HTMLElement.prototype.animate = previousAnimate
      else delete (HTMLElement.prototype as Partial<HTMLElement>).animate
    }
  })

  it.each(['realtime', 'pagination'])('preserves a reader in history when a joined row arrives through %s', (arrival) => {
    const history = Array.from({ length: 6 }, (_, index) => ({
      id: `reply-${index}`, client_message_id: `client-${index}`, workspace_id: 'ws-1', conversation_id: 'conv-1', sender_type: 'user' as const,
      content: `Reply ${index}`, is_internal: false, message_type: 'reply', created_at: '2026-09-07T10:00:00Z', updated_at: '2026-09-07T10:00:00Z',
    }))
    const joined = { ...history[0], id: 'joined', client_message_id: '', message_type: 'system', system_event_type: 'teammate_joined',
      metadata: JSON.stringify({ reply_client_message_id: 'client-0' }), content: 'Waqar joined the conversation.' }
    supportHooks.useConversation.mockReturnValue({ isFetched: true, data: { id: 'conv-1', workspace_id: 'ws-1', status: 'open', source: 'widget', created_at: history[0].created_at } })
    const container = document.createElement('div'); document.body.appendChild(container)
    const root = createRoot(container); const client = createTestQueryClient()
    const fetchNextPage = vi.fn()
    let pages = seedSupportMessagePages(history)
    const render = () => {
      supportHooks.useConversationMessages.mockReturnValue({ data: pages, isLoading: false, hasNextPage: arrival === 'pagination', fetchNextPage, isFetchingNextPage: false, isFetchNextPageError: arrival === 'pagination' })
      act(() => root.render(<QueryClientProvider client={client}><MessageThread workspaceId="ws-1" conversationId="conv-1" /></QueryClientProvider>))
      act(() => { vi.runAllTimers() })
    }
    render()
    const viewport = container.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')!
    let scrollTop = 200
    const rows = () => [...container.querySelectorAll<HTMLElement>('[data-support-message-id]')]
    Object.defineProperties(viewport, {
      scrollHeight: { configurable: true, get: () => rows().length * 100 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, get: () => scrollTop, set: (value) => { scrollTop = Math.max(0, Math.min(value, viewport.scrollHeight - 300)) } },
    })
    const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      const rowIndex = rows().indexOf(this)
      const top = rowIndex >= 0 ? rowIndex * 100 - scrollTop : 0
      return { top, bottom: top + (this === viewport ? 300 : 100), left: 0, right: 300, height: this === viewport ? 300 : 100, width: 300, x: 0, y: top, toJSON: () => ({}) }
    })
    try {
      act(() => { viewport.dispatchEvent(new Event('scroll')) })
      const anchor = container.querySelector<HTMLElement>('[data-support-message-id="reply-2"]')!
      const top = anchor.getBoundingClientRect().top
      if (arrival === 'pagination') {
        const button = [...container.querySelectorAll('button')].find((node) => node.textContent?.includes('Load earlier messages'))!
        act(() => button.click())
        expect(fetchNextPage).toHaveBeenCalledOnce()
        pages = { ...pages, pages: [...pages.pages, { data: [joined], has_more: false }], pageParams: [undefined, 'older'] }
      } else pages = appendMessageToNewestPage(pages, joined)
      render()
      expect(anchor.getBoundingClientRect().top).toBe(top)
      expect(scrollTop).toBe(300)
    } finally { act(() => root.unmount()); client.clear(); rect.mockRestore() }
  })

})
