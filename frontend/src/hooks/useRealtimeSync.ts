import { useCallback, useEffect, useRef } from 'react'
import { useWebSocket, type WSEvent } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'
import { pmStoryService } from '@/lib/services/pmStoryService'

const BOARD_ENTITIES = new Set(['story'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link'])

/** Debounce window (ms) for batching rapid websocket events into a single board refresh. */
const DEBOUNCE_MS = 200

export function useRealtimeSync(workspaceId: string) {
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const scheduleRefresh = useCallback(() => {
    clearTimeout(debounceTimer.current ?? undefined)
    debounceTimer.current = setTimeout(() => {
      usePMBoardStore.getState().refreshBoard()
    }, DEBOUNCE_MS)
  }, [])

  const onEvent = useCallback((event: WSEvent) => {
    // Story-level events → incremental patch when possible, debounced full refresh as fallback
    if (BOARD_ENTITIES.has(event.entity)) {
      const store = usePMBoardStore.getState()

      if (event.action === 'deleted') {
        // Delete can be patched locally without re-fetching
        const patched = store.patchStory('deleted', event.entity_id)
        if (!patched) scheduleRefresh()
      } else {
        // For created/updated/moved, fetch the updated story and patch it in
        pmStoryService.get(workspaceId, event.entity_id).then((res) => {
          if (res.data) {
            const patched = store.patchStory(event.action, event.entity_id, res.data.story)
            if (!patched) scheduleRefresh()
          } else {
            // Story might have been archived/deleted by the time we fetch.
            scheduleRefresh()
          }
        })
      }
    }

    // Dispatch custom DOM events for any component that listens
    // e.g. "story-updated", "comment-created", "epic-deleted"
    window.dispatchEvent(
      new CustomEvent(`${event.entity}-${event.action}`, {
        detail: event,
      })
    )

    // Child entity events → also dispatch a parent update event
    // so that open story detail panels can refetch
    if (CHILD_ENTITIES.has(event.entity) && event.parent_type && event.parent_id) {
      window.dispatchEvent(
        new CustomEvent(`${event.parent_type}-child-updated`, {
          detail: event,
        })
      )
    }
  }, [scheduleRefresh, workspaceId])

  useEffect(() => {
    return () => { clearTimeout(debounceTimer.current ?? undefined) }
  }, [])

  useWebSocket({ workspaceId, onEvent })
}
