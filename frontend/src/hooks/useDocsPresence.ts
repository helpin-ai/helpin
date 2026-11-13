import { useCallback, useEffect } from 'react'
import type { DocsEditingPresenceSignal } from '@/components/docs/DocsEditor'
import { useDocsPresenceStore } from '@/stores/docsPresenceStore'
import { useWSStore } from './useWebSocket'

// Renew before the server's 60-second document lease expires. Each renewal
// returns a fresh snapshot, reconciling missed stops and expired connections.
const PRESENCE_REFRESH_MS = 30_000

export function useDocsPresence(documentId: string) {
  const send = useWSStore((state) => state.send)

  useEffect(() => {
    if (!documentId) return
    const clearPresence = () => {
      const store = useDocsPresenceStore.getState()
      store.replaceViewingUsers(documentId, {})
      store.replaceEditingUsers(documentId, {})
    }
    clearPresence()
    if (!send) return

    let viewing = false
    let pageHidden = false
    const stopViewing = () => {
      if (viewing) {
        send({ type: 'docs:editing:stop', data: { document_id: documentId } })
        send({ type: 'docs:viewing:stop', data: { document_id: documentId } })
        viewing = false
      }
      clearPresence()
    }
    const refresh = () => {
      if (pageHidden || document.visibilityState !== 'visible') {
        stopViewing()
        return
      }
      viewing = true
      send({ type: 'docs:viewing:start', data: { document_id: documentId } })
    }
    const onPageHide = () => { pageHidden = true; stopViewing() }
    const onPageShow = () => { pageHidden = false; refresh() }
    refresh()
    const interval = window.setInterval(refresh, PRESENCE_REFRESH_MS)
    document.addEventListener('visibilitychange', refresh)
    window.addEventListener('pagehide', onPageHide)
    window.addEventListener('pageshow', onPageShow)
    return () => {
      window.clearInterval(interval)
      document.removeEventListener('visibilitychange', refresh)
      window.removeEventListener('pagehide', onPageHide)
      window.removeEventListener('pageshow', onPageShow)
      stopViewing()
    }
  }, [documentId, send])

  return useCallback((presence: DocsEditingPresenceSignal | null) => {
    if (!documentId || !send || document.visibilityState !== 'visible') return
    send(presence ? {
      type: 'docs:editing:update',
      data: { document_id: documentId, area: presence.area, section: presence.section },
    } : {
      type: 'docs:editing:stop',
      data: { document_id: documentId },
    })
  }, [documentId, send])
}
