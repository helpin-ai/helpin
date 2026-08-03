import { api } from '../api';
import type {
  DockChat,
  DockChatDetail,
  SendDockChatMessageRequest,
  UpdateDockChatRequest,
} from '../dockTypes';
import type {
  AgentRun,
  CodingSession,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  ResolveCodingSessionInteractionRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const dockChatService = {
  listChats: (workspaceId: string) =>
    api.get<{ chats: DockChat[] }>(`/dock/chats${qs(workspaceId)}`),
  createChat: (workspaceId: string, title = '') =>
    api.post<DockChat>(`/dock/chats${qs(workspaceId)}`, { title }),
  getChat: (workspaceId: string, chatId: string) =>
    api.get<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`),
  updateChat: (workspaceId: string, chatId: string, payload: UpdateDockChatRequest) =>
    api.patch<DockChat>(`/dock/chats/${encodeURIComponent(chatId)}${qs(workspaceId)}`, payload),
  sendMessage: (workspaceId: string, chatId: string, payload: SendDockChatMessageRequest) =>
    api.post<DockChatDetail>(`/dock/chats/${encodeURIComponent(chatId)}/messages${qs(workspaceId)}`, payload),
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
};
