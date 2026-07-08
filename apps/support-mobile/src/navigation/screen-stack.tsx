import { useEffect, useRef, useState } from 'react'
import { Outlet, useRouter, useRouterState } from '@tanstack/react-router'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { stackSpring } from '@mobile/lib/motion'
import { useEdgeSwipeBack } from './use-edge-swipe-back'

export function resolveDirection(prevIndex: number, nextIndex: number): 'push' | 'pop' | 'replace' {
  if (nextIndex > prevIndex) return 'push'
  if (nextIndex < prevIndex) return 'pop'
  return 'replace'
}

/** Where "back" goes when there is no prior history entry (cold-start deep link). */
export function backFallbackPath(pathname: string): string {
  const match = pathname.match(/^\/w\/([^/]+)\//)
  return match ? `/w/${match[1]}/support` : '/workspaces'
}

/** Tab-level roots crossfade instead of sliding. */
const TAB_ROOTS = [/^\/w\/[^/]+\/support$/, /^\/w\/[^/]+\/you$/]
const isTabRoot = (path: string) => TAB_ROOTS.some((re) => re.test(path))

export function ScreenStack() {
  const router = useRouter()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const historyIndex = useRouterState({
    select: (s) => (s.location.state as { __TSR_index?: number }).__TSR_index ?? 0,
  })
  const prevIndexRef = useRef(historyIndex)
  const [direction, setDirection] = useState<'push' | 'pop' | 'replace'>('replace')

  useEffect(() => {
    setDirection(resolveDirection(prevIndexRef.current, historyIndex))
    prevIndexRef.current = historyIndex
  }, [historyIndex])

  const reduced = useReducedMotion()
  const crossfade = reduced || (isTabRoot(pathname) && direction !== 'pop')

  const swipeEnabled = !isTabRoot(pathname) && pathname !== '/login' && pathname !== '/workspaces'
  const goBack = () => {
    if (router.history.canGoBack()) router.history.back()
    else router.navigate({ to: backFallbackPath(pathname) })
  }
  const { swipeRef, gestureX } = useEdgeSwipeBack({ enabled: swipeEnabled, onBack: goBack })

  const variants = crossfade
    ? {
        initial: { opacity: 0 },
        animate: { opacity: 1 },
        exit: { opacity: 0 },
      }
    : {
        initial: { x: direction === 'pop' ? '-25%' : '100%' },
        animate: { x: 0 },
        exit: { x: direction === 'pop' ? '100%' : '-25%' },
      }

  return (
    <div className="relative h-dvh overflow-hidden bg-background">
      <AnimatePresence initial={false} mode="popLayout">
        {/* Outer div: Motion owns its transform (push/pop variants).
            Inner div: the gesture owns its transform (a plain motion value).
            Never let both write to the same element. */}
        <motion.div
          key={pathname}
          className="absolute inset-0 bg-background"
          variants={variants}
          initial="initial"
          animate="animate"
          exit="exit"
          transition={crossfade ? { duration: 0.15 } : stackSpring}
        >
          <motion.div ref={swipeRef} style={{ x: gestureX }} className="h-full">
            <Outlet />
          </motion.div>
        </motion.div>
      </AnimatePresence>
    </div>
  )
}
