import { act, renderHook } from '@testing-library/react'
import type { PointerEvent, RefObject } from 'react'
import { activeGesture, claimGesture, releaseGesture } from '../gesture-claim'
import { usePullToRefresh } from '../use-pull-to-refresh'

function scrollRefAtTop(): RefObject<HTMLElement> {
  return { current: { scrollTop: 0 } as HTMLElement }
}

function pointer(clientY: number): PointerEvent {
  return { clientY } as PointerEvent
}

afterEach(() => {
  const owner = activeGesture()
  if (owner) releaseGesture(owner)
})

test('an unclaimed downward pull tracks rubber-banded distance and claims "pull"', () => {
  const { result } = renderHook(() => usePullToRefresh({ scrollRef: scrollRefAtTop(), onRefresh: async () => {} }))
  act(() => result.current.handlers.onPointerDown(pointer(0)))
  act(() => result.current.handlers.onPointerMove(pointer(40)))
  expect(result.current.pullDistance).toBe(20)
  expect(activeGesture()).toBe('pull')
})

test('an active row-swipe claim zeroes pull distance and ignores the rest of the stream', () => {
  const { result } = renderHook(() => usePullToRefresh({ scrollRef: scrollRefAtTop(), onRefresh: async () => {} }))
  act(() => result.current.handlers.onPointerDown(pointer(0)))
  // Small drift: below the 10px claim threshold, so 'pull' is not claimed yet.
  act(() => result.current.handlers.onPointerMove(pointer(8)))
  expect(result.current.pullDistance).toBe(4)

  // A row locks horizontal intent mid-gesture.
  claimGesture('row-swipe')
  act(() => result.current.handlers.onPointerMove(pointer(120)))
  expect(result.current.pullDistance).toBe(0)

  // Further moves stay ignored until the pointer lifts.
  act(() => result.current.handlers.onPointerMove(pointer(200)))
  expect(result.current.pullDistance).toBe(0)
  expect(activeGesture()).toBe('row-swipe')
})

test('unmounting mid-gesture releases a claimed "pull" token', () => {
  const { result, unmount } = renderHook(() =>
    usePullToRefresh({ scrollRef: scrollRefAtTop(), onRefresh: async () => {} }),
  )
  act(() => result.current.handlers.onPointerDown(pointer(0)))
  act(() => result.current.handlers.onPointerMove(pointer(60)))
  expect(activeGesture()).toBe('pull')

  // Screen unmounts (e.g. navigation) before pointerup/pointercancel fires —
  // the claim must not leak and block row-swipe gestures elsewhere.
  unmount()
  expect(activeGesture()).toBeNull()
})

test('releasing an unarmed pull frees the claim and resets distance', () => {
  const { result } = renderHook(() => usePullToRefresh({ scrollRef: scrollRefAtTop(), onRefresh: async () => {} }))
  act(() => result.current.handlers.onPointerDown(pointer(0)))
  act(() => result.current.handlers.onPointerMove(pointer(60)))
  expect(activeGesture()).toBe('pull')
  act(() => result.current.handlers.onPointerUp())
  expect(result.current.pullDistance).toBe(0)
  expect(result.current.refreshing).toBe(false)
  expect(activeGesture()).toBeNull()
})
