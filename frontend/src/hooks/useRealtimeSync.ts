import { useCallback, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWebSocket, type DocsPresenceSnapshot, type WSEvent, type WSSend, type PresenceSnapshot } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { useDocsPresenceStore } from '@/stores/docsPresenceStore'
import { useAuthStore } from '@/stores/authStore'
import { pmStoryService } from '@/lib/services/pmStoryService'
import { queryKeys } from '@/lib/queryKeys'
import { logPMDnD } from '@/lib/pmDnDDebug'

const BOARD_ENTITIES = new Set(['story'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link'])

let notificationAudio: HTMLAudioElement | null = null
function playNotificationSound() {
  try {
    if (!notificationAudio) {
      notificationAudio = new Audio('/sounds/ping.mp3')
      notificationAudio.volume = 0.5
    }
    notificationAudio.currentTime = 0
    notificationAudio.play().catch(() => {/* autoplay blocked */})
  } catch { /* audio not supported */ }
}

/** Debounce window (ms) for batching rapid websocket events into a single board refresh. */
const DEBOUNCE_MS = 200
/** Auto-clear typing indicator after this many ms without a refresh. */
const TYPING_TIMEOUT_MS = 10_000

export function useRealtimeSync(workspaceId: string): { wsSend: WSSend } {
  const queryClient = useQueryClient()
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  const selfIdRef = useRef<string | undefined>(useAuthStore.getState().user?.id)
  selfIdRef.current = useAuthStore.getState().user?.id
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
      const traceID = typeof event.data?.debug_trace_id === 'string' ? event.data.debug_trace_id : null
      logPMDnD('ws.story_event', {
        trace_id: traceID,
        action: event.action,
        entity: event.entity,
        story_id: event.entity_id,
        workspace_id: event.workspace_id,
        actor_id: event.actor_id,
      })

      if (event.action === 'deleted') {
        // Delete can be patched locally without re-fetching
        const patched = store.patchStory('deleted', event.entity_id)
        if (!patched) scheduleRefresh()
      } else if (event.action === 'moved' || event.action === 'reordered') {
        // Position changes renumber siblings; patching only the moved story leaves stale ordering.
        logPMDnD('ws.story_event_refresh', {
          trace_id: traceID,
          action: event.action,
          story_id: event.entity_id,
        })
        scheduleRefresh()
      } else {
        // For created/updated, fetch the updated story and patch it in
        pmStoryService.get(workspaceId, event.entity_id).then((res) => {
          if (res.data) {
            const story = { ...res.data.story }
            // Enrich with owner_name from StoryDetail owners for board display
            if (story.owner_member_id && !story.owner_name && res.data.owner_member) {
              story.owner_name = res.data.owner_member.display_name || res.data.owner_member.email
            }
            const patched = store.patchStory(event.action as 'created' | 'updated', event.entity_id, story)
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
    } else if (event.entity === 'workflow') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.workflows(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.epicStates(workspaceId) })
    } else if (event.entity === 'label') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.labels(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.labelsWithStats(workspaceId) })
    } else if (event.entity === 'view') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.views(workspaceId) })
    } else if (event.entity === 'story_template') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.templates(workspaceId) })
    } else if (event.entity === 'recurring_template') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.recurringTemplates(workspaceId) })
    } else if (event.entity === 'automation_rule') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.automationRules(workspaceId) })
      const workflowId = typeof event.data?.workflow_id === 'string' ? event.data.workflow_id : event.parent_id
      if (workflowId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.automationRulesByWorkflow(workspaceId, workflowId) })
      }
    } else if (event.entity === 'team_estimate_settings') {
      queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(workspaceId) })
    } else if (event.entity === 'epic') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'epics'] })
    } else if (event.entity === 'sprint') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints'] })
    } else if (event.entity === 'objective') {
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'objectives'] })
    } else if (event.entity === 'docs_document') {
      if (event.actor_id && event.actor_id === selfIdRef.current) {
        return
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.document(workspaceId, event.entity_id) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.content(workspaceId, event.entity_id) })
    } else if (event.entity === 'docs_space') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.spaces(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.space(workspaceId, event.entity_id) })
    } else if (event.entity === 'docs_collection') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.collections(workspaceId, event.parent_id ?? '') })
    } else if (event.entity === 'docs_version') {
      const docId = event.parent_id ?? ''
      if (docId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.versions(workspaceId, docId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.version(workspaceId, docId, event.entity_id) })
      }
    } else if (event.entity === 'docs_link') {
      const docId = event.parent_id ?? ''
      if (docId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.links(workspaceId, docId) })
      }
      const linkedObjectType = typeof event.data?.linked_object_type === 'string' ? event.data.linked_object_type : undefined
      const linkedObjectId = typeof event.data?.linked_object_id === 'string' ? event.data.linked_object_id : undefined
      if (linkedObjectType && linkedObjectId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.linkedDocs(workspaceId, linkedObjectType, linkedObjectId) })
      }
    } else if (event.entity === 'docs_helpcenter_config') {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.helpcenterConfig(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.helpcenterLocales(workspaceId) })
    } else if (event.entity === 'docs_document_presence') {
      if (!event.entity_id || !event.actor_id || event.actor_id === selfIdRef.current) return
      const docsStore = useDocsPresenceStore.getState()
      const timerKey = `${event.entity_id}:docs:viewing:${event.actor_id}`
      const prev = typingTimers.current.get(timerKey)
      if (prev) {
        clearTimeout(prev)
        typingTimers.current.delete(timerKey)
      }
      const metadata = {
        name: typeof event.data?.viewer_name === 'string' ? event.data.viewer_name : undefined,
        avatarUrl: typeof event.data?.viewer_avatar === 'string' ? event.data.viewer_avatar : undefined,
      }
      docsStore.setViewingUser(event.entity_id, event.actor_id, event.action === 'viewing_started', metadata)

      if (event.action === 'viewing_started') {
        const timer = setTimeout(() => {
          typingTimers.current.delete(timerKey)
          useDocsPresenceStore.getState().setViewingUser(event.entity_id, event.actor_id, false)
        }, 30_000)
        typingTimers.current.set(timerKey, timer)
      }
    } else if (event.entity === 'notification') {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.unreadCount(workspaceId) })
    } else if (event.entity === 'agent_run') {
      queryClient.invalidateQueries({ queryKey: ['agent_runs', workspaceId] })
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.all(workspaceId) })
      if (event.parent_type === 'story' && event.parent_id) {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.story(workspaceId, event.parent_id) })
      }
    } else if (event.entity === 'support_conversation') {
      if (event.action === 'typing_started' || event.action === 'typing_stopped') {
        if (!event.entity_id) return

        if (import.meta.env.DEV) {
          console.debug('[ws] typing event received:', event.action, 'conversation:', event.entity_id, 'actor:', event.actor_id)
        }
        const store = useSupportPresenceStore.getState()
        const convId = event.entity_id
        const content = (event.data?.content as string) || ''
        const agentName = typeof event.data?.agent_name === 'string' ? event.data.agent_name : undefined
        const agentAvatar = typeof event.data?.agent_avatar === 'string' ? event.data.agent_avatar : undefined
        const isWidget = event.actor_id?.startsWith('widget:')
        const timerKey = `${convId}:${isWidget ? 'customer' : `agent:${event.actor_id ?? 'unknown'}`}`

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
          // Agent typing — supports multiple agents per conversation
          if (event.action === 'typing_started') {
            store.setAgentTyping(convId, event.actor_id, content, {
              name: agentName,
              avatarUrl: agentAvatar,
            })
          } else {
            store.clearOneAgentTyping(convId, event.actor_id)
          }
        }

        // Auto-clear after timeout in case typing:stop is never received
        if (event.action === 'typing_started') {
          const timer = setTimeout(() => {
            typingTimers.current.delete(timerKey)
            const s = useSupportPresenceStore.getState()
            if (isWidget) {
              s.setTyping(convId, false)
            } else {
              s.clearOneAgentTyping(convId, event.actor_id)
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
        const store = useSupportPresenceStore.getState()
        store.setViewingAgent(event.entity_id, event.actor_id, event.action === 'viewing_started')

        // Auto-clear viewing after 30s in case viewing_stopped is never received
        if (event.action === 'viewing_started') {
          const timerKey = `${event.entity_id}:viewing:${event.actor_id}`
          const prev = typingTimers.current.get(timerKey)
          if (prev) clearTimeout(prev)
          const timer = setTimeout(() => {
            typingTimers.current.delete(timerKey)
            useSupportPresenceStore.getState().setViewingAgent(event.entity_id, event.actor_id, false)
          }, 30_000)
          typingTimers.current.set(timerKey, timer)
        }
      } else {
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, event.entity_id) })
        // Invalidate unread stats on any non-presence conversation update (includes reason=read)
        queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
      }
    } else if (event.entity === 'support_visitor') {
      const store = useSupportPresenceStore.getState()
      if (event.action === 'visitor_online') {
        store.setVisitorOnline(event.entity_id)
      } else if (event.action === 'visitor_offline') {
        store.setVisitorOffline(event.entity_id)
      }
    } else if (event.entity === 'support_teammate_presence') {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(workspaceId) })
    } else if (event.entity === 'support_conversation_message') {
      if (event.parent_id) {
        const s = useSupportPresenceStore.getState()
        // Clear typing state for whoever sent this message
        if (event.actor_id?.startsWith('widget:')) {
          s.setTyping(event.parent_id, false)
        } else if (event.actor_id) {
          s.clearOneAgentTyping(event.parent_id, event.actor_id)
        }

        // Play notification sound for messages from others
        if (event.actor_id !== selfIdRef.current) {
          playNotificationSound()
        }

        // Batch-invalidate all support conversation queries in a single call:
        // matches conversations list, conversation detail, and messages
        const parentId = event.parent_id
        queryClient.invalidateQueries({
          predicate: (query) => {
            const key = query.queryKey
            return key[0] === 'support' && key[1] === workspaceId && (
              // conversations list: ['support', wsId, 'conversations']
              key.length === 3 ||
              // conversation detail or messages: ['support', wsId, 'conversations', parentId, ...]
              key[3] === parentId
            )
          },
        })
        // New messages change unread counts
        queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
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

  const onPresenceSnapshot = useCallback((snapshot: PresenceSnapshot) => {
    const convId = snapshot.conversation_id
    if (!convId) return
    const store = useSupportPresenceStore.getState()
    const selfId = selfIdRef.current

    const nextViewers = snapshot.viewers
      .map((viewer) => viewer.user_id)
      .filter((uid) => uid && uid !== selfId)
    store.replaceViewingAgents(convId, nextViewers)

    const nextTypers = Object.fromEntries(
      Object.entries(snapshot.typers)
        .filter(([uid]) => uid !== selfId)
        .map(([uid, typing]) => [uid, {
          content: typing.content ?? '',
          name: typing.name,
          avatarUrl: typing.avatar,
        }])
    )
    store.replaceAgentTyping(convId, nextTypers)

    for (const [uid] of Object.entries(nextTypers)) {
      const timerKey = `${convId}:agent:${uid}`
      const prevTimer = typingTimers.current.get(timerKey)
      if (prevTimer) clearTimeout(prevTimer)
      const timer = setTimeout(() => {
        typingTimers.current.delete(timerKey)
        useSupportPresenceStore.getState().clearOneAgentTyping(convId, uid)
      }, TYPING_TIMEOUT_MS)
      typingTimers.current.set(timerKey, timer)
    }
  }, [])

  const onDocsPresenceSnapshot = useCallback((snapshot: DocsPresenceSnapshot) => {
    const docId = snapshot.document_id
    if (!docId) return
    const selfId = selfIdRef.current
    const nextViewers = Object.fromEntries(
      snapshot.viewers
        .filter((viewer) => viewer.user_id && viewer.user_id !== selfId)
        .map((viewer) => [viewer.user_id, {
          name: viewer.name,
          avatarUrl: viewer.avatar,
        }])
    )
    useDocsPresenceStore.getState().replaceViewingUsers(docId, nextViewers)
  }, [])

  useEffect(() => {
    return () => {
      clearTimeout(debounceTimer.current ?? undefined)
      typingTimers.current.forEach((t) => clearTimeout(t))
      typingTimers.current.clear()
    }
  }, [])

  const { send: wsSend, isConnected } = useWebSocket({ workspaceId, onEvent, onPresenceSnapshot, onDocsPresenceSnapshot })

  // Expose wsSend and connection state to components via the store
  useEffect(() => {
    useSupportPresenceStore.getState().setWsSend(wsSend)
    return () => {
      useSupportPresenceStore.getState().setWsSend(null)
    }
  }, [wsSend])

  useEffect(() => {
    useSupportPresenceStore.getState().setWsConnected(isConnected)
  }, [isConnected])

  return { wsSend }
}
