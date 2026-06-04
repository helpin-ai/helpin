// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportConversation } from '@/lib/pmTypes'
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

  it('shows a reply icon only when the latest message came from an agent', () => {
    const agentRow = renderRow(conversation({
      last_message_sender_type: 'user',
      last_message_sender_display_name: 'Rosa Marin',
    }))
    expect(agentRow.container.querySelector('[aria-label="Rosa Marin replied"]')).not.toBeNull()
    agentRow.cleanup()

    const customerRow = renderRow(conversation({
      last_message_sender_type: 'customer',
      last_message_sender_display_name: 'Alex Customer',
    }))
    expect(customerRow.container.querySelector('[aria-label$=" replied"]')).toBeNull()
    customerRow.cleanup()
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
      usesActionBackground: true,
      usesUnreadTypography: true,
      usesSelectionBar: false,
    })

    expect(getConversationRowVisualState(conversation({
      unread_count: 0,
      awaiting_reply: true,
    }))).toEqual({
      isUnread: false,
      needsTeamAction: true,
      usesActionBackground: true,
      usesUnreadTypography: false,
      usesSelectionBar: false,
    })

    expect(getConversationRowVisualState(conversation({
      unread_count: 0,
      awaiting_reply: false,
    }), true)).toEqual({
      isUnread: false,
      needsTeamAction: false,
      usesActionBackground: false,
      usesUnreadTypography: false,
      usesSelectionBar: true,
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
