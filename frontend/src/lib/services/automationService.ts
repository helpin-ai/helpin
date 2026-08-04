import { api, API_BASE } from '../api';
import type {
  AutomationInventoryResponse,
  AutomationTriggerCatalogEntry,
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListResponse,
} from '../types';
import type {
  Agent,
  AgentAnalyticsResponse,
  AgentTemplate,
  AgentVersion,
  AgentRun,
  AgentRunAttentionCountResponse,
  AgentRunArtifact,
  AgentRunMessage,
  AgentTriggerUsageSummary,
  ApproveAgentRunRequest,
  ContinueAgentRunRequest,
  AutomationRule,
  CreateAgentRequest,
  CreateAgentVersionRequest,
  CustomAgentDraftRequest,
  CustomAgentDraftResponse,
  CreateAgentFromTemplateRequest,
  CreateAgentFromTemplateResponse,
  CreateAutomationRuleRequest,
  FlowTemplateManifest,
  CreateWorkspaceSkillRequest,
  HandoffAgentRunRequest,
  InstallFlowTemplateRequest,
  InstallFlowTemplateResponse,
  UninstallFlowTemplateRequest,
  UninstallFlowTemplateResponse,
  PaginatedResponse,
  ResumeAgentRunRequest,
  SendAgentRunMessageRequest,
  SendAgentRunRequestChangesRequest,
  SkillCatalogResponse,
  ToolCatalogResponse,
  UpdateAgentRequest,
  UpdateAgentVersionRequest,
  UpdateAutomationRuleRequest,
  UpdateWorkspaceSkillRequest,
  WorkspaceSkillResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const activityQS = (workspaceId: string, filters: AutomationTriggerExecutionFilters = {}) => {
  const params = new URLSearchParams({ workspace_id: workspaceId });
  if (filters.execution_id) params.set('execution_id', filters.execution_id);
  if (filters.agent_id) params.set('agent_id', filters.agent_id);
  if (filters.binding_id) params.set('binding_id', filters.binding_id);
  if (filters.trigger_type) params.set('trigger_type', filters.trigger_type);
  if (filters.status) params.set('status', filters.status);
  if (filters.source) params.set('source', filters.source);
  if (filters.reference_id) params.set('reference_id', filters.reference_id);
  if (filters.run_id) params.set('run_id', filters.run_id);
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

  listFlowTemplates: (workspaceId: string) =>
    api.get<FlowTemplateManifest[]>(`/automation/templates${qs(workspaceId)}`),

  getFlowTemplate: (workspaceId: string, key: string) =>
    api.get<FlowTemplateManifest>(`/automation/templates/${key}${qs(workspaceId)}`),

  installFlowTemplate: (workspaceId: string, key: string, payload: InstallFlowTemplateRequest) =>
    api.post<InstallFlowTemplateResponse>(`/automation/templates/${key}/install${qs(workspaceId)}`, payload),

  uninstallFlowTemplate: (workspaceId: string, instanceId: string, payload: UninstallFlowTemplateRequest) =>
    api.post<UninstallFlowTemplateResponse>(`/automation/template-instances/${instanceId}/uninstall${qs(workspaceId)}`, payload),

  createFlow: (workspaceId: string, data: CreateAutomationRuleRequest) =>
    api.post<AutomationRule>(`/automation/flows${qs(workspaceId)}`, data),

  getFlow: (workspaceId: string, flowId: string) =>
    api.get<AutomationRule>(`/automation/flows/${flowId}${qs(workspaceId)}`),

  updateFlow: (workspaceId: string, flowId: string, data: UpdateAutomationRuleRequest) =>
    api.put<AutomationRule>(`/automation/flows/${flowId}${qs(workspaceId)}`, data),

  runFlowNow: (workspaceId: string, flowId: string) =>
    api.post<AgentRun>(`/automation/flows/${flowId}/run${qs(workspaceId)}`, {}),

  deleteFlow: (workspaceId: string, flowId: string) =>
    api.del(`/automation/flows/${flowId}${qs(workspaceId)}`),

  listTriggerCatalog: (workspaceId: string) =>
    api.get<AutomationTriggerCatalogEntry[]>(`/automation/library/triggers${qs(workspaceId)}`),

  listToolCatalog: (workspaceId: string) =>
    api.get<ToolCatalogResponse>(`/automation/library/tools${qs(workspaceId)}`),

  listSkillCatalog: (workspaceId: string) =>
    api.get<SkillCatalogResponse>(`/automation/library/skills${qs(workspaceId)}`),

  createSkill: (workspaceId: string, data: CreateWorkspaceSkillRequest) =>
    api.post<WorkspaceSkillResponse>(`/automation/library/skills${qs(workspaceId)}`, data),

  importSkill: async (workspaceId: string, file: File, sourceRuntime?: string): Promise<{ data: WorkspaceSkillResponse | null; error: string | null }> => {
    const formData = new FormData();
    formData.append('archive', file);
    if (sourceRuntime) formData.append('source_runtime', sourceRuntime);
    try {
      const res = await fetch(`${API_BASE}/automation/library/skills/import${qs(workspaceId)}`, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data = await res.json();
      return { data, error: null };
    } catch {
      return { data: null, error: 'Network error' };
    }
  },

  updateSkill: (workspaceId: string, skillId: string, data: UpdateWorkspaceSkillRequest) =>
    api.put<WorkspaceSkillResponse>(`/automation/library/skills/${skillId}${qs(workspaceId)}`, data),

  deleteSkill: (workspaceId: string, skillId: string) =>
    api.del(`/automation/library/skills/${skillId}${qs(workspaceId)}`),

  listAgents: (workspaceId: string) =>
    api.get<Agent[]>(`/automation/agents${qs(workspaceId)}`),

  listAgentTemplates: (workspaceId: string) =>
    api.get<AgentTemplate[]>(`/automation/agent-templates${qs(workspaceId)}`),

  getAgentTemplate: (workspaceId: string, id: string) =>
    api.get<AgentTemplate>(`/automation/agent-templates/${id}${qs(workspaceId)}`),

  getAgent: (workspaceId: string, id: string) =>
    api.get<Agent>(`/automation/agents/${id}${qs(workspaceId)}`),

  getAgentUsage: (workspaceId: string, id: string) =>
    api.get<AgentTriggerUsageSummary>(`/automation/agents/${id}/usage${qs(workspaceId)}`),

  getAgentAnalytics: (workspaceId: string, id: string, range: '7d' | '30d' | '90d' | '12m' = '30d') =>
    api.get<AgentAnalyticsResponse>(`/automation/agents/${id}/analytics${qs(workspaceId)}&range=${encodeURIComponent(range)}`),

  createAgent: (workspaceId: string, payload: CreateAgentRequest) =>
    api.post<Agent>(`/automation/agents${qs(workspaceId)}`, payload),

  draftCustomAgent: (workspaceId: string, payload: CustomAgentDraftRequest) =>
    api.post<CustomAgentDraftResponse>(`/automation/agents/draft${qs(workspaceId)}`, payload),

  createAgentFromTemplate: (workspaceId: string, templateId: string, payload: CreateAgentFromTemplateRequest) =>
    api.post<CreateAgentFromTemplateResponse>(`/automation/agent-templates/${templateId}/create-agent${qs(workspaceId)}`, payload),

  updateAgent: (workspaceId: string, id: string, payload: UpdateAgentRequest) =>
    api.put<Agent>(`/automation/agents/${id}${qs(workspaceId)}`, payload),

  listAgentVersions: (workspaceId: string, id: string) =>
    api.get<AgentVersion[]>(`/automation/agents/${id}/versions${qs(workspaceId)}`),

  createAgentVersion: (workspaceId: string, id: string, payload: CreateAgentVersionRequest) =>
    api.post<AgentVersion>(`/automation/agents/${id}/versions${qs(workspaceId)}`, payload),

  updateAgentVersion: (workspaceId: string, id: string, versionId: string, payload: UpdateAgentVersionRequest) =>
    api.put<AgentVersion>(`/automation/agents/${id}/versions/${versionId}${qs(workspaceId)}`, payload),

  activateAgentVersion: (workspaceId: string, id: string, versionId: string) =>
    api.post<Agent>(`/automation/agents/${id}/versions/${versionId}/activate${qs(workspaceId)}`, {}),

  deleteAgentVersion: (workspaceId: string, id: string, versionId: string) =>
    api.del<void>(`/automation/agents/${id}/versions/${versionId}${qs(workspaceId)}`),

  deleteAgent: (workspaceId: string, id: string) =>
    api.del(`/automation/agents/${id}${qs(workspaceId)}`),

  startRun: (workspaceId: string, payload: { target_type: string; target_id: string; agent_id: string; additional_context?: string; base_branch?: string; working_branch?: string }) =>
    api.post<AgentRun>(`/automation/runs${qs(workspaceId)}`, payload),

  listWorkspaceRuns: (workspaceId: string, page = 1, perPage = 100) =>
    api.get<PaginatedResponse<AgentRun[]>>(`/automation/runs${qs(workspaceId)}&page=${page}&per_page=${perPage}`),

  getRunAttentionCount: (workspaceId: string) =>
    api.get<AgentRunAttentionCountResponse>(`/automation/runs/attention-count${qs(workspaceId)}`),

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

  continueRun: (workspaceId: string, runId: string, payload?: ContinueAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/continue${qs(workspaceId)}`, payload ?? {}),

  cancelRun: (workspaceId: string, runId: string) =>
    api.post<AgentRun>(`/automation/runs/${runId}/cancel${qs(workspaceId)}`, {}),

  approveRun: (workspaceId: string, runId: string, payload?: ApproveAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/approve${qs(workspaceId)}`, payload ?? {}),

  requestRunChanges: (workspaceId: string, runId: string, payload: SendAgentRunRequestChangesRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/request-changes${qs(workspaceId)}`, payload),

  handoffRun: (workspaceId: string, runId: string, payload: HandoffAgentRunRequest) =>
    api.post<AgentRun>(`/automation/runs/${runId}/handoff${qs(workspaceId)}`, payload),
};
