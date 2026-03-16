import { useCallback, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWebSocket, type WSEvent } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { pmStoryService } from '@/lib/services/pmStoryService'
import { queryKeys } from '@/lib/queryKeys'

const BOARD_ENTITIES = new Set(['story'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link'])

/** Debounce window (ms) for batching rapid websocket events into a single board refresh. */
const DEBOUNCE_MS = 200
/** Auto-clear typing indicator after this many ms without a refresh. */
const TYPING_TIMEOUT_MS = 10_000

export function useRealtimeSync(workspaceId: string) {
  const queryClient = useQueryClient()
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
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
            const story = { ...res.data.story }
            // Enrich with owner_name from StoryDetail owners for board display
            if (story.owner_member_id && !story.owner_name && res.data.owner_member) {
              story.owner_name = res.data.owner_member.display_name || res.data.owner_member.email
            }
            const patched = store.patchStory(event.action as 'created' | 'updated' | 'moved', event.entity_id, story)
            if (!patched) scheduleRefresh()
          } else {
            // Story might have been archived/deleted by the time we fetch.
            scheduleRefresh()
          }
        })
      }
    }

    // Invalidate TanStack Query cache for the affected entity
    if (event.entity === 'story') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.story(workspaceId, event.entity_id) })
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'stories'] })
    } else if (event.entity === 'epic') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'epics'] })
    } else if (event.entity === 'sprint') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints'] })
    } else if (event.entity === 'objective') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'objectives'] })
    } else if (event.entity === 'docs_document') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.document(workspaceId, event.entity_id) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.content(workspaceId, event.entity_id) })
    } else if (event.entity === 'docs_space') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.spaces(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.space(workspaceId, event.entity_id) })
    } else if (event.entity === 'docs_collection') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.collections(workspaceId, event.parent_id ?? '') })
    } else if (event.entity === 'notification') {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.unreadCount(workspaceId) })
    } else if (event.entity === 'agent_run') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'epics'] })
      queryClient.invalidateQueries({ queryKey: ['agent_runs', workspaceId] })
      if (event.parent_type === 'story' && event.parent_id) {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.story(workspaceId, event.parent_id) })
      }
    } else if (event.entity === 'support_conversation') {
      if (event.action === 'typing_started' || event.action === 'typing_stopped') {
        if (!event.entity_id) return

        if (import.meta.env.DEV) {
          console.debug('[ws] typing event received:', event.action, 'conversation:', event.entity_id, 'actor:', event.actor_id)
        }
        const store = useSupportInboxStore.getState()
        const convId = event.entity_id
        const content = (event.data?.content as string) || ''
        const isWidget = event.actor_id?.startsWith('widget:')
        const timerKey = `${convId}:${isWidget ? 'customer' : 'agent'}`

        // Clear any existing auto-clear timer for this conversation+actor type
        const prevTimer = typingTimers.current.get(timerKey)
        if (prevTimer) {
          clearTimeout(prevTimer)
          typingTimers.current.delete(timerKey)
        }

        if (isWidget) {
          // Customer typing
          store.setTyping(convId, event.action === 'typing_started', content)
        } else {
          // Agent typing
          store.setAgentTyping(
            convId,
            event.action === 'typing_started' ? event.actor_id : null,
            content,
          )
        }

        // Auto-clear after timeout in case typing:stop is never received
        if (event.action === 'typing_started') {
          const timer = setTimeout(() => {
            typingTimers.current.delete(timerKey)
            const s = useSupportInboxStore.getState()
            if (isWidget) {
              s.setTyping(convId, false)
            } else {
              s.setAgentTyping(convId, null)
            }
          }, TYPING_TIMEOUT_MS)
          typingTimers.current.set(timerKey, timer)
        }
      } else if (event.action === 'viewing_started' || event.action === 'viewing_stopped') {
        if (!event.entity_id || !event.actor_id) return
        if (import.meta.env.DEV) {
          console.debug('[ws] viewing event:', event.action, 'conv:', event.entity_id, 'actor:', event.actor_id)
        }
        // Hub already filters out self-viewing events server-side
        const store = useSupportInboxStore.getState()
        store.setViewingAgent(event.entity_id, event.actor_id, event.action === 'viewing_started')

        // Auto-clear viewing after 90s in case viewing_stopped is never received
        if (event.action === 'viewing_started') {
          const timerKey = `${event.entity_id}:viewing:${event.actor_id}`
          const prev = typingTimers.current.get(timerKey)
          if (prev) clearTimeout(prev)
          const timer = setTimeout(() => {
            typingTimers.current.delete(timerKey)
            useSupportInboxStore.getState().setViewingAgent(event.entity_id, event.actor_id, false)
          }, 90_000)
          typingTimers.current.set(timerKey, timer)
        }
      } else {
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, event.entity_id) })
      }
    } else if (event.entity === 'support_conversation_message') {
      if (event.parent_id) {
        useSupportInboxStore.getState().setTyping(event.parent_id, false)
        queryClient.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, event.parent_id) })
      }
    }

    // Child entity events → invalidate parent query cache
    if (CHILD_ENTITIES.has(event.entity) && event.parent_type && event.parent_id) {
      if (event.entity === 'comment') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.comments(workspaceId, event.parent_id) })
      } else if (event.entity === 'checklist_item') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.checklists(workspaceId, event.parent_id) })
      } else if (event.entity === 'attachment') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.attachments(workspaceId, event.parent_id) })
      } else if (event.entity === 'external_link') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(workspaceId, event.parent_id) })
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
  }, [scheduleRefresh, workspaceId, queryClient])

  useEffect(() => {
    return () => {
      clearTimeout(debounceTimer.current ?? undefined)
      typingTimers.current.forEach((t) => clearTimeout(t))
      typingTimers.current.clear()
    }
  }, [])

  useWebSocket({ workspaceId, onEvent })
}
