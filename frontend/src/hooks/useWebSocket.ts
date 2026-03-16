import { useEffect, useRef, useCallback, useState } from 'react'
import { API_BASE } from '@/lib/api'

export interface WSEvent {
  action: 'created' | 'updated' | 'deleted' | 'moved' | 'typing_started' | 'typing_stopped' | 'viewing_started' | 'viewing_stopped'
  entity: string
  entity_id: string
  workspace_id: string
  actor_id: string
  parent_type?: string
  parent_id?: string
  data?: Record<string, unknown>
}

// Snapshot sent by server when agent starts viewing a conversation
export interface PresenceSnapshot {
  viewers: string[]
  typers: Record<string, string>
}

interface UseWebSocketOptions {
  workspaceId: string
  onEvent: (event: WSEvent) => void
  onPresenceSnapshot?: (conversationId: string, snapshot: PresenceSnapshot) => void
}

export type WSSend = (type: string, data: Record<string, unknown>) => void

function getWSUrl(workspaceId: string): string {
  const token = localStorage.getItem('access_token')
  if (!token || !workspaceId) return ''

  // Swap http(s) → ws(s) and replace trailing /api with /api/ws
  const base = API_BASE.replace(/^http/, 'ws').replace(/\/api\/?$/, '/api')
  return `${base}/ws?token=${encodeURIComponent(token)}&workspace_id=${encodeURIComponent(workspaceId)}`
}

export function useWebSocket({ workspaceId, onEvent, onPresenceSnapshot }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent
  const onSnapshotRef = useRef(onPresenceSnapshot)
  onSnapshotRef.current = onPresenceSnapshot
  const [isConnected, setIsConnected] = useState(false)

  const send: WSSend = useCallback((type, data) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, data }))
    }
  }, [])

  const connect = useCallback(() => {
    const url = getWSUrl(workspaceId)
    if (!url) return

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[ws] connected')
      retriesRef.current = 0
      setIsConnected(true)
    }

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        // Handle presence snapshot (sent as {type, data} envelope)
        if (msg.type === 'support:presence_snapshot' && msg.data) {
          const snapshot = msg.data as PresenceSnapshot & { conversation_id?: string }
          // The snapshot is sent in response to viewing:start — the conversation_id
          // is not in the snapshot itself; the client tracks which conversation it asked about.
          onSnapshotRef.current?.('', snapshot)
          return
        }
        // Standard event (has action/entity fields)
        if (msg.action && msg.entity) {
          onEventRef.current(msg as WSEvent)
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      console.log('[ws] disconnected')
      wsRef.current = null
      setIsConnected(false)
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
      clearTimeout(timerRef.current ?? undefined)
      if (wsRef.current) {
        wsRef.current.onclose = null // prevent reconnect on intentional close
        wsRef.current.close()
        wsRef.current = null
      }
      setIsConnected(false)
    }
  }, [connect])

  return { send, isConnected }
}
