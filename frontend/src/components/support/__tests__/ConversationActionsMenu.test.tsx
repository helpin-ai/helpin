// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { SupportConversation } from '@/lib/pmTypes'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { ConversationActionsMenu } from '../ConversationActionsMenu'

vi.mock('../SupportAIControl', () => ({ useSupportAIControl: () => ({ item: null, confirmation: null }) }))

const mockUpdateStatusMutate = vi.fn()
const mockMoveMutate = vi.fn()

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({ data: { permissions: ['support.edit'] } }),
}))

vi.mock('@/hooks/queries/useSupportOperations', () => {
  const mutation = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false });
  return {
    useUpdateConversationPriority: mutation,
    useSnoozeConversation: mutation,
    useUnsnoozeConversation: mutation,
  };
});

vi.mock('@/hooks/queries/useSupport', () => ({
  useDeleteConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationRead: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationUnread: () => ({ mutate: vi.fn(), isPending: false }),
  useMoveConversation: () => ({ mutate: mockMoveMutate, isPending: false }),
  useUpdateConversationSubject: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateConversationStatus: () => ({ mutate: mockUpdateStatusMutate, isPending: false }),
  useSendConversationTranscript: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('@/components/ui/confirm-dialog', () => ({
  useConfirm: () => vi.fn(async () => true),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) =>
    selector({ currentWorkspace: { slug: 'acme' } }),
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function conversation(overrides: Partial<SupportConversation> = {}): SupportConversation {
  return {
    id: 'conv-1',
    workspace_id: 'ws-1',
    display_id: 1,
    subject: 'Question',
    status: 'open',
    priority: 'medium',
    channel: 'widget',
    source: 'widget',
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  }
}

function renderMenu(item: SupportConversation, moveOptions: Array<{ id: string; name: string }> = []) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <ConversationActionsMenu
        workspaceId="ws-1"
        conversation={item}
        moveOptions={moveOptions}
        trigger={<button type="button">Actions</button>}
        open
      />,
    )
  })

  return {
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('ConversationActionsMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSupportInboxStore.setState({ selectedConversationId: null })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('marks an open conversation as spam without confirmation', () => {
    const rendered = renderMenu(conversation({ status: 'open' }))

    const spamItem = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
      .find((item) => item.textContent?.includes('Mark as spam')) as HTMLElement
    expect(spamItem).toBeTruthy()

    act(() => {
      spamItem.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(mockUpdateStatusMutate).toHaveBeenCalledWith(
      { conversationId: 'conv-1', status: 'spam' },
      expect.any(Object),
    )

    rendered.cleanup()
  })

  it('restores a spam conversation to inbox', () => {
    const rendered = renderMenu(conversation({ status: 'spam' }))

    const restoreItem = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
      .find((item) => item.textContent?.includes('Restore to inbox')) as HTMLElement
    expect(restoreItem).toBeTruthy()

    act(() => {
      restoreItem.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(mockUpdateStatusMutate).toHaveBeenCalledWith(
      { conversationId: 'conv-1', status: 'open' },
      expect.any(Object),
    )

    rendered.cleanup()
  })

  it('keeps inbox destinations in a second-layer move submenu', () => {
    const rendered = renderMenu(conversation(), [
      { id: 'support', name: 'Support' },
      { id: 'sales', name: 'Sales' },
    ])

    expect(document.body.textContent).toContain('Move to inbox')
    expect(document.body.textContent).not.toContain('Move to Support')
    expect(document.body.textContent).not.toContain('Move to Sales')

    rendered.cleanup()
  })

  it.each([
    { name: 'AI handling', flow_state: 'ai_handling', ai_state: null },
    { name: 'legacy AI handling', flow_state: null, ai_state: 'pending' },
  ])('allows moving $name to its current shared inbox', async ({ flow_state, ai_state }) => {
    const rendered = renderMenu(conversation({ mailbox_id: null, flow_state, ai_state }), [
      { id: 'shared', name: 'Shared Inbox' },
    ])
    try {
      const trigger = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
        .find((item) => item.textContent?.includes('Move to inbox')) as HTMLElement
      expect(trigger).toBeTruthy()
      await act(async () => trigger.click())
      const shared = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
        .find((item) => item.textContent === 'Shared Inbox') as HTMLElement
      expect(shared).toBeTruthy()
      await act(async () => shared.click())
      expect(mockMoveMutate).toHaveBeenCalledWith(
        { conversationId: 'conv-1', mailboxId: null }, expect.any(Object),
      )
    } finally {
      rendered.cleanup()
    }
  })

  it.each([
    { name: 'human handling', status: 'open', flow_state: 'waiting_for_human', human_takeover: true },
    { name: 'paused AI', status: 'open', flow_state: 'ai_handling', human_takeover: true },
    { name: 'resolved', status: 'resolved', flow_state: 'ai_handling', human_takeover: false },
  ])('hides the current shared inbox during $name', ({ status, flow_state, human_takeover }) => {
    const rendered = renderMenu(conversation({ status: status as SupportConversation['status'], mailbox_id: null, flow_state, human_takeover }), [
      { id: 'shared', name: 'Shared Inbox' },
    ])
    try {
      expect(document.body.textContent).not.toContain('Move to inbox')
    } finally {
      rendered.cleanup()
    }
  })
})
