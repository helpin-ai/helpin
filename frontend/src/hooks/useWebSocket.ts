import { useEffect, useRef, useCallback, useState } from 'react'
import { create } from 'zustand'
import { API_BASE } from '@/lib/api'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { buildWorkspaceWebSocketUrl } from '@helpin-ai/support-core'

export interface WSEvent {
  event_id?: string
  sent_at?: string
  action: 'created' | 'updated' | 'deleted' | 'moved' | 'reordered' | 'typing_started' | 'typing_stopped' | 'viewing_started' | 'viewing_stopped' | 'editing_updated' | 'editing_stopped' | 'visitor_online' | 'visitor_offline'
  entity: string
  entity_id: string
  workspace_id: string
  actor_id: string
  parent_type?: string
  parent_id?: string
  data?: Record<string, unknown>
}

// Snapshot sent by server when agent starts viewing a conversation
export interface PresenceSnapshotViewer {
  user_id: string
  name?: string
  avatar?: string
}

export interface PresenceSnapshotTyper {
  content: string
  name?: string
  avatar?: string
}

export interface PresenceSnapshot {
  conversation_id: string
  viewers: PresenceSnapshotViewer[]
  typers: Record<string, PresenceSnapshotTyper>
}

export interface DocsPresenceSnapshot {
  document_id: string
  viewers: PresenceSnapshotViewer[]
  editors: Record<string, DocsPresenceSnapshotEditor>
}

export interface DocsPresenceSnapshotEditor {
  user_id: string
  area: string
  section?: string
  name?: string
  avatar?: string
}

export const useWSStore = create<{ send: ((data: unknown) => void) | null }>(() => ({ send: null }))

interface UseWebSocketOptions {
  workspaceId: string
  onEvent: (event: WSEvent) => void
  onPresenceSnapshot?: (snapshot: PresenceSnapshot) => void
  onDocsPresenceSnapshot?: (snapshot: DocsPresenceSnapshot) => void
}

export type WSSend = (type: string, data: Record<string, unknown>) => void

function getWSUrl(workspaceId: string): string {
  return buildWorkspaceWebSocketUrl(API_BASE, workspaceId)
}

export function useWebSocket({ workspaceId, onEvent, onPresenceSnapshot, onDocsPresenceSnapshot }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const activityDirtyRef = useRef(false)
  const lastActivitySampleAtRef = useRef(0)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent
  const onSnapshotRef = useRef(onPresenceSnapshot)
  onSnapshotRef.current = onPresenceSnapshot
  const onDocsSnapshotRef = useRef(onDocsPresenceSnapshot)
  onDocsSnapshotRef.current = onDocsPresenceSnapshot
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
          const active = activityDirtyRef.current
          ws.send(JSON.stringify({ type: 'support:ping', data: active ? { active: true } : {} }))
          activityDirtyRef.current = false
        }
      }, 45_000)
    }

    ws.onmessage = (e) => {
      try {
        const parsed = JSON.parse(e.data)
        // Handle online visitors snapshot (sent on agent connect)
        if (parsed.type === 'support:online_visitors' && parsed.data?.visitors) {
          useSupportPresenceStore.getState().setOnlineVisitors(parsed.data.visitors as string[])
        } else if (parsed.type === 'support:presence_snapshot' && parsed.data) {
          const snapshot = parsed.data as PresenceSnapshot
          onSnapshotRef.current?.(snapshot)
        } else if (parsed.type === 'docs:presence_snapshot' && parsed.data) {
          const snapshot = parsed.data as DocsPresenceSnapshot
          onDocsSnapshotRef.current?.(snapshot)
        } else if (parsed.action && parsed.entity) {
          // Standard event (has action/entity fields)
          const event = parsed as WSEvent
          onEventRef.current(event)
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
    if (!workspaceId) return

    const markActivity = () => {
      activityDirtyRef.current = true
      lastActivitySampleAtRef.current = Date.now()
    }
    const sampleActivity = () => {
      const now = Date.now()
      if (now - lastActivitySampleAtRef.current < 15_000) return
      markActivity()
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        markActivity()
      }
    }

    window.addEventListener('pointerdown', markActivity, { passive: true })
    window.addEventListener('keydown', markActivity)
    window.addEventListener('focus', markActivity)
    window.addEventListener('wheel', sampleActivity, { passive: true })
    window.addEventListener('pointermove', sampleActivity, { passive: true })
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      window.removeEventListener('pointerdown', markActivity)
      window.removeEventListener('keydown', markActivity)
      window.removeEventListener('focus', markActivity)
      window.removeEventListener('wheel', sampleActivity)
      window.removeEventListener('pointermove', sampleActivity)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
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
      activityDirtyRef.current = false
      setIsConnected(false)
    }
  }, [connect])

  return { send, isConnected }
}
