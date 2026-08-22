import { useCallback, useEffect, useRef } from 'react'
import { useSupportPresenceStore } from '@helpin-ai/support-core'

/** How long after the last keystroke we auto-emit `stop` (mirrors web's 5s). */
const IDLE_STOP_MS = 5000
/** Minimum gap between successive start/update emits (mirrors web's 300ms throttle). */
const THROTTLE_MS = 300

/**
 * Broadcasts the agent's typing state to teammates + the customer over the
 * shared support websocket, mirroring the web ReplyComposer:
 *  - first keystroke emits `support:typing:start`, subsequent ones
 *    `support:typing:update` (throttled to THROTTLE_MS), each carrying the
 *    current draft as `content` for live preview;
 *  - 5s of inactivity, an explicit `stopTyping()` (send / switch to note), or
 *    unmount emits `support:typing:stop`.
 *
 * `enabled` is false in note mode: notes are private, so we never *start*
 * there — but a `stop` still fires when toggling into note mid-type, so the
 * indicator never sticks on the other end.
 *
 * `wsSend`/`wsConnected` come from the presence store, which the shared
 * realtime controller (running via useMobileRealtime) already populates.
 */
export function useTypingBroadcast(conversationId: string, enabled: boolean) {
  const wsSend = useSupportPresenceStore((s) => s.wsSend)
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected)

  const isTypingRef = useRef(false)
  const lastSentRef = useRef(0)
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Latest values read through refs so the stable callbacks below never go
  // stale (and so the unmount cleanup sees the current socket).
  const enabledRef = useRef(enabled)
  enabledRef.current = enabled
  const wsSendRef = useRef(wsSend)
  wsSendRef.current = wsSend
  const connectedRef = useRef(wsConnected)
  connectedRef.current = wsConnected

  const sendTyping = useCallback(
    (typing: boolean, content?: string) => {
      const send = wsSendRef.current
      if (!send || !connectedRef.current) return

      if (!typing) {
        if (idleTimerRef.current) {
          clearTimeout(idleTimerRef.current)
          idleTimerRef.current = null
        }
        // Only emit stop if we had actually announced typing.
        if (isTypingRef.current) {
          isTypingRef.current = false
          send('support:typing:stop', { conversation_id: conversationId })
        }
        return
      }

      // Never START typing while notes (private) are active.
      if (!enabledRef.current) return

      const now = Date.now()
      if (isTypingRef.current && now - lastSentRef.current < THROTTLE_MS) return
      const eventType = isTypingRef.current ? 'support:typing:update' : 'support:typing:start'
      isTypingRef.current = true
      lastSentRef.current = now
      send(eventType, { conversation_id: conversationId, content: content ?? '' })
    },
    [conversationId],
  )

  const notifyTyping = useCallback(
    (content: string) => {
      sendTyping(true, content)
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current)
      idleTimerRef.current = setTimeout(() => sendTyping(false), IDLE_STOP_MS)
    },
    [sendTyping],
  )

  const stopTyping = useCallback(() => sendTyping(false), [sendTyping])

  // Stop on unmount / conversation change (the composer is keyed by
  // conversationId, so a switch unmounts this and fires the cleanup).
  useEffect(() => {
    return () => {
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current)
      const send = wsSendRef.current
      if (isTypingRef.current && send) {
        isTypingRef.current = false
        send('support:typing:stop', { conversation_id: conversationId })
      }
    }
  }, [conversationId])

  return { notifyTyping, stopTyping }
}
