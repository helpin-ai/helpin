import { api } from '../api';
import type { AssignableMember } from '../types';
import type {
  SupportConversation,
  SupportMessage,
  SupportMessagePage,
  CreateConversationRequest,
  CreateConversationWithMessageRequest,
  CreateConversationWithMessageResponse,
  CreateMessageRequest,
  CreateTaskFromConversationRequest,
  CreateTaskFromConversationResponse,
  LinkTaskRequest,
  AssignConversationAgentRequest,
  AssignConversationUserRequest,
  UpdateConversationCRMContactRequest,
  UpdateConversationCRMCompanyRequest,
  UpdateConversationCustomerNameRequest,
  UpdateConversationEmailRecipientsRequest,
  AgentRun,
  ConversationStatus,
  SupportInstallationResponse,
  SupportInboxSettings,
  SupportRoutingUsageStatus,
  ConversationListResponse,
  UnreadStats,
  VisitorContextResponse,
  SupportTeammatePresenceStatus,
  SupportAIRewriteDraftRequest,
  SupportAIRewriteDraftResponse,
  SupportInboxScopeListResponse,
  SupportInboxView,
  SupportInboxViewCount,
  CreateSupportInboxViewRequest,
  UpdateSupportInboxViewRequest,
  UpdateSupportInboxBuiltinViewRequest,
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
  UpdateSupportEmailSenderRequest,
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
  SupportTag,
  SupportConversationSearchParams,
  SupportConversationSearchResponse,
  SendSupportConversationTranscriptResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const supportService = {
  searchConversations: (workspaceId: string, params: SupportConversationSearchParams) => {
    const query = new URLSearchParams({ workspace_id: workspaceId });
    Object.entries(params).forEach(([key, value]) => {
      if (value === undefined || value === null) return;
      const stringValue = String(value).trim();
      if (!stringValue) return;
      query.set(key, stringValue);
    });
    return api.get<SupportConversationSearchResponse>(`/support/inbox/search?${query.toString()}`);
  },
  listConversations: (
    workspaceId: string,
    filters?: {
      status?: string;
      statuses?: string;
      priority?: string;
      filter?: string;
      mailbox_id?: string | null;
      mailbox_ids?: string;
      ai_state?: string;
      ai?: string;
      flow_state?: string;
      search?: string;
      assigned_to?: string;
      sort?: string;
      tag_ids?: string;
      system_tags?: string;
      page?: number;
      per_page?: number;
    },
  ) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`;
    if (filters?.status) path += `&status=${encodeURIComponent(filters.status)}`;
    if (filters?.statuses) path += `&statuses=${encodeURIComponent(filters.statuses)}`;
    if (filters?.priority) path += `&priority=${encodeURIComponent(filters.priority)}`;
    if (filters?.filter) path += `&filter=${encodeURIComponent(filters.filter)}`;
    if (filters?.mailbox_id && filters.mailbox_id !== 'all') path += `&mailbox_id=${encodeURIComponent(filters.mailbox_id)}`;
    if (filters?.mailbox_ids) path += `&mailbox_ids=${encodeURIComponent(filters.mailbox_ids)}`;
    if (filters?.ai_state) path += `&ai_state=${encodeURIComponent(filters.ai_state)}`;
    if (filters?.ai) path += `&ai=${encodeURIComponent(filters.ai)}`;
    if (filters?.flow_state) path += `&flow_state=${encodeURIComponent(filters.flow_state)}`;
    if (filters?.search?.trim()) path += `&search=${encodeURIComponent(filters.search.trim())}`;
    if (filters?.assigned_to) path += `&assigned_to=${encodeURIComponent(filters.assigned_to)}`;
    if (filters?.sort) path += `&sort=${encodeURIComponent(filters.sort)}`;
    if (filters?.tag_ids) path += `&tag_ids=${encodeURIComponent(filters.tag_ids)}`;
    if (filters?.system_tags) path += `&system_tags=${encodeURIComponent(filters.system_tags)}`;
    if (filters?.page) path += `&page=${encodeURIComponent(String(filters.page))}`;
    if (filters?.per_page) path += `&per_page=${encodeURIComponent(String(filters.per_page))}`;
    return api.get<ConversationListResponse>(path);
  },
  listInboxScopes: (workspaceId: string) =>
    api.get<SupportInboxScopeListResponse>(`/support/inbox/mailboxes/scopes${qs(workspaceId)}`),
  listInboxViews: (workspaceId: string) =>
    api.get<SupportInboxView[]>(`/support/inbox/views${qs(workspaceId)}`),
  listBuiltinInboxViews: (workspaceId: string) =>
    api.get<SupportInboxView[]>(`/support/inbox/views/builtin${qs(workspaceId)}`),
  listInboxViewCounts: (workspaceId: string) =>
    api.get<SupportInboxViewCount[]>(`/support/inbox/views/counts${qs(workspaceId)}`),
  createInboxView: (workspaceId: string, payload: CreateSupportInboxViewRequest) =>
    api.post<SupportInboxView>(`/support/inbox/views${qs(workspaceId)}`, payload),
  updateInboxView: (workspaceId: string, viewId: string, payload: UpdateSupportInboxViewRequest) =>
    api.put<SupportInboxView>(`/support/inbox/views/${viewId}${qs(workspaceId)}`, payload),
  updateBuiltinInboxView: (workspaceId: string, viewKey: string, payload: UpdateSupportInboxBuiltinViewRequest) =>
    api.put<SupportInboxView>(`/support/inbox/views/builtin/${encodeURIComponent(viewKey)}${qs(workspaceId)}`, payload),
  deleteInboxView: (workspaceId: string, viewId: string) =>
    api.del(`/support/inbox/views/${viewId}${qs(workspaceId)}`),
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
  sendEmailRouteTest: (workspaceId: string, routeId: string, sourceAddress: string) =>
    api.post<SupportEmailRoute>(`/support/inbox/email-routes/${routeId}/send-test${qs(workspaceId)}`, {
      source_address: sourceAddress,
    }),
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
  updateEmailSender: (workspaceId: string, senderId: string, payload: UpdateSupportEmailSenderRequest) =>
    api.put<SupportEmailSender>(`/support/inbox/email-senders/${senderId}${qs(workspaceId)}`, payload),
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
  listTags: (workspaceId: string) =>
    api.get<SupportTag[]>(`/support/inbox/tags${qs(workspaceId)}`),
  createTag: (workspaceId: string, payload: { name: string; color?: string }) =>
    api.post<SupportTag>(`/support/inbox/tags${qs(workspaceId)}`, payload),
  updateTag: (workspaceId: string, tagId: string, payload: { name?: string; color?: string }) =>
    api.put<SupportTag>(`/support/inbox/tags/${tagId}${qs(workspaceId)}`, payload),
  deleteTag: (workspaceId: string, tagId: string) =>
    api.del(`/support/inbox/tags/${tagId}${qs(workspaceId)}`),
  addConversationTag: (workspaceId: string, conversationId: string, tagId: string) =>
    api.post<{ message: string }>(`/support/inbox/conversations/${conversationId}/tags/${tagId}${qs(workspaceId)}`, {}),
  removeConversationTag: (workspaceId: string, conversationId: string, tagId: string) =>
    api.del<{ message: string }>(`/support/inbox/conversations/${conversationId}/tags/${tagId}${qs(workspaceId)}`),
  getConversation: (workspaceId: string, id: string) =>
    api.get<SupportConversation>(`/support/inbox/conversations/${id}${qs(workspaceId)}`),
  listConversationAssignees: (workspaceId: string, conversationId: string) =>
    api.get<AssignableMember[]>(`/support/inbox/conversations/${conversationId}/assignees${qs(workspaceId)}`),
  createConversation: (workspaceId: string, payload: CreateConversationRequest) =>
    api.post<SupportConversation>(`/support/inbox/conversations${qs(workspaceId)}`, payload),
  createConversationWithMessage: (workspaceId: string, payload: CreateConversationWithMessageRequest) =>
    api.post<CreateConversationWithMessageResponse>(`/support/inbox/conversations/create-and-send${qs(workspaceId)}`, payload),
  listConversationMessages: (workspaceId: string, conversationId: string) =>
    api.get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`),
  listConversationMessagePage: (workspaceId: string, conversationId: string, limit: number, cursor?: string) => {
    const cursorQuery = cursor ? `&cursor=${encodeURIComponent(cursor)}` : '';
    return api.get<SupportMessagePage>(
      `/support/inbox/conversations/${conversationId}/message-pages${qs(workspaceId)}&limit=${limit}${cursorQuery}`,
    );
  },
  sendConversationTranscript: (workspaceId: string, conversationId: string, payload: { email?: string; update_customer_email?: boolean }) =>
    api.post<SendSupportConversationTranscriptResponse>(`/support/inbox/conversations/${conversationId}/transcript${qs(workspaceId)}`, payload),
  createConversationMessage: (workspaceId: string, conversationId: string, payload: CreateMessageRequest) =>
    api.post<SupportMessage>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`, payload),
  deleteConversationMessage: (workspaceId: string, conversationId: string, messageId: string, undo = false) => {
    let path = `/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}`;
    if (undo) path += '&undo=1';
    return api.del<SupportMessageActionResponse>(path);
  },
  getConversationMessageInfo: (workspaceId: string, conversationId: string, messageId: string) =>
    api.get<SupportMessageInfo>(`/support/inbox/conversations/${conversationId}/messages/${messageId}${qs(workspaceId)}`),
  rewriteNewDraft: (workspaceId: string, payload: SupportAIRewriteDraftRequest) =>
    api.post<SupportAIRewriteDraftResponse>(`/support/inbox/rewrite-draft${qs(workspaceId)}`, payload),
  rewriteConversationDraft: (workspaceId: string, conversationId: string, payload: SupportAIRewriteDraftRequest) =>
    api.post<SupportAIRewriteDraftResponse>(`/support/inbox/conversations/${conversationId}/rewrite-draft${qs(workspaceId)}`, payload),
  updateConversationStatus: (workspaceId: string, conversationId: string, status: ConversationStatus) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/status${qs(workspaceId)}`, { status }),
  linkConversationTask: (workspaceId: string, conversationId: string, payload: LinkTaskRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/link-story${qs(workspaceId)}`, { story_id: payload.task_id }),
  createTaskFromConversation: (
    workspaceId: string,
    conversationId: string,
    payload: CreateTaskFromConversationRequest,
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
  updateConversationCRMCompany: (workspaceId: string, conversationId: string, payload: UpdateConversationCRMCompanyRequest) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/crm-company${qs(workspaceId)}`, payload),
  updateConversationCustomerName: (workspaceId: string, conversationId: string, payload: UpdateConversationCustomerNameRequest) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/customer-name${qs(workspaceId)}`, payload),
  updateConversationEmailRecipients: (workspaceId: string, conversationId: string, payload: UpdateConversationEmailRecipientsRequest) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/email-recipients${qs(workspaceId)}`, payload),
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
  getRoutingUsageStatus: (workspaceId: string) =>
    api.get<SupportRoutingUsageStatus>(`/support/inbox/routing-usage${qs(workspaceId)}`),
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

  // Interactions raised by the conversation's AI chat run (e.g. child-launch
  // approvals a teammate must resolve).
  listAIRunInteractions: (workspaceId: string, conversationId: string) =>
    api.get<{ run_id: string; interactions: import('@/lib/pmTypes').CodingSessionInteraction[] }>(
      `/support/inbox/conversations/${conversationId}/ai-run/interactions${qs(workspaceId)}`,
    ),
  resolveAIRunInteraction: (
    workspaceId: string,
    conversationId: string,
    interactionId: string,
    payload: { response_payload: Record<string, unknown>; followup_message?: string },
  ) =>
    api.post(
      `/support/inbox/conversations/${conversationId}/ai-run/interactions/${interactionId}/resolve${qs(workspaceId)}`,
      payload,
    ),
};
