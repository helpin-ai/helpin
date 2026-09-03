// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'

import { settingsService } from '@/lib/services/settingsService'
import { useWorkspaceTeams } from '../useWorkspaceTeams'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

vi.mock('@/lib/services/settingsService', () => ({
  settingsService: { getAll: vi.fn() },
}))

const settingsPayload = {
  settings: {},
  teams: [
    { id: 't1', workspace_id: 'ws', name: 'platform' },
    { id: 't2', workspace_id: 'ws', name: 'Growth' },
  ],
  people: [
    { id: 'p1', name: 'Alice' },
    { id: 'p2', name: 'Bob' },
  ],
  memberships: [{ id: 'm1', team_id: 't1', person_id: 'p2' }],
  user_memberships: [{ id: 'um1', team_id: 't1', user_id: 'u1' }],
  managers: [],
  job_role_criteria: [],
  invitation_team_preassignments: [],
  team_estimate_settings: [],
  team_field_visibility: [],
  team_repo_defaults: [],
}

function Harness({ workspaceId }: { workspaceId: string | undefined }) {
  const result = useWorkspaceTeams(workspaceId)
  const snapshot = {
    loading: result.loading,
    teamNames: result.teams.map((t) => t.name),
    t1Name: result.findTeamName('t1') ?? null,
    noTeamName: result.findTeamName(undefined) ?? null,
    t1Members: result.getTeamMembers('t1').map((p) => p.id),
    userMemberships: result.userMemberships.length,
  }
  return <pre>{JSON.stringify(snapshot)}</pre>
}

async function mount(workspaceId: string | undefined) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const container = document.createElement('div')
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <QueryClientProvider client={client}>
        <Harness workspaceId={workspaceId} />
      </QueryClientProvider>,
    )
  })
  const read = () => JSON.parse(container.textContent ?? '{}') as {
    loading: boolean
    teamNames: string[]
    t1Name: string | null
    noTeamName: string | null
    t1Members: string[]
    userMemberships: number
  }
  const waitUntilLoaded = async () => {
    for (let i = 0; i < 50 && read().loading; i += 1) {
      await act(async () => { await new Promise((resolve) => setTimeout(resolve, 5)) })
    }
  }
  const unmount = async () => { await act(async () => { root.unmount() }) }
  return { read, waitUntilLoaded, unmount }
}

describe('useWorkspaceTeams', () => {
  it('reads teams from the settings query and capitalizes team names', async () => {
    vi.mocked(settingsService.getAll).mockResolvedValue({ data: settingsPayload, error: null } as never)

    const { read, waitUntilLoaded, unmount } = await mount('ws')
    await waitUntilLoaded()

    expect(read()).toEqual({
      loading: false,
      teamNames: ['Platform', 'Growth'],
      t1Name: 'Platform',
      noTeamName: null,
      t1Members: ['p2'],
      userMemberships: 1,
    })
    expect(settingsService.getAll).toHaveBeenCalledWith('ws')

    await unmount()
  })

  it('does not fetch and is not loading when no workspace id is given', async () => {
    vi.mocked(settingsService.getAll).mockClear()

    const { read, unmount } = await mount(undefined)

    expect(read()).toEqual({
      loading: false,
      teamNames: [],
      t1Name: null,
      noTeamName: null,
      t1Members: [],
      userMemberships: 0,
    })
    expect(settingsService.getAll).not.toHaveBeenCalled()

    await unmount()
  })
})
