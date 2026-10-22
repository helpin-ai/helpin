import { useEffect, useRef } from 'react'
import { customerPortalSocketUrl } from '@/lib/services/customerPortalService'

/** Refetch this often while the tab is visible and the socket is down. */
export const PORTAL_POLL_INTERVAL_MS = 25_000
const MAX_RECONNECT_DELAY_MS = 30_000

interface PortalLiveUpdateHandlers {
  /** A request changed; `null` means refresh everything (catch-up or poll). */
  onChange: (reference: string | null) => void
  onTyping?: (reference: string, typing: boolean) => void
}

type PortalSignal = { type: 'request:changed'; reference: string } | { type: 'request:typing'; reference: string; typing: boolean }

function parseSignal(data: unknown): PortalSignal | null {
  if (typeof data !== 'string') return null
  try {
    const signal = JSON.parse(data) as Partial<PortalSignal> & { typing?: boolean }
    if (typeof signal.reference !== 'string') return null
    if (signal.type === 'request:changed') return { type: signal.type, reference: signal.reference }
    if (signal.type === 'request:typing') return { type: signal.type, reference: signal.reference, typing: Boolean(signal.typing) }
  } catch {
    // Ignore malformed frames; the poll fallback keeps the page current.
  }
  return null
}

/**
 * usePortalLiveUpdates keeps portal pages current like chat does. A socket
 * delivers change and typing signals for the customer's own requests (never
 * content); the page refetches through the portal API. If the socket is
 * unavailable, it polls while the tab is visible and catches up on return.
 */
export function usePortalLiveUpdates(slug: string, enabled: boolean, handlers: PortalLiveUpdateHandlers) {
  const handlersRef = useRef(handlers)
  useEffect(() => {
    handlersRef.current = handlers
  })

  useEffect(() => {
    if (!enabled || typeof window === 'undefined') return undefined
    let socket: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let attempts = 0
    let stopped = false

    const connect = () => {
      if (stopped || typeof WebSocket === 'undefined') return
      socket = new WebSocket(customerPortalSocketUrl(slug))
      socket.onopen = () => {
        // Anything missed while disconnected is picked up by one refresh.
        if (attempts > 0) handlersRef.current.onChange(null)
        attempts = 0
      }
      socket.onmessage = (event) => {
        const signal = parseSignal(event.data)
        if (!signal) return
        if (signal.type === 'request:changed') handlersRef.current.onChange(signal.reference)
        else handlersRef.current.onTyping?.(signal.reference, signal.typing)
      }
      socket.onclose = () => {
        socket = null
        if (stopped) return
        attempts += 1
        const delay = Math.min(MAX_RECONNECT_DELAY_MS, 1000 * 2 ** Math.min(attempts - 1, 5))
        reconnectTimer = setTimeout(connect, delay)
      }
    }
    connect()

    const poll = setInterval(() => {
      if (document.visibilityState === 'visible' && socket?.readyState !== WebSocket.OPEN) handlersRef.current.onChange(null)
    }, PORTAL_POLL_INTERVAL_MS)
    const onVisible = () => {
      if (document.visibilityState === 'visible') handlersRef.current.onChange(null)
    }
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      stopped = true
      clearTimeout(reconnectTimer)
      clearInterval(poll)
      document.removeEventListener('visibilitychange', onVisible)
      if (socket) {
        socket.onclose = null
        socket.close()
      }
    }
  }, [enabled, slug])
}
