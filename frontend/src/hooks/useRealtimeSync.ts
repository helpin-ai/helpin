import { useCallback, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useWebSocket, type DocsPresenceSnapshot, type WSEvent, type WSSend, type PresenceSnapshot } from './useWebSocket'
import { usePMBoardStore } from '@/stores/pmBoardStore'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useDocsPresenceStore } from '@/stores/docsPresenceStore'
import { useAuthStore } from '@/stores/authStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { queryKeys } from '@/lib/queryKeys'
import { createAgentRunUpdateTracker, type AgentRunUpdateKind } from '@/lib/agentRunRealtime'
import { isForegroundNetworkAvailable } from './useRealtimeFallbackPolling'
import { buildPatchedTaskFromDetail } from '@/components/pm/task-detail/taskDetailEventPayload'
import { isSupportConversationListQueryKey, moveConversationToTopForMessageActivity, patchConversationDetailPersonalRead, patchConversationDetailStatus, patchConversationPersonalReadInCache, patchConversationStatusInCache, type SupportConversationListCache, type SupportConversationStatusPatch } from '@/lib/supportQueryCache'
import { appendMessageToNewestPage, type SupportMessagePages } from '@/lib/supportMessagePages'
import type { ConversationStatus, SupportConversation, SupportMessage, Task, TaskMemberColumn, TaskStateColumn } from '@/lib/pmTypes'

const BOARD_ENTITIES = new Set(['task'])
const CHILD_ENTITIES = new Set(['comment', 'checklist_item', 'attachment', 'external_link', 'task_git_link'])

let notificationAudio: HTMLAudioElement | null = null
function playNotificationSound() {
  try {
    if (!notificationAudio) {
      notificationAudio = new Audio('/sounds/ping-v2.mp3')
      notificationAudio.volume = 0.5
    }
    notificationAudio.currentTime = 0
    notificationAudio.play().catch(() => {/* autoplay blocked */})
  } catch { /* audio not supported */ }
}

/** Debounce window (ms) for batching rapid websocket events into a single board refresh. */
const DEBOUNCE_MS = 200
/** Trailing-window (ms) for coalescing bursty agent_run invalidations into a single flush. */
const AGENT_RUN_INVALIDATE_MS = 400
const SUPPORT_COUNTER_INVALIDATE_MS = 250
/** Auto-clear typing indicator after this many ms without a refresh. */
const TYPING_TIMEOUT_MS = 10_000
const DOC_EDITING_TIMEOUT_MS = 20_000
const AGENT_RUN_PAUSE_REASONS = new Set(['none', 'human_input', 'human_approval', 'authentication', 'awaiting_user_message'])
const SUPPORT_CONVERSATION_STATUSES = new Set(['open', 'waiting_on_customer', 'resolved', 'spam'])

function supportConversationStatusPatchFromEvent(event: WSEvent): SupportConversationStatusPatch | null {
  const status = event.data?.status
  if (typeof status !== 'string' || !SUPPORT_CONVERSATION_STATUSES.has(status)) {
    return null
  }
  const oldStatus = typeof event.data?.old_status === 'string' ? event.data.old_status : null
  const flowState = typeof event.data?.flow_state === 'string' || event.data?.flow_state === null
    ? event.data.flow_state as SupportConversation['flow_state']
    : undefined
  const updatedAt = typeof event.data?.updated_at === 'string'
    ? event.data.updated_at
    : event.sent_at
  const mailboxId = typeof event.data?.mailbox_id === 'string' || event.data?.mailbox_id === null
    ? event.data.mailbox_id as string | null
    : undefined

  return {
    conversationId: event.entity_id,
    oldStatus,
    status: status as ConversationStatus,
    flowState,
    updatedAt,
    mailboxId,
  }
}

function normalizeAgentRunPauseReason(value: unknown): Task['latest_run_pause_reason'] {
  if (typeof value !== 'string' || !value.trim()) return null
  return AGENT_RUN_PAUSE_REASONS.has(value) ? value as Task['latest_run_pause_reason'] : null
}

