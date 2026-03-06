import { useCallback, useEffect, useRef } from 'react'
import { useWebSocket, type WSEvent } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'
import { pmStoryService } from '@/lib/services/pmStoryService'

const BOARD_ENTITIES = new Set(['story'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link'])

/** Debounce window (ms) for batching rapid websocket events into a single board refresh. */
const DEBOUNCE_MS = 200

export function useRealtimeSync(workspaceId: string) {
  const debounceTimer = useRef<ReturnType<typeof setTimeout>>()

  const onEvent = useCallback((event: WSEvent) => {
    // Story-level events → incremental patch when possible, debounced full refresh as fallback
    if (BOARD_ENTITIES.has(event.entity)) {
      const store = usePMBoardStore.getState()

      if (event.action === 'deleted') {
        // Delete can be patched locally without re-fetching
        store.patchStory('deleted', event.entity_id)
      } else {
        // For created/updated/moved, fetch the updated story and patch it in
        pmStoryService.get(workspaceId, event.entity_id).then((res) => {
          if (res.data) {
            store.patchStory(event.action, event.entity_id, res.data.story)
          } else {
            // Story might have been archived/deleted by the time we fetch — schedule debounced refresh
            clearTimeout(debounceTimer.current)
            debounceTimer.current = setTimeout(() => {
              usePMBoardStore.getState().refreshBoard()
            }, DEBOUNCE_MS)
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
  }, [workspaceId])

  useEffect(() => {
    return () => { clearTimeout(debounceTimer.current) }
  }, [])

  useWebSocket({ workspaceId, onEvent })
}
