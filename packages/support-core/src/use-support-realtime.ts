import { useCallback, useEffect, useRef } from 'react'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { buildWorkspaceWebSocketUrl } from './auth-api'
import { supportQueryKeys } from './support-query-keys'
import { useSupportPresenceStore } from './support-presence-store'
import { useSupportRealtimeStore } from './support-realtime-store'

export interface SupportRealtimeEvent {
  event_id?: string
  sent_at?: string
  action:
    | 'created'
    | 'updated'
    | 'deleted'
    | 'moved'
    | 'reordered'
    | 'typing_started'
    | 'typing_stopped'
    | 'viewing_started'
    | 'viewing_stopped'
    | 'editing_updated'
    | 'editing_stopped'
    | 'visitor_online'
    | 'visitor_offline'
  entity: string
  entity_id: string
  workspace_id: string
  actor_id: string
  parent_type?: string
  parent_id?: string
  data?: Record<string, unknown>
}

interface SupportRealtimeOptions {
  apiBase: string
  workspaceId: string
  selectedConversationId?: string | null
}

const PING_INTERVAL_MS = 45_000
const RESUME_GAP_MS = 60_000
const TYPING_TIMEOUT_MS = 10_000

interface PresenceSnapshotViewer {
  user_id: string
  name?: string
  avatar?: string
}

interface PresenceSnapshotTyper {
  content: string
  name?: string
  avatar?: string
}

interface PresenceSnapshot {
  conversation_id: string
  viewers: PresenceSnapshotViewer[]
  typers: Record<string, PresenceSnapshotTyper>
}

interface OnlineVisitorsPayload {
  visitors?: string[]
}

function invalidateSupportRealtimeQueries(
  queryClient: QueryClient,
  workspaceId: string,
  conversationId?: string | null,
) {
  queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
  queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
  queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
  queryClient.invalidateQueries({ queryKey: supportQueryKeys.teammatePresence(workspaceId) })

  if (conversationId) {
    queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
    queryClient.invalidateQueries({ queryKey: supportQueryKeys.messages(workspaceId, conversationId) })
    queryClient.invalidateQueries({ queryKey: supportQueryKeys.visitorContext(workspaceId, conversationId) })
  }
}

