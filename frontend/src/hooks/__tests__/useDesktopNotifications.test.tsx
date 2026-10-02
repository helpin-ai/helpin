// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useDesktopNotifications } from '../useDesktopNotifications'
import { setDesktopEnabled } from '@/lib/desktopNotifications'
import type { WSEvent } from '../useWebSocket'

vi.mock('@/stores/authStore', () => ({ useAuthStore: Object.assign((select: (state: unknown) => unknown) => select({ user: { id: 'u1' } }), { getState: () => ({ user: { id: 'u1' } }) }) }))
vi.mock('@/stores/workspaceStore', () => ({ useWorkspaceStore: { getState: () => ({ currentWorkspace: { id: 'w1', slug: 'acme' } }) } }))
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }))
const { getPreferences } = vi.hoisted(() => ({ getPreferences: vi.fn() }))
vi.mock('@/lib/services/notificationsService', () => ({ notificationsService: { getPreferences } }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const event: WSEvent = { event_id: 'event-1', entity: 'notification', action: 'created', entity_id: 'n1', workspace_id: 'w1', actor_id: 'other', data: { recipient_id: 'u1', category: 'mentions', title: 'Sam mentioned you', entity_type: 'task', entity_id: 't1', status: 'unread' } }
const show = vi.fn()
let focused = true
let receive: ReturnType<typeof useDesktopNotifications>
let root: ReturnType<typeof createRoot>
let client: QueryClient
let resolvePreferences: (value: unknown) => void
const preferences = { data: { mute_workspace: false, do_not_disturb: false, channel_preferences: {} }, error: null }

beforeEach(() => {
  localStorage.clear()
  show.mockClear()
  getPreferences.mockReset().mockReturnValue(new Promise(resolve => { resolvePreferences = resolve }))
  vi.stubGlobal('isSecureContext', true)
  vi.stubGlobal('Notification', class { static permission = 'granted'; constructor(...args: unknown[]) { show(...args) } })
  focused = true
  vi.spyOn(document, 'hasFocus').mockImplementation(() => focused)
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
  setDesktopEnabled('u1', true)
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  root = createRoot(document.createElement('div'))
  function Harness() { receive = useDesktopNotifications('w1'); return null }
  act(() => root.render(<QueryClientProvider client={client}><Harness /></QueryClientProvider>))
})
afterEach(() => {
  act(() => root.unmount())
  client.clear()
  localStorage.clear()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function blur() { focused = false; window.dispatchEvent(new Event('blur')) }

it('does not turn a foreground event into a popup after a slow settings request and tab switch', async () => {
  const pending = receive(event)
  blur()
  await act(async () => { resolvePreferences(preferences); await pending })
  expect(show).not.toHaveBeenCalled()
  expect(getPreferences).not.toHaveBeenCalled()
})

it('also suppresses at receipt when another Helpin tab is focused', async () => {
  // The mounted focused tab has already published its focus lease.
  focused = false
  const pending = receive(event)
  window.dispatchEvent(new Event('blur'))
  await act(async () => { resolvePreferences(preferences); await pending })
  expect(show).not.toHaveBeenCalled()
})

it('still delivers events that arrive while working elsewhere', async () => {
  blur()
  const pending = receive(event)
  await act(async () => { resolvePreferences(preferences); await pending })
  expect(show).toHaveBeenCalledOnce()
})

it('rechecks focus when returning to Helpin while settings are loading', async () => {
  blur()
  const pending = receive(event)
  focused = true
  window.dispatchEvent(new Event('focus'))
  await act(async () => { resolvePreferences(preferences); await pending })
  expect(show).not.toHaveBeenCalled()
})
