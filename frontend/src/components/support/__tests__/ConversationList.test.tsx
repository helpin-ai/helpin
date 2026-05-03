// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'

const mockUseInfiniteConversations = vi.fn()
const mockUseMarkConversationRead = vi.fn()
const mockUseInboxScopes = vi.fn()
const mockUseCreateSupportInboxView = vi.fn()
const mockUseSupportTags = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useInfiniteConversations: (...args: unknown[]) => mockUseInfiniteConversations(...args),
  useMarkConversationRead: (...args: unknown[]) => mockUseMarkConversationRead(...args),
  useInboxScopes: (...args: unknown[]) => mockUseInboxScopes(...args),
  useCreateSupportInboxView: (...args: unknown[]) => mockUseCreateSupportInboxView(...args),
  useSupportTags: (...args: unknown[]) => mockUseSupportTags(...args),
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
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })
    useSupportPresenceStore.setState({
      typingIndicators: {},
      agentTyping: {},
      viewingAgents: {},
      onlineVisitors: {},
      wsSend: null,
      wsConnected: false,
    })
    mockUseInfiniteConversations.mockReturnValue({
      data: {
        pages: [
          {
            data: [
              { id: 'conv-1', status: 'open', updated_at: '2026-03-27T20:00:00Z' },
              { id: 'conv-2', status: 'open', updated_at: '2026-03-27T20:01:00Z' },
            ],
          },
        ],
      },
      isLoading: false,
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      error: null,
    })
    mockUseMarkConversationRead.mockReturnValue({
      mutate: vi.fn(),
    })
    mockUseCreateSupportInboxView.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    })
    mockUseInboxScopes.mockReturnValue({
      data: {
        shared_inbox: { id: 'shared', name: 'Shared Inbox' },
        mailboxes: [],
      },
    })
    mockUseSupportTags.mockReturnValue({
      data: [],
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

  it('auto-selects the first visible conversation when none is selected', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-2')
    expect(useSupportInboxStore.getState().activePanel).toBe('thread')

    act(() => root.unmount())
  })

  it('keeps an existing selected conversation instead of replacing it with the first row', () => {
    useSupportInboxStore.setState({ selectedConversationId: 'conv-2', activePanel: 'thread' })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-2')

    act(() => root.unmount())
  })

  it('fetches the next page when the list is scrolled near the bottom', () => {
    const fetchNextPage = vi.fn()
    mockUseInfiniteConversations.mockReturnValue({
      data: {
        pages: [
          {
            data: [
              { id: 'conv-1', status: 'open', updated_at: '2026-03-27T20:00:00Z' },
            ],
          },
        ],
      },
      isLoading: false,
      isFetchingNextPage: false,
      hasNextPage: true,
      fetchNextPage,
      error: null,
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const scrollEl = container.querySelector('[data-support-conversation-scroll]') as HTMLDivElement
    Object.defineProperty(scrollEl, 'scrollHeight', { configurable: true, value: 600 })
    Object.defineProperty(scrollEl, 'clientHeight', { configurable: true, value: 300 })
    Object.defineProperty(scrollEl, 'scrollTop', { configurable: true, value: 240 })

    act(() => {
      scrollEl.dispatchEvent(new Event('scroll', { bubbles: true }))
    })

    expect(fetchNextPage).toHaveBeenCalledTimes(1)

    act(() => root.unmount())
  })

  it('uses backend view filters for ownership-based sidebar views', () => {
    useSupportInboxStore.setState({
      navFilter: 'mine',
      selectedMailboxId: 'mailbox-billing',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(mockUseInfiniteConversations).toHaveBeenCalledWith('ws-1', {
      filter: 'mine',
      mailbox_id: 'mailbox-billing',
    })

    act(() => root.unmount())
  })

  it('shows Mine Assignment defaults without sending an extra assignment request filter', () => {
    useSupportInboxStore.setState({
      navFilter: 'mine',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(mockUseInfiniteConversations).toHaveBeenCalledWith('ws-1', {
      filter: 'mine',
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    expect(filterButton.querySelector('.bg-primary')).toBeNull()
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    for (const label of ['Assigned to me', 'Mentioned me', 'Opened by me']) {
      const button = Array.from(document.body.querySelectorAll('button')).find((item) => item.textContent === label) as HTMLButtonElement
      expect(button.className).toContain('bg-primary/10')
    }

    act(() => root.unmount())
  })

  it('keeps at least one Assignment chip selected in Mine', () => {
    useSupportInboxStore.setState({
      navFilter: 'mine',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const assignedToMeButton = Array.from(document.body.querySelectorAll('button')).find((item) => item.textContent === 'Assigned to me') as HTMLButtonElement
    act(() => {
      assignedToMeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().conversationListFilters.assignment).toEqual(['me'])

    act(() => root.unmount())
  })

  it('uses status directly for simple lifecycle sidebar views', () => {
    useSupportInboxStore.setState({
      navFilter: 'waiting',
      conversationListFilters: {
        states: ['waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(mockUseInfiniteConversations).toHaveBeenCalledWith('ws-1', {
      status: 'waiting_on_customer',
    })

    act(() => root.unmount())
  })

  it('saves the current filters as a support view', () => {
    const mutate = vi.fn((_payload, options?: { onSuccess?: () => void }) => options?.onSuccess?.())
    mockUseCreateSupportInboxView.mockReturnValue({
      mutate,
      isPending: false,
    })
    useSupportInboxStore.setState({
      navFilter: 'waiting',
      selectedMailboxId: 'mailbox-billing',
      searchQuery: 'refund',
      conversationListFilters: {
        states: ['waiting_on_customer'],
        assignment: ['unassigned'],
        mailboxIds: [],
        tagIds: ['tag-billing'],
        aiStates: ['handoff'],
        sort: 'oldest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" canCreateSharedViews />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const saveAsViewButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Save as view') as HTMLButtonElement
    act(() => {
      saveAsViewButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const nameInput = document.body.querySelector('#support-view-name') as HTMLInputElement
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(nameInput, 'Billing followups')
      nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    })

    const sharedSwitch = document.body.querySelector('#support-view-shared') as HTMLButtonElement
    act(() => {
      sharedSwitch.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const saveButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Save') as HTMLButtonElement
    act(() => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(mutate).toHaveBeenCalledWith({
      name: 'Billing followups',
      is_shared: true,
      filters: {
        nav_filter: 'waiting',
        states: 'waiting_on_customer',
        mailbox_id: 'mailbox-billing',
        search: 'refund',
        assignment: 'unassigned',
        tag_ids: 'tag-billing',
        ai: 'handoff',
        sort: 'oldest',
      },
    }, expect.any(Object))

    act(() => root.unmount())
  })

  it('only offers save as view after the current filters differ from the sidebar default', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as view')).toBe(false)

    const waitingButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Waiting') as HTMLButtonElement
    act(() => {
      waitingButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as view')).toBe(true)

    act(() => root.unmount())
  })

  it('does not show the filter dot for a selected team inbox context', () => {
    useSupportInboxStore.setState({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })
    mockUseInboxScopes.mockReturnValue({
      data: {
        shared_inbox: { id: 'shared', name: 'Main inbox' },
        mailboxes: [{ id: 'mailbox-billing', name: 'Billing' }],
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    expect(filterButton.querySelector('.bg-primary')).toBeNull()

    act(() => root.unmount())
  })

  it('keeps custom tags behind a searchable picker in the filter popover', () => {
    mockUseSupportTags.mockReturnValue({
      data: Array.from({ length: 16 }, (_, index) => ({
        id: `tag-${index + 1}`,
        name: `Tag ${index + 1}`,
        color: '#2563eb',
      })),
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Tag 16')).toBe(false)
    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Add')).toBe(true)

    act(() => root.unmount())
  })

  it('lets selected tag filters be removed from their pill', () => {
    mockUseSupportTags.mockReturnValue({
      data: [
        {
          id: 'tag-billing',
          name: 'Billing',
          color: '#2563eb',
        },
      ],
    })
    useSupportInboxStore.setState({
      conversationListFilters: {
        states: ['open'],
        assignment: [],
        mailboxIds: [],
        tagIds: ['tag-billing'],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const removeButton = document.body.querySelector('[aria-label="Remove Billing"]') as HTMLButtonElement
    expect(removeButton).not.toBeNull()
    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Add')).toBe(true)

    act(() => {
      removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().conversationListFilters.tagIds).toEqual([])

    act(() => root.unmount())
  })

  it('lets Main inbox replace All in the team inbox filter', () => {
    useSupportInboxStore.setState({
      navFilter: 'waiting',
      conversationListFilters: {
        states: ['waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const mainInboxButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Main inbox') as HTMLButtonElement
    act(() => {
      mainInboxButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().conversationListFilters.mailboxIds).toEqual(['shared'])
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all')

    act(() => root.unmount())
  })

  it('lets Inbox override Main inbox to All inboxes', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const allButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'All') as HTMLButtonElement
    act(() => {
      allButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().conversationListFilters.mailboxIds).toEqual(['all'])

    act(() => root.unmount())
  })
})
