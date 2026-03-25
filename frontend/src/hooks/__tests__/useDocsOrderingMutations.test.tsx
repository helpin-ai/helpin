// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'

const captured = {
  reorderCollections: null as ReturnType<typeof import('@/hooks/queries/useDocs').useReorderDocsCollections> | null,
}

vi.mock('@/lib/services/docsService', () => ({
  docsService: {
    reorderSpaces: vi.fn(),
    reorderCollections: vi.fn(),
    reorderDocuments: vi.fn(),
  },
}))

import { docsService } from '@/lib/services/docsService'
import { useReorderDocsCollections } from '@/hooks/queries/useDocs'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function ReorderCollectionsHarness({ wsId }: { wsId: string }) {
  captured.reorderCollections = useReorderDocsCollections(wsId)
  return null
}

describe('useDocs ordering mutations', () => {
  afterEach(() => {
    captured.reorderCollections = null
    vi.clearAllMocks()
  })

  it('rejects API errors for collection reorders so optimistic UI can revert', async () => {
    vi.mocked(docsService.reorderCollections).mockResolvedValue({
      data: null,
      error: 'boom',
      status: 400,
    } as never)

    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <ReorderCollectionsHarness wsId="ws-1" />
        </QueryClientProvider>,
      )
    })

    let thrown: unknown = null
    await act(async () => {
      try {
        await captured.reorderCollections!.mutateAsync({
          spaceId: 'space-1',
          data: { collection_ids: ['col-1', 'col-2'] },
        })
      } catch (error) {
        thrown = error
      }
    })

    expect(thrown).toBeInstanceOf(Error)
    expect((thrown as Error).message).toContain('boom')

    act(() => root.unmount())
    container.remove()
  })

  it('invalidates the specific space collections query after a successful reorder', async () => {
    vi.mocked(docsService.reorderCollections).mockResolvedValue({
      data: { message: 'order updated' },
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidateSpy = vi.spyOn(client, 'invalidateQueries')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <QueryClientProvider client={client}>
          <ReorderCollectionsHarness wsId="ws-1" />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.reorderCollections!.mutateAsync({
        spaceId: 'space-1',
        data: { collection_ids: ['col-2', 'col-1'] },
      })
    })

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: queryKeys.docs.collections('ws-1', 'space-1'),
    })

    act(() => root.unmount())
    container.remove()
  })
})
