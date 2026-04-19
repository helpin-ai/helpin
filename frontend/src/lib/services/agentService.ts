import { api } from '../api';
import { automationService } from './automationService';
import type {
  AgentRun,
  CreateAgentRequest,
  AgentPresetDefinition,
  UpdateAgentRequest,
  AgentModelProviderOption,
  CodexAuthState,
  ContinueAgentRunRequest,
  CreateWorkspaceAgentPresetVersionRequest,
  ApproveAgentRunRequest,
  HandoffAgentRunRequest,
  ResumeAgentRunRequest,
  StartAgentRunRequest,
  SendAgentRunMessageRequest,
  SendAgentRunRequestChangesRequest,
  PaginatedResponse,
  RunnerHealth,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const agentService = {
  list: (workspaceId: string) =>
    automationService.listAgents(workspaceId) as ReturnType<typeof automationService.listAgents>,
  get: (workspaceId: string, id: string) =>
    automationService.getAgent(workspaceId, id) as ReturnType<typeof automationService.getAgent>,
  getUsage: (workspaceId: string, id: string) =>
    automationService.getAgentUsage(workspaceId, id) as ReturnType<typeof automationService.getAgentUsage>,
  create: (workspaceId: string, payload: CreateAgentRequest) =>
    automationService.createAgent(workspaceId, payload) as ReturnType<typeof automationService.createAgent>,
  update: (workspaceId: string, id: string, payload: UpdateAgentRequest) =>
    automationService.updateAgent(workspaceId, id, payload) as ReturnType<typeof automationService.updateAgent>,
  delete: (workspaceId: string, id: string) =>
    automationService.deleteAgent(workspaceId, id) as ReturnType<typeof automationService.deleteAgent>,
  listPresets: (workspaceId: string) =>
    api.get<AgentPresetDefinition[]>(`/pm/agent-presets${qs(workspaceId)}`),
  createPresetVersion: (workspaceId: string, payload: CreateWorkspaceAgentPresetVersionRequest) =>
    api.post<AgentPresetDefinition>(`/pm/agent-preset-versions${qs(workspaceId)}`, payload),
  listModelProviders: (workspaceId: string) =>
    api.get<AgentModelProviderOption[]>(`/pm/agent-model-providers${qs(workspaceId)}`),
  getRunnerHealth: (workspaceId: string) =>
    api.get<RunnerHealth>(`/pm/runner-health${qs(workspaceId)}`),
  assignToTask: (workspaceId: string, taskId: string, agentId: string) =>
    api.post(`/pm/tasks/${taskId}/assign-agent${qs(workspaceId)}`, { agent_id: agentId }),
  runTask: (workspaceId: string, taskId: string, payload?: StartAgentRunRequest) =>
    api.post<AgentRun>(`/pm/tasks/${taskId}/run-agent${qs(workspaceId)}`, payload ?? {}),
  runEpic: (workspaceId: string, epicId: string, payload: StartAgentRunRequest) =>
    api.post<AgentRun>(`/pm/epics/${epicId}/run-agent${qs(workspaceId)}`, payload),
  listWorkspaceRuns: (workspaceId: string, page = 1, perPage = 100) =>
    automationService.listWorkspaceRuns(workspaceId, page, perPage) as ReturnType<typeof automationService.listWorkspaceRuns>,
  listTargetRuns: (workspaceId: string, targetType: string, targetId: string) =>
    automationService.listTargetRuns(workspaceId, targetType, targetId) as ReturnType<typeof automationService.listTargetRuns>,
  listRuns: (workspaceId: string, agentId: string) =>
    api.get<PaginatedResponse<AgentRun[]>>(`/pm/agents/${agentId}/runs${qs(workspaceId)}`),
  getRun: (workspaceId: string, runId: string) =>
    automationService.getRun(workspaceId, runId) as ReturnType<typeof automationService.getRun>,
  listRunMessages: (workspaceId: string, runId: string) =>
    automationService.listRunMessages(workspaceId, runId) as ReturnType<typeof automationService.listRunMessages>,
  resumeRun: (workspaceId: string, runId: string, payload: ResumeAgentRunRequest) =>
    automationService.resumeRun(workspaceId, runId, payload) as ReturnType<typeof automationService.resumeRun>,
  continueRun: (workspaceId: string, runId: string, payload?: ContinueAgentRunRequest) =>
    automationService.continueRun(workspaceId, runId, payload) as ReturnType<typeof automationService.continueRun>,
  sendRunMessage: (workspaceId: string, runId: string, payload: SendAgentRunMessageRequest) =>
    automationService.sendRunMessage(workspaceId, runId, payload) as ReturnType<typeof automationService.sendRunMessage>,
  listRunArtifacts: (workspaceId: string, runId: string) =>
    automationService.listRunArtifacts(workspaceId, runId) as ReturnType<typeof automationService.listRunArtifacts>,
  cancelRun: (workspaceId: string, runId: string) =>
    automationService.cancelRun(workspaceId, runId) as ReturnType<typeof automationService.cancelRun>,
  startCodexDeviceCodeAuth: (workspaceId: string, runId: string) =>
    api.post<CodexAuthState>(`/pm/agent-runs/${runId}/codex-auth/device-code/start${qs(workspaceId)}`, {}),
  cancelCodexDeviceCodeAuth: (workspaceId: string, runId: string) =>
    api.post<CodexAuthState>(`/pm/agent-runs/${runId}/codex-auth/device-code/cancel${qs(workspaceId)}`, {}),
  approveRun: (workspaceId: string, runId: string, payload?: ApproveAgentRunRequest) =>
    automationService.approveRun(workspaceId, runId, payload) as ReturnType<typeof automationService.approveRun>,
  requestRunChanges: (workspaceId: string, runId: string, payload: SendAgentRunRequestChangesRequest) =>
    automationService.requestRunChanges(workspaceId, runId, payload) as ReturnType<typeof automationService.requestRunChanges>,
  handoffRun: (workspaceId: string, runId: string, payload: HandoffAgentRunRequest) =>
    automationService.handoffRun(workspaceId, runId, payload) as ReturnType<typeof automationService.handoffRun>,
  listToolCatalog: (workspaceId: string) =>
    automationService.listToolCatalog(workspaceId) as ReturnType<typeof automationService.listToolCatalog>,
  // Knowledge Sources
  listKnowledgeSources: (workspaceId: string, agentId: string) =>
    api.get<import('../pmTypes').AgentKnowledgeSource[]>(`/pm/agents/${agentId}/knowledge-sources${qs(workspaceId)}`),
  updateKnowledgeSources: (workspaceId: string, agentId: string, spaceIds: string[]) =>
    api.put<import('../pmTypes').AgentKnowledgeSource[]>(`/pm/agents/${agentId}/knowledge-sources${qs(workspaceId)}`, { space_ids: spaceIds }),
  reindexKnowledgeSource: (workspaceId: string, agentId: string, spaceId: string) =>
    api.post<{ status: string }>(`/pm/agents/${agentId}/knowledge-sources/${spaceId}/reindex${qs(workspaceId)}`, {}),
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
  getContentSourcePage: (workspaceId: string, contentSourceId: string, pageId: string) =>
    api.get<import('../pmTypes').SupportContentPage>(`/pm/content-sources/${contentSourceId}/pages/${pageId}${qs(workspaceId)}`),
  reindexContentSource: (workspaceId: string, contentSourceId: string) =>
    api.post<{ status: string }>(`/pm/content-sources/${contentSourceId}/reindex${qs(workspaceId)}`, {}),
  listAgentContentSources: (workspaceId: string, agentId: string) =>
    api.get<string[]>(`/pm/agents/${agentId}/content-sources${qs(workspaceId)}`),
  updateAgentContentSources: (workspaceId: string, agentId: string, contentSourceIds: string[]) =>
    api.put<string[]>(`/pm/agents/${agentId}/content-sources${qs(workspaceId)}`, { content_source_ids: contentSourceIds }),
};
