import type { ApiResponse } from './auth-api'
import type {
  ConversationListResponse,
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
}
