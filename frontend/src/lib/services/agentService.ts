import { api } from '../api';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  CreateAgentRequest,
  UpdateAgentRequest,
  OrchestrationProposal,
  ProposedStory,
  RuntimeProfile,
  ApproveAgentRunRequest,
  HandoffAgentRunRequest,
  PaginatedResponse,
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
  // Orchestration
  orchestrateEpic: (workspaceId: string, epicId: string, additionalContext?: string) =>
    api.post<OrchestrationProposal>(`/pm/epics/${epicId}/orchestrate${qs(workspaceId)}`, { additional_context: additionalContext ?? '' }),
  confirmOrchestration: (workspaceId: string, epicId: string, proposedStories: ProposedStory[]) =>
    api.post(`/pm/epics/${epicId}/orchestrate/confirm${qs(workspaceId)}`, { proposed_stories: proposedStories }),
  assignOrchestrator: (workspaceId: string, epicId: string, agentId: string) =>
    api.post(`/pm/epics/${epicId}/assign-orchestrator${qs(workspaceId)}`, { agent_id: agentId }),
};
