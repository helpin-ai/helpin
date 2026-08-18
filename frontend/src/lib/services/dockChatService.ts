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
} from '../dockTypes';
import type {
  AgentRun,
  CodingSession,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  ResolveCodingSessionInteractionRequest,
  AgentRunMessage,
  CodexAuthState,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const dockChatService = {
  listChats: (workspaceId: string, cursor?: string | null, limit = 30) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: String(limit) });
    if (cursor) query.set('cursor', cursor);
    return api.get<DockChatListResponse>(`/dock/chats?${query.toString()}`);
  },
  createChat: (workspaceId: string, title = '', supportConversationId?: string, moduleId?: DockChatModule | null) =>
    api.post<DockChat>(`/dock/chats${qs(workspaceId)}`, {
      title,
      ...(supportConversationId ? { support_conversation_id: supportConversationId } : {}),
      ...(moduleId ? { module_id: moduleId, visibility: 'module' } : {}),
    }),
  getChat: (workspaceId: string, chatId: string) =>
    api.get<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`),
  updateChat: (workspaceId: string, chatId: string, payload: UpdateDockChatRequest) =>
    api.patch<DockChat>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`, payload),
  sendMessage: (workspaceId: string, chatId: string, payload: SendDockChatMessageRequest) =>
    api.post<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}/messages${qs(workspaceId)}`, payload),
  listMessages: (workspaceId: string, chatId: string, before?: number | null, limit = 50) => {
    const query = new URLSearchParams({ workspace_id: workspaceId, limit: String(limit) });
    if (before) query.set('before', String(before));
    return api.get<DockChatMessageListResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/messages?${query.toString()}`,
    );
  },
  getMessageWorkDetail: (workspaceId: string, chatId: string, messageId: string) =>
    api.get<DockChatWorkDetailResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/messages/${encodeURIComponent(messageId)}/work${qs(workspaceId)}`,
    ),
  generateTitle: (workspaceId: string, chatId: string, payload: GenerateDockChatTitleRequest) =>
    api.post<DockChat>(`/dock/chats/${encodeURIComponent(chatId)}/title${qs(workspaceId)}`, payload),
  getChatRun: (workspaceId: string, chatId: string) =>
    api.get<CodingSession>(`/dock/chats/${encodeURIComponent(chatId)}/run${qs(workspaceId)}`),
  listChatRunInteractions: (workspaceId: string, chatId: string) =>
    api.get<{ interactions: CodingSessionInteraction[] }>(
      `/dock/chats/${encodeURIComponent(chatId)}/run/interactions${qs(workspaceId)}`,
    ),
  listChatRunEvents: (workspaceId: string, chatId: string, after = 0) =>
    api.get<CodingSessionEventListResponse>(
      `/dock/chats/${encodeURIComponent(chatId)}/run/events${qs(workspaceId)}&after=${encodeURIComponent(after)}`,
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
  listRuns: (workspaceId: string) =>
    api.get<DockRunListResponse>(`/dock/runs${qs(workspaceId)}`),
  getRunSnapshot: (workspaceId: string, runId: string) =>
    api.get<CodingSession>(`/dock/runs/${encodeURIComponent(runId)}/snapshot${qs(workspaceId)}`),
  listRunEvents: (workspaceId: string, runId: string, after = 0) =>
    api.get<CodingSessionEventListResponse>(
      `/dock/runs/${encodeURIComponent(runId)}/events${qs(workspaceId)}&after=${encodeURIComponent(after)}`,
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
  startRunAuth: (workspaceId: string, runId: string) =>
    api.post<CodexAuthState>(`/dock/runs/${encodeURIComponent(runId)}/auth/device-code/start${qs(workspaceId)}`, {}),
  cancelRunAuth: (workspaceId: string, runId: string) =>
    api.post<CodexAuthState>(`/dock/runs/${encodeURIComponent(runId)}/auth/device-code/cancel${qs(workspaceId)}`, {}),
};
