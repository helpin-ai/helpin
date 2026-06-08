// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useWorkspaceStore } from '@/stores/workspaceStore'
import { MessageThread } from '../MessageThread'

const supportHooks = vi.hoisted(() => ({
  useConversation: vi.fn(),
  useConversationMessages: vi.fn(),
  markConversationRead: vi.fn(),
}))

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
  useRunConversationAgent: () => ({ mutate: vi.fn(), isPending: false }),
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

    act(() => {
      root.render(<MessageThread workspaceId="ws-1" conversationId="conv-1" />)
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

    act(() => {
      root.render(<MessageThread workspaceId="ws-1" conversationId="conv-1" />)
    })
    act(() => {
      vi.runAllTimers()
    })

    act(() => {
      root.render(<MessageThread workspaceId="ws-1" conversationId="conv-2" />)
    })

    expect(container.querySelector('[data-support-message-thread]')?.getAttribute('data-transitioning')).toBe('true')

    act(() => {
      vi.advanceTimersByTime(180)
    })

    expect(container.querySelector('[data-support-message-thread]')?.getAttribute('data-transitioning')).toBeNull()

    act(() => root.unmount())
  })

  it('marks an open unread thread read when the latest message is visible', () => {
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
          content: 'Still there?',
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

    act(() => {
      root.render(<MessageThread workspaceId="ws-1" conversationId="conv-1" />)
    })
    act(() => {
      vi.runAllTimers()
    })

    expect(supportHooks.markConversationRead).toHaveBeenCalledWith('conv-1')

    act(() => root.unmount())
  })
})
