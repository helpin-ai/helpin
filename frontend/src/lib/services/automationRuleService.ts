import { automationService } from './automationService';
import type { CreateAutomationRuleRequest, UpdateAutomationRuleRequest } from '../pmTypes';

export const automationRuleService = {
  list: (workspaceId: string) =>
    automationService.listFlows(workspaceId) as ReturnType<typeof automationService.listFlows>,

  listByWorkflow: (workspaceId: string, workflowId: string) =>
    automationService.listFlowsByWorkflow(workspaceId, workflowId) as ReturnType<typeof automationService.listFlowsByWorkflow>,

  create: (workspaceId: string, data: CreateAutomationRuleRequest) =>
    automationService.createFlow(workspaceId, data) as ReturnType<typeof automationService.createFlow>,

  get: (workspaceId: string, ruleId: string) =>
    automationService.getFlow(workspaceId, ruleId) as ReturnType<typeof automationService.getFlow>,

  update: (workspaceId: string, ruleId: string, data: UpdateAutomationRuleRequest) =>
    automationService.updateFlow(workspaceId, ruleId, data) as ReturnType<typeof automationService.updateFlow>,

  remove: (workspaceId: string, ruleId: string) =>
    automationService.deleteFlow(workspaceId, ruleId) as ReturnType<typeof automationService.deleteFlow>,
};
