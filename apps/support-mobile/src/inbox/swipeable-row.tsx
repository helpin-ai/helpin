import {
  animate,
  motion,
  useDragControls,
  useMotionValue,
  useTransform,
  type MotionValue,
  type PanInfo,
} from 'motion/react'
import {
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from 'react'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { stackSpring } from '@mobile/lib/motion'

/** Drag distance (px) past which a release commits the action. */
export const SWIPE_ARM_THRESHOLD = 96
/** Fraction of the row's width past which the action auto-commits mid-drag. */
export const SWIPE_AUTO_RATIO = 0.6
/** Horizontal-vs-vertical dominance (px) required before a swipe engages, so it never fights vertical scrolling. */
const INTENT_LOCK_THRESHOLD = 10

export type SwipeGestureState = 'idle' | 'armed' | 'auto'

/**
 * Pure classification of a horizontal drag offset against a row width.
 * `idle` (< 96px) never commits; `armed` (>= 96px) commits on release;
 * `auto` (>= 60% of width) commits immediately, without waiting for release.
 */
export function swipeState(dx: number, width: number): SwipeGestureState {
  const abs = Math.abs(dx)
  if (width > 0 && abs >= width * SWIPE_AUTO_RATIO) return 'auto'
  if (abs >= SWIPE_ARM_THRESHOLD) return 'armed'
  return 'idle'
}

export interface SwipeAction {
  label: string
  icon: ComponentType<{ className?: string }>
  tone: 'primary' | 'success'
  onCommit: () => void
}

export interface SwipeableRowProps {
  leading?: SwipeAction
  trailing?: SwipeAction
  children: ReactNode
  className?: string
}

const TONE_CLASSES: Record<SwipeAction['tone'], string> = {
  primary: 'bg-primary text-primary-foreground',
  // Matches the emerald used by Badge's `success` tone (badge.tsx) so the
  // resolve action reads as the same "success" color across the app.
  success: 'bg-emerald-500 text-white',
}

// Module-level "close others" coordination: opening/dragging one row closes
// every other currently-mounted row so only one can be swiped open at a time.
type CloseFn = () => void
const openRows = new Set<CloseFn>()
function closeOtherRows(self: CloseFn) {
  for (const close of openRows) {
    if (close !== self) close()
  }
}

function ActionUnderlay({
  action,
  side,
  progress,
}: {
  action: SwipeAction
  side: 'left' | 'right'
  progress: MotionValue<number>
}) {
  const Icon = action.icon
  const scale = useTransform(progress, [0, SWIPE_ARM_THRESHOLD], [0.7, 1], { clamp: true })
  const opacity = useTransform(progress, [0, 12], [0, 1], { clamp: true })
  return (
    <div
      className={cn(
        'absolute inset-y-0 flex w-full items-center overflow-hidden',
        side === 'left' ? 'left-0 justify-start pl-5' : 'right-0 justify-end pr-5',
        TONE_CLASSES[action.tone],
      )}
      aria-hidden="true"
    >
      <motion.div style={{ scale, opacity }} className="flex flex-col items-center gap-1">
        <Icon className="h-5 w-5" />
        <span className="text-footnote font-medium">{action.label}</span>
      </motion.div>
    </div>
  )
}

/**
 * Swipeable list row: drag the content horizontally to reveal a leading
 * (swipe right) and/or trailing (swipe left) action underneath. Tracks the
 * finger 1:1 via a transform (no layout thrash); crossing 96px arms the
 * gesture (haptic), crossing 60% of the row's width auto-commits mid-drag.
 */
export function SwipeableRow({ leading, trailing, children, className }: SwipeableRowProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const x = useMotionValue(0)
  const trailingProgress = useTransform(x, (v) => -v)
  const dragControls = useDragControls()
  const [width, setWidth] = useState(0)
  const armedRef = useRef<SwipeGestureState>('idle')
  const committedRef = useRef(false)
  const intentRef = useRef<{ startX: number; startY: number; decided: boolean } | null>(null)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) setWidth(entry.contentRect.width)
    })
    observer.observe(el)
    setWidth(el.getBoundingClientRect().width)
    return () => observer.disconnect()
  }, [])

  // Stable identity across renders (captures only the stable `x` motion
  // value) so `closeOtherRows` can reliably exclude this row's own entry.
  const closeRef = useRef<CloseFn>(() => animate(x, 0, stackSpring))

  useEffect(() => {
    const close = closeRef.current
    openRows.add(close)
    return () => {
      openRows.delete(close)
    }
  }, [])

  // Fires the action at most once per gesture. Deliberately does NOT also
  // spring the row closed — while a framer-motion drag is in flight it keeps
  // driving `x` from the live pointer position every frame, so an animate()
  // call here would be clobbered on the very next move. The row always
  // tracks the finger 1:1 until release; `handleDragEnd` is the single place
  // that springs it closed, whether or not this already committed.
  const commit = (action: SwipeAction | undefined) => {
    if (!action || committedRef.current) return
    committedRef.current = true
    haptic('impactMedium')
    action.onCommit()
  }

  const handlePointerDown = (event: ReactPointerEvent) => {
    intentRef.current = { startX: event.clientX, startY: event.clientY, decided: false }
    armedRef.current = 'idle'
    committedRef.current = false
  }

  const handlePointerMove = (event: ReactPointerEvent) => {
    const intent = intentRef.current
    if (!intent || intent.decided) return
    const dx = event.clientX - intent.startX
    const dy = event.clientY - intent.startY
    if (Math.abs(dx) < INTENT_LOCK_THRESHOLD && Math.abs(dy) < INTENT_LOCK_THRESHOLD) return
    intent.decided = true
    if (Math.abs(dx) > Math.abs(dy)) {
      closeOtherRows(closeRef.current)
      dragControls.start(event, { snapToCursor: false })
    }
  }

  const handleDrag = (_event: PointerEvent | MouseEvent | TouchEvent, info: PanInfo) => {
    const offset = info.offset.x
    if ((offset > 0 && !leading) || (offset < 0 && !trailing)) return
    const state = swipeState(offset, width)
    if (state !== armedRef.current) {
      if (state === 'armed' || state === 'auto') haptic('impactLight')
      armedRef.current = state
    }
    if (state === 'auto') {
      commit(offset > 0 ? leading : trailing)
    }
  }

  const handleDragEnd = (_event: PointerEvent | MouseEvent | TouchEvent, info: PanInfo) => {
    const offset = info.offset.x
    const state = swipeState(offset, width)
    if (!committedRef.current && state !== 'idle') {
      commit(offset > 0 ? leading : trailing)
    }
    animate(x, 0, stackSpring)
  }

  const dragConstraints = {
    left: trailing ? -width : 0,
    right: leading ? width : 0,
  }

  return (
    <div
      ref={containerRef}
      className={cn('relative overflow-hidden', className)}
      style={{ touchAction: 'pan-y' }}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
    >
      {leading && <ActionUnderlay action={leading} side="left" progress={x} />}
      {trailing && <ActionUnderlay action={trailing} side="right" progress={trailingProgress} />}
      <motion.div
        drag="x"
        dragListener={false}
        dragControls={dragControls}
        dragConstraints={dragConstraints}
        dragElastic={0.15}
        dragMomentum={false}
        style={{ x }}
        onDrag={handleDrag}
        onDragEnd={handleDragEnd}
        className="relative bg-background"
      >
        {children}
      </motion.div>
    </div>
  )
}
