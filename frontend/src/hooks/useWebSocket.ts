import { useEffect, useRef, useCallback } from 'react'
import { API_BASE } from '@/lib/api'

export interface WSEvent {
  action: 'created' | 'updated' | 'deleted' | 'moved'
  entity: string
  entity_id: string
  workspace_id: string
  actor_id: string
  parent_type?: string
  parent_id?: string
}

interface UseWebSocketOptions {
  workspaceId: string
  onEvent: (event: WSEvent) => void
}

function getWSUrl(workspaceId: string): string {
  const token = localStorage.getItem('access_token')
  if (!token) return ''

  // Swap http(s) → ws(s) and replace trailing /api with /api/ws
  const base = API_BASE.replace(/^http/, 'ws').replace(/\/api\/?$/, '/api')
  return `${base}/ws?token=${encodeURIComponent(token)}&workspace_id=${encodeURIComponent(workspaceId)}`
}

export function useWebSocket({ workspaceId, onEvent }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const connect = useCallback(() => {
    const url = getWSUrl(workspaceId)
    if (!url) return

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[ws] connected')
      retriesRef.current = 0
    }

    ws.onmessage = (e) => {
      try {
        const event: WSEvent = JSON.parse(e.data)
        onEventRef.current(event)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      console.log('[ws] disconnected')
      wsRef.current = null
      // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s cap
      const delay = Math.min(1000 * Math.pow(2, retriesRef.current), 30000)
      retriesRef.current++
      console.log(`[ws] reconnecting in ${delay}ms`)
      timerRef.current = setTimeout(connect, delay)
    }

    ws.onerror = () => {
      // onclose will fire after this
      ws.close()
    }
  }, [workspaceId])

  useEffect(() => {
    connect()

    return () => {
      clearTimeout(timerRef.current)
      if (wsRef.current) {
        wsRef.current.onclose = null // prevent reconnect on intentional close
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [connect])
}
