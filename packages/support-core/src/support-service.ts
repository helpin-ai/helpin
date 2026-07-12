import type { ApiResponse } from './auth-api'
import type {
  AssignableMember,
  ConversationListResponse,
  ConversationStatus,
  SupportConversation,
  SupportInboxScopeListResponse,
  SupportInboxView,
  SupportInboxViewCount,
  SupportMailbox,
  SupportMailboxMember,
  SupportMessage,
  SupportTeammatePresenceStatus,
  UnreadStats,
} from './support-types'
import type { VisitorContextResponse } from './visitor-types'

type ApiLike = {
  get: <T>(path: string) => Promise<ApiResponse<T>>
  post: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  put: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
}

/**
 * Body for POST /support/inbox/conversations/{id}/messages.
 * Field names verified against server/internal/model.CreateMessageRequest
 * (server/internal/model/support_inbox.go:808) and the web composer's payload
 * (frontend/src/lib/services/supportService.ts:214-215).
 */
export interface SendMessagePayload {
  content: string
  is_internal?: boolean
  channels?: ('chat' | 'email')[]
  cc_emails?: string[]
  bcc_emails?: string[]
  attachment_ids?: string[]
}

/**
 * Body for POST /support/inbox/conversations/{id}/assign-user.
 * Verified against frontend/src/lib/services/supportService.ts:244-245 and
 * the AssignConversationUserRequest type in frontend/src/lib/pm-types/support.ts:945-947.
 */
export interface AssignConversationUserPayload {
  user_id: string | null
}

let activeApi: ApiLike | null = null

export function configureSupportApi(api: ApiLike): void {
  activeApi = api
}

function getApi(): ApiLike {
  if (activeApi) {
    return activeApi
  }
  throw new Error('support api is not configured')
}

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

/**
 * Superset of the web's `ConversationListRequestFilters`
 * (frontend/src/lib/supportInboxFilters.ts) plus the mobile-legacy `priority`
 * and `ai_state` fields. Every present, non-empty field is serialized to the
 * `/support/inbox/conversations` query with its own key name so the mobile
 * inbox — which derives these via the web's `buildConversationListRequestFilters`
 * — sends a query identical to the web app for every view.
 */
export interface ConversationFilters {
  status?: string
  /** CSV of statuses (e.g. "open,waiting_on_customer"). */
  statuses?: string
  priority?: string
  filter?: string
  /**
   * Server-side "assigned_to" filter (CSV-capable). There is no
   * `filter=unassigned` value server-side; unassigned is only expressed here.
   */
  assigned_to?: string
  mailbox_id?: string | null
  /** CSV of mailbox ids. */
  mailbox_ids?: string
  /** Web param name for AI-state filtering (CSV, or "none"). */
  ai?: string
  /** Legacy single AI-state param (kept for existing mobile callers). */
  ai_state?: string
  flow_state?: string
  search?: string
  sort?: string
  tag_ids?: string
  system_tags?: string
}

// Serialized in a fixed order for stable, comparable query strings.
const CONVERSATION_FILTER_KEYS = [
  'status',
  'statuses',
  'priority',
  'filter',
  'mailbox_id',
  'mailbox_ids',
  'ai',
  'ai_state',
  'flow_state',
  'search',
  'assigned_to',
  'sort',
  'tag_ids',
  'system_tags',
] as const

export const supportService = {
  listConversations: (
    workspaceId: string,
    filters?: ConversationFilters,
  ) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`
    for (const key of CONVERSATION_FILTER_KEYS) {
      const value = filters?.[key]
      if (value == null || value === '') continue
      // Preserve prior behavior: a mailbox_id of "all" means "no scope".
      if (key === 'mailbox_id' && value === 'all') continue
      path += `&${key}=${encodeURIComponent(String(value))}`
    }
    return getApi().get<ConversationListResponse>(path)
  },
  listInboxScopes: (workspaceId: string) =>
    getApi().get<SupportInboxScopeListResponse>(`/support/inbox/mailboxes/scopes${qs(workspaceId)}`),
  listInboxViewCounts: (workspaceId: string) =>
    getApi().get<SupportInboxViewCount[]>(`/support/inbox/views/counts${qs(workspaceId)}`),
  listBuiltinInboxViews: (workspaceId: string) =>
    getApi().get<SupportInboxView[]>(`/support/inbox/views/builtin${qs(workspaceId)}`),
  listInboxViews: (workspaceId: string) =>
    getApi().get<SupportInboxView[]>(`/support/inbox/views${qs(workspaceId)}`),
  listMailboxes: (workspaceId: string) =>
    getApi().get<SupportMailbox[]>(`/support/inbox/mailboxes${qs(workspaceId)}`),
  listMailboxMembers: (workspaceId: string, mailboxId: string) =>
    getApi().get<SupportMailboxMember[]>(`/support/inbox/mailboxes/${mailboxId}/members${qs(workspaceId)}`),
  getConversation: (workspaceId: string, id: string) =>
    getApi().get<SupportConversation>(`/support/inbox/conversations/${id}${qs(workspaceId)}`),
  listConversationMessages: (workspaceId: string, conversationId: string) =>
    getApi().get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`),
  getVisitorContext: (workspaceId: string, conversationId: string) =>
    getApi().get<VisitorContextResponse>(`/support/inbox/conversations/${conversationId}/visitor-context${qs(workspaceId)}`),
  getUnreadStats: (workspaceId: string, mailboxId?: string | null) => {
    let path = `/support/inbox/unread-stats${qs(workspaceId)}`
    if (mailboxId && mailboxId !== 'all') {
      path += `&mailbox_id=${encodeURIComponent(mailboxId)}`
    }
    return getApi().get<UnreadStats>(path)
  },
  listTeammatePresence: (workspaceId: string) =>
    getApi().get<SupportTeammatePresenceStatus[]>(`/support/inbox/teammates/presence${qs(workspaceId)}`),
  markConversationRead: (workspaceId: string, conversationId: string) =>
    getApi().post(`/support/inbox/conversations/${conversationId}/read${qs(workspaceId)}`, {}),
  markConversationUnread: (workspaceId: string, conversationId: string) =>
    getApi().post(`/support/inbox/conversations/${conversationId}/unread${qs(workspaceId)}`, {}),
  sendMessage: (workspaceId: string, conversationId: string, payload: SendMessagePayload) =>
    getApi().post<SupportMessage>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`, payload),
  updateConversationStatus: (workspaceId: string, conversationId: string, status: ConversationStatus) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/status${qs(workspaceId)}`, { status }),
  assignConversationUser: (workspaceId: string, conversationId: string, payload: AssignConversationUserPayload) =>
    getApi().post<{ assigned: boolean }>(`/support/inbox/conversations/${conversationId}/assign-user${qs(workspaceId)}`, payload),
  listConversationAssignees: (workspaceId: string, conversationId: string) =>
    getApi().get<AssignableMember[]>(`/support/inbox/conversations/${conversationId}/assignees${qs(workspaceId)}`),
}
