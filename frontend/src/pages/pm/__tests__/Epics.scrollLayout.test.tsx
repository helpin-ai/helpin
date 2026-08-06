// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const stableMocks = vi.hoisted(() => ({
  access: {},
  empty: [] as never[],
  openCreate: vi.fn(),
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
  pmEpicService: { list: vi.fn(async () => ({ data: [], error: null })) },
}))

vi.mock('@/lib/services/pmLabelService', () => ({
  pmLabelService: { list: vi.fn(async () => ({ data: [], error: null })) },
}))

vi.mock('@/lib/services/pmObjectiveService', () => ({
  pmObjectiveService: { list: vi.fn(async () => ({ data: [], error: null })) },
}))

import { EpicsPage } from '../Epics'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('EpicsPage scroll layout', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    window.localStorage.clear()
    vi.clearAllMocks()
  })

  it('fills the route height so the virtual list can scroll vertically', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<EpicsPage />)
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(container.firstElementChild?.classList.contains('h-full')).toBe(true)

    act(() => root.unmount())
  })
})