function patchTaskLatestRun(task: Task, event: WSEvent, now: string): Task {
  const agentId = typeof event.data?.agent_id === 'string' && event.data.agent_id.trim()
    ? event.data.agent_id
    : task.latest_run_agent_id
  const status = typeof event.data?.status === 'string'
    ? event.data.status
    : task.latest_run_status
  const pauseReason = event.data && 'pause_reason' in event.data
    ? normalizeAgentRunPauseReason(event.data.pause_reason)
    : task.latest_run_pause_reason

  const runId = event.entity_id || task.latest_run_id
  const latestRunAt = event.sent_at || now
  if (
    task.latest_run_id === runId
    && task.latest_run_agent_id === agentId
    && task.latest_run_status === status
    && task.latest_run_pause_reason === pauseReason
  ) {
    return task
  }

  return {
    ...task,
    latest_run_id: runId,
    latest_run_agent_id: agentId,
    latest_run_status: status,
    latest_run_pause_reason: pauseReason,
    latest_run_at: latestRunAt,
  }
}

function patchAgentRunTaskColumns(columns: TaskStateColumn[], event: WSEvent, now: string): [TaskStateColumn[], boolean] {
  let patched = false
  const taskId = event.parent_id

  const nextColumns = columns.map((column) => {
    let columnPatched = false
    const tasks = column.tasks.map((task) => {
      if (task.id !== taskId) return task
      const nextTask = patchTaskLatestRun(task, event, now)
      if (nextTask === task) return task
      columnPatched = true
      patched = true
      return nextTask
    })
    const taskGroups = column.task_groups?.map((group) => {
      let groupPatched = false
      const groupTasks = group.tasks.map((task) => {
        if (task.id !== taskId) return task
        const nextTask = patchTaskLatestRun(task, event, now)
        if (nextTask === task) return task
        groupPatched = true
        patched = true
        return nextTask
      })
      return groupPatched ? { ...group, tasks: groupTasks } : group
    })

    if (!columnPatched && !taskGroups?.some((group, index) => group !== column.task_groups?.[index])) {
      return column
    }

    return {
      ...column,
      tasks,
      task_groups: taskGroups,
    }
  })

  return [patched ? nextColumns : columns, patched]
}

function patchAgentRunTaskMemberColumns(columns: TaskMemberColumn[], event: WSEvent, now: string): [TaskMemberColumn[], boolean] {
  let patched = false
  const taskId = event.parent_id

  const nextColumns = columns.map((column) => {
    let columnPatched = false
    const tasks = column.tasks.map((task) => {
      if (task.id !== taskId) return task
      const nextTask = patchTaskLatestRun(task, event, now)
      if (nextTask === task) return task
      columnPatched = true
      patched = true
      return nextTask
    })

    return columnPatched ? { ...column, tasks } : column
  })

  return [patched ? nextColumns : columns, patched]
}

function patchBoardTaskLatestRun(event: WSEvent) {
  if (event.entity !== 'agent_run' || event.parent_type !== 'task' || !event.parent_id) return

  const now = new Date().toISOString()
  usePMBoardStore.setState((state) => {
    const [columns, columnsPatched] = patchAgentRunTaskColumns(state.columns, event, now)
    const [memberColumns, memberColumnsPatched] = patchAgentRunTaskMemberColumns(state.memberColumns, event, now)

    if (!columnsPatched && !memberColumnsPatched) return state
    return {
      columns,
      memberColumns,
    }
  })
}

