import { useEffect, useMemo } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import '@/index.css'
import { ConversationList } from '@/components/support/ConversationList'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useRealtimeSync } from '@/hooks/useRealtimeSync'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useAuthStore } from '@/stores/authStore'

type SentFrame = {
  type: string
  data?: Record<string, unknown>
}

type IncomingEnvelope =
  | { type: 'support:presence_snapshot'; data: unknown }
  | { action: string; entity: string; entity_id: string; workspace_id: string; actor_id: string; data?: Record<string, unknown> }

type HarnessController = {
  clearSentMessages: () => void
  disconnect: () => void
  emit: (message: IncomingEnvelope) => void
  getPresenceState: () => unknown
  getSentMessages: () => SentFrame[]
  getSocketCount: () => number
  isReady: boolean
  sendTypingStart: (content: string) => void
  sendTypingStop: () => void
}

declare global {
  interface Window {
    __supportPresenceHarness?: HarnessController
  }
}

const WORKSPACE_ID = 'ws-1'
const CONVERSATION_ID = 'conv-1'
const EMPTY_AGENT_TYPING: Record<string, { content: string; name?: string; avatarUrl?: string }> = {}
const EMPTY_VIEWING_AGENTS: string[] = []

const MEMBERS = [
  {
    id: 'wm-1',
    user_id: 'user-a',
    full_name: 'Alice Agent',
    email: 'alice@example.com',
    avatar_url: 'https://example.com/alice.png',
  },
  {
    id: 'wm-2',
    user_id: 'user-b',
    full_name: 'Bob Agent',
    email: 'bob@example.com',
    avatar_url: 'https://example.com/bob.png',
  },
  {
    id: 'wm-3',
    user_id: 'user-c',
    full_name: 'Charlie Agent',
    email: 'charlie@example.com',
    avatar_url: 'https://example.com/charlie.png',
  },
]

const searchParams = new URLSearchParams(window.location.search)
const currentUserId = searchParams.get('userId') || 'user-a'
const currentUserName = searchParams.get('name') || (currentUserId === 'user-a' ? 'Alice Agent' : 'Bob Agent')
const unreadCountParam = Number.parseInt(searchParams.get('unreadCount') || '0', 10)
const initialUnreadCount = Number.isFinite(unreadCountParam) ? unreadCountParam : 0

const CONVERSATIONS = {
  data: [
    {
      id: CONVERSATION_ID,
      workspace_id: WORKSPACE_ID,
      subject: 'Support thread',
      status: 'open',
      priority: 'medium',
      source: 'widget',
      channel: 'widget',
      customer_name: 'Visitor Example',
      customer_email: 'visitor@example.com',
      anonymous_id: 'anon-1',
      updated_at: '2026-03-27T20:00:00Z',
      created_at: '2026-03-27T19:55:00Z',
      last_message: 'Initial message',
      unread_count: initialUnreadCount,
    },
  ],
  total: 1,
  page: 1,
  per_page: 50,
  total_pages: 1,
  meta: {
    unread: {
      total: initialUnreadCount,
      my_inbox: initialUnreadCount,
      unassigned: 0,
    },
  },
}

localStorage.setItem('access_token', 'support-presence-harness-token')

const originalFetch = window.fetch.bind(window)

window.fetch = async (input, init) => {
  const url = typeof input === 'string' ? input : input.url
  if (url.includes(`/workspaces/${WORKSPACE_ID}/members`)) {
    return new Response(JSON.stringify(MEMBERS), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }
  if (url.includes('/support/inbox/conversations?workspace_id=')) {
    return new Response(JSON.stringify(CONVERSATIONS), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }
  return originalFetch(input, init)
}

class MockWebSocket {
  static instances: MockWebSocket[] = []
  static sentMessages: SentFrame[] = []
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readonly url: string
  readyState = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
    queueMicrotask(() => {
      this.readyState = MockWebSocket.OPEN
      this.onopen?.(new Event('open'))
    })
  }

  send(data: string) {
    MockWebSocket.sentMessages.push(JSON.parse(data) as SentFrame)
  }

  close() {
    this.serverClose()
  }

  serverClose() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }

  serverEmit(payload: IncomingEnvelope) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(payload) }))
  }
}

Object.defineProperty(window, 'WebSocket', {
  value: MockWebSocket,
  writable: true,
})

