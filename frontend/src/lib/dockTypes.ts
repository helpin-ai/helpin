import type {
  AgentIconKey,
  AgentPresetKey,
  AgentRun,
  AgentRunMessage,
  CodexAuthState,
  CodingSession,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  CommandBarPageContext,
  CommandBarPlanSummary,
  ResolveCodingSessionInteractionRequest,
} from '@/lib/pmTypes'

/** One dock conversation with creator-owned settings and explicit visibility. */
export interface DockChat {
  id: string
  workspace_id: string
  user_id: string
  title: string
  visibility: DockChatVisibility
  module_id?: DockChatModule | null
  support_conversation_id?: string | null
  active_run_id?: string | null
  active_run_status?: AgentRun['status'] | null
  last_message_at?: string | null
  archived_at?: string | null
  created_at: string
  updated_at: string
}

export interface DockChatDetail {
  chat: DockChat
  run?: AgentRun | null
  accepted_message?: AgentRunMessage | null
  plan_ids: string[]
  plans?: CommandBarPlanSummary[]
}

export interface DockChatListResponse {
  chats: DockChat[]
  next_cursor?: string | null
}

export interface SendDockChatMessageRequest {
  client_message_id: string
  content: string
  page_context?: CommandBarPageContext
  references?: DockEntityReference[]
  attachment_ids?: string[]
}

export interface DockChatMessageListResponse {
  messages: AgentRunMessage[]
  next_before?: number | null
}

export interface DockChatWorkDetailResponse {
  messages: AgentRunMessage[]
}

export type DockEntityReference = CommandBarPageContext

export interface DockChatMediaAttachment {
  id?: string
  local_id: string
  file_name: string
  file_type: string
  file_size: number
  preview_url?: string
  status: 'uploading' | 'ready' | 'failed'
}

export interface DockChatPersistedMediaAttachment {
  id: string
  file_name: string
  file_type: string
  file_size: number
}

export interface GenerateDockChatTitleRequest {
  content: string
  page_context?: CommandBarPageContext
}

export interface UpdateDockChatRequest {
  title?: string
  archived?: boolean
  visibility?: DockChatVisibility
}

export type DockChatVisibility = 'private' | 'module' | 'workspace'
export type DockChatModule = 'support' | 'crm' | 'pm' | 'docs'

export function dockChatModuleForContext(context?: CommandBarPageContext | null): DockChatModule | null {
  switch (context?.entity_type) {
    case 'support_conversation':
      return 'support'
    case 'crm_contact':
    case 'crm_deal':
      return 'crm'
    case 'task':
    case 'epic':
    case 'repository':
      return 'pm'
    case 'document':
      return 'docs'
    default:
      return null
  }
}

export type DockRunAttentionKind = 'input' | 'approval' | 'authentication'

export interface DockAgentIdentity {
  id: string
  name: string
  icon_key?: AgentIconKey
  preset_key?: AgentPresetKey
}

export interface DockRunSummary {
  run: AgentRun
  agent: DockAgentIdentity
  attention_kind?: DockRunAttentionKind
  last_activity_at: string
}

export interface DockRunListResponse {
  runs: DockRunSummary[]
  attention_count: number
}

export type PublicShareResourceType = 'dock_chat' | 'agent_run'

export interface PublicShareLink {
	token: string
	url: string
}

export interface PublicSharedDockChat {
	title: string
	open_path?: string
	messages: AgentRunMessage[]
	updated_at: string
}

export interface PublicSharedAgentRun {
	title: string
	open_path?: string
	session?: CodingSession | null
	events: import('@/lib/pmTypes').CodingSessionEvent[]
	interactions: CodingSessionInteraction[]
	artifacts: import('@/lib/pmTypes').AgentRunArtifact[]
}

export interface PublicSharedResource {
	resource_type: PublicShareResourceType
	dock_chat?: PublicSharedDockChat
	agent_run?: PublicSharedAgentRun
}

export interface DockRunAPI {
  getSnapshot: (workspaceId: string, runId: string) => Promise<{ data: CodingSession | null; error: string | null }>
  listEvents: (workspaceId: string, runId: string, after?: number) => Promise<{ data: CodingSessionEventListResponse | null; error: string | null }>
  listInteractions: (workspaceId: string, runId: string) => Promise<{ data: { interactions: CodingSessionInteraction[] } | null; error: string | null }>
  resolveInteraction: (workspaceId: string, runId: string, interactionId: string, payload: ResolveCodingSessionInteractionRequest) => Promise<{ data: CodingSessionInteraction | null; error: string | null }>
  sendMessage: (workspaceId: string, runId: string, content: string) => Promise<{ data: AgentRunMessage | null; error: string | null }>
  continueRun: (workspaceId: string, runId: string, content?: string) => Promise<{ data: AgentRun | null; error: string | null }>
  cancelRun: (workspaceId: string, runId: string) => Promise<{ data: AgentRun | null; error: string | null }>
  startAuth: (workspaceId: string, runId: string) => Promise<{ data: CodexAuthState | null; error: string | null }>
  cancelAuth: (workspaceId: string, runId: string) => Promise<{ data: CodexAuthState | null; error: string | null }>
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
export const DOCK_REFERENCES_OPEN = '<references>'
export const DOCK_REFERENCES_CLOSE = '</references>'
export const DOCK_ATTACHMENTS_OPEN = '<attachments>'
export const DOCK_ATTACHMENTS_CLOSE = '</attachments>'
export const DOCK_SOURCE_ATTACHMENTS_OPEN = '<source_attachments>'
export const DOCK_SOURCE_ATTACHMENTS_CLOSE = '</source_attachments>'
export const DOCK_ATTACHMENT_ANALYSIS_OPEN = '<attachment_analysis>'
export const DOCK_ATTACHMENT_ANALYSIS_CLOSE = '</attachment_analysis>'
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
  let visible = content
  for (const [open, close] of [
    [DOCK_PAGE_CONTEXT_OPEN, DOCK_PAGE_CONTEXT_CLOSE],
    [DOCK_REFERENCES_OPEN, DOCK_REFERENCES_CLOSE],
    [DOCK_ATTACHMENTS_OPEN, DOCK_ATTACHMENTS_CLOSE],
    [DOCK_SOURCE_ATTACHMENTS_OPEN, DOCK_SOURCE_ATTACHMENTS_CLOSE],
    [DOCK_ATTACHMENT_ANALYSIS_OPEN, DOCK_ATTACHMENT_ANALYSIS_CLOSE],
  ] as const) {
    const start = visible.lastIndexOf(open)
    if (start < 0) continue
    const end = visible.indexOf(close, start)
    if (end < 0) continue
    visible = visible.slice(0, start) + visible.slice(end + close.length)
  }
  return visible.trim()
}

export function parseDockMediaAttachments(content: string): DockChatPersistedMediaAttachment[] {
  const start = content.lastIndexOf(DOCK_ATTACHMENTS_OPEN)
  if (start < 0) return []
  const end = content.indexOf(DOCK_ATTACHMENTS_CLOSE, start)
  if (end < 0) return []
  try {
    const parsed = JSON.parse(content.slice(start + DOCK_ATTACHMENTS_OPEN.length, end))
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is DockChatPersistedMediaAttachment => (
      !!item
      && typeof item.id === 'string'
      && typeof item.file_name === 'string'
      && typeof item.file_type === 'string'
      && typeof item.file_size === 'number'
    ))
  } catch {
    return []
  }
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
