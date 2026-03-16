import { api } from '../api';
import type { SupportConversation, SupportMessage, CreateConversationRequest, CreateMessageRequest, LinkStoryRequest, AssignConversationAgentRequest, PaginatedResponse, AgentRun, ConversationStatus, SupportInstallationResponse, SupportInboxSettings } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const supportService = {
  listConversations: (workspaceId: string, filters?: { status?: string; priority?: string }) => {
    let path = `/support/inbox/conversations${qs(workspaceId)}`;
    if (filters?.status) path += `&status=${filters.status}`;
    if (filters?.priority) path += `&priority=${filters.priority}`;
    return api.get<PaginatedResponse<SupportConversation[]>>(path);
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

  // Installation settings
  getInstallation: (workspaceId: string) =>
    api.get<SupportInstallationResponse>(`/support/inbox/installations${qs(workspaceId)}`),
  updateInstallationSettings: (workspaceId: string, settings: Partial<SupportInboxSettings>) =>
    api.patch<SupportInstallationResponse>(`/support/inbox/installations${qs(workspaceId)}`, settings),
  regenerateWidgetKey: (workspaceId: string) =>
    api.post<SupportInstallationResponse>(`/support/inbox/installations/regenerate-key${qs(workspaceId)}`, {}),
  sendTypingIndicator: (workspaceId: string, conversationId: string, isTyping: boolean, content?: string) =>
    api.post(`/support/inbox/conversations/${conversationId}/typing${qs(workspaceId)}`, { is_typing: isTyping, ...(content && { content }) }),
  sendViewingPresence: (workspaceId: string, conversationId: string, viewing: boolean) =>
    api.post(`/support/inbox/conversations/${conversationId}/viewing${qs(workspaceId)}`, { viewing }),
};
