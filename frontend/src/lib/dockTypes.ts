import type { AgentRun, CommandBarPageContext, CommandBarPlanSummary } from '@/lib/pmTypes'

/** One user-owned dock conversation, backed by an agent-runtime chat-mode run. */
export interface DockChat {
  id: string
  workspace_id: string
  user_id: string
  title: string
  active_run_id?: string | null
  last_message_at?: string | null
  archived_at?: string | null
  created_at: string
  updated_at: string
}

export interface DockChatDetail {
  chat: DockChat
  run?: AgentRun | null
  plan_ids: string[]
  plans?: CommandBarPlanSummary[]
}

export interface SendDockChatMessageRequest {
  content: string
  page_context?: CommandBarPageContext
}

export interface UpdateDockChatRequest {
  title?: string
  archived?: boolean
}

/**
 * Approval payload contract for dock launches: the chat agent calls
 * request_approval with this shape before any mutating agents.* tool call.
 * The dock renders it as the confirm card.
 */
export interface DockPlanConfirmPayload {
  kind?: string
  /** The runtime's request_approval tool carries the contract as phase. */
  phase?: string
  title?: string
  summary?: string
  action?: DockPlanConfirmAction
  raw_input?: { action?: DockPlanConfirmAction }
}

export interface DockPlanConfirmAction {
  steps?: DockPlanConfirmStep[]
  // Direct Dock execution proposal form.
  proposal_id?: string
  operations?: DockExecutionOperation[]
  expected_outcomes?: string[]
  // create_agent form
  name?: string
  description?: string
  // promote_run form
  run_id?: string
  allowed_tools?: string[]
  allowed_targets?: string[]
}

export interface DockExecutionOperation {
  tool_name: string
  max_calls?: number
  constraints?: Record<string, unknown>
}

export interface DockPlanConfirmStep {
  agent_id?: string
  use_command_agent?: boolean
  target?: { type?: string; id?: string }
  instructions?: string
  allowed_tools?: string[]
  depends_on_step_indexes?: number[]
}

/** Marker tags used inside chat-run message content. */
export const DOCK_PAGE_CONTEXT_OPEN = '<page_context>'
export const DOCK_PAGE_CONTEXT_CLOSE = '</page_context>'
export const DOCK_CHILD_RESULT_OPEN = '<child_run_result>'
export const DOCK_CHILD_RESULT_CLOSE = '</child_run_result>'
export const DOCK_PREVIOUS_CONVERSATION_OPEN = '<previous_conversation>'
export const DOCK_PREVIOUS_CONVERSATION_CLOSE = '</previous_conversation>'

export interface DockChildRunResult {
  plan_id: string
  status: string
  prompt?: string
  error?: string
  runs: Array<{
    run_id: string
    agent_name?: string
    status: string
    summary?: string
    summary_truncated?: boolean
    summary_char_count?: number
    result_available?: boolean
    artifacts?: Array<{
      artifact_id: string
      artifact_type: string
      format: string
      storage_mode: string
    }>
  }>
}

/** Strips a trailing page-context block from a user message for display. */
export function stripDockPageContext(content: string): string {
  const start = content.lastIndexOf(DOCK_PAGE_CONTEXT_OPEN)
  if (start < 0) return content
  const end = content.indexOf(DOCK_PAGE_CONTEXT_CLOSE, start)
  if (end < 0) return content
  return (content.slice(0, start) + content.slice(end + DOCK_PAGE_CONTEXT_CLOSE.length)).trim()
}

/** Removes backend-owned context envelopes prepended to successor-run turns. */
export function stripDockLeadingContext(content: string): string {
  const markers = [
    [DOCK_PREVIOUS_CONVERSATION_OPEN, DOCK_PREVIOUS_CONVERSATION_CLOSE],
    [DOCK_CHILD_RESULT_OPEN, DOCK_CHILD_RESULT_CLOSE],
  ] as const
  let visible = content.trim()

  while (visible) {
    const marker = markers.find(([open]) => visible.startsWith(open))
    if (!marker) break
    const [, close] = marker
    const end = visible.indexOf(close)
    if (end < 0) break
    visible = visible.slice(end + close.length).trimStart()
  }

  return visible.trim()
}

/** Parses a child-run-result block if the message is one, else null. */
export function parseDockChildResult(content: string): DockChildRunResult | null {
  const trimmed = content.trim()
  if (!trimmed.startsWith(DOCK_CHILD_RESULT_OPEN) || !trimmed.endsWith(DOCK_CHILD_RESULT_CLOSE)) {
    return null
  }
  const inner = trimmed.slice(DOCK_CHILD_RESULT_OPEN.length, trimmed.length - DOCK_CHILD_RESULT_CLOSE.length)
  try {
    const parsed = JSON.parse(inner) as DockChildRunResult
    if (!parsed || typeof parsed.plan_id !== 'string') return null
    return { ...parsed, runs: Array.isArray(parsed.runs) ? parsed.runs : [] }
  } catch {
    return null
  }
}

/** Parses a dock_plan_confirm approval request payload if it is one. */
export function parseDockPlanConfirm(payload: unknown): DockPlanConfirmPayload | null {
  if (!payload || typeof payload !== 'object') return null
  const candidate = payload as DockPlanConfirmPayload
  const phase = candidate.kind ?? candidate.phase
  if (phase !== 'dock_plan_confirm' && phase !== 'dock_execution_confirm') return null
  return { ...candidate, action: candidate.action ?? candidate.raw_input?.action }
}
