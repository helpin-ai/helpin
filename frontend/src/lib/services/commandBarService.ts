import { api } from '../api';
import type {
  CommandBarDispatchRequest,
  CommandBarDispatchResponse,
  CommandBarCancelPlanResponse,
  CommandBarPlanListResponse,
  CommandBarParseRequest,
  CommandBarParseResponse,
  CommandBarRetryPlanResponse,
  PromoteCommandBarRunRequest,
  PromoteCommandBarRunResponse,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const commandBarService = {
  parseIntent: (workspaceId: string, payload: CommandBarParseRequest) =>
    api.post<CommandBarParseResponse>(`/command-bar/intents/parse${qs(workspaceId)}`, payload),
  listPlans: (workspaceId: string, limit = 20) =>
    api.get<CommandBarPlanListResponse>(`/command-bar/plans${qs(workspaceId)}&limit=${encodeURIComponent(limit)}`),
  dispatchPlan: (workspaceId: string, payload: CommandBarDispatchRequest) =>
    api.post<CommandBarDispatchResponse>(`/command-bar/plans/dispatch${qs(workspaceId)}`, payload),
  cancelPlan: (workspaceId: string, planId: string) =>
    api.post<CommandBarCancelPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/cancel${qs(workspaceId)}`),
  retryPlan: (workspaceId: string, planId: string, stepIndex: number) =>
    api.post<CommandBarRetryPlanResponse>(`/command-bar/plans/${encodeURIComponent(planId)}/retry${qs(workspaceId)}`, { step_index: stepIndex }),
  promoteRunToAgent: (workspaceId: string, runId: string, payload: PromoteCommandBarRunRequest) =>
    api.post<PromoteCommandBarRunResponse>(`/command-bar/runs/${encodeURIComponent(runId)}/promote-agent${qs(workspaceId)}`, payload),
};