function dispatchAgentRunCompatibilityEvents(event: WSEvent, updateKind: AgentRunUpdateKind) {
  if (updateKind === 'duplicate') return

  const detail = {
    ...event,
    agent_id: typeof event.data?.agent_id === 'string' ? event.data.agent_id : undefined,
    status: typeof event.data?.status === 'string' ? event.data.status : undefined,
    pause_reason: typeof event.data?.pause_reason === 'string' ? event.data.pause_reason : undefined,
    approval_state: typeof event.data?.approval_state === 'string' ? event.data.approval_state : undefined,
    change_kind: typeof event.data?.change_kind === 'string' ? event.data.change_kind : undefined,
    dock_chat_id: typeof event.data?.dock_chat_id === 'string' ? event.data.dock_chat_id : undefined,
    update_kind: updateKind,
  }

  const eventName = `${event.entity}-${event.action}`
  window.dispatchEvent(new CustomEvent(eventName, { detail }))
}

function patchAgentRunCache(current: unknown, event: WSEvent): unknown {
  const patchRows = (rows: unknown[]): [unknown[], boolean] => {
    let changed = false
    const next = rows.map((row) => {
      if (!row || typeof row !== 'object' || (row as { id?: unknown }).id !== event.entity_id) return row
      const run = row as Record<string, unknown>
      const fields: Record<string, unknown> = {}
      for (const field of ['agent_id', 'status', 'pause_reason', 'approval_state'] as const) {
        const value = event.data?.[field]
        if (typeof value === 'string' && run[field] !== value) fields[field] = value
      }
      if (Object.keys(fields).length === 0) return row
      changed = true
      return { ...run, ...fields, updated_at: event.sent_at ?? run.updated_at }
    })
    return [changed ? next : rows, changed]
  }

  if (Array.isArray(current)) return patchRows(current)[0]
  if (!current || typeof current !== 'object') return current
  const data = (current as { data?: unknown }).data
  if (!Array.isArray(data)) return current
  const [nextData, changed] = patchRows(data)
  return changed ? { ...current, data: nextData } : current
}

