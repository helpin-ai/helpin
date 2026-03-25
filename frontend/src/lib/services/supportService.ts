import { api } from '../api';
import type { SupportConversation, SupportMessage, CreateConversationRequest, CreateMessageRequest, LinkStoryRequest, AssignConversationAgentRequest, AgentRun, ConversationStatus, SupportInstallationResponse, SupportInboxSettings, ConversationListResponse, UnreadStats, VisitorContextResponse, SupportTeammatePresenceStatus } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const supportService = {
  listConversations: (workspaceId: string, filters?: { status?: string; priority?: string; filter?: string }) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`;
    if (filters?.status) path += `&status=${filters.status}`;
    if (filters?.priority) path += `&priority=${filters.priority}`;
    if (filters?.filter) path += `&filter=${filters.filter}`;
    return api.get<ConversationListResponse>(path);
  },
  getConversation: (workspaceId: string, id: string) =>
    api.get<SupportConversation>(`/support/inbox/conversations/${id}${qs(workspaceId)}`),
  createConversation: (workspaceId: string, payload: CreateConversationRequest) =>
    api.post<SupportConversation>(`/support/inbox/conversations${qs(workspaceId)}`, payload),
  listConversationMessages: (workspaceId: string, conversationId: string) =>
    api.get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`),
  createConversationMessage: (workspaceId: string, conversationId: string, payload: CreateMessageRequest) =>
    api.post<SupportMessage>(`/support/inbox/conversations/${conversationId}/messages${qs(workspaceId)}`, payload),
  updateConversationStatus: (workspaceId: string, conversationId: string, status: ConversationStatus) =>
    api.put<SupportConversation>(`/support/inbox/conversations/${conversationId}/status${qs(workspaceId)}`, { status }),
  linkConversationStory: (workspaceId: string, conversationId: string, payload: LinkStoryRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/link-story${qs(workspaceId)}`, payload),
  assignConversationAgent: (workspaceId: string, conversationId: string, payload: AssignConversationAgentRequest) =>
    api.post(`/support/inbox/conversations/${conversationId}/assign-agent${qs(workspaceId)}`, payload),
  runAgent: (workspaceId: string, conversationId: string) =>
    api.post<AgentRun>(`/support/inbox/conversations/${conversationId}/run-agent${qs(workspaceId)}`, {}),

  // Visitor context
  getVisitorContext: (workspaceId: string, conversationId: string) =>
    api.get<VisitorContextResponse>(`/support/inbox/conversations/${conversationId}/visitor-context${qs(workspaceId)}`),

  // Unread
  getUnreadStats: (workspaceId: string) =>
    api.get<UnreadStats>(`/support/inbox/unread-stats${qs(workspaceId)}`),
  listTeammatePresence: (workspaceId: string) =>
    api.get<SupportTeammatePresenceStatus[]>(`/support/inbox/teammates/presence${qs(workspaceId)}`),
  updateMyTeammatePresence: (workspaceId: string, manualStatus: 'online' | 'away' | 'offline' | null) =>
    api.put<SupportTeammatePresenceStatus>(`/support/inbox/me/presence${qs(workspaceId)}`, { manual_status: manualStatus }),
  markConversationRead: (workspaceId: string, conversationId: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/read${qs(workspaceId)}`, {}),
  markConversationUnread: (workspaceId: string, conversationId: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/unread${qs(workspaceId)}`, {}),
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
};
