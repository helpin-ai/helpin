// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useWorkspaceStore } from '@/stores/workspaceStore'
import { MessageThread } from '../MessageThread'

const supportHooks = vi.hoisted(() => ({
  useConversation: vi.fn(),
  useConversationMessages: vi.fn(),
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
})
