import type { Page, Route } from '@playwright/test'

export const SUPPORT_API_BASE = 'http://localhost:8080/api'
export const WORKSPACE_ID = 'ws-1'
export const WORKSPACE_SLUG = 'workspace'
export const ORGANIZATION_ID = 'org-1'
export const CONVERSATION_ID = 'conv-1'

export const AGENT_ALICE = {
  id: 'user-a',
  name: 'Alice Agent',
  email: 'alice@example.com',
  avatarUrl: 'https://example.com/alice.png',
}

export const AGENT_BOB = {
  id: 'user-b',
  name: 'Bob Agent',
  email: 'bob@example.com',
  avatarUrl: 'https://example.com/bob.png',
}

export const AGENT_CHARLIE = {
  id: 'user-c',
  name: 'Charlie Agent',
  email: 'charlie@example.com',
  avatarUrl: 'https://example.com/charlie.png',
}

const NOW = '2026-03-27T20:00:00Z'

const USER_BOB = {
  id: AGENT_BOB.id,
  email: AGENT_BOB.email,
  full_name: AGENT_BOB.name,
  avatar_url: AGENT_BOB.avatarUrl,
  default_workspace_id: WORKSPACE_ID,
  created_at: NOW,
  updated_at: NOW,
}

const ORGANIZATIONS = [
  {
    id: ORGANIZATION_ID,
    name: 'Helpin',
    slug: 'helpin',
    owner_id: AGENT_BOB.id,
    role: 'owner',
    created_at: NOW,
    updated_at: NOW,
  },
]

const WORKSPACE = {
  id: WORKSPACE_ID,
  name: 'Workspace',
  slug: WORKSPACE_SLUG,
  owner_id: AGENT_BOB.id,
  organization_id: ORGANIZATION_ID,
  timezone: 'UTC',
  created_at: NOW,
  updated_at: NOW,
}

const WORKSPACE_MEMBERS = [
  {
    id: 'wm-1',
    user_id: AGENT_ALICE.id,
    role: 'admin',
    email: AGENT_ALICE.email,
    full_name: AGENT_ALICE.name,
    avatar_url: AGENT_ALICE.avatarUrl,
  },
  {
    id: 'wm-2',
    user_id: AGENT_BOB.id,
    role: 'admin',
    email: AGENT_BOB.email,
    full_name: AGENT_BOB.name,
    avatar_url: AGENT_BOB.avatarUrl,
  },
  {
    id: 'wm-3',
    user_id: AGENT_CHARLIE.id,
    role: 'member',
    email: AGENT_CHARLIE.email,
    full_name: AGENT_CHARLIE.name,
    avatar_url: AGENT_CHARLIE.avatarUrl,
  },
]

const ASSIGNABLE_MEMBERS = WORKSPACE_MEMBERS.map((member) => ({
  id: member.id,
  user_id: member.user_id,
  role: member.role,
  email: member.email,
  display_name: member.full_name,
  avatar_url: member.avatar_url,
  status: 'active',
  created_at: NOW,
  updated_at: NOW,
}))

const WORKSPACE_MEMBERSHIP = {
  id: 'membership-1',
  workspace_id: WORKSPACE_ID,
  user_id: AGENT_BOB.id,
  email: AGENT_BOB.email,
  display_name: AGENT_BOB.name,
  role: 'owner',
  status: 'active',
  created_at: NOW,
  updated_at: NOW,
}

const WORKSPACE_ACCESS = {
  workspace_id: WORKSPACE_ID,
  membership: {
    id: WORKSPACE_MEMBERSHIP.id,
    user_id: AGENT_BOB.id,
    role: 'owner',
    status: 'active',
  },
  permissions: [
    'workspace.read',
    'support.read',
    'support.edit',
    'support.admin',
    'workspace.members.read',
    'settings.read',
    'ws.connect',
  ],
  team_memberships: [],
}

const WORKSPACE_SETTINGS = {
  settings: {
    id: 'settings-1',
    workspace_id: WORKSPACE_ID,
    quarter_start_date: '2026-01-01',
    sprint_duration_weeks: 2,
    notifications_enabled: true,
    team_weight: 50,
  },
  teams: [],
  people: [],
  memberships: [],
  user_memberships: [],
  managers: [],
  job_role_criteria: [],
  invitation_team_preassignments: [],
  team_estimate_settings: [],
  team_field_visibility: [],
  team_repo_defaults: [],
}

const SUPPORT_INSTALLATION = {
  id: 'install-1',
  workspace_id: WORKSPACE_ID,
  widget_key: 'widget-key',
  active: true,
  created_at: NOW,
  updated_at: NOW,
  settings: {
    require_email_before_chat: false,
    require_phone_after_email: false,
    welcome_message: 'Hi there',
    ai_enabled: false,
    ai_agent_id: null,
    ai_confidence_threshold: 0.8,
    ai_response_mode: 'suggest',
    ai_max_followups: 2,
    ai_auto_resolve_timeout: 300,
    show_talk_to_human: true,
    escalation_message: 'A human will jump in.',
    handoff_behavior: 'queue',
    handoff_team_id: null,
    business_hours_enabled: false,
    business_hours_timezone: 'UTC',
    business_hours_schedule: {},
    outside_hours_message: '',
    email_fallback_enabled: false,
    email_fallback_delay_secs: 600,
    email_fallback_from_name: 'Helpin',
    brand_color: '#2563eb',
    show_branding: true,
    color_scheme: 'light',
    button_color: '#2563eb',
    button_icon_color: '#ffffff',
    logo_url: '',
    launcher_position: 'right',
    launcher_icon: 'message',
    widget_name: 'Support',
    widget_avatar_url: '',
    widget_help_space_ids: [],
    csat_enabled: false,
    file_uploads_enabled: true,
    force_visitor_identity: false,
  },
}