export function useSupportRealtime({
  apiBase,
  workspaceId,
  selectedConversationId,
}: SupportRealtimeOptions) {
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const wakeCheckRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const typingTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  const retryCountRef = useRef(0)
  const intentionalCloseRef = useRef(false)
  const shouldResyncOnOpenRef = useRef(false)
  const lastWakeSampleAtRef = useRef(Date.now())
  const latestSelectedConversationIdRef = useRef<string | null>(selectedConversationId ?? null)
  latestSelectedConversationIdRef.current = selectedConversationId ?? null

  const setRealtimeStatus = useSupportRealtimeStore((state) => state.setStatus)
  const markConnected = useSupportRealtimeStore((state) => state.markConnected)
  const markEvent = useSupportRealtimeStore((state) => state.markEvent)
  const markResynced = useSupportRealtimeStore((state) => state.markResynced)
  const resetRealtime = useSupportRealtimeStore((state) => state.reset)

  const clearTimers = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current)
      pingIntervalRef.current = null
    }
    typingTimersRef.current.forEach((timer) => clearTimeout(timer))
    typingTimersRef.current.clear()
  }, [])

  const runResync = useCallback(
    (conversationId?: string | null) => {
      if (!workspaceId) {
        return
      }
      invalidateSupportRealtimeQueries(queryClient, workspaceId, conversationId ?? latestSelectedConversationIdRef.current)
      markResynced()
    },
    [markResynced, queryClient, workspaceId],
  )

  const handleSupportEvent = useCallback(
    (event: SupportRealtimeEvent) => {
      markEvent()

      if (event.entity === 'support_conversation_message') {
        const conversationId = event.parent_id || latestSelectedConversationIdRef.current
        if (conversationId && event.actor_id?.startsWith('widget:')) {
          useSupportPresenceStore.getState().setTyping(conversationId, false)
        } else if (conversationId && event.actor_id) {
          useSupportPresenceStore.getState().clearOneAgentTyping(conversationId, event.actor_id)
        }
        runResync(conversationId)
        return
      }

      if (event.entity === 'support_conversation') {
        if (event.action === 'typing_started' || event.action === 'typing_stopped') {
          const conversationId = event.entity_id
          if (event.actor_id?.startsWith('widget:')) {
            useSupportPresenceStore.getState().setTyping(
              conversationId,
              event.action === 'typing_started',
              typeof event.data?.content === 'string' ? event.data.content : '',
            )
          } else if (event.action === 'typing_started') {
            useSupportPresenceStore.getState().setAgentTyping(
              conversationId,
              event.actor_id,
              typeof event.data?.content === 'string' ? event.data.content : '',
              {
                name: typeof event.data?.agent_name === 'string' ? event.data.agent_name : undefined,
                avatarUrl: typeof event.data?.agent_avatar === 'string' ? event.data.agent_avatar : undefined,
              },
            )
          } else if (event.actor_id) {
            useSupportPresenceStore.getState().clearOneAgentTyping(conversationId, event.actor_id)
          }

          if (event.action === 'typing_started' && !event.actor_id?.startsWith('widget:') && event.actor_id) {
            const timerKey = `${conversationId}:agent:${event.actor_id}`
            const previous = typingTimersRef.current.get(timerKey)
            if (previous) {
              clearTimeout(previous)
            }
            const timer = setTimeout(() => {
              typingTimersRef.current.delete(timerKey)
              useSupportPresenceStore.getState().clearOneAgentTyping(conversationId, event.actor_id)
            }, TYPING_TIMEOUT_MS)
            typingTimersRef.current.set(timerKey, timer)
          }
          return
        }

        if (event.action === 'viewing_started' || event.action === 'viewing_stopped') {
          if (event.entity_id && event.actor_id) {
            useSupportPresenceStore
              .getState()
              .setViewingAgent(event.entity_id, event.actor_id, event.action === 'viewing_started')
          }
          return
        }

        runResync(event.entity_id || latestSelectedConversationIdRef.current)
        return
      }

      if (event.entity === 'support_visitor') {
        if (event.action === 'visitor_online') {
          useSupportPresenceStore.getState().setVisitorOnline(event.entity_id)
        } else if (event.action === 'visitor_offline') {
          useSupportPresenceStore.getState().setVisitorOffline(event.entity_id)
        }
        if (latestSelectedConversationIdRef.current) {
          queryClient.invalidateQueries({
            queryKey: supportQueryKeys.visitorContext(workspaceId, latestSelectedConversationIdRef.current),
          })
        }
        return
      }

      if (event.entity === 'support_teammate_presence') {
        queryClient.invalidateQueries({ queryKey: supportQueryKeys.teammatePresence(workspaceId) })
        return
      }
    },
    [markEvent, queryClient, runResync, workspaceId],
  )

  const connectRef = useRef<() => void>(() => {})

  const scheduleReconnect = useCallback(
    (reason: 'reconnect' | 'offline' | 'stale') => {
      clearTimers()
      if (!workspaceId) {
        return
      }
      if (typeof navigator !== 'undefined' && !navigator.onLine) {
        setRealtimeStatus('offline', 'Waiting for network')
        return
      }

      const retryCount = retryCountRef.current
      const delay = reason === 'stale' ? 250 : Math.min(1000 * Math.pow(2, retryCount), 30_000)
      setRealtimeStatus(
        reason === 'stale' ? 'stale' : 'reconnecting',
        reason === 'stale' ? 'Refreshing after sleep or stale connection' : 'Reconnecting…',
        retryCount,
      )
      reconnectTimerRef.current = setTimeout(() => {
        connectRef.current()
      }, delay)
    },
    [clearTimers, setRealtimeStatus, workspaceId],
  )

  const restartConnection = useCallback(
    (reason: 'stale' | 'online') => {
      clearTimers()
      shouldResyncOnOpenRef.current = true

      if (wsRef.current) {
        const current = wsRef.current
        wsRef.current = null
        current.onclose = null
        current.close()
      }

      if (!workspaceId) {
        return
      }

      if (reason === 'stale') {
        scheduleReconnect('stale')
        return
      }

      retryCountRef.current = 0
      setRealtimeStatus('reconnecting', 'Reconnecting…', 0)
      connectRef.current()
    },
    [clearTimers, scheduleReconnect, setRealtimeStatus, workspaceId],
  )

  const connect = useCallback(() => {
    if (!workspaceId) {
      return
    }

    const url = buildWorkspaceWebSocketUrl(apiBase, workspaceId)
    if (!url) {
      setRealtimeStatus('auth_expired', 'Session expired')
      return
    }

    clearTimers()
    if (!wsRef.current) {
      setRealtimeStatus(retryCountRef.current > 0 ? 'reconnecting' : 'connecting', null, retryCountRef.current)
    }

    const ws = new WebSocket(url)
    wsRef.current = ws
    useSupportPresenceStore.getState().setWsConnected(false)
    useSupportPresenceStore.getState().setWsSend(null)

    ws.onopen = () => {
      retryCountRef.current = 0
      useSupportPresenceStore.getState().setWsConnected(true)
      useSupportPresenceStore.getState().setWsSend((type, data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type, data }))
        }
      })
      markConnected()

      pingIntervalRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'support:ping', data: {} }))
        }
      }, PING_INTERVAL_MS)

      if (shouldResyncOnOpenRef.current) {
        shouldResyncOnOpenRef.current = false
        runResync()
      }
    }

    ws.onmessage = (event) => {
      markEvent()

      try {
        const parsed = JSON.parse(event.data) as
          | SupportRealtimeEvent
          | { type?: string; data?: OnlineVisitorsPayload | PresenceSnapshot }

        if (parsed && typeof parsed === 'object' && 'type' in parsed && parsed.type === 'support:online_visitors') {
          const payload = parsed.data as OnlineVisitorsPayload | undefined
          useSupportPresenceStore.getState().setOnlineVisitors(payload?.visitors ?? [])
          return
        }

        if (parsed && typeof parsed === 'object' && 'type' in parsed && parsed.type === 'support:presence_snapshot') {
          const snapshot = parsed.data as PresenceSnapshot | undefined
          if (snapshot?.conversation_id) {
            const nextViewers = (snapshot.viewers ?? []).map((viewer) => viewer.user_id).filter(Boolean)
            useSupportPresenceStore.getState().replaceViewingAgents(snapshot.conversation_id, nextViewers)

            const nextTypers = Object.fromEntries(
              Object.entries(snapshot.typers ?? {}).map(([uid, typing]) => [
                uid,
                {
                  content: typing.content ?? '',
                  name: typing.name,
                  avatarUrl: typing.avatar,
                },
              ]),
            )
            useSupportPresenceStore.getState().replaceAgentTyping(snapshot.conversation_id, nextTypers)
          }
          return
        }

        if (parsed && typeof parsed === 'object' && 'entity' in parsed && typeof parsed.entity === 'string') {
          handleSupportEvent(parsed as SupportRealtimeEvent)
        }
      } catch {
        // Ignore malformed websocket messages.
      }
    }

    ws.onclose = () => {
      clearTimers()
      useSupportPresenceStore.getState().setWsConnected(false)
      useSupportPresenceStore.getState().setWsSend(null)
      wsRef.current = null

      if (intentionalCloseRef.current) {
        return
      }

      retryCountRef.current += 1
      scheduleReconnect('reconnect')
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [
    apiBase,
    clearTimers,
    handleSupportEvent,
    markConnected,
    markEvent,
    runResync,
    scheduleReconnect,
    setRealtimeStatus,
    workspaceId,
  ])

  connectRef.current = connect

  useEffect(() => {
    if (!workspaceId || typeof window === 'undefined') {
      return
    }

    intentionalCloseRef.current = false
    shouldResyncOnOpenRef.current = true
    lastWakeSampleAtRef.current = Date.now()
    connect()

    const handleOnline = () => {
      restartConnection('online')
    }

    const handleOffline = () => {
      clearTimers()
      useSupportPresenceStore.getState().setWsConnected(false)
      setRealtimeStatus('offline', 'Waiting for network')
    }

    const handleVisibilityOrFocus = () => {
      if (document.visibilityState === 'visible') {
        const now = Date.now()
        if (now - lastWakeSampleAtRef.current > RESUME_GAP_MS) {
          restartConnection('stale')
        }
        lastWakeSampleAtRef.current = now
      }
    }

    wakeCheckRef.current = setInterval(() => {
      const now = Date.now()
      if (
        typeof document !== 'undefined' &&
        document.visibilityState === 'visible' &&
        now - lastWakeSampleAtRef.current > RESUME_GAP_MS
      ) {
        restartConnection('stale')
      }
      lastWakeSampleAtRef.current = now
    }, 20_000)

    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)
    window.addEventListener('focus', handleVisibilityOrFocus)
    window.addEventListener('pageshow', handleVisibilityOrFocus)
    document.addEventListener('visibilitychange', handleVisibilityOrFocus)

    return () => {
      intentionalCloseRef.current = true
      clearTimers()
      if (wakeCheckRef.current) {
        clearInterval(wakeCheckRef.current)
        wakeCheckRef.current = null
      }
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
      window.removeEventListener('focus', handleVisibilityOrFocus)
      window.removeEventListener('pageshow', handleVisibilityOrFocus)
      document.removeEventListener('visibilitychange', handleVisibilityOrFocus)

      useSupportPresenceStore.getState().setWsConnected(false)
      useSupportPresenceStore.getState().setWsSend(null)
      if (wsRef.current) {
        const current = wsRef.current
        wsRef.current = null
        current.onclose = null
        current.close()
      }
      resetRealtime()
    }
  }, [clearTimers, connect, resetRealtime, restartConnection, setRealtimeStatus, workspaceId])
}
