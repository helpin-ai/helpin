// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { EpicWithStats } from '@/lib/pmTypes'
import { queryKeys } from '@/lib/queryKeys'
import { EPIC_PRESET_COLORS } from '@/components/pm/ColorPicker'

const stableMocks = vi.hoisted(() => ({
  access: {},
  empty: [] as never[],
  openCreate: vi.fn(),
  listEpics: vi.fn(),
  listLabels: vi.fn(),
  listObjectives: vi.fn(),
  updateEpic: vi.fn(),
  navigate: vi.fn(),
  canEdit: true,
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => stableMocks.navigate,
}))

vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () => Array.from({ length: count }, (_, index) => ({ index, start: index * 40 })),
    getTotalSize: () => count * 40,
  }),
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
  usePermissions: () => ({ canEdit: stableMocks.canEdit }),
  useEpicStates: () => ({ data: stableMocks.empty }),
}))

vi.mock('@/lib/services/pmEpicService', () => ({
  pmEpicService: { list: stableMocks.listEpics, update: stableMocks.updateEpic },
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
    stableMocks.canEdit = true
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

  it.each([false, true])('edits color from the list and handles a failed save (%s)', async (failSave) => {
    const entry = {
      epic: { id: 'epic-color', workspace_id: 'ws-1', name: 'Color initiative', color: '#788596', health: 'no_health', created_at: '2026-09-03T10:00:00Z', updated_at: '2026-09-03T10:00:00Z' },
      labels: [], objectives: [], stats: { task_count: 0, done_task_count: 0, total_points: 0, done_points: 0 },
    } as EpicWithStats
    stableMocks.listEpics.mockReturnValue(new Promise(() => {}))
    let resolveUpdate!: (value: unknown) => void
    stableMocks.updateEpic.mockReturnValue(new Promise((resolve) => { resolveUpdate = resolve }))
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const listKey = [...queryKeys.pm.epics('ws-1'), { archived: false, team_id: undefined }]
    queryClient.setQueryData(listKey, [entry])
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    await act(async () => root.render(<QueryClientProvider client={queryClient}><TooltipProvider><EpicsPage /></TooltipProvider></QueryClientProvider>))
    const click = async (label: string) => {
      const button = Array.from(document.querySelectorAll('button')).find((node) => node.getAttribute('aria-label') === label || node.textContent === label)
      expect(button, label).toBeDefined()
      await act(async () => button!.click())
    }
    await click('Change epic color')
    await click(`Select color ${EPIC_PRESET_COLORS[8]}`)
    await click('Apply')
    expect(stableMocks.navigate).not.toHaveBeenCalled()
    expect(stableMocks.updateEpic).toHaveBeenCalledWith('ws-1', 'epic-color', { color: EPIC_PRESET_COLORS[8] })
    expect(queryClient.getQueryData<EpicWithStats[]>(listKey)?.[0].epic.color).toBe(EPIC_PRESET_COLORS[8])
    const updated = { ...entry, epic: { ...entry.epic, color: EPIC_PRESET_COLORS[8] } }
    await act(async () => {
      resolveUpdate(failSave ? { data: null, error: 'Unable to save color' } : { data: updated, error: null })
      await new Promise((resolve) => setTimeout(resolve, 25))
    })
    expect(queryClient.getQueryData<EpicWithStats[]>(listKey)?.[0].epic.color).toBe(failSave ? '#788596' : EPIC_PRESET_COLORS[8])
    if (failSave) expect(container.textContent).toContain('Unable to save color')
    else expect(queryClient.getQueryData(queryKeys.pm.epic('ws-1', 'epic-color'))).toEqual(updated)
    act(() => root.unmount())
    queryClient.clear()
  })
})
