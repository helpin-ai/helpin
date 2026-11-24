// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SupportConversationLoadError } from '@/lib/supportConversationError'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { SupportInboxLayout } from '../SupportInboxLayout'

type Conversation = { id: string; mailbox_id: string | null }
type Lookup = { data?: Conversation; error?: Error }

const harness = vi.hoisted(() => ({
  route: { conversationId: undefined as string | undefined, search: {} as Record<string, unknown> },
  lookups: new Map<string, { data?: unknown; error?: Error }>(),
  scopes: { data: undefined as unknown, isFetched: false },
  navigate: vi.fn(),
  threads: [] as { conversationId: string | null; onBackToInbox?: () => void }[],
  views: { data: [] as unknown[] },
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => harness.navigate,
  useLocation: () => ({ pathname: '/w/acme/support', search: harness.route.search }),
  useParams: () => ({ conversationId: harness.route.conversationId }),
}))

vi.mock('@/hooks/queries/useSupport', () => ({
  useConversation: (_workspaceId: string, id: string | null) => (id ? harness.lookups.get(id) ?? {} : {}),
  useInboxScopes: () => harness.scopes,
  useSupportMailboxes: () => ({ data: [] }),
  useSupportRoutingUsage: () => ({ data: undefined }),
  // Stable like real query data, so the route sync effect does not rerun every render.
  useSupportInboxViews: () => harness.views,
}))

