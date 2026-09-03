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
const mockUseUpdateSupportInboxView = vi.fn()
const mockUseUpdateSupportBuiltinInboxView = vi.fn()
const mockUseSupportInboxViews = vi.fn()
const mockUseSupportTags = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useInfiniteConversations: (...args: unknown[]) => mockUseInfiniteConversations(...args),
  useMarkConversationRead: (...args: unknown[]) => mockUseMarkConversationRead(...args),
  useInboxScopes: (...args: unknown[]) => mockUseInboxScopes(...args),
  useCreateSupportInboxView: (...args: unknown[]) => mockUseCreateSupportInboxView(...args),
  useUpdateSupportInboxView: (...args: unknown[]) => mockUseUpdateSupportInboxView(...args),
  useUpdateSupportBuiltinInboxView: (...args: unknown[]) => mockUseUpdateSupportBuiltinInboxView(...args),
  useSupportInboxViews: (...args: unknown[]) => mockUseSupportInboxViews(...args),
  useSupportTags: (...args: unknown[]) => mockUseSupportTags(...args),
}))

vi.mock('../ConversationRow', () => ({
  ConversationRow: ({
    conversation,
    isTransitioningOut,
    onSelectConversation,
  }: {
    conversation: { id: string; unread_count?: number };
    isTransitioningOut?: boolean;
    onSelectConversation: (id: string, unreadCount?: number) => void;
  }) => <button type="button" data-conversation-id={conversation.id} data-transitioning-out={isTransitioningOut ? 'true' : undefined} onClick={() => onSelectConversation(conversation.id, conversation.unread_count)} />,
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
      conversationHandoff: null,
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
        mailboxIds: [],
        tagIds: [],
        aiStates: ['handoff'],
        sort: 'newest',
      },
      activeCustomViewId: null,
      customViewDirty: false,
      builtinViewFilters: {},
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
      isFetching: false,
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
    mockUseUpdateSupportInboxView.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    })
    mockUseUpdateSupportBuiltinInboxView.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    })
    mockUseSupportInboxViews.mockReturnValue({
      data: [],
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

  it('reserves inline header space for the collapsed workspace sidebar opener', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const header = container.querySelector('[data-slot="support-inbox-panel-header"]')
    expect(header?.className).toContain('group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14')

    act(() => root.unmount())
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

  it('keeps the mobile Inbox on the list until a conversation is opened', () => {
    const onConversationOpen = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <ConversationList
          workspaceId="ws-1"
          userId="user-1"
          autoSelectFirst={false}
          onConversationOpen={onConversationOpen}
        />,
      )
    })

    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()
    expect(container.firstElementChild?.className).toContain('w-full')

    act(() => {
      container.querySelector<HTMLButtonElement>('[data-conversation-id="conv-2"]')?.click()
    })

    expect(onConversationOpen).toHaveBeenCalledWith('conv-2')
    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()

    act(() => root.unmount())
  })

  it('waits for the rendered thread before marking an unread conversation read', async () => {
    const markRead = vi.fn()
    mockUseMarkConversationRead.mockReturnValue({ mutate: markRead })
    mockUseInfiniteConversations.mockReturnValue({
      data: { pages: [{ data: [{ id: 'conv-unread', status: 'open', unread_count: 1, updated_at: '2026-03-27T20:02:00Z' }] }] },
      isLoading: false,
      isFetching: false,
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      error: null,
    })
    useSupportInboxStore.setState({ selectedConversationId: 'conv-existing' })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => root.render(<ConversationList workspaceId="ws-1" userId="user-1" />))

    await act(async () => {
      container.querySelector<HTMLButtonElement>('[data-conversation-id="conv-unread"]')?.click()
      await new Promise((resolve) => window.setTimeout(resolve, 25))
    })
    expect(markRead).not.toHaveBeenCalled()

    act(() => root.unmount())
  })

  it('shows a loader while an empty inbox switch is fetching, then selects the first loaded row', () => {
    mockUseInfiniteConversations.mockReturnValue({
      data: {
        pages: [
          {
            data: [],
          },
        ],
      },
      isLoading: false,
      isFetching: true,
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      error: null,
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(container.textContent).not.toContain('Inbox is clear')
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0)
    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()

    mockUseInfiniteConversations.mockReturnValue({
      data: {
        pages: [
          {
            data: [
              { id: 'conv-loaded', status: 'open', updated_at: '2026-03-27T20:02:00Z' },
            ],
          },
        ],
      },
      isLoading: false,
      isFetching: false,
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      error: null,
    })

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-loaded')
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

  it('marks the conversation being resolved as transitioning out', () => {
    useSupportInboxStore.setState({
      selectedConversationId: 'conv-1',
      activePanel: 'thread',
      conversationHandoff: {
        fromConversationId: 'conv-1',
        toConversationId: 'conv-2',
      },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(container.querySelector('[data-conversation-id="conv-1"]')?.getAttribute('data-transitioning-out')).toBe('true')
    expect(container.querySelector('[data-conversation-id="conv-2"]')?.getAttribute('data-transitioning-out')).toBeNull()

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

  it('uses the header search action for global support search', () => {
    const onSearchClick = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" onSearchClick={onSearchClick} />)
    })

    const searchButton = container.querySelector('[aria-label="Search conversations"]') as HTMLButtonElement
    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement

    expect(searchButton).toBeTruthy()
    expect(filterButton).toBeTruthy()

    act(() => {
      searchButton.click()
    })

    expect(onSearchClick).toHaveBeenCalledTimes(1)
    expect(container.querySelector('input[placeholder="Search conversations..."]')).toBeNull()

    act(() => root.unmount())
  })

  it('keeps backend-returned mentioned conversations visible in Mine', () => {
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
    mockUseInfiniteConversations.mockReturnValue({
      data: {
        pages: [
          {
            data: [
              {
                id: 'mentioned-conv',
                status: 'open',
                subject: 'Mentioned conversation',
                updated_at: '2026-03-27T20:00:00Z',
              },
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

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    expect(container.querySelector('[data-conversation-id="mentioned-conv"]')).not.toBeNull()

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

    const saveAsViewButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Save as new view') as HTMLButtonElement
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

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as new view')).toBe(false)
    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Update view')).toBe(false)

    const waitingButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Waiting') as HTMLButtonElement
    act(() => {
      waitingButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as new view')).toBe(true)
    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Update view')).toBe(true)

    act(() => root.unmount())
  })

  it('updates the saved filters for a default sidebar item', () => {
    const updateBuiltinView = vi.fn((payload, options?: { onSuccess?: (view: { view_key: string; filters: Record<string, string> }) => void }) => {
      options?.onSuccess?.({ view_key: payload.view_key, filters: payload.filters })
    })
    mockUseUpdateSupportBuiltinInboxView.mockReturnValue({
      mutate: updateBuiltinView,
      isPending: false,
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

    const waitingButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Waiting') as HTMLButtonElement
    act(() => {
      waitingButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const updateButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Update view') as HTMLButtonElement
    expect(updateButton.getAttribute('title')).toBe('Updates this view for you only.')
    act(() => {
      updateButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().builtinViewFilters['nav:inbox']).toEqual({
      nav_filter: 'inbox',
      states: 'open',
    })
    expect(updateBuiltinView).toHaveBeenCalledWith({
      view_key: 'nav:inbox',
      filters: {
        nav_filter: 'inbox',
        states: 'open',
      },
    }, expect.any(Object))
    expect(filterButton.querySelector('.bg-primary')).toBeNull()

    act(() => root.unmount())
  })

  it('updates the saved filters for a team inbox sidebar item', () => {
    const updateBuiltinView = vi.fn((payload, options?: { onSuccess?: (view: { view_key: string; filters: Record<string, string> }) => void }) => {
      options?.onSuccess?.({ view_key: payload.view_key, filters: payload.filters })
    })
    mockUseUpdateSupportBuiltinInboxView.mockReturnValue({
      mutate: updateBuiltinView,
      isPending: false,
    })
    useSupportInboxStore.setState({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
        mailboxIds: [],
        tagIds: [],
        aiStates: ['handoff'],
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
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const oldestButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Oldest') as HTMLButtonElement
    act(() => {
      oldestButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const updateButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Update view') as HTMLButtonElement
    act(() => {
      updateButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().builtinViewFilters['team:mailbox-billing']).toEqual({
      nav_filter: 'inbox',
      states: 'open,waiting_on_customer',
      mailbox_id: 'mailbox-billing',
      sort: 'oldest',
    })
    expect(updateBuiltinView).toHaveBeenCalledWith({
      view_key: 'team:mailbox-billing',
      filters: {
        nav_filter: 'inbox',
        states: 'open,waiting_on_customer',
        mailbox_id: 'mailbox-billing',
        sort: 'oldest',
      },
    }, expect.any(Object))
    expect(filterButton.querySelector('.bg-primary')).toBeNull()

    act(() => root.unmount())
  })

  it('resets a team inbox to its saved filters without leaving that team inbox', () => {
    useSupportInboxStore.setState({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      searchQuery: 'changed',
      conversationListFilters: {
        states: ['open'],
        assignment: ['unassigned'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'oldest',
      },
      builtinViewFilters: {
        'team:mailbox-billing': {
          nav_filter: 'inbox',
          states: 'open,waiting_on_customer',
          mailbox_id: 'mailbox-billing',
          search: 'invoice',
        },
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
    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const resetButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Reset') as HTMLButtonElement
    act(() => {
      resetButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing')
    expect(useSupportInboxStore.getState().searchQuery).toBe('invoice')
    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['open', 'waiting_on_customer'],
      assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
      mailboxIds: [],
      tagIds: [],
      aiStates: ['handoff'],
      sort: 'newest',
    })

    act(() => root.unmount())
  })

  it('does not show filter changes for an unchanged custom view', () => {
    const updateView = vi.fn((payload, options?: { onSuccess?: (view: { id: string; filters: Record<string, string> }) => void }) => {
      options?.onSuccess?.({ id: payload.id, filters: payload.filters })
    })
    mockUseUpdateSupportInboxView.mockReturnValue({
      mutate: updateView,
      isPending: false,
    })
    useSupportInboxStore.setState({
      activeCustomViewId: 'view-billing',
      customViewDirty: false,
      navFilter: 'waiting',
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
    mockUseSupportTags.mockReturnValue({
      data: [
        {
          id: 'tag-billing',
          name: 'Billing',
          color: '#2563eb',
        },
      ],
    })
    mockUseSupportInboxViews.mockReturnValue({
      data: [
        {
          id: 'view-billing',
          name: 'Billing followups',
          filters: {},
          is_shared: false,
          view_type: 'custom',
          created_by: 'user-1',
          created_at: '2026-05-04T00:00:00Z',
          updated_at: '2026-05-04T00:00:00Z',
        },
      ],
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<ConversationList workspaceId="ws-1" userId="user-1" />)
    })

    const filterButton = container.querySelector('[aria-label="Conversation filters"]') as HTMLButtonElement
    expect(container.textContent).toContain('Billing followups')
    expect(filterButton.querySelector('.bg-primary')).toBeNull()

    act(() => {
      filterButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as new view')).toBe(false)
    expect(document.body.textContent).toContain('Billing followups filters')

    const openButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Open') as HTMLButtonElement
    act(() => {
      openButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(useSupportInboxStore.getState().activeCustomViewId).toBe('view-billing')
    expect(useSupportInboxStore.getState().customViewDirty).toBe(true)
    expect(filterButton.querySelector('.bg-primary')).not.toBeNull()
    expect(Array.from(document.body.querySelectorAll('button')).some((button) => button.textContent === 'Save as new view')).toBe(true)

    const updateButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent === 'Update view') as HTMLButtonElement
    act(() => {
      updateButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(updateView).toHaveBeenCalledWith({
      id: 'view-billing',
      filters: {
        nav_filter: 'waiting',
        states: 'waiting_on_customer,open',
        search: 'refund',
        assignment: 'unassigned',
        tag_ids: 'tag-billing',
        ai: 'handoff',
        sort: 'oldest',
      },
    }, expect.any(Object))
    expect(useSupportInboxStore.getState().customViewDirty).toBe(false)

    act(() => root.unmount())
  })

  it('does not show the filter dot for a selected team inbox context', () => {
    useSupportInboxStore.setState({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      conversationListFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
        mailboxIds: [],
        tagIds: [],
        aiStates: ['handoff'],
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
