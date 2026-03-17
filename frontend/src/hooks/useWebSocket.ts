import { useEffect, useRef, useCallback, useState } from 'react'
import { create } from 'zustand'
import { API_BASE } from '@/lib/api'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'

export interface WSEvent {
  action: 'created' | 'updated' | 'deleted' | 'moved' | 'typing_started' | 'typing_stopped' | 'viewing_started' | 'viewing_stopped' | 'visitor_online' | 'visitor_offline'
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

export const useWSStore = create<{ send: ((data: unknown) => void) | null }>(() => ({ send: null }))

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
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
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
      const sendFn = (data: unknown) => {
        if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(data))
      }
      useWSStore.setState({ send: sendFn })

      // Start keepalive ping every 45s to refresh server-side presence keys.
      pingIntervalRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'support:ping', data: {} }))
        }
      }, 45_000)
    }

    ws.onmessage = (e) => {
      try {
        const parsed = JSON.parse(e.data)
        // Handle online visitors snapshot (sent on agent connect)
        if (parsed.type === 'support:online_visitors' && parsed.data?.visitors) {
          useSupportPresenceStore.getState().setOnlineVisitors(parsed.data.visitors as string[])
        } else if (parsed.session_id && parsed.type) {
          // Stream event — dispatch as DOM CustomEvent for usePlanningStream
          window.dispatchEvent(new CustomEvent('planning-stream', { detail: parsed }))
        } else if (parsed.type === 'support:presence_snapshot' && parsed.data) {
          // Handle presence snapshot (sent as {type, data} envelope)
          const snapshot = parsed.data as PresenceSnapshot & { conversation_id?: string }
          onSnapshotRef.current?.('', snapshot)
        } else if (parsed.action && parsed.entity) {
          // Standard event (has action/entity fields)
          onEventRef.current(parsed as WSEvent)
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      console.log('[ws] disconnected')
      wsRef.current = null
      setIsConnected(false)
      useWSStore.setState({ send: null })
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current)
        pingIntervalRef.current = null
      }
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
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current)
        pingIntervalRef.current = null
      }
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
