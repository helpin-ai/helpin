// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { SlackNotificationSettings } from '../SlackNotificationSettings'
import { slackService, type SlackNotificationPreferences } from '@/lib/services/slackService'

vi.mock('@/stores/authStore', () => ({ useAuthStore: () => 'member' }))
vi.mock('@/lib/services/slackService', () => ({
  slackService: { notificationPreferences: vi.fn(), updateNotificationPreferences: vi.fn() },
}))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: ReturnType<typeof createRoot>
let container: HTMLDivElement
let client: QueryClient
const key = ['slack', 'ws', 'notifications', 'member']

beforeEach(() => {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  client.clear()
  vi.clearAllMocks()
})

function render(pref: SlackNotificationPreferences) {
  client.setQueryData(key, pref)
  act(() => root.render(<QueryClientProvider client={client}>
    <SlackNotificationSettings workspaceId="ws" slug="test" canManage={false} />
  </QueryClientProvider>))
  act(() => container.querySelector('summary')!.click())
}

it('offers account linking when Slack is connected and this member is unlinked', () => {
  render({ team_id: 'T1', bot_user_id: 'Ubot', linked: false, mentions: false, assignments: false })
  expect(container.textContent).toContain('link account')
  expect(container.querySelector('a')?.href).toBe('slack://user?team=T1&id=Ubot')
  expect(container.querySelector('[role=switch]')).toBeNull()
})

it('lets a member opt into assignments independently of mentions', async () => {
  render({ team_id: 'T1', linked: true, mentions: false, assignments: false })
  const switches = container.querySelectorAll<HTMLElement>('[role=switch]')
  expect(Array.from(switches, node => node.getAttribute('aria-checked'))).toEqual(['false', 'false'])
  await act(async () => switches[0].click())
  expect(slackService.updateNotificationPreferences).toHaveBeenCalledWith('ws', { assignments: true })
  expect(client.getQueryData(key)).toMatchObject({ assignments: true, mentions: false })
})
