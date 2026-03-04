import { useCallback } from 'react'
import { useWebSocket, type WSEvent } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'

const BOARD_ENTITIES = new Set(['story'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link'])

export function useRealtimeSync(workspaceId: string) {
  const onEvent = useCallback((event: WSEvent) => {
    // Story-level events → refresh the board
    if (BOARD_ENTITIES.has(event.entity)) {
      usePMBoardStore.getState().refreshBoard()
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
  }, [])

  useWebSocket({ workspaceId, onEvent })
}
