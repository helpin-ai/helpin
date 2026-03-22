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
  runAgent: (workspaceId: string, storyId: string) =>
    api.post<AgentRun>(`/pm/stories/${storyId}/run-agent${qs(workspaceId)}`, {}),
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
  // Knowledge Sources
  listKnowledgeSources: (workspaceId: string, agentId: string) =>
    api.get<import('../pmTypes').AgentKnowledgeSource[]>(`/pm/agents/${agentId}/knowledge-sources${qs(workspaceId)}`),
  updateKnowledgeSources: (workspaceId: string, agentId: string, spaceIds: string[]) =>
    api.put<import('../pmTypes').AgentKnowledgeSource[]>(`/pm/agents/${agentId}/knowledge-sources${qs(workspaceId)}`, { space_ids: spaceIds }),
  listContentSources: (workspaceId: string) =>
    api.get<import('../pmTypes').SupportContentSource[]>(`/pm/content-sources${qs(workspaceId)}`),
  createContentSource: (workspaceId: string, payload: import('../pmTypes').CreateSupportContentSourceRequest) =>
    api.post<import('../pmTypes').SupportContentSource>(`/pm/content-sources${qs(workspaceId)}`, payload),
  updateContentSource: (workspaceId: string, contentSourceId: string, payload: import('../pmTypes').UpdateSupportContentSourceRequest) =>
    api.put<import('../pmTypes').SupportContentSource>(`/pm/content-sources/${contentSourceId}${qs(workspaceId)}`, payload),
  deleteContentSource: (workspaceId: string, contentSourceId: string) =>
    api.del(`/pm/content-sources/${contentSourceId}${qs(workspaceId)}`),
  listContentSourcePages: (workspaceId: string, contentSourceId: string) =>
    api.get<import('../pmTypes').SupportContentPage[]>(`/pm/content-sources/${contentSourceId}/pages${qs(workspaceId)}`),
  reindexContentSource: (workspaceId: string, contentSourceId: string) =>
    api.post<{ status: string }>(`/pm/content-sources/${contentSourceId}/reindex${qs(workspaceId)}`, {}),
  listAgentContentSources: (workspaceId: string, agentId: string) =>
    api.get<string[]>(`/pm/agents/${agentId}/content-sources${qs(workspaceId)}`),
  updateAgentContentSources: (workspaceId: string, agentId: string, contentSourceIds: string[]) =>
    api.put<string[]>(`/pm/agents/${agentId}/content-sources${qs(workspaceId)}`, { content_source_ids: contentSourceIds }),
};
