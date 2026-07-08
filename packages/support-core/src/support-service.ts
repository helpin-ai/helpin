import type { ApiResponse } from './auth-api'
import type {
  AssignableMember,
  ConversationListResponse,
  ConversationStatus,
  SupportConversation,
  SupportInboxScopeListResponse,
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

export interface ConversationFilters {
  status?: string
  priority?: string
  filter?: string
  /**
   * Server-side "assigned_to" filter (CSV-capable, but the mobile client only
   * ever sends a single value: "unassigned" — see
   * server/internal/repository/support_inbox.go:966 and
   * server/internal/handler/support_inbox.go:71). There is no `filter=unassigned`
   * value server-side; unassigned is only expressed through this param.
   */
  assigned_to?: string
  mailbox_id?: string | null
  ai_state?: string
  flow_state?: string
}

export const supportService = {
  listConversations: (
    workspaceId: string,
    filters?: ConversationFilters,
  ) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`
    if (filters?.status) path += `&status=${filters.status}`
    if (filters?.priority) path += `&priority=${filters.priority}`
    if (filters?.filter) path += `&filter=${filters.filter}`
    if (filters?.assigned_to) path += `&assigned_to=${encodeURIComponent(filters.assigned_to)}`
    if (filters?.mailbox_id && filters.mailbox_id !== 'all') {
      path += `&mailbox_id=${encodeURIComponent(filters.mailbox_id)}`
    }
    if (filters?.ai_state) path += `&ai_state=${encodeURIComponent(filters.ai_state)}`
    if (filters?.flow_state) path += `&flow_state=${encodeURIComponent(filters.flow_state)}`
    return getApi().get<ConversationListResponse>(path)
  },
  listInboxScopes: (workspaceId: string) =>
    getApi().get<SupportInboxScopeListResponse>(`/support/inbox/mailboxes/scopes${qs(workspaceId)}`),
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
