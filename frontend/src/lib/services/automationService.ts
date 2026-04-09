import { api } from '../api';
import type {
  AutomationInventoryResponse,
  AutomationTriggerCatalogEntry,
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListResponse,
} from '../types';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  AgentRunMessage,
  AgentTriggerUsageSummary,
  ApproveAgentRunRequest,
  AutomationRule,
  CreateAgentRequest,
  CreateAutomationRuleRequest,
  HandoffAgentRunRequest,
  PaginatedResponse,
  ResumeAgentRunRequest,
  SendAgentRunMessageRequest,
  SendAgentRunRequestChangesRequest,
  ToolCatalogResponse,
  UpdateAgentRequest,
  UpdateAutomationRuleRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const activityQS = (workspaceId: string, filters: AutomationTriggerExecutionFilters = {}) => {
  const params = new URLSearchParams({ workspace_id: workspaceId });
  if (filters.agent_id) params.set('agent_id', filters.agent_id);
  if (filters.binding_id) params.set('binding_id', filters.binding_id);
  if (filters.trigger_type) params.set('trigger_type', filters.trigger_type);
  if (filters.status) params.set('status', filters.status);
  if (filters.source) params.set('source', filters.source);
  if (filters.reference_id) params.set('reference_id', filters.reference_id);
  if (filters.fired_after) params.set('fired_after', filters.fired_after);
  if (filters.fired_before) params.set('fired_before', filters.fired_before);
  if (filters.page && filters.page > 0) params.set('page', String(filters.page));
  if (filters.per_page && filters.per_page > 0) params.set('per_page', String(filters.per_page));
  return `?${params.toString()}`;
};

export const automationService = {
  getOverview: (workspaceId: string) =>
    api.get<AutomationInventoryResponse>(`/automation/overview${qs(workspaceId)}`),

  listActivity: (workspaceId: string, filters: AutomationTriggerExecutionFilters = {}) =>
    api.get<AutomationTriggerExecutionListResponse>(`/automation/activity${activityQS(workspaceId, filters)}`),

  listFlows: (workspaceId: string) =>
    api.get<AutomationRule[]>(`/automation/flows${qs(workspaceId)}`),

  listFlowsByWorkflow: (workspaceId: string, workflowId: string) =>
    api.get<AutomationRule[]>(`/automation/flows${qs(workspaceId)}&workflow_id=${encodeURIComponent(workflowId)}`),

  createFlow: (workspaceId: string, data: CreateAutomationRuleRequest) =>
    api.post<AutomationRule>(`/automation/flows${qs(workspaceId)}`, data),

  getFlow: (workspaceId: string, flowId: string) =>
    api.get<AutomationRule>(`/automation/flows/${flowId}${qs(workspaceId)}`),

  updateFlow: (workspaceId: string, flowId: string, data: UpdateAutomationRuleRequest) =>
    api.put<AutomationRule>(`/automation/flows/${flowId}${qs(workspaceId)}`, data),

  deleteFlow: (workspaceId: string, flowId: string) =>
    api.del(`/automation/flows/${flowId}${qs(workspaceId)}`),

  listTriggerCatalog: (workspaceId: string) =>
    api.get<AutomationTriggerCatalogEntry[]>(`/automation/library/triggers${qs(workspaceId)}`),

  listToolCatalog: (workspaceId: string) =>
    api.get<ToolCatalogResponse>(`/automation/library/tools${qs(workspaceId)}`),

  listAgents: (workspaceId: string) =>
    api.get<Agent[]>(`/automation/agents${qs(workspaceId)}`),

  getAgent: (workspaceId: string, id: string) =>
    api.get<Agent>(`/automation/agents/${id}${qs(workspaceId)}`),

  getAgentUsage: (workspaceId: string, id: string) =>
    api.get<AgentTriggerUsageSummary>(`/automation/agents/${id}/usage${qs(workspaceId)}`),

  createAgent: (workspaceId: string, payload: CreateAgentRequest) =>
    api.post<Agent>(`/automation/agents${qs(workspaceId)}`, payload),

  updateAgent: (workspaceId: string, id: string, payload: UpdateAgentRequest) =>
    api.put<Agent>(`/automation/agents/${id}${qs(workspaceId)}`, payload),

  deleteAgent: (workspaceId: string, id: string) =>
    api.del(`/automation/agents/${id}${qs(workspaceId)}`),

  startRun: (workspaceId: string, payload: { target_type: string; target_id: string; agent_id: string; additional_context?: string; base_branch?: string; working_branch?: string }) =>
    api.post<AgentRun>(`/automation/runs${qs(workspaceId)}`, payload),

  listWorkspaceRuns: (workspaceId: string, page = 1, perPage = 100) =>
    api.get<PaginatedResponse<AgentRun[]>>(`/automation/runs${qs(workspaceId)}&page=${page}&per_page=${perPage}`),

  listTargetRuns: (workspaceId: string, targetType: string, targetId: string) =>
    api.get<AgentRun[]>(`/automation/runs${qs(workspaceId)}&target_type=${encodeURIComponent(targetType)}&target_id=${encodeURIComponent(targetId)}`),

  getRun: (workspaceId: string, runId: string) =>
    api.get<AgentRun>(`/automation/runs/${runId}${qs(workspaceId)}`),

  listRunMessages: (workspaceId: string, runId: string) =>
    api.get<AgentRunMessage[]>(`/automation/runs/${runId}/messages${qs(workspaceId)}`),

  sendRunMessage: (workspaceId: string, runId: string, payload: SendAgentRunMessageRequest) =>
    api.post<AgentRunMessage>(`/automation/runs/${runId}/messages${qs(workspaceId)}`, payload),

  listRunArtifacts: (workspaceId: string, runId: string) =>
    api.get<AgentRunArtifact[]>(`/automation/runs/${runId}/artifacts${qs(workspaceId)}`),

  resumeRun: (workspaceId: string, runId: string, payload: ResumeAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/resume${qs(workspaceId)}`, payload),

  cancelRun: (workspaceId: string, runId: string) =>
    api.post<AgentRun>(`/automation/runs/${runId}/cancel${qs(workspaceId)}`, {}),

  approveRun: (workspaceId: string, runId: string, payload?: ApproveAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/approve${qs(workspaceId)}`, payload ?? {}),

  requestRunChanges: (workspaceId: string, runId: string, payload: SendAgentRunRequestChangesRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/request-changes${qs(workspaceId)}`, payload),

  handoffRun: (workspaceId: string, runId: string, payload: HandoffAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/handoff${qs(workspaceId)}`, payload),
};