export function useRealtimeSync(workspaceId: string): { wsSend: WSSend } {
  const queryClient = useQueryClient()
  const agentRunTracker = useRef(createAgentRunUpdateTracker())
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  const agentRunInvalidateTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const supportCounterInvalidateTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const supportWorkspaceUnreadPending = useRef(false)
  const pendingAgentRunInvalidations = useRef<Map<string, readonly unknown[]>>(new Map())
  const selfId = useAuthStore((state) => state.user?.id)
  const selfIdRef = useRef<string | undefined>(selfId)
  useEffect(() => {
    selfIdRef.current = selfId
  }, [selfId])
  const scheduleRefresh = useCallback(() => {
    clearTimeout(debounceTimer.current ?? undefined)
    debounceTimer.current = setTimeout(() => {
      usePMBoardStore.getState().refreshBoard()
    }, DEBOUNCE_MS)
  }, [])

  // Queue an agent_run-related invalidation and flush all unique pending keys
  // in a single pass at the end of the trailing window. A burst of events
  // targeting the same keys collapses into one refetch instead of N.
  const scheduleAgentRunInvalidation = useCallback((queryKey: readonly unknown[]) => {
    pendingAgentRunInvalidations.current.set(JSON.stringify(queryKey), queryKey)
    if (agentRunInvalidateTimer.current) return
    agentRunInvalidateTimer.current = setTimeout(() => {
      agentRunInvalidateTimer.current = null
      const pending = pendingAgentRunInvalidations.current
      pendingAgentRunInvalidations.current = new Map()
      pending.forEach((key) => {
        queryClient.invalidateQueries({ queryKey: key, refetchType: isForegroundNetworkAvailable() ? 'active' : 'none' })
      })
    }, AGENT_RUN_INVALIDATE_MS)
  }, [queryClient])

  const scheduleSupportCounterInvalidation = useCallback((refreshWorkspaceUnread = false) => {
    supportWorkspaceUnreadPending.current ||= refreshWorkspaceUnread
    if (supportCounterInvalidateTimer.current) return
    supportCounterInvalidateTimer.current = setTimeout(() => {
      supportCounterInvalidateTimer.current = null
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) })
      if (supportWorkspaceUnreadPending.current) {
        supportWorkspaceUnreadPending.current = false
        queryClient.invalidateQueries({ queryKey: queryKeys.support.workspaceUnread() })
      }
    }, SUPPORT_COUNTER_INVALIDATE_MS)
  }, [queryClient, workspaceId])

  const onEvent = useCallback((event: WSEvent) => {
    // Task-level events → incremental patch when possible, debounced full refresh as fallback
    if (BOARD_ENTITIES.has(event.entity)) {
      const store = usePMBoardStore.getState()

      if (event.action === 'deleted') {
        // Delete can be patched locally without re-fetching
        const patched = store.patchTask('deleted', event.entity_id)
        if (!patched) scheduleRefresh()
      } else if (event.action === 'moved' || event.action === 'reordered') {
        // Position changes renumber siblings; patching only the moved task leaves stale ordering.
        scheduleRefresh()
      } else {
        // For created/updated, fetch the updated task and patch it in
        pmTaskService.get(workspaceId, event.entity_id).then((res) => {
          if (res.data) {
            const task = buildPatchedTaskFromDetail(res.data)
            const patched = store.patchTask(event.action as 'created' | 'updated', event.entity_id, task)
            if (!patched) scheduleRefresh()
          } else {
            // Task might have been archived/deleted by the time we fetch.
            scheduleRefresh()
          }
        })
      }
    }

    // Invalidate TanStack Query cache for the affected entity
    if (event.entity === 'task') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.task(workspaceId, event.entity_id) })
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks'] })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintBacklogTasksRoot(workspaceId) })
    } else if (event.entity === 'workflow') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.workflows(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.epicStates(workspaceId) })
    } else if (event.entity === 'label') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.labels(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.labelsWithStats(workspaceId) })
    } else if (event.entity === 'view') {
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.views(workspaceId) })
    } else if (event.entity === 'task_template' || event.entity === 'story_template') {
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
      // Collection writes can reparent children or move documents between
      // buckets (safe delete flatten, tree reparent). Invalidate the
      // per-space collection list plus the workspace-wide collection
      // cache and every document query so nav trees stay in sync after
      // a peer's mutation.
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.collections(workspaceId, event.parent_id ?? '') })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.allCollections(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.helpcenterConfig(workspaceId) })
    } else if (event.entity === 'docs_change_proposal') {
      const docId = event.parent_id || (typeof event.data?.document_id === 'string' ? event.data.document_id : '')
      if (docId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.changeProposals(workspaceId, docId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.changeProposal(workspaceId, docId, event.entity_id) })
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(workspaceId) })
        if (event.data?.status === 'applied') {
          queryClient.invalidateQueries({ queryKey: queryKeys.docs.content(workspaceId, docId) })
          queryClient.invalidateQueries({ queryKey: queryKeys.docs.blocks(workspaceId, docId) })
          queryClient.invalidateQueries({ queryKey: queryKeys.docs.document(workspaceId, docId), exact: true })
        }
      }
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
      if (event.action === 'viewing_started' || event.action === 'viewing_stopped') {
        const metadata = {
          name: typeof event.data?.viewer_name === 'string' ? event.data.viewer_name : undefined,
          avatarUrl: typeof event.data?.viewer_avatar === 'string' ? event.data.viewer_avatar : undefined,
        }
        docsStore.setViewingUser(event.entity_id, event.actor_id, event.action === 'viewing_started', metadata)
      } else if (event.action === 'editing_updated' || event.action === 'editing_stopped') {
        const timerKey = `${event.entity_id}:docs:editing:${event.actor_id}`
        const prev = typingTimers.current.get(timerKey)
        if (prev) {
          clearTimeout(prev)
          typingTimers.current.delete(timerKey)
        }
        const metadata = {
          area: typeof event.data?.editor_area === 'string' ? event.data.editor_area : 'body',
          section: typeof event.data?.editor_section === 'string' ? event.data.editor_section : undefined,
          name: typeof event.data?.editor_name === 'string' ? event.data.editor_name : undefined,
          avatarUrl: typeof event.data?.editor_avatar === 'string' ? event.data.editor_avatar : undefined,
        }
        docsStore.setEditingUser(event.entity_id, event.actor_id, event.action === 'editing_updated', metadata)

        if (event.action === 'editing_updated') {
          const timer = setTimeout(() => {
            typingTimers.current.delete(timerKey)
            useDocsPresenceStore.getState().setEditingUser(event.entity_id, event.actor_id, false)
          }, DOC_EDITING_TIMEOUT_MS)
          typingTimers.current.set(timerKey, timer)
        }
      }
    } else if (event.entity === 'notification') {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.unreadCount(workspaceId) })

      if (event.action === 'created') {
        const data = event.data ?? {}
        const eventType = typeof data.event_type === 'string' ? data.event_type : ''
        const recipientId = typeof data.recipient_id === 'string' ? data.recipient_id : ''
        const parentTaskId = typeof data.parent_task_id === 'string' ? data.parent_task_id : ''
        const selfId = selfIdRef.current
        if (
          eventType === 'task.agent_attention_required'
          && !!selfId
          && recipientId === selfId
          && event.actor_id !== selfId
        ) {
          const slug = useWorkspaceStore.getState().currentWorkspace?.slug
          toast('Agent needs your attention', {
            description: 'An agent has paused and is waiting for your input.',
            duration: 10_000,
            action: slug && parentTaskId ? {
              label: 'Open task',
              onClick: () => {
                window.location.href = `/w/${slug}/pm/tasks?task=${parentTaskId}`
              },
            } : undefined,
          })
        }
      }
    } else if (event.entity === 'agent_run' || (event.entity === 'coding_session' && event.data?.change_kind)) {
      const { kind: updateKind, attentionChanged } = agentRunTracker.current(event)
      patchBoardTaskLatestRun(event)
      queryClient.setQueriesData(
        { queryKey: queryKeys.automation.runsRoot(workspaceId) },
        (current) => patchAgentRunCache(current, event),
      )
      dispatchAgentRunCompatibilityEvents(event, updateKind)
      if (attentionChanged) scheduleAgentRunInvalidation(queryKeys.automation.runAttentionCount(workspaceId))
      if (updateKind === 'lifecycle') {
        scheduleAgentRunInvalidation(queryKeys.billing.workspace(workspaceId))
        scheduleAgentRunInvalidation(queryKeys.automation.runsRoot(workspaceId))
        scheduleAgentRunInvalidation(queryKeys.automation.activityRoot(workspaceId))
        scheduleAgentRunInvalidation(queryKeys.automation.overview(workspaceId))
        scheduleAgentRunInvalidation(queryKeys.automation.agentFleet(workspaceId))
        const eventAgentId = typeof event.data?.agent_id === 'string' ? event.data.agent_id : ''
        if (eventAgentId) {
          scheduleAgentRunInvalidation(queryKeys.automation.agent(workspaceId, eventAgentId))
          scheduleAgentRunInvalidation(queryKeys.automation.agentUsage(workspaceId, eventAgentId))
        } else {
          scheduleAgentRunInvalidation(queryKeys.automation.agentsRoot(workspaceId))
        }
        if (event.parent_type === 'task' && event.parent_id) {
          scheduleAgentRunInvalidation(queryKeys.pm.task(workspaceId, event.parent_id))
          scheduleAgentRunInvalidation(['pm', workspaceId, 'tasks'])
        }
      }
    } else if (event.entity === 'crm_contact') {
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.contacts(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.enrichments(workspaceId) })
      queryClient.invalidateQueries({ queryKey: ['support', workspaceId] })
      if (event.entity_id) {
        queryClient.invalidateQueries({ queryKey: queryKeys.crm.contact(workspaceId, event.entity_id) })
        queryClient.invalidateQueries({ queryKey: queryKeys.crm.contactActivities(workspaceId, event.entity_id) })
      }
    } else if (event.entity === 'crm_company') {
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.companies(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.enrichments(workspaceId) })
      if (event.entity_id) {
        queryClient.invalidateQueries({ queryKey: queryKeys.crm.company(workspaceId, event.entity_id) })
        queryClient.invalidateQueries({ queryKey: queryKeys.crm.companyActivities(workspaceId, event.entity_id) })
      }
      queryClient.invalidateQueries({
        predicate: (query) => query.queryKey[0] === 'support' && query.queryKey[1] === workspaceId && query.queryKey.includes('visitor-context'),
      })
    } else if (event.entity === 'crm_deal') {
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.deals(workspaceId) })
    } else if (event.entity === 'support_personal_read') {
      if (event.target_user_id && event.target_user_id !== selfIdRef.current) return
      const unreadCount = typeof event.data?.unread_count === 'number' ? event.data.unread_count : null
      const version = typeof event.data?.personal_state_version === 'number' ? event.data.personal_state_version : null
      if (unreadCount === null || version === null || !event.entity_id) return
      const patch = { conversationId: event.entity_id, unreadCount, version }
      queryClient.setQueriesData<SupportConversationListCache>(
        { predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId) },
        (current) => patchConversationPersonalReadInCache(current, patch),
      )
      queryClient.setQueryData<SupportConversation>(
        queryKeys.support.conversation(workspaceId, event.entity_id),
        (current) => patchConversationDetailPersonalRead(current, patch),
      )
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) })
      queryClient.invalidateQueries({ queryKey: queryKeys.support.workspaceUnread() })
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
        const statusPatch = supportConversationStatusPatchFromEvent(event)
        if (statusPatch) {
          queryClient.setQueriesData<SupportConversationListCache>(
            {
              predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
            },
            (current) => patchConversationStatusInCache(current, statusPatch),
          )
          queryClient.setQueryData(
            queryKeys.support.conversation(workspaceId, event.entity_id),
            (current: SupportConversation | undefined) => patchConversationDetailStatus(current, statusPatch),
          )

          const inboxStore = useSupportInboxStore.getState()
          if (
            statusPatch.oldStatus === 'resolved' &&
            statusPatch.status === 'open' &&
            inboxStore.selectedConversationId === event.entity_id
          ) {
            inboxStore.showReopenedConversationInInbox(event.entity_id, statusPatch.mailboxId)
          }
          queryClient.invalidateQueries({ queryKey: queryKeys.support.workspaceUnread() })
        }
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, event.entity_id) })
        // Invalidate unread stats on any non-presence conversation update (includes reason=read)
        queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) })
        queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) })
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
      queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.memberPresence(workspaceId) })
    } else if (event.entity === 'support_conversation_message') {
      if (event.parent_id) {
        const s = useSupportPresenceStore.getState()
        // Clear typing state for whoever sent this message
        if (event.actor_id?.startsWith('widget:')) {
          s.setTyping(event.parent_id, false)
        } else if (event.actor_id) {
          s.clearOneAgentTyping(event.parent_id, event.actor_id)
        }

        // Play notification sound for hydrated messages from others.
        // Internal/data-less message events (e.g. internal system events
        // accompanying an escalation) are refetch signals only — backend
        // strips event.data for IsInternal rows, so skipping when data is
        // absent prevents double pings on a single conversational event.
        if (event.data && event.actor_id !== selfIdRef.current) {
          playNotificationSound()
        }

        // Batch-invalidate all support conversation queries in a single call:
        // matches conversations list, conversation detail, and messages
        const parentId = event.parent_id
        queryClient.setQueriesData<SupportConversationListCache>(
          {
            predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
          },
          (current) => moveConversationToTopForMessageActivity(current, {
            conversationId: parentId,
            messageId: event.entity_id,
            timestamp: typeof event.data?.created_at === 'string' ? event.data.created_at : event.sent_at ?? new Date().toISOString(),
            message: event.data,
          }),
        )
        const messageData = event.data
        const hasHydratedMessage = messageData
          && typeof messageData.content === 'string'
          && typeof messageData.sender_type === 'string'
        if (hasHydratedMessage && messageData) {
          const createdAt = typeof messageData.created_at === 'string'
            ? messageData.created_at
            : event.sent_at ?? new Date().toISOString()
          const realtimeMessage = {
            ...messageData,
            id: event.entity_id,
            workspace_id: workspaceId,
            conversation_id: parentId,
            content: messageData.content,
            sender_type: messageData.sender_type,
            is_internal: Boolean(messageData.is_internal),
            created_at: createdAt,
            updated_at: typeof messageData.updated_at === 'string' ? messageData.updated_at : createdAt,
          } as SupportMessage
          queryClient.setQueryData<SupportMessagePages>(
            queryKeys.support.messages(workspaceId, parentId),
            (current) => current ? appendMessageToNewestPage(current, realtimeMessage) : current,
          )
        }
        queryClient.invalidateQueries({
          predicate: (query) => {
            const key = query.queryKey
            return key[0] === 'support' && key[1] === workspaceId && (
              // conversations list: ['support', wsId, 'conversations']
              key.length === 3 ||
              // conversation detail or messages: ['support', wsId, 'conversations', parentId, ...]
              (key[3] === parentId && !(hasHydratedMessage && key[4] === 'messages'))
            )
          },
        })
        // Coalesce message bursts into one counter refresh. Core counts are
        // materialized; custom views remain exact and converge after the same
        // short trailing window.
        scheduleSupportCounterInvalidation(
          messageData?.sender_type === 'customer' && !messageData.is_internal,
        )
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
        if (event.parent_type) {
          queryClient.invalidateQueries({ queryKey: queryKeys.pm.entityExternalLinks(workspaceId, event.parent_type, event.parent_id) })
        }
      } else if (event.entity === 'task_git_link') {
        queryClient.invalidateQueries({ queryKey: queryKeys.git.taskLinks(workspaceId, event.parent_id) })
      }
    }

    // Dispatch custom DOM events for any component that listens
    // e.g. "task-updated", "comment-created", "epic-deleted"
    if (event.entity !== 'agent_run' && !(event.entity === 'coding_session' && event.data?.change_kind)) {
      window.dispatchEvent(
        new CustomEvent(`${event.entity}-${event.action}`, {
          detail: event,
        })
      )
    }

    // Child entity events → also dispatch a parent update event
    // so that open task detail panels can refetch
    if (CHILD_ENTITIES.has(event.entity) && event.parent_type && event.parent_id) {
      window.dispatchEvent(
        new CustomEvent(`${event.parent_type}-child-updated`, {
          detail: event,
        })
      )
    }
  }, [scheduleRefresh, scheduleAgentRunInvalidation, scheduleSupportCounterInvalidation, workspaceId, queryClient])

  const onPresenceSnapshot = useCallback((snapshot: PresenceSnapshot) => {
    const convId = snapshot.conversation_id
    if (!convId) return
    const store = useSupportPresenceStore.getState()
    const selfId = selfIdRef.current

    const viewersArr = Array.isArray(snapshot.viewers) ? snapshot.viewers : []
    const nextViewers = viewersArr
      .map((viewer) => viewer.user_id)
      .filter((uid) => uid && uid !== selfId)
    store.replaceViewingAgents(convId, nextViewers)

    const typersObj = snapshot.typers && typeof snapshot.typers === 'object' ? snapshot.typers : {}
    const nextTypers = Object.fromEntries(
      Object.entries(typersObj)
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
    const docViewersArr = Array.isArray(snapshot.viewers) ? snapshot.viewers : []
    const nextViewers = Object.fromEntries(
      docViewersArr
        .filter((viewer) => viewer.user_id && viewer.user_id !== selfId)
        .map((viewer) => [viewer.user_id, {
          name: viewer.name,
          avatarUrl: viewer.avatar,
        }])
    )
    const nextEditors = Object.fromEntries(
      Object.entries(snapshot.editors ?? {})
        .filter(([uid, editor]) => uid !== selfId && editor?.user_id)
        .map(([uid, editor]) => [uid, {
          area: editor.area,
          section: editor.section,
          name: editor.name,
          avatarUrl: editor.avatar,
        }])
    )
    const store = useDocsPresenceStore.getState()
    store.replaceViewingUsers(docId, nextViewers)
    store.replaceEditingUsers(docId, nextEditors)

    for (const [uid] of Object.entries(nextEditors)) {
      const timerKey = `${docId}:docs:editing:${uid}`
      const prevTimer = typingTimers.current.get(timerKey)
      if (prevTimer) clearTimeout(prevTimer)
      const timer = setTimeout(() => {
        typingTimers.current.delete(timerKey)
        useDocsPresenceStore.getState().setEditingUser(docId, uid, false)
      }, DOC_EDITING_TIMEOUT_MS)
      typingTimers.current.set(timerKey, timer)
    }
  }, [])

  useEffect(() => {
    agentRunTracker.current = createAgentRunUpdateTracker()
    const timers = typingTimers.current
    return () => {
      clearTimeout(debounceTimer.current ?? undefined)
      clearTimeout(agentRunInvalidateTimer.current ?? undefined)
      clearTimeout(supportCounterInvalidateTimer.current ?? undefined)
      debounceTimer.current = null
      agentRunInvalidateTimer.current = null
      supportCounterInvalidateTimer.current = null
      supportWorkspaceUnreadPending.current = false
      pendingAgentRunInvalidations.current.clear()
      timers.forEach((t) => clearTimeout(t))
      timers.clear()
    }
  }, [workspaceId, selfId])

  const { send: wsSend, isConnected } = useWebSocket({ workspaceId, onEvent, onPresenceSnapshot, onDocsPresenceSnapshot })

  const previousConnection = useRef({ workspaceId, selfId, connected: isConnected })
  useEffect(() => {
    const previous = previousConnection.current
    previousConnection.current = { workspaceId, selfId, connected: isConnected }
    if (!isConnected || previous.connected || previous.workspaceId !== workspaceId || previous.selfId !== selfId) return
    agentRunTracker.current = createAgentRunUpdateTracker()
    const refetchType = isForegroundNetworkAvailable() ? 'active' : 'none'
    const keys = [
      queryKeys.automation.runsRoot(workspaceId), queryKeys.automation.runAttentionCount(workspaceId),
      queryKeys.automation.activityRoot(workspaceId), queryKeys.automation.overview(workspaceId),
      queryKeys.automation.agentFleet(workspaceId), queryKeys.automation.agentsRoot(workspaceId),
      queryKeys.notifications.all(workspaceId), queryKeys.support.teammatePresence(workspaceId),
      queryKeys.workspaces.memberPresence(workspaceId), queryKeys.support.workspaceUnread(),
      queryKeys.support.routingUsage(workspaceId), queryKeys.support.unreadStats(workspaceId),
      queryKeys.support.inboxScopes(workspaceId), queryKeys.support.inboxViewCounts(workspaceId),
    ]
    keys.forEach((queryKey) => { void queryClient.invalidateQueries({ queryKey, refetchType }) })
    void queryClient.invalidateQueries({
      predicate: ({ queryKey }) => queryKey[0] === 'support' && queryKey[1] === workspaceId && queryKey.at(-1) === 'ai-run-interactions',
      refetchType,
    })
  }, [isConnected, workspaceId, selfId, queryClient])

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
