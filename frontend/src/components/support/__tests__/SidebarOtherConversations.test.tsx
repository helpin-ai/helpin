// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

const mock = vi.hoisted(() => ({
  permissions: ['support.read', 'support.merge'],
  conversations: {
    'conv-1': { id: 'conv-1', display_id: 42, subject: "Can't export invoices to CSV", status: 'open', source: 'widget', customer_name: 'Maya Patel', customer_email: 'maya@example.com' },
    'conv-2': { id: 'conv-2', display_id: 38, subject: 'CSV export times out', status: 'resolved', source: 'widget', customer_name: 'Maya Patel', customer_email: 'maya@example.com' },
    'conv-4': { id: 'conv-4', display_id: 17, subject: 'Invoice address', status: 'resolved', source: 'widget', merged_into_id: 'conv-3', customer_email: 'maya@example.com' },
  } as Record<string, Record<string, unknown>>,
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, params, to: _to, ...props }: { children: ReactNode; params: { conversationId: string }; to: string }) => (
    <a href={`/w/acme/support/${params.conversationId}`} {...props}>{children}</a>
  ),
}))
vi.mock('@/hooks/queries/useSupport', () => ({
  useVisitorContext: () => ({
    isLoading: false,
    data: {
      total_conversations: 3,
      other_conversations: [
        { id: 'conv-2', display_id: 38, subject: 'CSV export times out', status: 'resolved', created_at: '2026-10-06T09:00:00Z' },
        { id: 'conv-4', display_id: 17, subject: 'Invoice address', status: 'resolved', merged_into_id: 'conv-3', created_at: '2026-08-27T09:00:00Z' },
      ],
    },
  }),
  useConversation: (_workspaceId: string, id: string | null) => ({ data: id ? mock.conversations[id] : undefined }),
}))
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: { permissions: mock.permissions } }) }))
vi.mock('../ConversationTranscript', () => ({
  ConversationTranscript: ({ conversation }: { conversation: { id: string } }) => <div data-testid="transcript">{conversation.id}</div>,
}))
vi.mock('../ConversationMergeDialog', () => ({
  ConversationMergeDialog: ({ conversations, initialTarget }: { conversations: Array<{ id: string }>; initialTarget: { id: string } }) => (
    <div data-testid="merge-review">{conversations[0].id} into {initialTarget.id}</div>
  ),
  ConversationPreviewHeading: () => null,
}))

import { SidebarOtherConversations } from '../SidebarVisitorContext'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function render() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => root.render(<TooltipProvider><SidebarOtherConversations workspaceId="ws-1" conversationId="conv-1" /></TooltipProvider>))
  return { container, cleanup: () => { act(() => root.unmount()); container.remove() } }
}

function click(element: Element, init: MouseEventInit = {}) {
  const event = new MouseEvent('click', { bubbles: true, cancelable: true, ...init })
  act(() => { element.dispatchEvent(event) })
  return event
}

const dialog = () => document.querySelector('[role="dialog"]')
const button = (name: string) => Array.from(document.querySelectorAll('button, a')).find((node) => node.textContent?.trim() === name)

describe('SidebarOtherConversations', () => {
  afterEach(() => {
    mock.permissions = ['support.read', 'support.merge']
    document.body.innerHTML = ''
  })

  it('previews a recent conversation in place with Merge and Open', () => {
    const { container, cleanup } = render()
    const row = container.querySelector('a[href="/w/acme/support/conv-2"]')!

    expect(click(row).defaultPrevented).toBe(true)
    expect(dialog()?.textContent).toContain('#38')
    expect(dialog()?.textContent).toContain('CSV export times out')
    expect(dialog()?.textContent).toContain('Resolved')
    expect(dialog()?.textContent).toContain('Maya Patel · Shared Inbox')
    expect(document.querySelector('[data-testid="transcript"]')?.textContent).toBe('conv-2')
    expect(button('Merge')).toBeTruthy()
    expect(button('Open conversation')?.getAttribute('href')).toBe('/w/acme/support/conv-2')
    cleanup()
  })

  it('keeps the link for new tabs', () => {
    const { container, cleanup } = render()
    const event = click(container.querySelector('a[href="/w/acme/support/conv-2"]')!, { metaKey: true })

    expect(event.defaultPrevented).toBe(false)
    expect(dialog()).toBeNull()
    cleanup()
  })

  it('reviews a merge of the previewed conversation into the open one', () => {
    const { container, cleanup } = render()
    click(container.querySelector('a[href="/w/acme/support/conv-2"]')!)
    click(button('Merge')!)

    expect(document.querySelector('[data-testid="merge-review"]')?.textContent).toBe('conv-2 into conv-1')
    cleanup()
  })

  it('leaves out Merge without merge access', () => {
    mock.permissions = ['support.read']
    const { container, cleanup } = render()
    click(container.querySelector('a[href="/w/acme/support/conv-2"]')!)

    expect(button('Open conversation')).toBeTruthy()
    expect(button('Merge')).toBeUndefined()
    cleanup()
  })

  it('shows a merged conversation as read-only history', () => {
    const { container, cleanup } = render()
    click(Array.from(container.querySelectorAll('button')).find((node) => node.textContent?.includes('Invoice address'))!)

    expect(dialog()?.textContent).toContain('Original conversation')
    expect(button('Open conversation')).toBeUndefined()
    expect(button('Merge')).toBeUndefined()
    cleanup()
  })
})
