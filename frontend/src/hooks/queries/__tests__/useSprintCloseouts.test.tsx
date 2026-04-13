// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'
import type { SprintCloseoutListResponse, SprintCloseoutResponse } from '@/lib/pmTypes'

const captured = {
  closeout: null as ReturnType<typeof import('@/hooks/queries/useSprints').useSprintCloseout> | null,
  closeouts: null as ReturnType<typeof import('@/hooks/queries/useSprints').useSprintCloseouts> | null,
}

vi.mock('@/lib/services/pmSprintService', () => ({
  pmSprintService: {
    getCloseout: vi.fn(),
    listCloseouts: vi.fn(),
  },
}))

import { pmSprintService } from '@/lib/services/pmSprintService'
import { useSprintCloseout, useSprintCloseouts } from '@/hooks/queries/useSprints'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  captured.closeout = useSprintCloseout('ws-1', 'sprint-1')
  captured.closeouts = useSprintCloseouts('ws-1', { team_id: 'team-1' })
  return null
}

describe('useSprintCloseout queries', () => {
  afterEach(() => {
    captured.closeout = null
    captured.closeouts = null
    vi.clearAllMocks()
  })

  it('loads a single sprint closeout response', async () => {
    vi.mocked(pmSprintService.getCloseout).mockResolvedValue({
      data: {
        closeout: {
          id: 'closeout-1',
          sprint_id: 'sprint-1',
          workspace_id: 'ws-1',
          committed_count: 8,
          completed_count: 5,
          unfinished_count: 3,
          rolled_over_count: 2,
          committed_points: 21,
          completed_points: 13,
          unfinished_points: 8,
          rolled_over_points: 5,
          closed_at: '2026-04-13T00:00:00Z',
          created_at: '2026-04-13T00:00:00Z',
          updated_at: '2026-04-13T00:00:00Z',
        },
        rolled_in_from: [],
      } satisfies SprintCloseoutResponse,
      error: null,
      status: 200,
    } as never)
    vi.mocked(pmSprintService.listCloseouts).mockResolvedValue({
      data: { items: [] } satisfies SprintCloseoutListResponse,
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.closeout?.refetch()
    })

    expect(pmSprintService.getCloseout).toHaveBeenCalledWith('ws-1', 'sprint-1')
    expect(
      client.getQueryData<SprintCloseoutResponse>(queryKeys.pm.sprintCloseout('ws-1', 'sprint-1'))?.closeout?.completed_count,
    ).toBe(5)

    act(() => root.unmount())
    container.remove()
  })

  it('loads workspace closeout summaries with filters', async () => {
    vi.mocked(pmSprintService.getCloseout).mockResolvedValue({
      data: { closeout: null, rolled_in_from: [] } satisfies SprintCloseoutResponse,
      error: null,
      status: 200,
    } as never)
    vi.mocked(pmSprintService.listCloseouts).mockResolvedValue({
      data: {
        items: [{
          closeout_id: 'closeout-1',
          sprint_id: 'sprint-1',
          sprint_name: 'Sprint 1',
          team_id: 'team-1',
          team_name: 'Growth',
          committed_count: 8,
          completed_count: 5,
          unfinished_count: 3,
          rolled_over_count: 2,
          committed_points: 21,
          completed_points: 13,
          unfinished_points: 8,
          rolled_over_points: 5,
          completion_rate: 0.625,
          closed_at: '2026-04-13T00:00:00Z',
        }],
      } satisfies SprintCloseoutListResponse,
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.closeouts?.refetch()
    })

    expect(pmSprintService.listCloseouts).toHaveBeenCalledWith('ws-1', { team_id: 'team-1' })
    expect(
      client.getQueryData<SprintCloseoutListResponse>(queryKeys.pm.sprintCloseouts('ws-1', { team_id: 'team-1' }))?.items,
    ).toHaveLength(1)

    act(() => root.unmount())
    container.remove()
  })
})
