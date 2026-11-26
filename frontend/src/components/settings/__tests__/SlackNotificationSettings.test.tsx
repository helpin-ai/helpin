// @vitest-environment jsdom
import { NotificationCategoriesCard } from '../NotificationPreferencesPanels'
import { MODULE_NOTIFICATION_SECTIONS } from '@/lib/slackNotificationRows'
import { TooltipProvider } from '@/components/ui/tooltip'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { SlackConnectionNotice, useSlackNotificationSettings } from '../SlackNotificationSettings'
import { slackService, type SlackNotificationPreferences } from '@/lib/services/slackService'

const { saveChannels } = vi.hoisted(() => ({ saveChannels: vi.fn() }))
vi.mock('@/hooks/queries', () => ({
  useNotificationPreferences: () => ({ data: { channel_preferences: {} } }),
  useUpdateNotificationPreferences: () => ({ isPending: false, mutate: saveChannels }),
}))
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

function Settings({ module }: { module: string }) {
  const controls = useSlackNotificationSettings('ws')
  const section = MODULE_NOTIFICATION_SECTIONS.find(section => section.module === module)!
  return <TooltipProvider>
    <SlackConnectionNotice controls={controls} slug="test" canManage={false} />
    <NotificationCategoriesCard workspaceId="ws" title={section.title} categories={section.rows} controls={controls} target={section.rows[0].slackRow} />
  </TooltipProvider>
}

function render(pref: SlackNotificationPreferences, module = 'support') {
  client.setQueryData(key, pref)
  act(() => root.render(<QueryClientProvider client={client}>
    <Settings module={module} />
  </QueryClientProvider>))
}

it('offers account linking when Slack is connected and this member is unlinked', () => {
  render({ team_id: 'T1', bot_user_id: 'Ubot', linked: false, mentions: false, assignments: false })
  expect(container.textContent).toContain('link account')
  expect(container.querySelector('a')?.href).toBe('slack://user?team=T1&id=Ubot')
  expect(container.querySelector<HTMLButtonElement>('[aria-label="Support inbox, Assignments: Slack"]')?.disabled).toBe(true)
})

it('opens the linked module and changes one Slack row independently', async () => {
  render({ team_id: 'T1', linked: true, mentions: true, assignments: true, categories: { 'support.assignments': true, 'support.mentions': true } })
  const assignment = container.querySelector<HTMLButtonElement>('[aria-label="Support inbox, Assignments: Slack"]')!
  expect(container.querySelector('details')?.open).toBe(true)
  expect(document.activeElement).toBe(assignment.closest('[tabindex="-1"]'))
  expect(assignment.getAttribute('aria-checked')).toBe('true')
  await act(async () => assignment.click())
  expect(slackService.updateNotificationPreferences).toHaveBeenCalledWith('ws', { categories: { 'support.assignments': false } })
  expect(client.getQueryData(key)).toMatchObject({ categories: { 'support.assignments': false, 'support.mentions': true } })
})

it('offers independent in-app, desktop and email choices before Slack is linked', () => {
  render({ linked: false, mentions: false, assignments: false })
  for (const [channel, enabled] of [['in-app', true], ['desktop', false], ['email', false]] as const) {
    const control = container.querySelector<HTMLButtonElement>(`[aria-label="Assignments: ${channel}"]`)
    expect(control?.disabled).toBe(false)
    expect(control?.getAttribute('aria-checked')).toBe(String(enabled))
  }
  act(() => container.querySelector<HTMLButtonElement>('[aria-label="Assignments: email"]')!.click())
  expect(saveChannels).toHaveBeenCalledWith({ channel_changes: { 'support.assignments': { email: true } } }, expect.anything())
})

it('defaults flow failures to in-app while routine flow activity stays quiet', () => {
  render({ linked: true, assignments: true, mentions: true }, 'automation')
  for (const [label, checked] of [
    ['Flow failures: in-app', true],
    ['Flow failures: desktop', false],
    ['Flow failures: email', false],
    ['Automation, Flow failures: Slack', false],
    ['Flow activity: in-app', false],
  ] as const) {
    expect(container.querySelector(`[aria-label="${label}"]`)?.getAttribute('aria-checked')).toBe(String(checked))
  }
})
