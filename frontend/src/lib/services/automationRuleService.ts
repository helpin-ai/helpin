import { api } from '../api';
import type { AutomationRule, CreateAutomationRuleRequest, UpdateAutomationRuleRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const automationRuleService = {
  list: (workspaceId: string) =>
    api.get<AutomationRule[]>(`/pm/automation-rules${qs(workspaceId)}`),

  listByWorkflow: (workspaceId: string, workflowId: string) =>
    api.get<AutomationRule[]>(`/pm/automation-rules${qs(workspaceId)}&workflow_id=${encodeURIComponent(workflowId)}`),

  create: (workspaceId: string, data: CreateAutomationRuleRequest) =>
    api.post<AutomationRule>(`/pm/automation-rules${qs(workspaceId)}`, data),

  get: (workspaceId: string, ruleId: string) =>
    api.get<AutomationRule>(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`),

  update: (workspaceId: string, ruleId: string, data: UpdateAutomationRuleRequest) =>
    api.put<AutomationRule>(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`, data),

  remove: (workspaceId: string, ruleId: string) =>
    api.del(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`),
};
