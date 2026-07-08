import { useRef, useState, type PointerEvent, type RefObject } from 'react'
import { haptic } from '@mobile/lib/haptics'

/** Distance (px) of downward overscroll required to arm a refresh. */
export const PULL_ARM_THRESHOLD = 70

/** Rubber-band resistance applied to the raw pointer delta while pulling. */
export function rubberBand(dy: number): number {
  return dy * 0.5
}

export interface UsePullToRefreshOptions {
  scrollRef: RefObject<HTMLElement | null>
  onRefresh: () => Promise<void>
}

export interface UsePullToRefreshResult {
  /** Rubber-banded distance (px) the spinner container should expand to. */
  pullDistance: number
  /** True from the moment the gesture arms+releases until `onRefresh` settles. */
  refreshing: boolean
  /** Attach to the scroll container's pointer handlers. */
  handlers: {
    onPointerDown: (event: PointerEvent) => void
    onPointerMove: (event: PointerEvent) => void
    onPointerUp: () => void
    onPointerCancel: () => void
  }
}

/**
 * Pull-to-refresh gesture recognizer for a scrollable container. Only engages
 * when the container is already scrolled to the top (scrollTop === 0), so it
 * never fights a normal downward scroll mid-list. Uses Pointer Events so the
 * same code path drives touch, pen, and mouse-drag (handy for desktop preview
 * testing of the Tauri shell).
 */
export function usePullToRefresh({ scrollRef, onRefresh }: UsePullToRefreshOptions): UsePullToRefreshResult {
  const [pullDistance, setPullDistance] = useState(0)
  const [refreshing, setRefreshing] = useState(false)
  const startYRef = useRef<number | null>(null)
  const armedRef = useRef(false)
  const trackingRef = useRef(false)

  const reset = () => {
    trackingRef.current = false
    armedRef.current = false
    startYRef.current = null
  }

  const onPointerDown = (event: PointerEvent) => {
    if (refreshing) return
    const el = scrollRef.current
    if (!el || el.scrollTop > 0) return
    trackingRef.current = true
    armedRef.current = false
    startYRef.current = event.clientY
  }

  const onPointerMove = (event: PointerEvent) => {
    if (!trackingRef.current || startYRef.current === null) return
    const el = scrollRef.current
    if (el && el.scrollTop > 0) {
      // Scrolled away from the top mid-gesture — bail out cleanly.
      reset()
      setPullDistance(0)
      return
    }
    const rawDelta = event.clientY - startYRef.current
    if (rawDelta <= 0) {
      setPullDistance(0)
      return
    }
    const distance = rubberBand(rawDelta)
    setPullDistance(distance)
    if (!armedRef.current && distance >= PULL_ARM_THRESHOLD) {
      armedRef.current = true
      haptic('impactLight')
    }
  }

  const finish = () => {
    if (!trackingRef.current) return
    const wasArmed = armedRef.current
    reset()

    if (!wasArmed) {
      setPullDistance(0)
      return
    }

    setRefreshing(true)
    void onRefresh().finally(() => {
      setRefreshing(false)
      setPullDistance(0)
    })
  }

  return {
    pullDistance: refreshing ? Math.max(pullDistance, PULL_ARM_THRESHOLD) : pullDistance,
    refreshing,
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp: finish,
      onPointerCancel: finish,
    },
  }
}
