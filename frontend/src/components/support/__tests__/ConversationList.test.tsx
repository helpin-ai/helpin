// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'

const mockUseConversations = vi.fn()
const mockUseMarkConversationRead = vi.fn()
const mockUseInboxScopes = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversations: (...args: unknown[]) => mockUseConversations(...args),
  useMarkConversationRead: (...args: unknown[]) => mockUseMarkConversationRead(...args),
  useInboxScopes: (...args: unknown[]) => mockUseInboxScopes(...args),
}))

vi.mock('../ConversationRow', () => ({
  ConversationRow: ({
    conversation,
    onSelectConversation,
  }: {
    conversation: { id: string; unread_count?: number | null }
    onSelectConversation: (id: string, unreadCount?: number) => void
  }) => (
    <button
      type="button"
      data-conversation-id={conversation.id}
      onClick={() => onSelectConversation(conversation.id, conversation.unread_count ?? 0)}
    />
  ),
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
      selectedMailboxId: 'all',
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
          { id: 'conv-1', unread_count: 1, updated_at: '2026-03-27T20:00:00Z' },
          { id: 'conv-2', unread_count: 0, updated_at: '2026-03-27T20:01:00Z' },
        ],
      },
      isLoading: false,
      error: null,
    })
    mockUseMarkConversationRead.mockReturnValue({
      mutate: vi.fn(),
    })
    mockUseInboxScopes.mockReturnValue({
      data: {
        shared_inbox: { id: 'shared', name: 'Shared Inbox' },
        mailboxes: [],
      },
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

  it('marks a newly selected unread conversation as read', () => {
    const mutate = vi.fn()
    mockUseMarkConversationRead.mockReturnValue({ mutate })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const row = container.querySelector('[data-conversation-id="conv-1"]')
    expect(row).not.toBeNull()

    act(() => {
      (row as HTMLButtonElement).click()
    })

    expect(mutate).toHaveBeenCalledWith('conv-1')

    act(() => root.unmount())
  })

  it('does not mark an already selected unread conversation as read again', () => {
    const mutate = vi.fn()
    mockUseMarkConversationRead.mockReturnValue({ mutate })
    useSupportInboxStore.setState({ selectedConversationId: 'conv-1' })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const row = container.querySelector('[data-conversation-id="conv-1"]')
    expect(row).not.toBeNull()

    act(() => {
      (row as HTMLButtonElement).click()
    })

    expect(mutate).not.toHaveBeenCalled()

    act(() => root.unmount())
  })
})