export type SupportE2EController = {
  clearSentMessages: () => void
  disconnect: () => void
  emit: (payload: unknown) => void
  getSentMessages: () => Array<{ type: string; data?: Record<string, unknown> }>
  getSocketCount: () => number
  isReady: boolean
}

declare global {
  interface Window {
    __supportE2E?: SupportE2EController
  }
}

export function buildConversation(unreadCount = 0) {
  return {
    id: CONVERSATION_ID,
    workspace_id: WORKSPACE_ID,
    display_id: 42,
    subject: 'Support thread',
    status: 'open',
    priority: 'medium',
    source: 'widget',
    customer_name: 'Visitor Example',
    customer_email: 'visitor@example.com',
    anonymous_id: 'anon-1',
    last_message: 'Initial message',
    unread_count: unreadCount,
    created_at: '2026-03-27T19:55:00Z',
    updated_at: '2026-03-27T20:00:00Z',
  }
}

export const SUPPORT_MESSAGES = [
  {
    id: 'msg-1',
    workspace_id: WORKSPACE_ID,
    conversation_id: CONVERSATION_ID,
    sender_type: 'customer',
    sender_display_name: 'Visitor Example',
    content: 'Initial message',
    is_internal: false,
    via_channel: 'widget',
    created_at: '2026-03-27T19:56:00Z',
    updated_at: '2026-03-27T19:56:00Z',
  },
]

type MockOptions = {
  unreadCount?: number
}

function fulfillJSON(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}

export async function installSupportAppMocks(page: Page, options: MockOptions = {}) {
  const unreadCount = options.unreadCount ?? 0

  await page.addInitScript(() => {
    localStorage.setItem('access_token', 'support-e2e-token')
    localStorage.setItem('refresh_token', 'support-e2e-refresh-token')

    class MockWebSocket {
      static instances: MockWebSocket[] = []
      static sentMessages: Array<{ type: string; data?: Record<string, unknown> }> = []
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
          if (window.__supportE2E) {
            window.__supportE2E.isReady = true
          }
        })
      }

      send(data: string) {
        MockWebSocket.sentMessages.push(JSON.parse(data) as { type: string; data?: Record<string, unknown> })
      }

      close() {
        this.serverClose()
      }

      serverClose() {
        this.readyState = MockWebSocket.CLOSED
        this.onclose?.(new CloseEvent('close'))
      }

      serverEmit(payload: unknown) {
        this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(payload) }))
      }
    }

    Object.defineProperty(window, 'WebSocket', {
      value: MockWebSocket,
      writable: true,
    })

    window.__supportE2E = {
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
      getSentMessages: () => [...MockWebSocket.sentMessages],
      getSocketCount: () => MockWebSocket.instances.length,
    }
  })

  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    const method = route.request().method()

    if (method === 'GET' && path === '/api/auth/me') {
      return fulfillJSON(route, USER_BOB)
    }
    if (method === 'GET' && path === '/api/organizations') {
      return fulfillJSON(route, ORGANIZATIONS)
    }
    if (method === 'GET' && path === `/api/workspaces/by-slug/${WORKSPACE_SLUG}`) {
      return fulfillJSON(route, WORKSPACE)
    }
    if (method === 'GET' && path === `/api/workspaces/${WORKSPACE_ID}/my-membership`) {
      return fulfillJSON(route, WORKSPACE_MEMBERSHIP)
    }
    if (method === 'GET' && path === `/api/workspaces/${WORKSPACE_ID}/me`) {
      return fulfillJSON(route, WORKSPACE_ACCESS)
    }
    if (method === 'GET' && path === '/api/settings' && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, WORKSPACE_SETTINGS)
    }
    if (method === 'GET' && path === `/api/workspaces/${WORKSPACE_ID}/members`) {
      return fulfillJSON(route, WORKSPACE_MEMBERS)
    }
    if (method === 'GET' && path === `/api/workspaces/${WORKSPACE_ID}/assignable-members`) {
      return fulfillJSON(route, ASSIGNABLE_MEMBERS)
    }
    if (method === 'GET' && path === '/api/support/inbox/unread-stats' && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, {
        total: unreadCount,
        my_inbox: unreadCount,
        unassigned: 0,
      })
    }
    if (method === 'GET' && path === '/api/support/inbox/teammates/presence' && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, [])
    }
    if (method === 'GET' && path === '/api/support/inbox/installations' && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, SUPPORT_INSTALLATION)
    }
    if (method === 'GET' && path === '/api/support/inbox/conversations' && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, {
        data: [buildConversation(unreadCount)],
        total: 1,
        page: 1,
        per_page: 50,
        total_pages: 1,
        meta: {
          unread: {
            total: unreadCount,
            my_inbox: unreadCount,
            unassigned: 0,
          },
        },
      })
    }
    if (method === 'GET' && path === `/api/support/inbox/conversations/${CONVERSATION_ID}` && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, buildConversation(unreadCount))
    }
    if (method === 'GET' && path === `/api/support/inbox/conversations/${CONVERSATION_ID}/messages` && url.searchParams.get('workspace_id') === WORKSPACE_ID) {
      return fulfillJSON(route, SUPPORT_MESSAGES)
    }

    return fulfillJSON(route, { error: `Unhandled e2e API mock for ${method} ${path}` }, 500)
  })
}
