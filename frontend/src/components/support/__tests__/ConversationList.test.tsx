// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'

const mockUseConversations = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversations: (...args: unknown[]) => mockUseConversations(...args),
}))

vi.mock('../ConversationRow', () => ({
  ConversationRow: ({ conversation }: { conversation: { id: string } }) => <div data-conversation-id={conversation.id} />,
}))

import { ConversationList } from '../ConversationList'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('ConversationList presence resync', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSupportInboxStore.setState({
      statusFilter: 'all',
      searchQuery: '',
      selectedConversationId: null,
      navFilter: 'all',
    })
    useSupportPresenceStore.setState({
      typingIndicators: {},
      agentTyping: {},
      viewingAgents: {},
      onlineVisitors: {},
      wsSend: null,
      wsConnected: false,
    })
    mockUseConversations.mockReturnValue({
      data: {
        data: [
          { id: 'conv-1', updated_at: '2026-03-27T20:00:00Z' },
          { id: 'conv-2', updated_at: '2026-03-27T20:01:00Z' },
        ],
      },
      isLoading: false,
      error: null,
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('requests presence snapshots for visible conversations when websocket is connected', () => {
    const wsSend = vi.fn()
    useSupportPresenceStore.setState({ wsSend, wsConnected: true })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(wsSend).toHaveBeenCalledWith('support:presence:sync', {
      conversation_ids: expect.arrayContaining(['conv-1', 'conv-2']),
    })

    act(() => root.unmount())
  })
})
