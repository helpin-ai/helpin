import { api } from '../api';
import type { AssignableMember } from '../types';
import type {
  SupportConversation,
  SupportMessage,
  CreateConversationRequest,
  CreateMessageRequest,
  CreateTaskFromConversationRequest,
  CreateTaskFromConversationResponse,
  LinkTaskRequest,
  AssignConversationAgentRequest,
  AssignConversationUserRequest,
  UpdateConversationCRMContactRequest,
  AgentRun,
  ConversationStatus,
  SupportInstallationResponse,
  SupportInboxSettings,
  ConversationListResponse,
  UnreadStats,
  VisitorContextResponse,
  SupportTeammatePresenceStatus,
  SupportAIRewriteDraftRequest,
  SupportAIRewriteDraftResponse,
  SupportInboxScopeListResponse,
  SupportWorkspaceUnreadCount,
  SupportMailbox,
  CreateSupportMailboxRequest,
  UpdateSupportMailboxRequest,
  SupportMailboxMember,
  SupportEmailRoute,
  CreateSupportEmailRouteRequest,
  SupportEmailSender,
  CreateSupportEmailSenderRequest,
  SetSupportEmailSenderDefaultRequest,
  SupportEmailSenderDomain,
  CreateSupportEmailSenderDomainRequest,
  SupportTriageRule,
  CreateSupportTriageRuleRequest,
  UpdateSupportTriageRuleRequest,
  SupportConversationTriage,
  SupportMessageActionResponse,
  SupportMessageEmailDetail,
  SupportCannedResponse,
  CreateCannedResponseRequest,
  UpdateCannedResponseRequest,
  SupportMessageInfo,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const supportService = {
  listConversations: (
    workspaceId: string,
    filters?: {
      status?: string;
      priority?: string;
      filter?: string;
      mailbox_id?: string | null;
      ai_state?: string;
      flow_state?: string;
      search?: string;
      page?: number;
      per_page?: number;
    },
  ) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`;
    if (filters?.status) path += `&status=${encodeURIComponent(filters.status)}`;
    if (filters?.priority) path += `&priority=${encodeURIComponent(filters.priority)}`;
    if (filters?.filter) path += `&filter=${encodeURIComponent(filters.filter)}`;
    if (filters?.mailbox_id && filters.mailbox_id !== 'all') path += `&mailbox_id=${encodeURIComponent(filters.mailbox_id)}`;
    if (filters?.ai_state) path += `&ai_state=${encodeURIComponent(filters.ai_state)}`;
    if (filters?.flow_state) path += `&flow_state=${encodeURIComponent(filters.flow_state)}`;
    if (filters?.search?.trim()) path += `&search=${encodeURIComponent(filters.search.trim())}`;
    if (filters?.page) path += `&page=${encodeURIComponent(String(filters.page))}`;
    if (filters?.per_page) path += `&per_page=${encodeURIComponent(String(filters.per_page))}`;
    return api.get<ConversationListResponse>(path);
  },
  listInboxScopes: (workspaceId: string) =>
    api.get<SupportInboxScopeListResponse>(`/support/inbox/mailboxes/scopes${qs(workspaceId)}`),
  listWorkspaceUnread: () =>
    api.get<SupportWorkspaceUnreadCount[]>(`/support/workspace-unread`),
  listMailboxes: (workspaceId: string) =>
    api.get<SupportMailbox[]>(`/support/inbox/mailboxes${qs(workspaceId)}`),
  createMailbox: (workspaceId: string, payload: CreateSupportMailboxRequest) =>
    api.post<SupportMailbox>(`/support/inbox/mailboxes${qs(workspaceId)}`, payload),
  updateMailbox: (workspaceId: string, mailboxId: string, payload: UpdateSupportMailboxRequest) =>
    api.put<SupportMailbox>(`/support/inbox/mailboxes/${mailboxId}${qs(workspaceId)}`, payload),
  archiveMailbox: (workspaceId: string, mailboxId: string) =>
    api.post<SupportMailbox>(`/support/inbox/mailboxes/${mailboxId}/archive${qs(workspaceId)}`, {}),
  reorderMailboxes: (workspaceId: string, mailboxIds: string[]) =>
    api.post(`/support/inbox/mailboxes/reorder${qs(workspaceId)}`, { mailbox_ids: mailboxIds }),
  listMailboxMembers: (workspaceId: string, mailboxId: string) =>
    api.get<SupportMailboxMember[]>(`/support/inbox/mailboxes/${mailboxId}/members${qs(workspaceId)}`),
  listEmailRoutes: (workspaceId: string) =>
    api.get<SupportEmailRoute[]>(`/support/inbox/email-routes${qs(workspaceId)}`),
  createEmailRoute: (workspaceId: string, payload: CreateSupportEmailRouteRequest) =>
    api.post<SupportEmailRoute>(`/support/inbox/email-routes${qs(workspaceId)}`, payload),
  disableEmailRoute: (workspaceId: string, routeId: string) =>
    api.post(`/support/inbox/email-routes/${routeId}/disable${qs(workspaceId)}`, {}),
  listEmailSenders: (workspaceId: string) =>
    api.get<SupportEmailSender[]>(`/support/inbox/email-senders${qs(workspaceId)}`),
  createEmailSender: (workspaceId: string, payload: CreateSupportEmailSenderRequest) =>
    api.post<SupportEmailSender>(`/support/inbox/email-senders${qs(workspaceId)}`, payload),
  verifyEmailSender: (workspaceId: string, senderId: string) =>
    api.post<SupportEmailSender>(`/support/inbox/email-senders/${senderId}/verify-dns${qs(workspaceId)}`, {}),
  setDefaultEmailSender: (workspaceId: string, senderId: string, payload: SetSupportEmailSenderDefaultRequest) =>
    api.post<SupportEmailSender>(`/support/inbox/email-senders/${senderId}/set-default${qs(workspaceId)}`, payload),
  disableEmailSender: (workspaceId: string, senderId: string) =>
    api.post(`/support/inbox/email-senders/${senderId}/disable${qs(workspaceId)}`, {}),
  listEmailSenderDomains: (workspaceId: string) =>
    api.get<SupportEmailSenderDomain[]>(`/support/inbox/email-sender-domains${qs(workspaceId)}`),
  createEmailSenderDomain: (workspaceId: string, payload: CreateSupportEmailSenderDomainRequest) =>
    api.post<SupportEmailSenderDomain>(`/support/inbox/email-sender-domains${qs(workspaceId)}`, payload),
  verifyEmailSenderDomain: (workspaceId: string, domainId: string) =>
    api.post<SupportEmailSenderDomain>(`/support/inbox/email-sender-domains/${domainId}/verify${qs(workspaceId)}`, {}),
  activateEmailSenderDomain: (workspaceId: string, domainId: string) =>
    api.post<SupportEmailSenderDomain>(`/support/inbox/email-sender-domains/${domainId}/activate${qs(workspaceId)}`, {}),
  deactivateEmailSenderDomain: (workspaceId: string, domainId: string) =>
    api.post(`/support/inbox/email-sender-domains/${domainId}/deactivate${qs(workspaceId)}`, {}),
  listTriageRules: (workspaceId: string) =>
    api.get<SupportTriageRule[]>(`/support/inbox/triage-rules${qs(workspaceId)}`),
  createTriageRule: (workspaceId: string, payload: CreateSupportTriageRuleRequest) =>
    api.post<SupportTriageRule>(`/support/inbox/triage-rules${qs(workspaceId)}`, payload),
  updateTriageRule: (workspaceId: string, ruleId: string, payload: UpdateSupportTriageRuleRequest) =>
    api.put<SupportTriageRule>(`/support/inbox/triage-rules/${ruleId}${qs(workspaceId)}`, payload),
  deleteTriageRule: (workspaceId: string, ruleId: string) =>
    api.del(`/support/inbox/triage-rules/${ruleId}${qs(workspaceId)}`),
  listCannedResponses: (workspaceId: string) =>
    api.get<SupportCannedResponse[]>(`/support/inbox/canned-responses${qs(workspaceId)}`),
  searchCannedResponses: (workspaceId: string, query: string) =>
    api.get<SupportCannedResponse[]>(`/support/inbox/canned-responses/search${qs(workspaceId)}&q=${encodeURIComponent(query)}`),
  createCannedResponse: (workspaceId: string, payload: CreateCannedResponseRequest) =>
    api.post<SupportCannedResponse>(`/support/inbox/canned-responses${qs(workspaceId)}`, payload),
  updateCannedResponse: (workspaceId: string, responseId: string, payload: UpdateCannedResponseRequest) =>
    api.put<SupportCannedResponse>(`/support/inbox/canned-responses/${responseId}${qs(workspaceId)}`, payload),
  deleteCannedResponse: (workspaceId: string, responseId: string) =>
    api.del(`/support/inbox/canned-responses/${responseId}${qs(workspaceId)}`),
  getConversation: (workspaceId: string, id: string) =>
    api.get<SupportConversation>(`/support/inbox/conversations/${id}${qs(workspaceId)}`),
  listConversationAssignees: (workspaceId: string, conversationId: string) =>
    api.get<AssignableMember[]>(`/support/inbox/conversations/${conversationId}/assignees${qs(workspaceId)}`),
  createConversation: (workspaceId: string, payload: CreateConversationRequest) =>
    api.post<SupportConversation>(`/support/inbox/conversations${qs(workspaceId)}`, payload),
  listConversationMessages: (workspaceId: string, conversationId: string) =>
    api.get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`),
  createConversationMessage: (workspaceId: string, conversationId: string, payload: CreateMessageRequest) =>
    api.post<SupportMessage>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`, payload),
  deleteConversationMessage: (workspaceId: string, conversationId: string, messageId: string, undo = false) => {
    let path = `/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}`;
    if (undo) path += '&undo=1';
    return api.del<SupportMessageActionResponse>(path);
  },
  getConversationMessageInfo: (workspaceId: string, conversationId: string, messageId: string) =>
    api.get<SupportMessageInfo>(`/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}`),
  rewriteConversationDraft: (workspaceId: string, conversationId: string, payload: SupportAIRewriteDraftRequest) =>
    api.post<SupportAIRewriteDraftResponse>(`/support/inbox/conversations/${conversationId}/rewrite-draft${qs(workspaceId)}`, payload),
  updateConversationStatus: (workspaceId: string, conversationId: string, status: ConversationStatus) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/status${qs(workspaceId)}`, { status }),
  linkConversationTask: (workspaceId: string, conversationId: string, payload: LinkTaskRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/link-story${qs(workspaceId)}`, { story_id: payload.task_id }),
  createTaskFromConversation: (
    workspaceId: string,
    conversationId: string,
    payload: CreateTaskFromConversationRequest = {},
  ) => api.post<CreateTaskFromConversationResponse>(
    `/support/inbox/conversations/${conversationId}/create-task${qs(workspaceId)}`,
    payload,
  ),
  /** @deprecated Use linkConversationTask instead */
  linkConversationStory: (workspaceId: string, conversationId: string, payload: { story_id: string }) =>
    api.post(`/support/inbox/conversations/${conversationId}/link-story${qs(workspaceId)}`, payload),
  assignConversationAgent: (workspaceId: string, conversationId: string, payload: AssignConversationAgentRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/assign-agent${qs(workspaceId)}`, payload),
  assignConversationUser: (workspaceId: string, conversationId: string, payload: AssignConversationUserRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/assign-user${qs(workspaceId)}`, payload),
  updateConversationCRMContact: (workspaceId: string, conversationId: string, payload: UpdateConversationCRMContactRequest) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/crm-contact${qs(workspaceId)}`, payload),
  runAgent: (workspaceId: string, conversationId: string) =>
    api.post<AgentRun>(`/support/inbox/conversations/${conversationId}/run-agent${qs(workspaceId)}`, {}),

  // Visitor context
  getVisitorContext: (workspaceId: string, conversationId: string) =>
    api.get<VisitorContextResponse>(`/support/inbox/conversations/${conversationId}/visitor-context${qs(workspaceId)}`),

  // Unread
  getUnreadStats: (workspaceId: string, mailboxId?: string | null) => {
    let path = `/support/inbox/unread-stats${qs(workspaceId)}`;
    if (mailboxId && mailboxId !== 'all') path += `&mailbox_id=${encodeURIComponent(mailboxId)}`;
    return api.get<UnreadStats>(path);
  },
  listTeammatePresence: (workspaceId: string) =>
    api.get<SupportTeammatePresenceStatus[]>(`/support/inbox/teammates/presence${qs(workspaceId)}`),
  updateMyTeammatePresence: (workspaceId: string, manualStatus: 'online' | 'away' | 'offline' | null) =>
    api.put<SupportTeammatePresenceStatus>(`/support/inbox/me/presence${qs(workspaceId)}`, { manual_status: manualStatus }),
  markConversationRead: (workspaceId: string, conversationId: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/read${qs(workspaceId)}`, {}),
  markConversationUnread: (workspaceId: string, conversationId: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/unread${qs(workspaceId)}`, {}),
  moveConversation: (workspaceId: string, conversationId: string, mailboxId: string | null) =>
    api.post<SupportConversation>(`/support/inbox/conversations/${conversationId}/move${qs(workspaceId)}`, { mailbox_id: mailboxId }),
  dismissConversationTriage: (workspaceId: string, conversationId: string) =>
    api.post<SupportConversationTriage>(`/support/inbox/conversations/${conversationId}/triage/dismiss${qs(workspaceId)}`, {}),
  updateConversationSubject: (workspaceId: string, conversationId: string, subject: string) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/subject${qs(workspaceId)}`, { subject }),
  deleteConversation: (workspaceId: string, conversationId: string) =>
    api.del(`/support/inbox/conversations/${conversationId}${qs(workspaceId)}`),

  // Installation settings
  getInstallation: (workspaceId: string) =>
    api.get<SupportInstallationResponse>(`/support/inbox/installations${qs(workspaceId)}`),
  updateInstallationSettings: (workspaceId: string, settings: Partial<SupportInboxSettings>) =>
    api.patch<SupportInstallationResponse>(`/support/inbox/installations${qs(workspaceId)}`, settings),
  regenerateWidgetKey: (workspaceId: string) =>
    api.post<SupportInstallationResponse>(`/support/inbox/installations/regenerate-key${qs(workspaceId)}`, {}),
  /**
   * @deprecated Use WebSocket `support:typing:start` / `support:typing:stop` messages instead.
   * Kept as HTTP fallback for clients without an active WebSocket connection.
   */
  sendTypingIndicator: (workspaceId: string, conversationId: string, isTyping: boolean, content?: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/typing${qs(workspaceId)}`, { is_typing: isTyping, ...(content && { content }) }),
  /**
   * @deprecated Use WebSocket `support:viewing:start` / `support:viewing:stop` messages instead.
   * Kept as HTTP fallback for clients without an active WebSocket connection.
   */
  sendViewingPresence: (workspaceId: string, conversationId: string, viewing: boolean) =>
    api.post(`/support/inbox/conversations/${conversationId}/viewing${qs(workspaceId)}`, { viewing }),

  // Email details for an individual message (only when via_channel === 'email').
  getMessageEmailDetail: (workspaceId: string, messageId: string) =>
    api.get<SupportMessageEmailDetail>(`/support/inbox/messages/${messageId}/email${qs(workspaceId)}`),
};