vi.mock('@/hooks/queries', () => ({
  useWorkspaceAccess: () => ({ data: { permissions: ['support.read'] } }),
  usePermissions: () => ({ isAdmin: false }),
}))
vi.mock('@/hooks/useNotificationView', () => ({ useNotificationView: () => undefined }))
vi.mock('@/hooks/useSupportInboxShortcuts', () => ({ useSupportInboxShortcuts: () => undefined }))
vi.mock('@/hooks/use-min-width', () => ({ useMinWidth: (width: number) => width < 1000 }))
vi.mock('../ConversationList', () => ({ ConversationList: () => <div data-testid="list" /> }))
vi.mock('../MessageThread', () => ({
  MessageThread: (props: { conversationId: string | null; onBackToInbox?: () => void }) => {
    harness.threads.push(props)
    return <div data-testid="thread" data-conversation-id={props.conversationId ?? ''} />
  },
}))
vi.mock('../NewConversationDialog', () => ({ NewConversationDialog: () => null }))
vi.mock('../SupportShortcutsDialog', () => ({ SupportShortcutsDialog: () => null }))
vi.mock('../TeamInboxDialog', () => ({ TeamInboxDialog: () => null }))
vi.mock('@/components/ui/sheet', () => ({
  Sheet: () => null,
  SheetContent: () => null,
  SheetTitle: () => null,
  SheetDescription: () => null,
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const selectable = { data: { shared_inbox: { id: 'shared' }, mailboxes: [{ id: 'billing', active: true }, { id: 'sales', active: true }] }, isFetched: true }

function setLookup(id: string, lookup: Lookup) {
  harness.lookups.set(id, lookup)
}

function correctionCalls() {
  return harness.navigate.mock.calls.map(([options]) => options).filter((options) => options.to === '/w/$slug/support/$conversationId')
}

describe('SupportInboxLayout routed conversation context', () => {
  let container: HTMLDivElement
  let root: Root

  const render = () => act(() => root.render(<SupportInboxLayout />))
  // Apply a replace navigation to the mocked route, as the router would.
  const followNavigation = () => {
    const last = harness.navigate.mock.calls.at(-1)?.[0]
    harness.route = { conversationId: last.params.conversationId, search: last.search ?? {} }
    render()
  }

  beforeEach(() => {
    harness.route = { conversationId: undefined, search: {} }
    harness.lookups.clear()
    harness.scopes = selectable
    harness.navigate.mockReset()
    harness.threads = []
    useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', name: 'Acme', slug: 'acme' } } as never)
    useSupportInboxStore.setState({ selectedConversationId: null, selectedMailboxId: 'all', navFilter: 'inbox', activeCustomViewId: null })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  it('keeps a bare link open after the conversation moved, even with a stale persisted inbox', () => {
    useSupportInboxStore.setState({ selectedMailboxId: 'billing' })
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: {} }
    render()

    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all')
    expect(correctionCalls()).toEqual([])
    expect(harness.navigate.mock.calls.some(([options]) => options.to === '/w/$slug/support')).toBe(false)
  })

  it('replaces a stale team_inbox hint with the conversation inbox and keeps the ID', () => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()

    expect(correctionCalls()).toEqual([{
      to: '/w/$slug/support/$conversationId',
      params: { slug: 'acme', conversationId: 'conv-1' },
      search: { view: 'team', team_inbox: 'sales' },
      replace: true,
    }])
    followNavigation()
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('sales')
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    render()
    expect(correctionCalls()).toHaveLength(1)
    expect(harness.navigate.mock.calls.some(([options]) => options.to === '/w/$slug/support')).toBe(false)
  })

  it('treats a legacy inbox hint the same way and drops stale list constraints', () => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: { inbox: 'billing', mailbox_ids: 'billing', q: 'refund' } }
    render()

    expect(correctionCalls().at(-1)?.search).toEqual({ view: 'team', team_inbox: 'sales' })
  })

  it.each([
    ['is not in the inbox picker', 'hidden'],
    ['has no mailbox', null],
  ])('uses the general inbox when the conversation mailbox %s', (_label, mailboxId) => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: mailboxId } })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()

    expect(correctionCalls().at(-1)).toMatchObject({ params: { conversationId: 'conv-1' }, search: {}, replace: true })
    followNavigation()
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all')
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    render()
    expect(correctionCalls()).toHaveLength(1)
  })

  it('waits for the inbox picker before choosing a context', () => {
    harness.scopes = { data: undefined, isFetched: false }
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()
    expect(correctionCalls()).toEqual([])

    harness.scopes = selectable
    render()
    expect(correctionCalls().at(-1)?.search).toEqual({ view: 'team', team_inbox: 'sales' })
  })

  it('keeps an unavailable conversation selected without revealing a destination or auto-closing', () => {
    // Cached detail from before the denial must not drive a redirect.
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' }, error: new SupportConversationLoadError('conversation not found', 404) })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()

    expect(harness.navigate).not.toHaveBeenCalled()
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    expect(harness.threads.at(-1)?.conversationId).toBe('conv-1')
  })

  it('ignores an older lookup for a previous route ID', () => {
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()
    // conv-1 resolves (in another inbox) only after the route moved to conv-2.
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-2', search: { view: 'team', team_inbox: 'billing' } }
    render()

    expect(correctionCalls()).toEqual([])
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-2')
  })

  it('lets the viewer go back to the inbox from the desktop thread', () => {
    setLookup('conv-1', { error: new SupportConversationLoadError('conversation not found', 404) })
    harness.route = { conversationId: 'conv-1', search: {} }
    render()

    act(() => harness.threads.at(-1)?.onBackToInbox?.())
    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()
    expect(harness.navigate).toHaveBeenLastCalledWith({ to: '/w/$slug/support', params: { slug: 'acme' }, search: {} })
  })

  it('does not correct an explicit inbox switch or a closed thread', () => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: {} }
    render()

    act(() => useSupportInboxStore.getState().setSelectedMailboxId('billing'))
    harness.route = { conversationId: undefined, search: { view: 'team', team_inbox: 'billing' } }
    render()

    expect(correctionCalls()).toEqual([])
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('billing')
    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()
  })

  it('closes the thread when the open conversation moves out of the selected inbox in-session', () => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'sales' } }
    render()
    expect(correctionCalls()).toEqual([])

    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'billing' } })
    render()

    expect(correctionCalls()).toEqual([])
    expect(useSupportInboxStore.getState().selectedConversationId).toBeNull()
    expect(harness.navigate.mock.calls.at(-1)?.[0]).toMatchObject({ to: '/w/$slug/support', replace: true })
  })

  it('does not close a corrected link while its navigation is still pending', () => {
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    harness.route = { conversationId: 'conv-1', search: { view: 'team', team_inbox: 'billing' } }
    render()
    expect(correctionCalls()).toHaveLength(1)

    // A refetch lands before the router applies the replace navigation.
    setLookup('conv-1', { data: { id: 'conv-1', mailbox_id: 'sales' } })
    render()
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1')
    expect(harness.navigate.mock.calls.some(([options]) => options.to === '/w/$slug/support')).toBe(false)
  })
})
