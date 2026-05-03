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
import { ConversationRow, getVisibleSupportTagCount } from '../ConversationRow'

vi.mock('@/hooks/queries/useWorkspaces', () => ({
  useWorkspaceMembers: () => ({ data: [] }),
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

  it('renders all conversation tags instead of limiting the row to two tags', () => {
    const { container, cleanup } = renderRow(conversation({
      system_tags: ['ai_handoff'],
      tags: [
        { id: 'tag-billing', workspace_id: 'ws-1', name: 'Billing', color: '#2563eb', created_at: '', updated_at: '' },
        { id: 'tag-vip', workspace_id: 'ws-1', name: 'VIP', color: '#16a34a', created_at: '', updated_at: '' },
        { id: 'tag-renewal', workspace_id: 'ws-1', name: 'Renewal', color: '#c2410c', created_at: '', updated_at: '' },
      ],
    }))

    expect(container.textContent).toContain('AI handoff')
    expect(container.textContent).toContain('Billing')
    expect(container.textContent).toContain('VIP')
    expect(container.textContent).toContain('Renewal')
    expect(container.textContent).not.toContain('+2')

    cleanup()
  })

  it('calculates how many complete tag pills fit in the available row width', () => {
    expect(getVisibleSupportTagCount([60, 44, 70], 108)).toBe(2)
    expect(getVisibleSupportTagCount([140, 44], 80)).toBe(1)
    expect(getVisibleSupportTagCount([30, 30, 30], 0)).toBe(3)
  })
})
