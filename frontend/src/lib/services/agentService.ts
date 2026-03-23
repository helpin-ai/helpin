import { api } from '../api';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  AgentRunMessage,
  CreateAgentRequest,
  AgentPresetDefinition,
  UpdateAgentRequest,
  AgentModelProviderOption,
  ApproveAgentRunRequest,
  HandoffAgentRunRequest,
  StartAgentRunRequest,
  SendAgentRunMessageRequest,
  SendAgentRunRequestChangesRequest,
  PaginatedResponse,
  RunnerHealth,
  ToolCatalogResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const agentService = {
  list: (workspaceId: string) =>
    api.get<Agent[]>(`/pm/agents${qs(workspaceId)}`),
  get: (workspaceId: string, id: string) =>
    api.get<Agent>(`/pm/agents/${id}${qs(workspaceId)}`),
  create: (workspaceId: string, payload: CreateAgentRequest) =>
    api.post<Agent>(`/pm/agents${qs(workspaceId)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateAgentRequest) =>
    api.put<Agent>(`/pm/agents/${id}${qs(workspaceId)}`, payload),
  delete: (workspaceId: string, id: string) =>
    api.del(`/pm/agents/${id}${qs(workspaceId)}`),
  listPresets: (workspaceId: string) =>
    api.get<AgentPresetDefinition[]>(`/pm/agent-presets${qs(workspaceId)}`),
  listModelProviders: (workspaceId: string) =>
    api.get<AgentModelProviderOption[]>(`/pm/agent-model-providers${qs(workspaceId)}`),
  getRunnerHealth: (workspaceId: string) =>
    api.get<RunnerHealth>(`/pm/runner-health${qs(workspaceId)}`),
  assignToStory: (workspaceId: string, storyId: string, agentId: string) =>
    api.post(`/pm/stories/${storyId}/assign-agent${qs(workspaceId)}`, { agent_id: agentId }),
  runStory: (workspaceId: string, storyId: string, payload?: StartAgentRunRequest) =>
    api.post<AgentRun>(`/pm/stories/${storyId}/run-agent${qs(workspaceId)}`, payload ?? {}),
  runEpic: (workspaceId: string, epicId: string, payload: StartAgentRunRequest) =>
    api.post<AgentRun>(`/pm/epics/${epicId}/run-agent${qs(workspaceId)}`, payload),
  listWorkspaceRuns: (workspaceId: string, page = 1, perPage = 100) =>
    api.get<PaginatedResponse<AgentRun[]>>(
      `/pm/agent-runs/workspace${qs(workspaceId)}&page=${page}&per_page=${perPage}`,
    ),
  listTargetRuns: (workspaceId: string, targetType: string, targetId: string) =>
    api.get<AgentRun[]>(
      `/pm/agent-runs${qs(workspaceId)}&target_type=${encodeURIComponent(targetType)}&target_id=${encodeURIComponent(targetId)}`,
    ),
  listRuns: (workspaceId: string, agentId: string) =>
    api.get<PaginatedResponse<AgentRun[]>>(`/pm/agents/${agentId}/runs${qs(workspaceId)}`),
  getRun: (workspaceId: string, runId: string) =>
    api.get<AgentRun>(`/pm/agent-runs/${runId}${qs(workspaceId)}`),
  listRunMessages: (workspaceId: string, runId: string) =>
    api.get<AgentRunMessage[]>(`/pm/agent-runs/${runId}/messages${qs(workspaceId)}`),
  sendRunMessage: (workspaceId: string, runId: string, payload: SendAgentRunMessageRequest) =>
    api.post<AgentRunMessage>(`/pm/agent-runs/${runId}/messages${qs(workspaceId)}`, payload),
  listRunArtifacts: (workspaceId: string, runId: string) =>
    api.get<AgentRunArtifact[]>(`/pm/agent-runs/${runId}/artifacts${qs(workspaceId)}`),
  cancelRun: (workspaceId: string, runId: string) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/cancel${qs(workspaceId)}`, {}),
  approveRun: (workspaceId: string, runId: string, payload?: ApproveAgentRunRequest) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/approve${qs(workspaceId)}`, payload ?? {}),
  requestRunChanges: (workspaceId: string, runId: string, payload: SendAgentRunRequestChangesRequest) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/request-changes${qs(workspaceId)}`, payload),
  handoffRun: (workspaceId: string, runId: string, payload: HandoffAgentRunRequest) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/handoff${qs(workspaceId)}`, payload),
  listToolCatalog: (workspaceId: string) =>
    api.get<ToolCatalogResponse>(`/pm/tool-catalog${qs(workspaceId)}`),
};
