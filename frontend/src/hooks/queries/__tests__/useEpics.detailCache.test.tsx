// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'
import type { EpicWithStats } from '@/lib/pmTypes'

const serviceMocks = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
}))

vi.mock('@/lib/services/pmEpicService', () => ({
  pmEpicService: {
    get: serviceMocks.get,
    update: serviceMocks.update,
  },
}))

import { useEpic, useUpdateEpic } from '../useEpics'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const cachedEpic = {
  epic: {
    id: 'epic-2',
    workspace_id: 'ws-1',
    name: 'Cached epic',
  },
  labels: [],
  objectives: [],
  stats: { task_count: 12 },
} as EpicWithStats

function Harness({ onRender }: { onRender: (value: ReturnType<typeof useEpic>) => void }) {
  onRender(useEpic('ws-1', 'epic-2'))
  return null
}

describe('useEpic detail cache', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('stores the saved color and invalidates lists after a successful edit', async () => {
    const updatedEpic = { ...cachedEpic, epic: { ...cachedEpic.epic, color: '#4e8fea' } }
    serviceMocks.update.mockResolvedValue({ data: updatedEpic, error: null })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const listKey = [...queryKeys.pm.epics('ws-1'), undefined]
    client.setQueryData(listKey, [cachedEpic])
    client.setQueryData(queryKeys.pm.epic('ws-1', 'epic-2'), cachedEpic)
    let mutation: ReturnType<typeof useUpdateEpic>
    function EditHarness() { mutation = useUpdateEpic('ws-1'); return null }
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><EditHarness /></QueryClientProvider>))
    await act(async () => { await mutation.mutateAsync({ id: 'epic-2', color: '#4e8fea' }) })
    expect(serviceMocks.update).toHaveBeenCalledWith('ws-1', 'epic-2', { color: '#4e8fea' })
    expect(client.getQueryData<EpicWithStats>(queryKeys.pm.epic('ws-1', 'epic-2'))?.epic.color).toBe('#4e8fea')
    expect(client.getQueryState(listKey)?.isInvalidated).toBe(true)
    act(() => root.unmount())
    client.clear()
  })

  it('retains the saved color when an update fails', async () => {
    serviceMocks.update.mockResolvedValue({ data: null, error: 'Unable to save color' })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const detailKey = queryKeys.pm.epic('ws-1', 'epic-2')
    const saved = { ...cachedEpic, epic: { ...cachedEpic.epic, color: '#788596' } }
    client.setQueryData(detailKey, saved)
    let mutation: ReturnType<typeof useUpdateEpic>
    function EditHarness() { mutation = useUpdateEpic('ws-1'); return null }
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><EditHarness /></QueryClientProvider>))
    await act(async () => {
      await expect(mutation.mutateAsync({ id: 'epic-2', color: '#4e8fea' })).rejects.toThrow('Unable to save color')
    })
    expect(client.getQueryData(detailKey)).toEqual(saved)
    act(() => root.unmount())
    client.clear()
  })

  it('shows an epic cached by a filtered list while its detail refresh is pending', async () => {
    serviceMocks.get.mockReturnValue(new Promise(() => {}))
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData(
      queryKeys.pm.epicTasks('ws-1', 'epic-2'),
      [{ id: 'task-1', epic: { id: 'epic-2' } }],
    )
    client.setQueryData(
      [...queryKeys.pm.epics('ws-1'), { archived: false, team_id: 'team-1' }],
      [cachedEpic],
    )
    const renders: Array<ReturnType<typeof useEpic>> = []
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness onRender={(value) => renders.push(value)} />
        </QueryClientProvider>,
      )
      await Promise.resolve()
    })

    expect(renders.at(-1)?.data).toEqual(cachedEpic)
    expect(renders.at(-1)?.isLoading).toBe(false)
    expect(serviceMocks.get).toHaveBeenCalledWith('ws-1', 'epic-2')

    act(() => root.unmount())
  })
})
