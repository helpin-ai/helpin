import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { supportQueryKeys, useSupportRealtime } from '@helpin-ai/support-core'
import { API_BASE } from '@mobile/lib/api'

/**
 * Mirrors the shared controller's own wake/stale threshold — see
 * `RESUME_GAP_MS` in `packages/support-core/src/use-support-realtime.ts`.
 */
const RESUME_GAP_MS = 60_000

/**
 * Mobile wrapper around the shared support-core realtime controller.
 *
 * ## What the shared controller already does (verified by reading
 * `use-support-realtime.ts` in full before writing this)
 *
 * `useSupportRealtime` already owns the websocket lifecycle end to end:
 *  - Listens to `document.visibilitychange` (plus `focus`, `pageshow`,
 *    `online`, `offline`) itself, and additionally runs a 20s-interval
 *    backstop poll of the same staleness check — so a missed
 *    `visibilitychange` event in the webview is not a single point of
 *    failure.
 *  - When the page has been hidden for longer than its own `RESUME_GAP_MS`
 *    (60s), it tears down and reopens the socket
 *    (`restartConnection('stale')`), then — once the new socket's `onopen`
 *    fires — runs a full resync (`runResync`) that invalidates
 *    `conversations`, `unread-stats`, `inbox-scopes`, `teammate-presence`,
 *    and, if a conversation is selected, `conversation`, `messages`, and
 *    `visitor-context`.
 *  - Handles `online`/`offline` transitions independently of the visibility
 *    logic.
 *
 * There is no evidence this needs a Tauri-specific workaround: the app
 * already trusts plain `document.visibilitychange` in this same webview for
 * `setupVisibilityRefresh` (Task 8, in support-core's `auth-api.ts`), which
 * drives token-refresh-on-resume with no special native lifecycle plumbing.
 *
 * ## What this hook adds
 *
 * An explicit, independently-testable invalidate of `conversations` +
 * `unread-stats` the instant the page becomes visible again after a >60s
 * hide — firing directly off `visibilitychange`, ahead of (and independent
 * from) the shared controller's own reconnect+resync, which only lands once
 * the *new* websocket's `onopen` callback runs. That reconnect can take a
 * beat (backoff/network re-establishment); this closes the gap so the inbox
 * list and unread badge refresh immediately on resume even before the socket
 * is back up. It intentionally does NOT duplicate reconnect/backoff logic —
 * the shared controller is the sole owner of the socket and already
 * reconnects on its own.
 */
export function useMobileRealtime(workspaceId: string, selectedConversationId: string | null): void {
  useSupportRealtime({ apiBase: API_BASE, workspaceId, selectedConversationId })

  const queryClient = useQueryClient()
  const hiddenAtRef = useRef<number | null>(null)

  useEffect(() => {
    if (!workspaceId || typeof document === 'undefined') {
      return
    }

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        hiddenAtRef.current = Date.now()
        return
      }

      const hiddenAt = hiddenAtRef.current
      hiddenAtRef.current = null
      if (hiddenAt !== null && Date.now() - hiddenAt >= RESUME_GAP_MS) {
        queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
        queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [workspaceId, queryClient])
}
