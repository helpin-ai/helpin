import type { ApiResponse } from './auth-api'
import type {
  AssignableMember,
  ConversationListResponse,
  ConversationStatus,
  CreateConversationWithMessageRequest,
  CreateConversationWithMessageResponse,
  CreateTaskFromConversationRequest,
  CreateTaskFromConversationResponse,
  SendSupportConversationTranscriptResponse,
  SupportAIRewriteDraftRequest,
  SupportAIRewriteDraftResponse,
  SupportAIRunInteractionsResponse,
  SupportAgentRun,
  SupportAgentRunMessage,
  SupportDockChat,
  SupportDockChatDetail,
  SupportDockChatMessageListResponse,
  SupportDockPageContext,
  SendSupportDockChatMessageRequest,
  SupportAttachmentInitResponse,
  SupportCannedResponse,
  SupportConversation,
  SupportConversationSearchParams,
  SupportConversationSearchResponse,
  SupportInboxScopeListResponse,
  SupportInstallation,
  SupportInboxView,
  SupportInboxViewCount,
  SupportMailbox,
  SupportMailboxMember,
  SupportMessage,
  SupportMessageActionResponse,
  SupportMessageInfo,
  SupportMessagePage,
  SupportRunInteraction,
  SupportTag,
  SupportTeammatePresenceStatus,
  ResolveSupportRunInteractionRequest,
  UpdateConversationEmailRecipientsRequest,
  UnreadStats,
} from './support-types'
import type { VisitorContextResponse } from './visitor-types'

