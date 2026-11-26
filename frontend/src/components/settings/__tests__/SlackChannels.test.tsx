// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { expect, it, vi } from 'vitest'
import { SlackChannels } from '../channels/SlackChannels'
import { slackService } from '@/lib/services/slackService'

vi.mock('@/hooks/queries/useSupport', () => ({
  useInboxScopes: () => ({ data: { mailboxes: [
    { id: 'a', name: 'Billing' }, { id: 'b', name: 'Returns' }, { id: 'c', name: 'Orders' },
  ] } }),
}))
vi.mock('@/hooks/useAccessibleTeams', () => ({ useAccessibleTeams: () => ({ teams: [] }) }))
vi.mock('@/hooks/queries/useEpics', () => ({ useEpics: () => ({}) }))
vi.mock('@/hooks/queries/useCRM', () => ({ useCompany: () => ({}) }))
vi.mock('@/components/automation/CRMRecordPicker', () => ({ CRMRecordPicker: () => null }))
vi.mock('@/lib/services/slackService', () => ({
  slackService: { settings: vi.fn(), channels: vi.fn(), save: vi.fn() },
}))

it.each([
  { ids: ['a'], summary: 'from Billing,' },
  { ids: ['a', 'b'], summary: 'from Billing and Returns,' },
  { ids: ['a', 'b', 'c'], summary: 'from the 3 selected inboxes,', list: 'Billing, Returns, and Orders' },
])('shows the public audience for $ids and saves directly', async ({ ids, summary, list }) => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
  const route = {
    purpose: 'inbox' as const, channel_id: 'C1', channel_name: 'support',
    mailbox_ids: ids, mailbox_id: null, epic_id: null, team_id: null,
  }
  const settings = { connected: true, configured: true, routes: [route] }
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } })
  client.setQueryData(['slack', 'ws'], settings)
  client.setQueryData(['slackChannels', 'ws'], [{ id: 'C1', name: 'support', is_private: false, is_ext_shared: false }])
  vi.mocked(slackService.settings).mockResolvedValue(settings)
  const container = document.body.appendChild(document.createElement('div'))
  const root = createRoot(container)
  try {
    await act(async () => root.render(<QueryClientProvider client={client}>
      <SlackChannels workspaceId="ws" slug="test" initialInbox="a" hasSupport canManageFeed hasPM={false} />
    </QueryClientProvider>))
    const dialog = document.querySelector('[role=dialog]')!
    const save = [...dialog.querySelectorAll('button')].find(button => button.textContent === 'Save rule')!
    expect(save.disabled).toBe(false)
    const warning = dialog.querySelector('[role=status]')?.textContent
    for (const text of ['#support is public', summary, 'internal notes']) expect(warning).toContain(text)
    if (list) expect(warning).toContain(list)
    await act(async () => save.click())
    expect(slackService.save).toHaveBeenCalledWith('ws', { ...route, public_team_inboxes: true })
  } finally {
    act(() => root.unmount())
    container.remove()
    client.clear()
    vi.clearAllMocks()
  }
})
