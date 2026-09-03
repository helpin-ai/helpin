// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'

const stableMocks = vi.hoisted(() => ({
  access: {},
  empty: [] as never[],
  openCreate: vi.fn(),
  listEpics: vi.fn(),
  listLabels: vi.fn(),
  listObjectives: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock('@/hooks/useTitle', () => ({ useTitle: vi.fn() }))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: unknown) => unknown) => selector({
    currentWorkspace: { id: 'ws-1', slug: 'acme' },
  }),
}))

vi.mock('@/stores/globalCreateStore', () => ({
  useGlobalCreateStore: (selector: (state: unknown) => unknown) => selector({ openCreate: stableMocks.openCreate }),
}))

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => ({ teams: stableMocks.empty, findTeamName: () => null }),
}))

vi.mock('@/hooks/useAssignableWorkspaceMembers', () => ({
  useAssignableWorkspaceMembers: () => ({ members: stableMocks.empty }),
}))

vi.mock('@/hooks/queries', () => ({
  useWorkspaceAccess: () => ({ data: stableMocks.access }),
  usePermissions: () => ({ canEdit: true }),
  useEpicStates: () => ({ data: stableMocks.empty }),
}))

vi.mock('@/lib/services/pmEpicService', () => ({
  pmEpicService: { list: stableMocks.listEpics },
}))

vi.mock('@/lib/services/pmLabelService', () => ({
  pmLabelService: { list: stableMocks.listLabels },
}))

vi.mock('@/lib/services/pmObjectiveService', () => ({
  pmObjectiveService: { list: stableMocks.listObjectives },
}))

import { EpicsPage } from '../Epics'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('EpicsPage scroll layout', () => {
  beforeEach(() => {
    stableMocks.listEpics.mockResolvedValue({ data: [], error: null })
    stableMocks.listLabels.mockResolvedValue({ data: [], error: null })
    stableMocks.listObjectives.mockResolvedValue({ data: [], error: null })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    window.localStorage.clear()
    vi.clearAllMocks()
  })

  it('fills the route height so the virtual list can scroll vertically', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    await act(async () => {
      root.render(<QueryClientProvider client={queryClient}><TooltipProvider><EpicsPage /></TooltipProvider></QueryClientProvider>)
      await new Promise((resolve) => setTimeout(resolve, 25))
    })

    expect(container.firstElementChild?.classList.contains('h-full')).toBe(true)

    act(() => root.unmount())
  })

  it('renders cached epics without waiting for label reference data', async () => {
    const epics = [{
      epic: {
        id: 'epic-fast',
        workspace_id: 'ws-1',
        name: 'Fast initiative',
        health: 'no_health',
        archived: false,
        position: 0,
        started: false,
        completed: false,
        planning_state: 'not_started',
        spec_clarifications: [],
        created_at: '2026-09-03T10:00:00Z',
        updated_at: '2026-09-03T10:00:00Z',
      },
      labels: [],
      objectives: [],
      stats: {
        task_count: 0,
        done_task_count: 0,
        total_points: 0,
        done_points: 0,
        in_progress_count: 0,
        unstarted_count: 0,
      },
      suggested_health: 'no_health',
    }]
    stableMocks.listEpics.mockReturnValue(new Promise(() => {}))
    stableMocks.listLabels.mockReturnValue(new Promise(() => {}))
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    queryClient.setQueryData(
      ['pm', 'ws-1', 'epics', { archived: false, team_id: undefined }],
      epics,
    )
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<QueryClientProvider client={queryClient}><TooltipProvider><EpicsPage /></TooltipProvider></QueryClientProvider>)
      await new Promise((resolve) => setTimeout(resolve, 25))
    })

    expect(container.textContent).toContain('Show Archived')
    expect(container.textContent).not.toContain('Create your first epic')

    act(() => root.unmount())
  })
})
