// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { SupportConversation } from '@/lib/pmTypes'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { ConversationActionsMenu } from '../ConversationActionsMenu'

const mockUpdateStatusMutate = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useDeleteConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationRead: () => ({ mutate: vi.fn(), isPending: false }),
  useMarkConversationUnread: () => ({ mutate: vi.fn(), isPending: false }),
  useMoveConversation: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateConversationSubject: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateConversationStatus: () => ({ mutate: mockUpdateStatusMutate, isPending: false }),
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

function renderMenu(item: SupportConversation) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <ConversationActionsMenu
        workspaceId="ws-1"
        conversation={item}
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
})
