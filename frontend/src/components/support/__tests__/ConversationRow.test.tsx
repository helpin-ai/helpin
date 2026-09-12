// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportConversation } from '@/lib/pmTypes'
import type { DockChat } from '@/lib/dockTypes'
import { useDockStore } from '@/stores/dockStore'
import { useAuthStore } from '@/stores/authStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { ConversationRow, getConversationRowVisualState, getSupportTagPillStyle, getVisibleSupportTagCount } from '../ConversationRow'

const mockWorkspaceMembers = vi.hoisted(() => ({
  data: [] as Array<{
    user_id: string
    full_name?: string | null
    email?: string | null
    avatar_url?: string | null
    avatar_style?: string | null
    avatar_seed?: string | null
    avatar_background_mode?: string | null
    avatar_background_color?: string | null
  }>,
}))

vi.mock('@/hooks/queries/useWorkspaces', () => ({
  useWorkspaceMembers: () => ({ data: mockWorkspaceMembers.data }),
}))

vi.mock('@/hooks/queries/useSupport', () => ({
  useDeleteConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationRead: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationUnread: () => ({ mutate: vi.fn(), isPending: false }),
  useMoveConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useSendConversationTranscript: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateConversationSubject: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateConversationStatus: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('@/components/ui/confirm-dialog', () => ({
  useConfirm: () => vi.fn(async () => true),
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function conversation(overrides: Partial<SupportConversation> = {}): SupportConversation {
  return {
    id: 'conv-1',
    workspace_id: 'ws-1',
    display_id: 1,
    subject: 'Billing question',
    status: 'open',
    priority: 'medium',
    customer_name: 'Alex Customer',
    customer_email: 'alex@example.com',
    source: 'email',
    last_message: 'Can you help with my invoice?',
    created_at: '2026-05-01T10:00:00Z',
    updated_at: '2026-05-01T10:00:00Z',
    ...overrides,
  }
}

function renderRow(conversationValue: SupportConversation) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <TooltipProvider>
        <ConversationRow
          workspaceId="ws-1"
          conversation={conversationValue}
          moveOptions={[]}
          onSelectConversation={vi.fn()}
        />
      </TooltipProvider>,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('ConversationRow', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: { id: 'user-1', email: 'agent@example.com', full_name: 'Agent' } })
    useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', name: 'Workspace', slug: 'workspace' } })
    useSupportInboxStore.setState({ selectedConversationId: null, drafts: {} })
    useDockStore.setState({ chats: [] })
    mockWorkspaceMembers.data = []
    useSupportPresenceStore.setState({
      typingIndicators: {},
      agentTyping: {},
      viewingAgents: {},
      onlineVisitors: {},
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it.each(['queued', 'running'] as const)('shows %s Ask Agent activity alongside unread messages and clears on completion', (status) => {
    const chat: DockChat = {
      id: 'chat-1', workspace_id: 'ws-1', user_id: 'user-1', title: 'Help',
      visibility: 'private', support_conversation_id: 'conv-1', active_run_id: 'run-1',
      active_run_status: status, created_at: '', updated_at: '',
    }
    const { container, cleanup } = renderRow(conversation({ unread_count: 3 }))
    expect(container.querySelector('[aria-label="Ask Agent is running"]')).toBeNull()
    act(() => useDockStore.setState({ chats: [chat] }))
    expect(container.querySelector('[aria-label="Ask Agent is running"]')).not.toBeNull()
    expect(container.querySelector('[data-agent-work-loader]')).not.toBeNull()
    expect(container.textContent).toContain('Can you help with my invoice?')
    expect(container.textContent).toContain('3')
    act(() => useDockStore.setState({ chats: [{ ...chat, active_run_status: 'paused' }] }))
    expect(container.querySelector('[aria-label="Ask Agent is waiting for input"]')).not.toBeNull()
    expect(container.querySelector('[data-agent-work-loader]')).toBeNull()
    for (const status of ['completed', 'failed', 'cancelled'] as const) {
      act(() => useDockStore.setState({ chats: [{ ...chat, active_run_status: status }] }))
      expect(container.querySelector('[aria-label^="Ask Agent is"]')).toBeNull()
    }
    cleanup()
  })

  it('ignores activity from other conversations, workspaces, and archived chats', () => {
    const chat: DockChat = {
      id: 'chat-1', workspace_id: 'ws-1', user_id: 'user-1', title: 'Help',
      visibility: 'private', support_conversation_id: 'conv-1', active_run_id: 'run-1',
      active_run_status: 'running', created_at: '', updated_at: '',
    }
    useDockStore.setState({ chats: [
      { ...chat, id: 'other-conversation', support_conversation_id: 'conv-2' },
      { ...chat, id: 'other-workspace', workspace_id: 'ws-2' },
      { ...chat, id: 'archived', archived_at: '2026-09-12T00:00:00Z' },
    ] })
    const { container, cleanup } = renderRow(conversation())
    expect(container.querySelector('[aria-label^="Ask Agent is"]')).toBeNull()
    cleanup()
  })

  it('shows timeline activity age without changing the last reply preview', () => {
    const now = Date.now()
    const { container, cleanup } = renderRow(conversation({
      last_message: 'The last actual reply',
      list_last_message_at: new Date(now - 9 * 3600000).toISOString(),
      list_last_activity_at: new Date(now - 2 * 3600000).toISOString(),
      updated_at: new Date(now - 11 * 60000).toISOString(),
    }))
    expect(container.querySelector('.tabular-nums')?.textContent).toBe('2h')
    expect(container.textContent).toContain('The last actual reply')
    cleanup()
  })

  it.each([true, false])('shows message age despite recent metadata updates (has message: %s)', (hasMessage) => {
    const now = Date.now()
    const { container, cleanup } = renderRow(conversation({
      created_at: new Date(now - 12 * 3600000).toISOString(),
      list_last_message_at: hasMessage ? new Date(now - 9 * 3600000).toISOString() : null,
      updated_at: new Date(now - 11 * 60000).toISOString(),
    }))
    expect(container.querySelector('.tabular-nums')?.textContent).toBe(hasMessage ? '9h' : '12h')
    cleanup()
  })

  it('renders AI handoff as an icon and keeps user tags visible in the compact row', () => {
    const { container, cleanup } = renderRow(conversation({
      system_tags: ['ai_handoff'],
      tags: [
        { id: 'tag-billing', workspace_id: 'ws-1', name: 'Billing', color: '#2563eb', created_at: '', updated_at: '' },
        { id: 'tag-vip', workspace_id: 'ws-1', name: 'VIP', color: '#16a34a', created_at: '', updated_at: '' },
        { id: 'tag-renewal', workspace_id: 'ws-1', name: 'Renewal', color: '#c2410c', created_at: '', updated_at: '' },
      ],
    }))

    expect(container.querySelector('[aria-label="AI handed off to team"]')).not.toBeNull()
    expect(container.textContent).not.toContain('AI handoff')
    expect(container.textContent).toContain('Billing')
    expect(container.textContent).toContain('VIP')
    expect(container.textContent).toContain('+1')

    cleanup()
  })

  it('renders AI resolved as a compact status icon', () => {
    const { container, cleanup } = renderRow(conversation({
      status: 'resolved',
      system_tags: ['ai_resolved'],
      ai_state: 'resolved',
      flow_state: 'resolved_by_ai',
    }))

    expect(container.querySelector('[aria-label="Resolved by AI"]')).not.toBeNull()
    expect(container.textContent).not.toContain('AI resolved')

    cleanup()
  })

  it('shows a waiting-for-human badge with relative wait time when queued', () => {
    const { container, cleanup } = renderRow(conversation({
      flow_state: 'queued_for_human',
      customer_requested_human_at: new Date(Date.now() - 90 * 60000).toISOString(),
    }))

    const badge = container.querySelector('[aria-label^="Waiting for human"]')
    expect(badge).not.toBeNull()
    expect(badge!.textContent).toContain('Waiting for human')
    expect(badge!.textContent).toContain('1h')

    cleanup()
  })

  it('shows the waiting-for-human badge for after-hours queue conversations', () => {
    const { container, cleanup } = renderRow(conversation({
      flow_state: 'after_hours_queue',
      updated_at: new Date(Date.now() - 5 * 60000).toISOString(),
    }))

    expect(container.querySelector('[aria-label^="Waiting for human"]')).not.toBeNull()

    cleanup()
  })

  it('does not render the waiting-for-human badge for other flow states', () => {
    const { container, cleanup } = renderRow(conversation({
      flow_state: 'ai_handling',
    }))

    expect(container.querySelector('[aria-label^="Waiting for human"]')).toBeNull()

    cleanup()
  })

  it('does not render the visitor country flag in the compact row', () => {
    const { container, cleanup } = renderRow(conversation({
      country_code: 'AT',
      country_name: 'Austria',
    }))

    expect(container.querySelector('[aria-label="Austria"]')).toBeNull()

    cleanup()
  })

  it('does not render the current agent as a viewing avatar on the selected row', () => {
    useAuthStore.setState({ user: { id: 'user-1', email: 'agent@example.com', full_name: 'Zed Agent' } })
    useSupportInboxStore.setState({ selectedConversationId: 'conv-1', drafts: {} })
    mockWorkspaceMembers.data = [{ user_id: 'user-1', full_name: 'Zed Agent', email: 'agent@example.com' }]

    const { container, cleanup } = renderRow(conversation())

    expect(container.textContent).not.toContain('Z')

    cleanup()
  })

  it('renders the channel icon before the customer name so its tooltip is not hidden by row actions', () => {
    const { container, cleanup } = renderRow(conversation({ source: 'email' }))
    const channelIcon = container.querySelector('[aria-label="Email"]')
    const name = container.querySelector('[data-conversation-customer-name="true"]')

    expect(channelIcon).not.toBeNull()
    expect(name).not.toBeNull()
    expect(channelIcon!.compareDocumentPosition(name!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    cleanup()
  })

  it('uses the second-line sender slot for agent replies and customer replies', () => {
    const agentRow = renderRow(conversation({
      last_message_sender_type: 'user',
      last_message_sender_display_name: 'Rosa Marin',
    }))
    expect(agentRow.container.querySelector('[aria-label="Rosa Marin replied"]')).not.toBeNull()
    expect(agentRow.container.querySelector('[aria-label="Customer replied"]')).toBeNull()
    agentRow.cleanup()

    const customerRow = renderRow(conversation({
      last_message_sender_type: 'customer',
      last_message_sender_display_name: 'Alex Customer',
    }))
    expect(customerRow.container.querySelector('[aria-label="Customer replied"]')).not.toBeNull()
    expect(customerRow.container.querySelector('[aria-label="Agent replied"]')).toBeNull()
    customerRow.cleanup()
  })

  it('does not tint an action-needed conversation row blue', () => {
    const { container, cleanup } = renderRow(conversation({
      awaiting_reply: true,
      last_message_sender_type: 'customer',
    }))

    const row = container.querySelector('[role="button"][tabindex="0"]')
    expect(row?.className).not.toContain('bg-blue-50/70')
    expect(row?.className).not.toContain('dark:bg-blue-950/20')

    cleanup()
  })

  it('keeps the subject dialog mounted after opening it from row actions', () => {
    const { container, cleanup } = renderRow(conversation())

    const actionButton = container.querySelector('[aria-label="Open actions for Alex Customer"]') as HTMLButtonElement
    expect(actionButton).not.toBeNull()

    act(() => {
      actionButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const setSubjectItem = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
      .find((item) => item.textContent?.includes('Set subject')) as HTMLElement
    expect(setSubjectItem).toBeTruthy()

    act(() => {
      setSubjectItem.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(document.body.textContent).toContain('Set conversation subject')

    cleanup()
  })

  it('separates unread, action-needed, and selected visual states', () => {
    expect(getConversationRowVisualState(conversation({
      unread_count: 3,
      awaiting_reply: true,
    }))).toEqual({
      isUnread: true,
      needsTeamAction: true,
      usesUnreadTypography: true,
      usesSelectionBar: false,
    })

    expect(getConversationRowVisualState(conversation({
      unread_count: 0,
      awaiting_reply: true,
    }))).toEqual({
      isUnread: false,
      needsTeamAction: true,
      usesUnreadTypography: false,
      usesSelectionBar: false,
    })

    expect(getConversationRowVisualState(conversation({
      unread_count: 0,
      awaiting_reply: false,
      customer_awaiting_response: true,
    }), true)).toEqual({
      isUnread: false,
      needsTeamAction: true,
      usesUnreadTypography: false,
      usesSelectionBar: true,
    })

    expect(getConversationRowVisualState(conversation({
      unread_count: 4,
      awaiting_reply: true,
      customer_awaiting_response: false,
    }))).toEqual({
      isUnread: true,
      needsTeamAction: false,
      usesUnreadTypography: true,
      usesSelectionBar: false,
    })
  })

  it('calculates how many complete tag pills fit in the available row width', () => {
    expect(getVisibleSupportTagCount([60, 44, 70], 108)).toBe(2)
    expect(getVisibleSupportTagCount([140, 44], 80)).toBe(1)
    expect(getVisibleSupportTagCount([30, 30, 30], 0)).toBe(3)
  })

  it('uses a subtle tint for colored tag pills', () => {
    expect(getSupportTagPillStyle('#2563eb')).toEqual({
      backgroundColor: 'rgba(37, 99, 235, 0.08)',
      borderColor: 'rgba(37, 99, 235, 0.22)',
      color: 'rgba(37, 99, 235, 0.82)',
    })
    expect(getSupportTagPillStyle('not-a-color')).toBeUndefined()
  })
})
