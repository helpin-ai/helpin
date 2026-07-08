import { useEffect, useRef } from 'react'
import { animate, useMotionValue } from 'motion/react'
import { stackSpring } from '@mobile/lib/motion'

const EDGE_PX = 28
const INTENT_LOCK_PX = 10
const COMMIT_RATIO = 0.35
const COMMIT_VELOCITY = 0.5 // px per ms

export function useEdgeSwipeBack({ enabled, onBack }: { enabled: boolean; onBack: () => void }) {
  const swipeRef = useRef<HTMLDivElement | null>(null)
  const gestureX = useMotionValue(0)

  useEffect(() => {
    const el = swipeRef.current
    if (!el || !enabled) return

    let startX = 0
    let startY = 0
    let startT = 0
    let tracking = false
    let locked: 'horizontal' | 'vertical' | null = null

    const onStart = (e: TouchEvent) => {
      const t = e.touches[0]
      if (t.clientX > EDGE_PX) return
      tracking = true
      locked = null
      startX = t.clientX
      startY = t.clientY
      startT = performance.now()
    }
    const onMove = (e: TouchEvent) => {
      if (!tracking) return
      const t = e.touches[0]
      const dx = t.clientX - startX
      const dy = t.clientY - startY
      if (!locked && (Math.abs(dx) > INTENT_LOCK_PX || Math.abs(dy) > INTENT_LOCK_PX)) {
        locked = Math.abs(dx) > Math.abs(dy) ? 'horizontal' : 'vertical'
      }
      if (locked !== 'horizontal') return
      e.preventDefault() // needs { passive: false } — stops vertical scroll fighting the gesture
      gestureX.set(Math.max(0, dx))
    }
    const onEnd = (e: TouchEvent) => {
      if (!tracking) return
      tracking = false
      if (locked !== 'horizontal') return
      const dx = Math.max(0, e.changedTouches[0].clientX - startX)
      const dt = Math.max(1, performance.now() - startT)
      const commit = dx > el.clientWidth * COMMIT_RATIO || dx / dt > COMMIT_VELOCITY
      if (commit) {
        // No gestureX reset after onBack(): gesture values are per screen
        // instance, so this one belongs to the dying, already-swiped-off
        // screen — snapping it back to x:0 would visually "un-dismiss" it
        // before its exit replays. Fresh instances start at 0 anyway.
        animate(gestureX, el.clientWidth, { duration: 0.18, ease: 'easeOut' }).then(() => {
          onBack()
        })
      } else {
        animate(gestureX, 0, stackSpring)
      }
    }

    el.addEventListener('touchstart', onStart, { passive: true })
    el.addEventListener('touchmove', onMove, { passive: false })
    el.addEventListener('touchend', onEnd, { passive: true })
    el.addEventListener('touchcancel', onEnd, { passive: true })
    return () => {
      el.removeEventListener('touchstart', onStart)
      el.removeEventListener('touchmove', onMove)
      el.removeEventListener('touchend', onEnd)
      el.removeEventListener('touchcancel', onEnd)
    }
  }, [enabled, onBack, gestureX])

  return { swipeRef, gestureX }
}
