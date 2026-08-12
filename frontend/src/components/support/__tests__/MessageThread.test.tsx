// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useWorkspaceStore } from '@/stores/workspaceStore'
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
  MessageBubble: () => <div data-testid="message-bubble" />,
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('MessageThread', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    supportHooks.useConversation.mockReturnValue({ data: undefined, isFetched: false })
    supportHooks.useConversationMessages.mockReturnValue({ data: [], isLoading: true })
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
    supportHooks.useConversationMessages.mockReturnValue({ isLoading: false, data: [] })
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
      data: [
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
      ],
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

    expect(supportHooks.markConversationRead).toHaveBeenCalledWith('conv-1')
    const scrollArea = container.querySelector('[data-slot="scroll-area"]')
    expect(scrollArea?.className).toContain('min-w-0')
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
    supportHooks.useConversationMessages.mockReturnValue({ isLoading: false, data: [] })
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
})
