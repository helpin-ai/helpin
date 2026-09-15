import { api } from '../api';
import type {
  DockChat,
  DockChatModule,
  DockChatDetail,
  DockChatListResponse,
  DockChatMessageListResponse,
  DockChatWorkDetailResponse,
  GenerateDockChatTitleRequest,
  DockRunListResponse,
  SendDockChatMessageRequest,
  UpdateDockChatRequest,
	PublicShareLink,
	PublicShareResourceType,
	PublicSharedResource,
} from '../dockTypes';
import type {
  AgentRun,
  CodingSession,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  ResolveCodingSessionInteractionRequest,
  AgentRunMessage,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const dockChatService = {
  getAIDefaults: (workspaceId: string) =>
    api.get<{ ai_profile_id: string | null }>(`/dock/ai-defaults${qs(workspaceId)}`),
	getPublicShare: (workspaceId: string, resourceType: PublicShareResourceType, resourceId: string) =>
		api.get<PublicShareLink | null>(`/dock/shares/${resourceType}/${encodeURIComponent(resourceId)}${qs(workspaceId)}`),
	createPublicShare: (workspaceId: string, resourceType: PublicShareResourceType, resourceId: string) =>
		api.post<PublicShareLink>(`/dock/shares/${resourceType}/${encodeURIComponent(resourceId)}${qs(workspaceId)}`, {}),
	revokePublicShare: (workspaceId: string, resourceType: PublicShareResourceType, resourceId: string) =>
		api.del<void>(`/dock/shares/${resourceType}/${encodeURIComponent(resourceId)}${qs(workspaceId)}`),
	getPublicSharedResource: (token: string) =>
		api.get<PublicSharedResource>(`/public/shares/${encodeURIComponent(token)}`),
  listChats: (workspaceId: string, cursor?: string | null, limit = 30, signal?: AbortSignal) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: String(limit) });
    if (cursor) query.set('cursor', cursor);
    return api.get<DockChatListResponse>(`/dock/chats?${query.toString()}`, { signal });
  },
  findSupportConversationChat: (workspaceId: string, conversationId: string) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, conversation_id: conversationId });
    return api.get<DockChat | null>(`/dock/chats/support-conversation?${query.toString()}`);
  },
  createChat: (workspaceId: string, title = '', supportConversationId?: string, moduleId?: DockChatModule | null) =>
    api.post<DockChat>(`/dock/chats${qs(workspaceId)}`, {
      title,
      ...(supportConversationId ? { support_conversation_id: supportConversationId } : {}),
      ...(moduleId ? { module_id: moduleId, visibility: 'module' } : {}),
    }),
  getChat: (workspaceId: string, chatId: string, signal?: AbortSignal) =>
    api.get<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`, { signal }),
  updateChat: (workspaceId: string, chatId: string, payload: UpdateDockChatRequest) =>
    api.patch<DockChat>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`, payload),
  sendMessage: (workspaceId: string, chatId: string, payload: SendDockChatMessageRequest) =>
    api.post<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}/messages${qs(workspaceId)}`, payload),
  listMessages: (workspaceId: string, chatId: string, before?: number | null, limit = 50, signal?: AbortSignal) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: String(limit) });
    if (before) query.set('before', String(before));
    return api.get<DockChatMessageListResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/messages?${query.toString()}`,
      { signal },
    );
  },
  getMessageWorkDetail: (workspaceId: string, chatId: string, messageId: string) =>
    api.get<DockChatWorkDetailResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/messages/${encodeURIComponent(messageId)}/work${qs(workspaceId)}`,
    ),
  generateTitle: (workspaceId: string, chatId: string, payload: GenerateDockChatTitleRequest) =>
    api.post<DockChat>(`/dock/chats/${encodeURIComponent(chatId)}/title${qs(workspaceId)}`, payload),
  getChatRun: (workspaceId: string, chatId: string, signal?: AbortSignal) =>
    api.get<CodingSession>(`/dock/chats/${encodeURIComponent(chatId)}/run${qs(workspaceId)}`, { signal }),
  listChatRunInteractions: (workspaceId: string, chatId: string) =>
    api.get<{ interactions: CodingSessionInteraction[] }>(
      `/dock/chats/${encodeURIComponent(chatId)}/run/interactions${qs(workspaceId)}`,
    ),
  listChatRunEvents: (workspaceId: string, chatId: string, after = 0, signal?: AbortSignal) =>
    api.get<CodingSessionEventListResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/run/events${qs(workspaceId)}&after=${encodeURIComponent(after)}&include_snapshot=false`,
      { signal },
    ),
  resolveInteraction: (
    workspaceId: string,
    chatId: string,
    interactionId: string,
    payload: ResolveCodingSessionInteractionRequest,
  ) =>
    api.post<CodingSessionInteraction>(
      `/dock/chats/${encodeURIComponent(chatId)}/interactions/${encodeURIComponent(interactionId)}/resolve${qs(workspaceId)}`,
      payload,
    ),
  cancelChatRun: (workspaceId: string, chatId: string) =>
    api.post<AgentRun>(`/dock/chats/${encodeURIComponent(chatId)}/run/cancel${qs(workspaceId)}`),
  listRuns: (workspaceId: string, cursor?: string | null, limit = 30, signal?: AbortSignal) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: String(limit) });
    if (cursor) query.set('cursor', cursor);
    return api.get<DockRunListResponse>(`/dock/runs?${query.toString()}`, { signal });
  },
  getRunSnapshot: (workspaceId: string, runId: string, signal?: AbortSignal) =>
    api.get<CodingSession>(`/dock/runs/${encodeURIComponent(runId)}/snapshot${qs(workspaceId)}`, { signal }),
  listRunEvents: (workspaceId: string, runId: string, after = 0, signal?: AbortSignal) =>
    api.get<CodingSessionEventListResponse>(
      `/dock/runs/${encodeURIComponent(runId)}/events${qs(workspaceId)}&after=${encodeURIComponent(after)}&include_snapshot=false`,
      { signal },
    ),
  listRunInteractions: (workspaceId: string, runId: string) =>
    api.get<{ interactions: CodingSessionInteraction[] }>(
      `/dock/runs/${encodeURIComponent(runId)}/interactions${qs(workspaceId)}`,
    ),
  resolveRunInteraction: (
    workspaceId: string,
    runId: string,
    interactionId: string,
    payload: ResolveCodingSessionInteractionRequest,
  ) => api.post<CodingSessionInteraction>(
    `/dock/runs/${encodeURIComponent(runId)}/interactions/${encodeURIComponent(interactionId)}/resolve${qs(workspaceId)}`,
    payload,
  ),
  sendRunMessage: (workspaceId: string, runId: string, content: string) =>
    api.post<AgentRunMessage>(`/dock/runs/${encodeURIComponent(runId)}/messages${qs(workspaceId)}`, { content }),
  continueRun: (workspaceId: string, runId: string, content?: string) =>
    api.post<AgentRun>(`/dock/runs/${encodeURIComponent(runId)}/continue${qs(workspaceId)}`, content ? { content } : {}),
  cancelRun: (workspaceId: string, runId: string) =>
    api.post<AgentRun>(`/dock/runs/${encodeURIComponent(runId)}/cancel${qs(workspaceId)}`, {}),
};
