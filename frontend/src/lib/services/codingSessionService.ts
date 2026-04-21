import { api } from '../api';
import type {
  ApproveAgentRunRequest,
  AgentRun,
  AgentRunArtifact,
  CodingSession,
  CodingSessionDiff,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  CodingSessionRepoState,
  ContinueAgentRunRequest,
  CodexAuthState,
  ResolveCodingSessionInteractionRequest,
  ResumeAgentRunRequest,
  SendAgentRunMessageRequest,
  SendAgentRunRequestChangesRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const codingSessionService = {
  get: (workspaceId: string, sessionId: string) =>
    api.get<CodingSession>(`/pm/coding-sessions/${sessionId}${qs(workspaceId)}`),
  listEvents: (workspaceId: string, sessionId: string, after = 0) =>
    api.get<CodingSessionEventListResponse>(`/pm/coding-sessions/${sessionId}/events${qs(workspaceId)}&after=${after}`),
  getRepo: (workspaceId: string, sessionId: string) =>
    api.get<CodingSessionRepoState>(`/pm/coding-sessions/${sessionId}/repo${qs(workspaceId)}`),
  getDiff: (workspaceId: string, sessionId: string, path?: string) =>
    api.get<CodingSessionDiff>(`/pm/coding-sessions/${sessionId}/diff${qs(workspaceId)}${path ? `&path=${encodeURIComponent(path)}` : ''}`),
  listArtifacts: (workspaceId: string, sessionId: string) =>
    api.get<AgentRunArtifact[]>(`/pm/agent-runs/${sessionId}/artifacts${qs(workspaceId)}`),
  resolveInteraction: (workspaceId: string, sessionId: string, interactionId: string, payload: ResolveCodingSessionInteractionRequest) =>
    api.post<CodingSessionInteraction>(`/pm/coding-sessions/${sessionId}/interactions/${interactionId}/resolve${qs(workspaceId)}`, payload),
  sendMessage: (workspaceId: string, sessionId: string, payload: SendAgentRunMessageRequest) =>
    api.post(`/pm/coding-sessions/${sessionId}/message${qs(workspaceId)}`, payload),
  continue: (workspaceId: string, sessionId: string, payload?: ContinueAgentRunRequest) =>
    api.post<AgentRun>(`/pm/coding-sessions/${sessionId}/continue${qs(workspaceId)}`, payload ?? {}),
  resume: (workspaceId: string, sessionId: string, payload: ResumeAgentRunRequest) =>
    api.post(`/pm/coding-sessions/${sessionId}/resume${qs(workspaceId)}`, payload),
  approve: (workspaceId: string, sessionId: string, payload?: ApproveAgentRunRequest) =>
    api.post(`/pm/coding-sessions/${sessionId}/approve${qs(workspaceId)}`, payload ?? {}),
  requestChanges: (workspaceId: string, sessionId: string, payload: SendAgentRunRequestChangesRequest) =>
    api.post(`/pm/coding-sessions/${sessionId}/request-changes${qs(workspaceId)}`, payload),
  cancel: (workspaceId: string, sessionId: string) =>
    api.post(`/pm/coding-sessions/${sessionId}/cancel${qs(workspaceId)}`, {}),
  startDeviceCodeAuth: (workspaceId: string, sessionId: string) =>
    api.post<CodexAuthState>(`/pm/coding-sessions/${sessionId}/auth/device-code/start${qs(workspaceId)}`, {}),
  cancelDeviceCodeAuth: (workspaceId: string, sessionId: string) =>
    api.post<CodexAuthState>(`/pm/coding-sessions/${sessionId}/auth/device-code/cancel${qs(workspaceId)}`, {}),
};