type ApiLike = {
  get: <T>(path: string) => Promise<ApiResponse<T>>
  post: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  put: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  patch?: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  del: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
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

export interface CreateSupportAttachmentPayload {
  file_name: string
  file_size: number
  content_type: string
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
  page?: number
  per_page?: number
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
  'page',
  'per_page',
] as const

export const supportService = {
  searchConversations: (workspaceId: string, params: SupportConversationSearchParams) => {
    const query = new URLSearchParams({ workspace_id: workspaceId })
    Object.entries(params).forEach(([key, value]) => {
      if (value === undefined || value === null) return
      const stringValue = String(value).trim()
      if (!stringValue) return
      query.set(key, stringValue)
    })
    return getApi().get<SupportConversationSearchResponse>(`/support/inbox/search?${query.toString()}`)
  },
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
  listCannedResponses: (workspaceId: string) =>
    getApi().get<SupportCannedResponse[]>(`/support/inbox/canned-responses${qs(workspaceId)}`),
  listTags: (workspaceId: string) =>
    getApi().get<SupportTag[]>(`/support/inbox/tags${qs(workspaceId)}`),
  addConversationTag: (workspaceId: string, conversationId: string, tagId: string) =>
    getApi().post<{ message: string }>(
      `/support/inbox/conversations/${conversationId}/tags/${tagId}${qs(workspaceId)}`,
      {},
    ),
  removeConversationTag: (workspaceId: string, conversationId: string, tagId: string) =>
    getApi().del<{ message: string }>(
      `/support/inbox/conversations/${conversationId}/tags/${tagId}${qs(workspaceId)}`,
    ),
  getInstallation: (workspaceId: string) =>
    getApi().get<SupportInstallation>(`/support/inbox/installations${qs(workspaceId)}`),
  updateConversationSubject: (workspaceId: string, conversationId: string, subject: string) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/subject${qs(workspaceId)}`, {
      subject,
    }),
  moveConversation: (workspaceId: string, conversationId: string, mailboxId: string | null) =>
    getApi().post<SupportConversation>(`/support/inbox/conversations/${conversationId}/move${qs(workspaceId)}`, {
      mailbox_id: mailboxId,
    }),
  dismissConversationTriage: (workspaceId: string, conversationId: string) =>
    getApi().post<SupportConversation['triage']>(
      `/support/inbox/conversations/${conversationId}/triage/dismiss${qs(workspaceId)}`,
      {},
    ),
  deleteConversation: (workspaceId: string, conversationId: string) =>
    getApi().del<void>(`/support/inbox/conversations/${conversationId}${qs(workspaceId)}`),
  rewriteConversationDraft: (workspaceId: string, conversationId: string, payload: SupportAIRewriteDraftRequest) =>
    getApi().post<SupportAIRewriteDraftResponse>(
      `/support/inbox/conversations/${conversationId}/rewrite-draft${qs(workspaceId)}`,
      payload,
    ),
  rewriteNewDraft: (workspaceId: string, payload: SupportAIRewriteDraftRequest) =>
    getApi().post<SupportAIRewriteDraftResponse>(
      `/support/inbox/rewrite-draft${qs(workspaceId)}`,
      payload,
    ),
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
  updateConversationCRMCompany: (workspaceId: string, conversationId: string, companyId: string | null) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/crm-company${qs(workspaceId)}`, {
      crm_company_id: companyId,
    }),
  updateConversationCRMContact: (workspaceId: string, conversationId: string, contactId: string | null) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/crm-contact${qs(workspaceId)}`, {
      crm_contact_id: contactId,
    }),
  updateConversationCustomerName: (workspaceId: string, conversationId: string, customerName: string) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/customer-name${qs(workspaceId)}`, {
      customer_name: customerName,
    }),
  updateConversationEmailRecipients: (
    workspaceId: string,
    conversationId: string,
    payload: UpdateConversationEmailRecipientsRequest,
  ) =>
    getApi().put<SupportConversation>(
      `/support/inbox/conversations/${conversationId}/email-recipients${qs(workspaceId)}`,
      payload,
    ),
  createConversationWithMessage: (workspaceId: string, payload: CreateConversationWithMessageRequest) =>
    getApi().post<CreateConversationWithMessageResponse>(
      `/support/inbox/conversations/create-and-send${qs(workspaceId)}`,
      payload,
    ),
  createTaskFromConversation: (
    workspaceId: string,
    conversationId: string,
    payload: CreateTaskFromConversationRequest,
  ) => getApi().post<CreateTaskFromConversationResponse>(
    `/support/inbox/conversations/${conversationId}/create-task${qs(workspaceId)}`,
    payload,
  ),
  listAIRunInteractions: (workspaceId: string, conversationId: string) =>
    getApi().get<SupportAIRunInteractionsResponse>(
      `/support/inbox/conversations/${conversationId}/ai-run/interactions${qs(workspaceId)}`,
    ),
  resolveAIRunInteraction: (
    workspaceId: string,
    conversationId: string,
    interactionId: string,
    payload: ResolveSupportRunInteractionRequest,
  ) => getApi().post<SupportRunInteraction>(
    `/support/inbox/conversations/${conversationId}/ai-run/interactions/${encodeURIComponent(interactionId)}/resolve${qs(workspaceId)}`,
    payload,
  ),
  listConversationAgentRuns: (workspaceId: string, conversationId: string) =>
    getApi().get<SupportAgentRun[]>(
      `/automation/runs${qs(workspaceId)}&target_type=support_conversation&target_id=${encodeURIComponent(conversationId)}`,
    ),
  listAgentRunMessages: (workspaceId: string, runId: string) =>
    getApi().get<SupportAgentRunMessage[]>(
      `/automation/runs/${encodeURIComponent(runId)}/messages${qs(workspaceId)}`,
    ),
  approveAgentRun: (workspaceId: string, runId: string) =>
    getApi().post<SupportAgentRun>(
      `/automation/runs/${encodeURIComponent(runId)}/approve${qs(workspaceId)}`,
      { send_message: true },
    ),
  ensureConversationDockChat: (workspaceId: string, conversationId: string) =>
    getApi().post<SupportDockChat>(`/dock/chats${qs(workspaceId)}`, {
      title: '',
      support_conversation_id: conversationId,
      module_id: 'support',
      visibility: 'module',
    }),
  getSupportDockChat: (workspaceId: string, chatId: string) =>
    getApi().get<SupportDockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`),
  listSupportDockChatMessages: (workspaceId: string, chatId: string, before?: number | null) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: '50' })
    if (before) query.set('before', String(before))
    return getApi().get<SupportDockChatMessageListResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/messages?${query.toString()}`,
    )
  },
  sendSupportDockChatMessage: (
    workspaceId: string,
    chatId: string,
    payload: SendSupportDockChatMessageRequest,
  ) => getApi().post<SupportDockChatDetail>(
    `/dock/chats/${encodeURIComponent(chatId)}/messages${qs(workspaceId)}`,
    payload,
  ),
  generateSupportDockChatTitle: (
    workspaceId: string,
    chatId: string,
    content: string,
    pageContext: SupportDockPageContext,
  ) => getApi().post<SupportDockChat>(
    `/dock/chats/${encodeURIComponent(chatId)}/title${qs(workspaceId)}`,
    { content, page_context: pageContext },
  ),
  listSupportDockRunInteractions: (workspaceId: string, chatId: string) =>
    getApi().get<{ interactions: SupportRunInteraction[] }>(
      `/dock/chats/${encodeURIComponent(chatId)}/run/interactions${qs(workspaceId)}`,
    ),
  resolveSupportDockRunInteraction: (
    workspaceId: string,
    chatId: string,
    interactionId: string,
    payload: ResolveSupportRunInteractionRequest,
  ) => getApi().post<SupportRunInteraction>(
    `/dock/chats/${encodeURIComponent(chatId)}/interactions/${encodeURIComponent(interactionId)}/resolve${qs(workspaceId)}`,
    payload,
  ),
  listConversationMessages: (workspaceId: string, conversationId: string) =>
    getApi().get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`),
  deleteConversationMessage: (
    workspaceId: string,
    conversationId: string,
    messageId: string,
    undo = false,
  ) =>
    getApi().del<SupportMessageActionResponse>(
      `/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}${undo ? '&undo=1' : ''}`,
    ),
  getConversationMessageInfo: (workspaceId: string, conversationId: string, messageId: string) =>
    getApi().get<SupportMessageInfo>(
      `/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}`,
    ),
  listConversationMessagePage: (
    workspaceId: string,
    conversationId: string,
    limit: number,
    cursor?: string,
  ) => {
    const cursorQuery = cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''
    return getApi().get<SupportMessagePage>(
      `/support/inbox/conversations/${conversationId}/message-pages${qs(workspaceId)}&limit=${limit}${cursorQuery}`,
    )
  },
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
  updateMyTeammatePresence: (
    workspaceId: string,
    manualStatus: SupportTeammatePresenceStatus['manual_status'] | null,
  ) => getApi().put<SupportTeammatePresenceStatus>(
    `/support/inbox/me/presence${qs(workspaceId)}`,
    { manual_status: manualStatus },
  ),
  markConversationRead: (workspaceId: string, conversationId: string) =>
    getApi().post(`/support/inbox/conversations/${conversationId}/read${qs(workspaceId)}`, {}),
  markConversationUnread: (workspaceId: string, conversationId: string) =>
    getApi().post(`/support/inbox/conversations/${conversationId}/unread${qs(workspaceId)}`, {}),
  sendMessage: (workspaceId: string, conversationId: string, payload: SendMessagePayload) =>
    getApi().post<SupportMessage>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`, payload),
  sendConversationTranscript: (workspaceId: string, conversationId: string, payload: {
    email: string
    update_customer_email?: boolean
  }) => getApi().post<SendSupportConversationTranscriptResponse>(
    `/support/inbox/conversations/${conversationId}/transcript${qs(workspaceId)}`,
    payload,
  ),
  initiateAttachmentUpload: (
    workspaceId: string,
    conversationId: string,
    payload: CreateSupportAttachmentPayload,
  ) => getApi().post<SupportAttachmentInitResponse>(
    `/support/inbox/conversations/${conversationId}/attachments${qs(workspaceId)}`,
    payload,
  ),
  confirmAttachmentUpload: (workspaceId: string, attachmentId: string) =>
    getApi().patch!<{ message: string }>(
      `/support/inbox/attachments/${attachmentId}/confirm${qs(workspaceId)}`,
    ),
  deleteAttachment: (workspaceId: string, attachmentId: string) =>
    getApi().del<{ message: string }>(
      `/support/inbox/attachments/${attachmentId}${qs(workspaceId)}`,
    ),
  updateConversationStatus: (workspaceId: string, conversationId: string, status: ConversationStatus) =>
    getApi().put<SupportConversation>(`/support/inbox/conversations/${conversationId}/status${qs(workspaceId)}`, { status }),
  assignConversationUser: (workspaceId: string, conversationId: string, payload: AssignConversationUserPayload) =>
    getApi().post<{ assigned: boolean }>(`/support/inbox/conversations/${conversationId}/assign-user${qs(workspaceId)}`, payload),
  listConversationAssignees: (workspaceId: string, conversationId: string) =>
    getApi().get<AssignableMember[]>(`/support/inbox/conversations/${conversationId}/assignees${qs(workspaceId)}`),
}
