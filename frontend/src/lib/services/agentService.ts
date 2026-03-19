import { api } from '../api';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  CreateAgentRequest,
  UpdateAgentRequest,
  RuntimeProfile,
  AgentModelProviderOption,
  ApproveAgentRunRequest,
  HandoffAgentRunRequest,
  PaginatedResponse,
  RunnerHealth,
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
  listRuntimeProfiles: (workspaceId: string) =>
    api.get<RuntimeProfile[]>(`/pm/runtime-profiles${qs(workspaceId)}`),
  listModelProviders: (workspaceId: string) =>
    api.get<AgentModelProviderOption[]>(`/pm/agent-model-providers${qs(workspaceId)}`),
  getRunnerHealth: (workspaceId: string) =>
    api.get<RunnerHealth>(`/pm/runner-health${qs(workspaceId)}`),
  assignToStory: (workspaceId: string, storyId: string, agentId: string) =>
    api.post(`/pm/stories/${storyId}/assign-agent${qs(workspaceId)}`, { agent_id: agentId }),
  listRuns: (workspaceId: string, agentId: string) =>
    api.get<PaginatedResponse<AgentRun[]>>(`/pm/agents/${agentId}/runs${qs(workspaceId)}`),
  getRun: (workspaceId: string, runId: string) =>
    api.get<AgentRun>(`/pm/agent-runs/${runId}${qs(workspaceId)}`),
  listRunArtifacts: (workspaceId: string, runId: string) =>
    api.get<AgentRunArtifact[]>(`/pm/agent-runs/${runId}/artifacts${qs(workspaceId)}`),
  cancelRun: (workspaceId: string, runId: string) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/cancel${qs(workspaceId)}`, {}),
  approveRun: (workspaceId: string, runId: string, payload?: ApproveAgentRunRequest) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/approve${qs(workspaceId)}`, payload ?? {}),
  handoffRun: (workspaceId: string, runId: string, payload: HandoffAgentRunRequest) =>
    api.post<AgentRun>(`/pm/agent-runs/${runId}/handoff${qs(workspaceId)}`, payload),
};
