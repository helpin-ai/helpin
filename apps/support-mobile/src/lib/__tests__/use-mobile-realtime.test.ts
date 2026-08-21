import { createElement, type ReactNode } from 'react'
import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { supportQueryKeys, useSupportRealtime } from '@helpin-ai/support-core'
import { useMobileRealtime } from '../use-mobile-realtime'

vi.mock('@helpin-ai/support-core', async () => {
  const actual = await vi.importActual<typeof import('@helpin-ai/support-core')>('@helpin-ai/support-core')
  return { ...actual, useSupportRealtime: vi.fn() }
})

const mockUseSupportRealtime = vi.mocked(useSupportRealtime)

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { value: state, configurable: true })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('useMobileRealtime', () => {
  let queryClient: QueryClient
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- vi.spyOn's overload for a generic method (invalidateQueries<TTaggedQueryKey>) doesn't narrow cleanly to a storable type.
  let invalidateSpy: any

  beforeEach(() => {
    vi.useFakeTimers()
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    setVisibility('visible')
  })

  afterEach(() => {
    vi.useRealTimers()
    mockUseSupportRealtime.mockReset()
  })

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children)
  }

  test('delegates to the shared support-core realtime controller', () => {
    renderHook(() => useMobileRealtime('ws-1', 'conv-1'), { wrapper })
    expect(mockUseSupportRealtime).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId: 'ws-1', selectedConversationId: 'conv-1' }),
    )
  })

  test('does not invalidate when hidden for less than the resume gap', () => {
    renderHook(() => useMobileRealtime('ws-1', null), { wrapper })

    setVisibility('hidden')
    vi.advanceTimersByTime(30_000)
    setVisibility('visible')

    expect(invalidateSpy).not.toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: supportQueryKeys.conversations('ws-1') }),
    )
  })

  test('invalidates conversations + unread-stats after a visibility resume exceeding the gap', () => {
    renderHook(() => useMobileRealtime('ws-1', null), { wrapper })

    setVisibility('hidden')
    vi.advanceTimersByTime(61_000)
    setVisibility('visible')

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: supportQueryKeys.conversations('ws-1') }),
    )
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: supportQueryKeys.unreadStats('ws-1') }),
    )
  })

  test('is a no-op with no workspaceId (no listeners attached, no crash)', () => {
    expect(() => {
      renderHook(() => useMobileRealtime('', null), { wrapper })
      setVisibility('hidden')
      vi.advanceTimersByTime(61_000)
      setVisibility('visible')
    }).not.toThrow()

    expect(invalidateSpy).not.toHaveBeenCalled()
  })
})
