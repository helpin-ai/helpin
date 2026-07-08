import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { useSupportRealtimeStore } from '@helpin-ai/support-core'

export type BannerVisibility = 'hidden' | 'offline' | 'reconnecting'

/** Realtime statuses that should NOT show the banner (startup + steady state). */
const CONNECTED_LIKE_STATUSES = new Set(['idle', 'connecting', 'connected'])

/**
 * Pure decision function for the offline/reconnecting banner.
 *  - Device/network offline always wins, regardless of realtime status.
 *  - Online but the realtime socket isn't connected/starting (reconnecting,
 *    stale, disconnected, controller-reported offline, auth_expired) shows
 *    "Reconnecting…".
 *  - Otherwise ('idle' pre-connect, 'connecting', 'connected') stays hidden,
 *    so the banner never flashes during ordinary app startup.
 */
export function bannerState(online: boolean, realtimeStatus: string): BannerVisibility {
  if (!online) return 'offline'
  if (CONNECTED_LIKE_STATUSES.has(realtimeStatus)) return 'hidden'
  return 'reconnecting'
}

function useOnline(): boolean {
  const [online, setOnline] = useState(() => (typeof navigator === 'undefined' ? true : navigator.onLine))

  useEffect(() => {
    const handleOnline = () => setOnline(true)
    const handleOffline = () => setOnline(false)
    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)
    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])

  return online
}

const COPY: Record<Exclude<BannerVisibility, 'hidden'>, string> = {
  offline: "You're offline",
  reconnecting: 'Reconnecting…',
}

/** Slim amber status bar, meant to sit directly under a screen's `<TopBar>`. */
export function OfflineBanner() {
  const online = useOnline()
  const realtimeStatus = useSupportRealtimeStore((state) => state.status)
  const state = bannerState(online, realtimeStatus)

  return (
    <AnimatePresence initial={false}>
      {state !== 'hidden' && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: 28, opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={{ duration: 0.2 }}
          className="overflow-hidden bg-amber-500/15"
        >
          <div className="flex h-[28px] items-center justify-center text-footnote text-amber-700 dark:text-amber-400">
            {COPY[state]}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