function ThreadPresencePreview() {
  const wsSend = useSupportPresenceStore((s) => s.wsSend)
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected)
  const customerTyping = useSupportPresenceStore((s) => s.typingIndicators[CONVERSATION_ID] || false)
  const agentTypingByConversation = useSupportPresenceStore((s) => s.agentTyping)
  const viewingAgentsByConversation = useSupportPresenceStore((s) => s.viewingAgents)
  const agentTyping = agentTypingByConversation[CONVERSATION_ID] ?? EMPTY_AGENT_TYPING
  const viewingAgents = viewingAgentsByConversation[CONVERSATION_ID] ?? EMPTY_VIEWING_AGENTS

  useEffect(() => {
    if (!wsSend || !wsConnected) return
    wsSend('support:viewing:start', { conversation_id: CONVERSATION_ID })
    return () => {
      wsSend('support:viewing:stop', { conversation_id: CONVERSATION_ID })
    }
  }, [wsConnected, wsSend])

  useEffect(() => {
    if (!window.__supportPresenceHarness) return
    window.__supportPresenceHarness.sendTypingStart = (content: string) => {
      const send = useSupportPresenceStore.getState().wsSend
      if (send) {
        send('support:typing:start', { conversation_id: CONVERSATION_ID, content })
      }
    }
    window.__supportPresenceHarness.sendTypingStop = () => {
      const send = useSupportPresenceStore.getState().wsSend
      if (send) {
        send('support:typing:stop', { conversation_id: CONVERSATION_ID })
      }
    }
    window.__supportPresenceHarness.getPresenceState = () => useSupportPresenceStore.getState()
  }, [wsSend])

  const agentTypingLabels = useMemo(
    () =>
      Object.values(agentTyping)
        .map((typing) => typing.name || 'Agent')
        .join(', '),
    [agentTyping],
  )

  return (
    <section className="space-y-2 rounded-lg border border-border bg-background p-4">
      <h2 className="text-sm font-semibold">Thread Preview</h2>
      <div data-testid="thread-customer-typing">{typeof customerTyping === 'string' ? customerTyping || 'typing…' : ''}</div>
      <div data-testid="thread-agent-typing">{agentTypingLabels}</div>
      <div data-testid="thread-viewers">{viewingAgents.join(', ')}</div>
    </section>
  )
}

function HarnessApp() {
  useRealtimeSync(WORKSPACE_ID)
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected)

  useEffect(() => {
    useWorkspaceStore.getState().setCurrentWorkspace({
      id: WORKSPACE_ID,
      name: 'Workspace',
      slug: 'workspace',
    } as never)
    useAuthStore.setState({
      user: {
        id: currentUserId,
        full_name: currentUserName,
        email: `${currentUserId}@example.com`,
      } as never,
      loading: false,
      serverUnreachable: false,
    })
    useSupportInboxStore.setState({
      selectedConversationId: CONVERSATION_ID,
      navFilter: 'all',
      statusFilter: 'all',
      searchQuery: '',
      activePanel: 'thread',
      replyMode: 'reply',
      createDialogOpen: false,
      navCollapsed: false,
      detailSidebarCollapsed: false,
      drafts: {},
    })
  }, [])

  useEffect(() => {
    if (window.__supportPresenceHarness && wsConnected) {
      window.__supportPresenceHarness.isReady = true
    }
  }, [wsConnected])

  return (
    <div className="min-h-screen bg-muted/30 p-6 text-foreground">
      <div className="mx-auto grid max-w-6xl grid-cols-[300px_1fr] gap-6">
        <ConversationList workspaceId={WORKSPACE_ID} userId={currentUserId} />
        <ThreadPresencePreview />
      </div>
    </div>
  )
}

window.__supportPresenceHarness = {
  isReady: false,
  clearSentMessages: () => {
    MockWebSocket.sentMessages = []
  },
  disconnect: () => {
    MockWebSocket.instances.at(-1)?.serverClose()
  },
  emit: (payload) => {
    MockWebSocket.instances.at(-1)?.serverEmit(payload)
  },
  getPresenceState: () => useSupportPresenceStore.getState(),
  getSentMessages: () => [...MockWebSocket.sentMessages],
  getSocketCount: () => MockWebSocket.instances.length,
  sendTypingStart: () => {},
  sendTypingStop: () => {},
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      staleTime: 0,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <HarnessApp />
    </TooltipProvider>
  </QueryClientProvider>,
)
