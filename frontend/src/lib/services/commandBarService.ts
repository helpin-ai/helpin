import { api } from '../api';
import type {
  CommandBarDispatchRequest,
  CommandBarDispatchResponse,
  CommandBarCancelPlanResponse,
  CommandBarPlanDetailResponse,
  CommandBarPlanListResponse,
  CommandBarResumePlanResponse,
  CommandBarRetryPlanResponse,
  CommandBarToolCatalogResponse,
  PromoteCommandBarRunRequest,
  PromoteCommandBarRunResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const commandBarService = {
  listPlans: (workspaceId: string, limit = 20) =>
    api.get<CommandBarPlanListResponse>(`/command-bar/plans${qs(workspaceId)}&limit=${encodeURIComponent(limit)}`),
  getPlan: (workspaceId: string, planId: string) =>
    api.get<CommandBarPlanDetailResponse>(`/command-bar/plans/${encodeURIComponent(planId)}${qs(workspaceId)}`),
  listEpicPlans: (workspaceId: string, epicId: string) =>
    api.get<CommandBarPlanListResponse>(`/pm/epics/${encodeURIComponent(epicId)}/command-bar-plans${qs(workspaceId)}`),
  getAgentToolCatalog: (workspaceId: string, agentId: string, selectedTools?: string[]) => {
    const params = (selectedTools ?? []).map((tool) => `&selected=${encodeURIComponent(tool)}`).join('');
    return api.get<CommandBarToolCatalogResponse>(`/command-bar/agents/${encodeURIComponent(agentId)}/tools${qs(workspaceId)}${params}`);
  },
  dispatchPlan: (workspaceId: string, payload: CommandBarDispatchRequest) =>
    api.post<CommandBarDispatchResponse>(`/command-bar/plans/dispatch${qs(workspaceId)}`, payload),
  cancelPlan: (workspaceId: string, planId: string) =>
    api.post<CommandBarCancelPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/cancel${qs(workspaceId)}`),
  resumePlan: (workspaceId: string, planId: string) =>
    api.post<CommandBarResumePlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/resume${qs(workspaceId)}`),
  retryPlan: (workspaceId: string, planId: string, stepIndex: number) =>
    api.post<CommandBarRetryPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/retry${qs(workspaceId)}`, { step_index: stepIndex }),
  dismissPlans: (workspaceId: string, planIds: string[]) =>
    api.post<void>(`/command-bar/plans/dismiss${qs(workspaceId)}`, { plan_ids: planIds }),
  dismissPlan: (workspaceId: string, planId: string) =>
    api.post<void>(`/command-bar/plans/${encodeURIComponent(planId)}/dismiss${qs(workspaceId)}`),
  promoteRunToAgent: (workspaceId: string, runId: string, payload: PromoteCommandBarRunRequest) =>
    api.post<PromoteCommandBarRunResponse>(`/command-bar/runs/${encodeURIComponent(runId)}/promote-agent${qs(workspaceId)}`, payload),
};
